package event

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

// These drive the shared negative fixture (VALIDATION-CONTRACT.md §4.2): every
// forged event MUST be rejected and every genuine control MUST be accepted,
// identically across the Go, JS, and Python SDKs. The fixture is consumed
// verbatim and never modified; its sha256 is pinned by the root package's
// TestFixtureChecksums.

type fixtureEvent struct {
	Kind    int        `json:"kind"`
	Content string     `json:"content"`
	Tags    [][]string `json:"tags"`
}

type validationCase struct {
	Name           string       `json:"name"`
	Assertion      string       `json:"assertion"`
	Kind           int          `json:"kind"`
	Expect         string       `json:"expect"`
	Check          string       `json:"check"`
	Reason         string       `json:"reason"`
	Event          fixtureEvent `json:"event"`
	PrimaryChainID string       `json:"primary_chain_id"`
}

func (fe fixtureEvent) toEvent() *nostr.Event {
	tags := make(nostr.Tags, len(fe.Tags))
	for i, tg := range fe.Tags {
		tags[i] = nostr.Tag(append([]string(nil), tg...))
	}
	return &nostr.Event{Kind: fe.Kind, Content: fe.Content, Tags: tags}
}

// validateFixtureEvent routes an event to its kind's Parse + Validate. An error
// from either means the event is rejected; nil means accepted.
func validateFixtureEvent(ev *nostr.Event, primaryChainID string) error {
	switch ev.Kind {
	case KindUnpaidInvoice, KindSettledInvoice:
		inv, err := ParseInvoice(ev)
		if err != nil {
			return err
		}
		return inv.Validate()
	case KindPendingShare, KindFinalizedShare, KindSharePayment:
		s, err := ParseShare(ev)
		if err != nil {
			return err
		}
		return s.Validate(ev.Kind)
	case KindMinerIdentity:
		m, err := ParseMinerIdentity(ev)
		if err != nil {
			return err
		}
		return m.Validate()
	case KindPoolIdentity:
		p, err := ParsePoolIdentity(ev)
		if err != nil {
			return err
		}
		return p.Validate()
	case KindHashrate:
		tel, err := ParseTelemetry(ev)
		if err != nil {
			return err
		}
		return tel.Validate()
	case KindRawSharenote:
		_, err := ValidateRawSharenoteEvent(ev, primaryChainID)
		return err
	default:
		return fmt.Errorf("event: unhandled fixture kind %d", ev.Kind)
	}
}

func loadValidationCases(t *testing.T) []validationCase {
	t.Helper()
	data, err := os.ReadFile("../testdata/validation_tests.json")
	if err != nil {
		t.Fatalf("read validation fixture: %v", err)
	}
	var file struct {
		Cases []validationCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("unmarshal validation fixture: %v", err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("no validation cases loaded")
	}
	return file.Cases
}

func TestValidationFixture(t *testing.T) {
	var accepts, rejects int
	for _, tc := range loadValidationCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Kind != tc.Event.Kind {
				t.Fatalf("%s: case kind %d != event kind %d", tc.Name, tc.Kind, tc.Event.Kind)
			}
			err := validateFixtureEvent(tc.Event.toEvent(), tc.PrimaryChainID)
			switch tc.Expect {
			case "accept":
				if err != nil {
					t.Fatalf("%s (%s): expected accept, got error: %v", tc.Name, tc.Assertion, err)
				}
			case "reject":
				if err == nil {
					t.Fatalf("%s (%s): expected reject (%s), but it was accepted", tc.Name, tc.Assertion, tc.Reason)
				}
			default:
				t.Fatalf("%s: unknown expect %q", tc.Name, tc.Expect)
			}
		})
		switch tc.Expect {
		case "accept":
			accepts++
		case "reject":
			rejects++
		}
	}
	// Guard against a fixture that is satisfiable by rejecting everything.
	if accepts == 0 || rejects == 0 {
		t.Fatalf("fixture must carry both accept controls and reject cases: %d accept, %d reject", accepts, rejects)
	}
	t.Logf("validation fixture: %d accept controls, %d reject cases", accepts, rejects)
}

// TestForgedCoordinateRejected is the negative coordinate test the retained
// HeightHash/ShareID fields now make writable: a parsed invoice or share whose
// "d"/"h" coordinate was swapped to another block's hash is rejected, where
// before the discarded fields let the forgery through (A7/A9/A10).
func TestForgedCoordinateRejected(t *testing.T) {
	const (
		good = 843000
		evil = 999999
	)

	// Invoice: forge the "d" HeightHash to another height's hash.
	inv := Invoice{Height: good, BlockHash: hash32, Amount: 1}
	ev, err := inv.UnpaidEvent()
	if err != nil {
		t.Fatal(err)
	}
	findTag(ev.Tags, "d")[1] = HeightHash(evil) // swap coordinate
	forged, err := ParseInvoice(ev)
	if err != nil {
		t.Fatal(err)
	}
	if forged.Validate() == nil {
		t.Fatal("expected forged invoice HeightHash to be rejected (A7)")
	}
	// Sanity: the untouched invoice validates.
	cleanEv, err := inv.UnpaidEvent()
	if err != nil {
		t.Fatal(err)
	}
	clean, err := ParseInvoice(cleanEv)
	if err != nil {
		t.Fatal(err)
	}
	if err := clean.Validate(); err != nil {
		t.Fatalf("clean invoice should validate: %v", err)
	}

	// Share: forge the "h" HeightHash.
	s := Share{Address: "fc1qminer", BlockHash: hash32, Workers: []string{"rig-01"}, Height: good, Amount: 1}
	pev, err := s.PendingEvent()
	if err != nil {
		t.Fatal(err)
	}
	findTag(pev.Tags, "h")[1] = HeightHash(evil)
	fh, err := ParseShare(pev)
	if err != nil {
		t.Fatal(err)
	}
	if fh.Validate(KindPendingShare) == nil {
		t.Fatal("expected forged share HeightHash to be rejected (A9)")
	}

	// Share: forge the "d" ShareID (swap the pending id into a finalized share).
	s2 := Share{Address: "fc1qminer", BlockHash: hash32, Workers: []string{"rig-01"}, Height: good, Amount: 1}
	fev, err := s2.FinalizedEvent()
	if err != nil {
		t.Fatal(err)
	}
	findTag(fev.Tags, "d")[1] = PendingShareID(good, "fc1qminer", "rig-01") // wrong for 35504
	fd, err := ParseShare(fev)
	if err != nil {
		t.Fatal(err)
	}
	if fd.Validate(KindFinalizedShare) == nil {
		t.Fatal("expected forged finalized ShareID to be rejected (A10)")
	}
}

// TestLargeAmountRoundTripExact proves D4 at the event layer: integer tags above
// 2^53 and up to max int64 round-trip through build/parse with no precision loss
// (Go uses int64, never float64).
func TestLargeAmountRoundTripExact(t *testing.T) {
	const (
		amount2p53p1 = int64(9007199254740993)    // 2^53 + 1
		maxInt64     = int64(9223372036854775807) // 2^63 - 1
		thresholdBig = int64(100000000000000001)  // 1e17 + 1
	)

	inv := Invoice{Height: 843000, BlockHash: hash32, Amount: amount2p53p1, Workers: 1}
	invEv, err := inv.UnpaidEvent()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseInvoice(invEv)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != amount2p53p1 {
		t.Fatalf("invoice amount = %d, want %d", got.Amount, amount2p53p1)
	}

	s := Share{Address: "fc1qminer", BlockHash: hash32, ChainID: "15", Workers: []string{"rig-01"},
		Height: 843000, Amount: maxInt64,
		Settlement: &Settlement{Txid: "9b1e", Confirmed: true, BlockHeight: 843006, BlockHash: hash32}}
	pev, err := s.PaymentEvent()
	if err != nil {
		t.Fatal(err)
	}
	gs, err := ParseShare(pev)
	if err != nil {
		t.Fatal(err)
	}
	if gs.Amount != maxInt64 {
		t.Fatalf("payment amount = %d, want %d", gs.Amount, maxInt64)
	}

	p := PoolIdentity{
		Addresses:  []PoolAddress{{ChainID: "15", Address: "fc1qpool"}},
		Payouts:    []PayoutScheme{{Scheme: "pps"}},
		Thresholds: []Threshold{{ChainID: "15", Amount: thresholdBig}},
	}
	pEv, err := p.Event()
	if err != nil {
		t.Fatal(err)
	}
	gp, err := ParsePoolIdentity(pEv)
	if err != nil {
		t.Fatal(err)
	}
	if gp.Thresholds[0].Amount != thresholdBig {
		t.Fatalf("threshold = %d, want %d", gp.Thresholds[0].Amount, thresholdBig)
	}
	// Formatting back must reproduce the exact decimal string.
	if strconv.FormatInt(gp.Thresholds[0].Amount, 10) != "100000000000000001" {
		t.Fatalf("threshold lost precision: %d", gp.Thresholds[0].Amount)
	}
}

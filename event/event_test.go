package event

import (
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

const hash32 = "000000000000000000023a1f1c9d00000000000000000000000000000000abe9"

func TestHashHelpers(t *testing.T) {
	// Stable, cross-SDK byte formats.
	if got := HeightHash(843000); len(got) != 64 || !isHex(got) {
		t.Fatalf("HeightHash not 32-byte hex: %q", got)
	}
	if PendingShareID(843000, "addr", "rig-01") == FinalizedShareID(843000, "addr") {
		t.Fatal("pending and finalized ShareIDs must differ")
	}
	// Determinism.
	if HeightHash(1) != HeightHash(1) {
		t.Fatal("HeightHash is not deterministic")
	}
}

func TestMinerIdentityRoundTrip(t *testing.T) {
	m := MinerIdentity{
		Addresses: []ChainAddress{
			{ChainID: "15", Address: "fc1qminer"},
			{ChainID: "01", Address: "bc1qminer"},
		},
		Payout: "pplns",
	}
	ev, err := m.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != KindMinerIdentity {
		t.Fatalf("kind = %d, want %d", ev.Kind, KindMinerIdentity)
	}
	got, err := ParseMinerIdentity(ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Addresses) != 2 || got.Addresses[0].Address != "fc1qminer" || got.Payout != "pplns" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Duplicate chain IDs are rejected (A5).
	dup := MinerIdentity{Addresses: []ChainAddress{{"15", "a"}, {"15", "b"}}}
	if err := dup.Validate(); err == nil {
		t.Fatal("expected duplicate chain ID error")
	}
	if _, err := (MinerIdentity{}).Event(); err == nil {
		t.Fatal("expected error for no addresses")
	}
}

func TestPoolIdentityRoundTrip(t *testing.T) {
	p := PoolIdentity{
		Profile:   PoolProfile{Name: "DeltaPool", Website: "https://deltapool.io"},
		Addresses: []PoolAddress{{ChainID: "15", Address: "fc1qpool", FeeBps: 150}},
		Payouts: []PayoutScheme{
			{Scheme: "pps", Params: []string{"fee:300"}},
			{Scheme: "pplns", Params: []string{"fee:150", "n:5000"}},
		},
		MinDenomination: "30z00",
		Thresholds:      []Threshold{{ChainID: "15", Amount: 100000}},
	}
	ev, err := p.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Content == "" {
		t.Fatal("expected JSON profile content")
	}
	got, err := ParsePoolIdentity(ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile.Name != "DeltaPool" || got.Addresses[0].FeeBps != 150 ||
		len(got.Payouts) != 2 || got.MinDenomination != "30z00" || got.Thresholds[0].Amount != 100000 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// pplns without n: is rejected (A6).
	bad := PoolIdentity{
		Addresses: []PoolAddress{{ChainID: "15", Address: "a"}},
		Payouts:   []PayoutScheme{{Scheme: "pplns"}},
	}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected pplns n: error")
	}
	// Non-JSON content is rejected on parse.
	if _, err := ParsePoolIdentity(&nostr.Event{Kind: KindPoolIdentity, Content: "{bad",
		Tags: nostr.Tags{{"a", "15", "x", "0"}, {"payout", "pps"}}}); err == nil {
		t.Fatal("expected JSON error")
	}
}

func TestInvoiceRoundTrip(t *testing.T) {
	inv := Invoice{
		Height:    843000,
		BlockHash: hash32,
		Amount:    312500000,
		Workers:   128,
		Shares:    "58Z42",
	}
	ev, err := inv.UnpaidEvent()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != KindUnpaidInvoice {
		t.Fatalf("kind = %d", ev.Kind)
	}
	// The "d" tag is the derived HeightHash.
	if d := findTag(ev.Tags, "d"); d == nil || d[1] != HeightHash(843000) {
		t.Fatalf("d tag not HeightHash: %v", d)
	}
	got, err := ParseInvoice(ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.Height != 843000 || got.Amount != 312500000 || got.Shares != "58Z42" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Settled requires a confirmed settlement.
	if _, err := inv.SettledEvent(); err == nil {
		t.Fatal("expected error: settled without settlement")
	}
	inv.Settlement = &Settlement{Txid: "9b1e", Confirmed: true, BlockHeight: 843006, BlockHash: hash32}
	sev, err := inv.SettledEvent()
	if err != nil {
		t.Fatal(err)
	}
	sgot, err := ParseInvoice(sev)
	if err != nil {
		t.Fatal(err)
	}
	if sgot.Settlement == nil || !sgot.Settlement.Confirmed || sgot.Settlement.BlockHeight != 843006 {
		t.Fatalf("settlement mismatch: %+v", sgot.Settlement)
	}
}

func TestShareRoundTripAndShareIDs(t *testing.T) {
	s := Share{
		Address:         "fc1qminer",
		BlockHash:       hash32,
		Workers:         []string{"rig-01", "rig-02"},
		Height:          843000,
		Amount:          2441406,
		Shares:          "34Z10",
		ShareCount:      128,
		TotalShares:     "58Z42",
		TotalShareCount: 16384,
		Timestamp:       1743638400123,
		SharenoteIDs:    []string{"snid1", "snid2"},
	}
	// Pending uses Hash(height, address, first_worker).
	pev, err := s.PendingEvent()
	if err != nil {
		t.Fatal(err)
	}
	if d := findTag(pev.Tags, "d"); d[1] != PendingShareID(843000, "fc1qminer", "rig-01") {
		t.Fatalf("pending ShareID mismatch: %v", d)
	}
	pgot, err := ParseShare(pev)
	if err != nil {
		t.Fatal(err)
	}
	if len(pgot.Workers) != 2 || pgot.ShareCount != 128 || len(pgot.SharenoteIDs) != 2 {
		t.Fatalf("pending round trip mismatch: %+v", pgot)
	}
	if err := pgot.Validate(KindPendingShare); err != nil {
		t.Fatalf("Validate pending: %v", err)
	}

	// Finalized uses Hash(height, address).
	fev, err := s.FinalizedEvent()
	if err != nil {
		t.Fatal(err)
	}
	if d := findTag(fev.Tags, "d"); d[1] != FinalizedShareID(843000, "fc1qminer") {
		t.Fatalf("finalized ShareID mismatch: %v", d)
	}

	// Payment requires an explicit chain (D5: no silent default) and a confirmed
	// settlement.
	s.Settlement = &Settlement{Txid: "9b1e", Confirmed: true, BlockHeight: 843006, BlockHash: hash32}
	if _, err := s.PaymentEvent(); err == nil {
		t.Fatal("expected error: payment without an explicit chain (A13/D5)")
	}
	s.ChainID = "15"
	if _, err := (Share{Address: "a", BlockHash: hash32, ChainID: "15", Workers: []string{"w"}, Height: 1}).PaymentEvent(); err == nil {
		t.Fatal("expected error: payment without settlement")
	}
	payev, err := s.PaymentEvent()
	if err != nil {
		t.Fatal(err)
	}
	if c := findTag(payev.Tags, "chain"); c == nil || c[1] != "15" {
		t.Fatalf("payment chain tag missing: %v", c)
	}
	// "x" must be present in full form (A13).
	paygot, err := ParseShare(payev)
	if err != nil {
		t.Fatal(err)
	}
	if err := paygot.Validate(KindSharePayment); err != nil {
		t.Fatalf("Validate payment: %v", err)
	}

	// A pending event parsed as a payment is missing its full-form x.
	if _, err := ParseShare(&nostr.Event{Kind: KindSharePayment, Tags: pev.Tags}); err == nil {
		t.Fatal("expected missing x error for payment")
	}
}

// TestParseShareChainlessRoundTripIdentity pins the cross-SDK parity fix: a
// chainless pending/finalized share (35503/35504) MUST parse with ChainID == ""
// — ParseShare never injects DefaultChainID ("15") — so re-building the parsed
// share yields byte-identical tags, matching the JS and Python SDKs. (Go was the
// outlier that re-emitted a ["chain","15"] tag absent from the wire, breaking
// build→parse→build round-trip parity.)
func TestParseShareChainlessRoundTripIdentity(t *testing.T) {
	// A chainless finalized (35504) share: ChainID is left unset, so
	// FinalizedEvent omits the "chain" tag entirely.
	s := Share{
		Address:      "fc1qminer",
		BlockHash:    hash32,
		Workers:      []string{"rig-01", "rig-02"},
		Height:       843000,
		Amount:       2441406,
		Shares:       "34Z10",
		ShareCount:   128,
		TotalShares:  "58Z42",
		Timestamp:    1743638400123,
		SharenoteIDs: []string{"snid1"},
	}
	ev, err := s.FinalizedEvent()
	if err != nil {
		t.Fatal(err)
	}
	// Wire absence: the built event carries no "chain" tag.
	if c := findTag(ev.Tags, "chain"); c != nil {
		t.Fatalf("chainless build should omit the chain tag, got %v", c)
	}

	got, err := ParseShare(ev)
	if err != nil {
		t.Fatal(err)
	}
	// The fix: parse preserves wire absence; no "15" is injected into the struct.
	if got.ChainID != "" {
		t.Fatalf("ParseShare injected a default ChainID %q; want \"\" (wire had no chain tag)", got.ChainID)
	}
	// The "default to 15" semantics are still available to consumers on demand,
	// without polluting the parsed struct.
	if eff := got.EffectiveChainID(KindFinalizedShare); eff != DefaultChainID {
		t.Fatalf("EffectiveChainID = %q, want %q", eff, DefaultChainID)
	}

	// Round-trip identity: re-building the parsed share reproduces the exact
	// wire tags, byte-for-byte (the parity property JS/Python hold).
	reev, err := got.FinalizedEvent()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ev.Tags, reev.Tags) {
		t.Fatalf("round-trip tags are not byte-identical:\n  built:   %v\n  rebuilt: %v", ev.Tags, reev.Tags)
	}
	if c := findTag(reev.Tags, "chain"); c != nil {
		t.Fatalf("rebuilt event re-injected a chain tag: %v", c)
	}
}

func TestTelemetryRoundTrip(t *testing.T) {
	tel := Telemetry{
		Address:       "fc1qfarm",
		TotalHashrate: "480000000000",
		MeanSharenote: "34Z10",
		Workers: []Worker{
			{Name: "rig-01", Hashrate: "240000000000", MinDenomination: "30Z00", MeanSharenote: "34Z12",
				CountAccepted: 1842, CountRejected: 3, MeanTimeSec: "7", LastAcceptedUnix: 1743638400, UserAgent: "bmminer/2.0.0"},
			{Name: "rig-02", Hashrate: "240000000000", CountAccepted: 1790},
		},
	}
	ev, err := tel.Event()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseTelemetry(ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != "fc1qfarm" || got.TotalHashrate != "480000000000" || got.MeanSharenote != "34Z10" {
		t.Fatalf("telemetry round trip mismatch: %+v", got)
	}
	if len(got.Workers) != 2 || got.Workers[0].CountRejected != 3 || got.Workers[0].UserAgent != "bmminer/2.0.0" {
		t.Fatalf("worker round trip mismatch: %+v", got.Workers)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// crsn omitted when zero.
	for _, field := range findTag(ev.Tags, "w:rig-02") {
		if field == "crsn:0" {
			t.Fatal("crsn:0 should be omitted")
		}
	}
	// Both all and h, or neither, is invalid.
	if (Telemetry{Address: "a"}).Validate() == nil {
		t.Fatal("expected error: no hashrate")
	}
	if (Telemetry{Address: "a", TotalHashrate: "1", SingleHashrate: "2"}).Validate() == nil {
		t.Fatal("expected error: both all and h")
	}
}

// A real mined header/hash pair (the validation_tests.json A14 anchor): the
// byte-reversed double-SHA256 of rawHeaderHex is rawHeaderHash, which has 30
// leading zero bits and so clears a 26z00 bound.
const (
	rawHeaderHex  = "00000020605f4e3d2c1b0af9e8d7c6b5a4938271605f4e3d2c1b0a00000000000000000091807f6e5d4c3b2a1908f7e6d5c4b3a291807f6e5d4c3b2a1f0e9d8c7b6a2f4d00000066db3103179e2dc501"
	rawHeaderHash = "0000000288f122c9859094ea06dc2f713ad691610eadd6cb6dd386480f398bbe"
	auxBlockHash  = "000000000000000000023a1f1c9d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d"
)

func TestRawSharenoteRoundTrip(t *testing.T) {
	r := RawSharenote{
		HeaderHash:   rawHeaderHash,
		Address:      "fc1qxyz",
		Worker:       "antminer-s19",
		Agent:        "bmminer/2.0.0",
		Denomination: "26Z00", // mixed case on input; lowercased on build
		Branches: []Branch{
			{BlockHash: rawHeaderHash, ChainID: "15", Height: 843000, Solved: true, Label: "26z00"},
			{BlockHash: auxBlockHash, ChainID: "2a", Height: 110000, Solved: false, Label: "22z00"},
		},
		HeaderHex: rawHeaderHex,
	}
	ev, err := r.Event()
	if err != nil {
		t.Fatal(err)
	}
	// z is lowercased; dd is the last tag.
	if z := findTag(ev.Tags, "z"); z[1] != "26z00" {
		t.Fatalf("z not lowercased: %v", z)
	}
	if last := ev.Tags[len(ev.Tags)-1]; last[0] != "dd" {
		t.Fatalf("dd is not the last tag: %v", last)
	}
	got, err := ParseRawSharenote(ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Branches) != 2 || !got.Branches[0].Solved || got.Branches[1].ChainID != "2a" {
		t.Fatalf("branch round trip mismatch: %+v", got.Branches)
	}
	if got.Worker != "antminer-s19" || got.HeaderHex != rawHeaderHex {
		t.Fatalf("attribution/header mismatch: %+v", got)
	}
	// Full A14 validation (clause 1 header hash + clause 2 z-bound + primary).
	if err := got.Validate("15"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// The single-call entry point (parity with Python validate_raw_event and
	// TypeScript validateRawSharenoteEvent) accepts the same valid event and
	// returns the parsed note.
	if v, err := ValidateRawSharenoteEvent(ev, "15"); err != nil {
		t.Fatalf("ValidateRawSharenoteEvent: %v", err)
	} else if v.HeaderHex != rawHeaderHex || len(v.Branches) != 2 {
		t.Fatalf("ValidateRawSharenoteEvent returned wrong note: %+v", v)
	}
	// It also surfaces a semantic violation (wrong primary chain) that the
	// structural parse alone would not catch.
	if _, err := ValidateRawSharenoteEvent(ev, "ff"); err == nil {
		t.Fatal("expected error: declared primary chain absent")
	}
	if _, err := (RawSharenote{HeaderHash: "x"}).Event(); err == nil {
		t.Fatal("expected error: no branches")
	}
	// A w tag with other than six elements is rejected at parse.
	badEv := &nostr.Event{Kind: KindRawSharenote, Tags: nostr.Tags{
		{"d", "x"}, {"a", "addr"}, {"z", "34z10"}, {"w", "hash", "15"}, {"dd", "00"},
	}}
	if _, err := ParseRawSharenote(badEv); err == nil {
		t.Fatal("expected error: short w tag")
	}
}

func TestOperatorMessageRoundTrip(t *testing.T) {
	m := OperatorMessage{
		Content:      "<ciphertext>",
		Counterparty: "minerpub",
		ThreadID:     "thread-1",
		ChainID:      "15",
		Address:      "fc1qminer",
		Subject:      "Payout threshold change",
	}
	ev, err := m.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Content != "<ciphertext>" {
		t.Fatalf("content not passed through: %q", ev.Content)
	}
	got, err := ParseOperatorMessage(ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counterparty != "minerpub" || got.ThreadID != "thread-1" || got.Subject != "Payout threshold change" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if got.Address != "fc1qminer" {
		t.Fatalf("context address mismatch: %q", got.Address)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := (OperatorMessage{Counterparty: "p", ThreadID: "d"}).Event(); err == nil {
		t.Fatal("expected error: empty content (must be encrypted)")
	}
}

func TestParseWrongKindAndNil(t *testing.T) {
	wrong := &nostr.Event{Kind: 1}
	if _, err := ParseInvoice(wrong); err == nil {
		t.Fatal("expected wrong-kind error")
	}
	if _, err := ParseShare(nil); err != ErrNilEvent {
		t.Fatalf("expected ErrNilEvent, got %v", err)
	}
	if _, err := ParseMinerIdentity(nil); err != ErrNilEvent {
		t.Fatalf("expected ErrNilEvent, got %v", err)
	}
}

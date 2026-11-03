// Package event builds and parses the Nostr events of the NIP-SN Sharenote
// protocol: identity, hashrate telemetry, pool accounting, raw merged-mining
// proofs, and operator messaging.
//
// Each domain type (for example [Invoice], [Share], [MinerIdentity]) exposes a
// builder method that returns a [github.com/nbd-wtf/go-nostr.Event] with its
// Kind, Content, and Tags set — the caller fills in CreatedAt and signs it — and
// a package-level Parse function (for example [ParseInvoice]) that checks the
// event kind and required tags and returns the typed value. The deeper
// normative constraints of NIP-SN §Validation Constraints are available through
// each type's Validate method.
//
// The denomination maths live in the parent package
// github.com/soprinter/go-sharenote.
package event

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"golang.org/x/crypto/blake2b"
)

// The NIP-SN event kinds.
const (
	KindMinerIdentity   = 10520 // replaceable; miner's chain addresses
	KindPoolIdentity    = 10521 // replaceable; pool profile, chains, fees, schemes
	KindUnpaidInvoice   = 35500 // addressable; a secured block's reward, miners unpaid
	KindSettledInvoice  = 35501 // addressable; the invoice's payout confirmed
	KindHashrate        = 35502 // addressable; live per-farm and per-worker telemetry
	KindPendingShare    = 35503 // addressable; unconfirmed per-worker payout slice
	KindFinalizedShare  = 35504 // addressable; consolidated unpaid slice per address
	KindSharePayment    = 35505 // addressable; confirmed payout with on-chain tx
	KindRawSharenote    = 35510 // addressable; raw block header and AuxPoW branches
	KindOperatorMessage = 35515 // addressable; encrypted operator<->miner message
)

// DefaultChainID is the chain ID to assume wherever an OPTIONAL chain-scoped tag
// omits one (NIP-SN rule A16): hex "15", i.e. decimal chain 21.
//
// It is a consumer-facing default, NOT a value any Parse function injects:
// [ParseShare] preserves wire absence (ChainID == "" when the event has no
// "chain" tag) so a build→parse→build round-trip is byte-identical across the
// Go, JS, and Python SDKs. Apply it explicitly via [Share.EffectiveChainID]
// when you need the defaulted value. It never applies to a payment (35505),
// whose "chain" tag is REQUIRED (decision D5).
const DefaultChainID = "15"

// ErrNilEvent is returned by the Parse functions when passed a nil event.
var ErrNilEvent = errors.New("event: nil event")

// HeightHash returns the blake2b-256 hash, hex-encoded, of a block height's
// decimal representation. It is the addressable "d" coordinate of an invoice
// (kinds 35500/35501) and the "h" block anchor of a share (NIP-SN rules A7, A9).
func HeightHash(height int64) string {
	return hashHex([]byte(strconv.FormatInt(height, 10)))
}

// PendingShareID returns the ShareID of a pending share (kind 35503): the
// blake2b-256 hash, hex-encoded, of "{height}/{address}/{worker}" (rule A10).
func PendingShareID(height int64, address, worker string) string {
	return hashHex([]byte(fmt.Sprintf("%d/%s/%s", height, address, worker)))
}

// FinalizedShareID returns the ShareID of a finalized or paid share (kinds
// 35504/35505): the blake2b-256 hash, hex-encoded, of "{height}/{address}"
// (rule A10).
func FinalizedShareID(height int64, address string) string {
	return hashHex([]byte(fmt.Sprintf("%d/%s", height, address)))
}

func hashHex(data []byte) string {
	sum := blake2b.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Settlement is an on-chain settlement transaction, carried by an "x" tag. In
// short form (Confirmed false) it names only the Txid; in full form (Confirmed
// true) it also carries the confirming block's Height and Hash.
type Settlement struct {
	Txid        string
	Confirmed   bool
	BlockHeight int64
	BlockHash   string
}

func (s *Settlement) tag() nostr.Tag {
	if s.Confirmed {
		return nostr.Tag{"x", s.Txid, strconv.FormatInt(s.BlockHeight, 10), s.BlockHash}
	}
	return nostr.Tag{"x", s.Txid}
}

func parseSettlement(tag nostr.Tag) (*Settlement, error) {
	if len(tag) < 2 || tag[1] == "" {
		return nil, errors.New("event: settlement \"x\" tag missing txid")
	}
	s := &Settlement{Txid: tag[1]}
	if len(tag) >= 4 {
		h, err := strconv.ParseInt(tag[2], 10, 64)
		if err == nil && tag[3] != "" {
			s.BlockHeight = h
			s.BlockHash = tag[3]
			s.Confirmed = true
		}
	}
	return s, nil
}

// --- tag helpers ---

func findTag(tags nostr.Tags, key string) nostr.Tag {
	for _, t := range tags {
		if len(t) > 0 && t[0] == key {
			return t
		}
	}
	return nil
}

func findAllTags(tags nostr.Tags, key string) nostr.Tags {
	var out nostr.Tags
	for _, t := range tags {
		if len(t) > 0 && t[0] == key {
			out = append(out, t)
		}
	}
	return out
}

func requireString(tags nostr.Tags, key string) (string, error) {
	t := findTag(tags, key)
	if t == nil || len(t) < 2 {
		return "", fmt.Errorf("event: required tag %q missing", key)
	}
	return t[1], nil
}

func optionalString(tags nostr.Tags, key string) string {
	t := findTag(tags, key)
	if t == nil || len(t) < 2 {
		return ""
	}
	return t[1]
}

func requireInt64(tags nostr.Tags, key string) (int64, error) {
	s, err := requireString(tags, key)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("event: tag %q is not an integer: %w", key, err)
	}
	return v, nil
}

func optionalInt64(tags nostr.Tags, key string) (int64, error) {
	t := findTag(tags, key)
	if t == nil || len(t) < 2 {
		return 0, nil
	}
	v, err := strconv.ParseInt(t[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("event: tag %q is not an integer: %w", key, err)
	}
	return v, nil
}

// isHex reports whether s is a non-empty, even-length lowercase-or-mixed hex
// string.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// is32ByteHex reports whether s is exactly 64 hex characters (a 32-byte key).
func is32ByteHex(s string) bool {
	return len(s) == 64 && isHex(s)
}

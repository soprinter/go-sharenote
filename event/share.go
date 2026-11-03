package event

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

// Share is a miner's payout slice of a block (kinds 35503, 35504, and 35505): a
// pending per-worker slice, a finalized per-address slice, or a confirmed
// payment. Its addressable ShareID and block-anchor HeightHash are derived from
// the other fields — callers set Height, Address, and Workers, and the builders
// compute the hashes.
type Share struct {
	// Address is the miner's chain receiving address ("a" tag).
	Address string
	// BlockHash is the 32-byte block hash, hex-encoded ("b" tag).
	BlockHash string
	// ChainID is the hex chain ID exactly as carried on the wire: [ParseShare]
	// leaves it "" when the event has no "chain" tag and never injects a
	// default, so a build→parse→build round-trip stays byte-identical across the
	// Go, JS, and Python SDKs. On a pending/finalized share (35503/35504) the tag
	// is OPTIONAL and an absent one is treated as [DefaultChainID] by consumers
	// on demand (see [Share.EffectiveChainID]); on a payment (35505) it is
	// REQUIRED (NIP-SN rule A13 / decision D5).
	ChainID string
	// Workers are the worker names merged into this slice; at least one is
	// required. A pending share's ShareID uses Workers[0].
	Workers []string
	// Height is the block height.
	Height int64
	// Amount is the miner's payout amount, in the coin's base unit (its
	// smallest unit; satoshis on Bitcoin, for example).
	Amount int64
	// Shares is the miner's difficulty label, e.g. "34Z10".
	Shares string
	// ShareCount is the miner's sharenote count (second value of the "shares"
	// tag); omitted when zero.
	ShareCount uint64
	// TotalShares is the pool's aggregate difficulty label.
	TotalShares string
	// TotalShareCount is the pool's total sharenote count; omitted when zero.
	TotalShareCount uint64
	// Timestamp is the creation time, in milliseconds since the Unix epoch.
	Timestamp int64
	// Fee is an optional transaction fee deducted from the payout, in the coin's
	// base unit (its smallest unit; satoshis on Bitcoin, for example).
	Fee int64
	// EstPaymentHeight is an optional estimated payment block height ("eph").
	EstPaymentHeight int64
	// SharenoteIDs reference the Raw Sharenote events (kind 35510) this slice
	// derives from ("sn" tags).
	SharenoteIDs []string
	// Settlement is the on-chain payout transaction ("x" tag); required in full
	// form on a payment (35505).
	Settlement *Settlement

	// HeightHash is the block anchor ("h" tag) as carried on the wire. It is set
	// by [ParseShare] and left empty on a hand-built Share (the builders derive
	// it). [Share.Validate] recomputes HeightHash(Height) and rejects a parsed
	// share whose "h" was forged (NIP-SN rule A9).
	HeightHash string
	// ShareID is the addressable "d" coordinate as carried on the wire. It is
	// set by [ParseShare] and left empty on a hand-built Share. [Share.Validate]
	// recomputes the kind's expected ShareID and rejects a forged one (A10).
	ShareID string
}

// PendingEvent builds the kind 35503 (pending share) event. The "d" ShareID is
// PendingShareID(Height, Address, Workers[0]). The "chain" tag is optional and
// emitted only when ChainID is set.
func (s Share) PendingEvent() (*nostr.Event, error) {
	if err := s.checkCommon(); err != nil {
		return nil, err
	}
	id := PendingShareID(s.Height, s.Address, s.Workers[0])
	return &nostr.Event{Kind: KindPendingShare, Tags: s.tags(id)}, nil
}

// FinalizedEvent builds the kind 35504 (finalized share) event. The "d" ShareID
// is FinalizedShareID(Height, Address), consolidating all of the address's
// workers. The "chain" tag is optional and emitted only when ChainID is set.
func (s Share) FinalizedEvent() (*nostr.Event, error) {
	if err := s.checkCommon(); err != nil {
		return nil, err
	}
	id := FinalizedShareID(s.Height, s.Address)
	return &nostr.Event{Kind: KindFinalizedShare, Tags: s.tags(id)}, nil
}

// PaymentEvent builds the kind 35505 (share payment) event. The ShareID matches
// a finalized share. NIP-SN rule A13 (decision D5) requires an explicit "chain"
// tag on a payment, so ChainID MUST be set — it is never silently defaulted to
// [DefaultChainID]. A confirmed full-form [Settlement] is required.
func (s Share) PaymentEvent() (*nostr.Event, error) {
	if err := s.checkCommon(); err != nil {
		return nil, err
	}
	if s.ChainID == "" {
		return nil, errors.New("event: share payment requires an explicit chain tag (A13/D5)")
	}
	if s.Settlement == nil || !s.Settlement.Confirmed {
		return nil, errors.New("event: share payment needs a confirmed settlement")
	}
	id := FinalizedShareID(s.Height, s.Address)
	return &nostr.Event{Kind: KindSharePayment, Tags: s.tags(id)}, nil
}

func (s Share) checkCommon() error {
	if s.Height <= 0 {
		return errors.New("event: share height must be > 0")
	}
	if s.Address == "" {
		return errors.New("event: share address is required")
	}
	if s.BlockHash == "" {
		return errors.New("event: share block hash is required")
	}
	if len(s.Workers) == 0 {
		return errors.New("event: share needs at least one worker")
	}
	return nil
}

// tags marshals the share's tags. The "chain" tag is emitted only when ChainID
// is set; no default is ever injected (decision D5), so a payment must carry an
// explicit ChainID, which [Share.PaymentEvent] enforces.
func (s Share) tags(shareID string) nostr.Tags {
	tags := nostr.Tags{
		{"d", shareID},
		{"a", s.Address},
		{"h", HeightHash(s.Height)},
		{"b", s.BlockHash},
	}
	if s.ChainID != "" {
		tags = append(tags, nostr.Tag{"chain", s.ChainID})
	}
	tags = append(tags, append(nostr.Tag{"workers"}, s.Workers...))
	tags = append(tags,
		nostr.Tag{"height", strconv.FormatInt(s.Height, 10)},
		nostr.Tag{"amount", strconv.FormatInt(s.Amount, 10)},
	)
	if s.Shares != "" {
		tag := nostr.Tag{"shares", s.Shares}
		if s.ShareCount > 0 {
			tag = append(tag, strconv.FormatUint(s.ShareCount, 10))
		}
		tags = append(tags, tag)
	}
	if s.TotalShares != "" {
		tag := nostr.Tag{"totalshares", s.TotalShares}
		if s.TotalShareCount > 0 {
			tag = append(tag, strconv.FormatUint(s.TotalShareCount, 10))
		}
		tags = append(tags, tag)
	}
	tags = append(tags, nostr.Tag{"timestamp", strconv.FormatInt(s.Timestamp, 10)})
	if s.Fee != 0 {
		tags = append(tags, nostr.Tag{"fee", strconv.FormatInt(s.Fee, 10)})
	}
	if s.EstPaymentHeight != 0 {
		tags = append(tags, nostr.Tag{"eph", strconv.FormatInt(s.EstPaymentHeight, 10)})
	}
	for _, id := range s.SharenoteIDs {
		if strings.TrimSpace(id) != "" {
			tags = append(tags, nostr.Tag{"sn", id})
		}
	}
	if s.Settlement != nil {
		tags = append(tags, s.Settlement.tag())
	}
	return tags
}

// ParseShare extracts a [Share] from a kind 35503, 35504, or 35505 event,
// checking the kind and the required "d", "a", "h", "b", "workers", "height",
// and "amount" tags.
func ParseShare(ev *nostr.Event) (Share, error) {
	if ev == nil {
		return Share{}, ErrNilEvent
	}
	if ev.Kind != KindPendingShare && ev.Kind != KindFinalizedShare && ev.Kind != KindSharePayment {
		return Share{}, fmt.Errorf("event: expected kind %d, %d, or %d, got %d",
			KindPendingShare, KindFinalizedShare, KindSharePayment, ev.Kind)
	}
	var (
		s   Share
		err error
	)
	if s.ShareID, err = requireString(ev.Tags, "d"); err != nil {
		return Share{}, err
	}
	if s.Address, err = requireString(ev.Tags, "a"); err != nil {
		return Share{}, err
	}
	if s.HeightHash, err = requireString(ev.Tags, "h"); err != nil {
		return Share{}, err
	}
	if s.BlockHash, err = requireString(ev.Tags, "b"); err != nil {
		return Share{}, err
	}
	// ChainID is taken verbatim from the wire: ParseShare NEVER injects a
	// default. An absent "chain" tag parses to ChainID == "" on every kind, so
	// parse(build(x)) stays byte-identical across the Go, JS, and Python SDKs
	// (Go used to re-emit a ["chain","15"] tag the others omit — the parity bug
	// this preserves against). The "chain" tag is OPTIONAL on a pending/finalized
	// share (35503/35504) and defaults to DefaultChainID for consumers only,
	// applied on demand via [Share.EffectiveChainID] — never materialised into
	// the parsed struct (NIP-SN A16 / decision D5). On a payment (35505) it is
	// REQUIRED, so an absent chain is left empty for [Share.Validate] to reject.
	s.ChainID = optionalString(ev.Tags, "chain")
	if w := findTag(ev.Tags, "workers"); w != nil && len(w) > 1 {
		s.Workers = append([]string(nil), w[1:]...)
	}
	if len(s.Workers) == 0 {
		return Share{}, errors.New("event: share has no worker names")
	}
	if s.Height, err = requireInt64(ev.Tags, "height"); err != nil {
		return Share{}, err
	}
	if s.Amount, err = requireInt64(ev.Tags, "amount"); err != nil {
		return Share{}, err
	}
	if sh := findTag(ev.Tags, "shares"); len(sh) >= 2 {
		s.Shares = sh[1]
		if len(sh) >= 3 {
			s.ShareCount, _ = strconv.ParseUint(sh[2], 10, 64)
		}
	}
	if ts := findTag(ev.Tags, "totalshares"); len(ts) >= 2 {
		s.TotalShares = ts[1]
		if len(ts) >= 3 {
			s.TotalShareCount, _ = strconv.ParseUint(ts[2], 10, 64)
		}
	}
	if s.Timestamp, err = optionalInt64(ev.Tags, "timestamp"); err != nil {
		return Share{}, err
	}
	if s.Fee, err = optionalInt64(ev.Tags, "fee"); err != nil {
		return Share{}, err
	}
	if s.EstPaymentHeight, err = optionalInt64(ev.Tags, "eph"); err != nil {
		return Share{}, err
	}
	for _, sn := range findAllTags(ev.Tags, "sn") {
		if len(sn) >= 2 {
			s.SharenoteIDs = append(s.SharenoteIDs, sn[1:]...)
		}
	}
	if x := findTag(ev.Tags, "x"); x != nil {
		if s.Settlement, err = parseSettlement(x); err != nil {
			return Share{}, err
		}
	}
	if ev.Kind == KindSharePayment && (s.Settlement == nil || !s.Settlement.Confirmed) {
		return Share{}, errors.New("event: share payment is missing a full-form \"x\" tag")
	}
	return s, nil
}

// Validate applies NIP-SN rules A9, A10, A13, and A16 for the given kind: Height
// must be positive, a worker must be present, the block hash must be 32-byte
// hex, the "h" HeightHash must equal HeightHash(Height), the "d" ShareID must
// equal the kind's expected ShareID, a present ChainID must be valid hex, and a
// payment must carry an explicit chain and a full-form settlement.
//
// HeightHash and ShareID are compared only when set (i.e. for a parsed Share);
// a hand-built Share leaves them empty and relies on the builders deriving them.
// This closes the forged-coordinate gap: a relayed event whose "h" or "d" was
// altered is now rejected.
func (s Share) Validate(kind int) error {
	if s.Height <= 0 {
		return errors.New("event: share height must be > 0")
	}
	if len(s.Workers) == 0 {
		return errors.New("event: share needs at least one worker")
	}
	if !is32ByteHex(s.BlockHash) {
		return errors.New("event: share block hash must be 32-byte hex")
	}
	if s.ChainID != "" && !isHex(s.ChainID) {
		return fmt.Errorf("event: chain ID %q is not valid hex (A16)", s.ChainID)
	}
	// A9: the "h" block anchor must be the blake2b-256 of the height.
	if want := HeightHash(s.Height); s.HeightHash != "" && s.HeightHash != want {
		return fmt.Errorf("event: share HeightHash %q does not match blake2b-256(height) %q (A9)", s.HeightHash, want)
	}
	// A10: the "d" ShareID must be the kind's expected hash.
	switch kind {
	case KindPendingShare:
		if want := PendingShareID(s.Height, s.Address, s.Workers[0]); s.ShareID != "" && s.ShareID != want {
			return fmt.Errorf("event: pending ShareID %q does not match Hash(height,address,worker) %q (A10)", s.ShareID, want)
		}
	case KindFinalizedShare:
		if want := FinalizedShareID(s.Height, s.Address); s.ShareID != "" && s.ShareID != want {
			return fmt.Errorf("event: finalized ShareID %q does not match Hash(height,address) %q (A10)", s.ShareID, want)
		}
	case KindSharePayment:
		if want := FinalizedShareID(s.Height, s.Address); s.ShareID != "" && s.ShareID != want {
			return fmt.Errorf("event: payment ShareID %q does not match Hash(height,address) %q (A10)", s.ShareID, want)
		}
		if s.ChainID == "" {
			return errors.New("event: share payment requires a chain tag (A13)")
		}
		if s.Settlement == nil || !s.Settlement.Confirmed {
			return errors.New("event: share payment requires a full-form settlement (A13)")
		}
	default:
		return fmt.Errorf("event: unknown share kind %d", kind)
	}
	return nil
}

// EffectiveChainID returns the chain ID to assume for a share of the given kind,
// applying the NIP-SN A16 default without mutating the parsed [Share]. On a
// pending or finalized share (35503/35504) the "chain" tag is OPTIONAL, so an
// absent one (ChainID == "") defaults to [DefaultChainID]. On a payment (35505)
// the tag is REQUIRED (decision D5), so no default is applied and an absent
// chain is returned as "" (which [Share.Validate] rejects).
//
// This is the documented accessor for the "default to 15" semantics:
// [ParseShare] deliberately preserves wire absence in [Share.ChainID] (so
// round-trips stay byte-identical across the Go, JS, and Python SDKs), and
// consumers that want the defaulted value call this instead of reading ChainID
// directly.
func (s Share) EffectiveChainID(kind int) string {
	if s.ChainID == "" && (kind == KindPendingShare || kind == KindFinalizedShare) {
		return DefaultChainID
	}
	return s.ChainID
}

package event

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
)

// Invoice is a pool's declaration of a secured block's reward (kinds 35500 and
// 35501). An unpaid invoice (35500) records the total reward and signals that
// miners are not yet paid; a settled invoice (35501) carries the confirmed
// payout transaction. Both are addressable by the block's HeightHash, which is
// derived from Height — callers set Height, not the hash.
type Invoice struct {
	// Height is the block height; it must be greater than zero.
	Height int64
	// BlockHash is the 32-byte block hash, hex-encoded ("b" tag).
	BlockHash string
	// Amount is the total block reward available, in the coin's base unit (its
	// smallest unit; satoshis on Bitcoin, for example).
	Amount int64
	// Workers is the number of participating workers.
	Workers int64
	// Shares is the pool's combined difficulty label, e.g. "58Z42".
	Shares string
	// Settlement is the on-chain payout transaction. It is optional on an
	// unpaid invoice and required, in full form, on a settled one.
	Settlement *Settlement

	// HeightHash is the addressable "d" coordinate as carried on the wire. It is
	// set by [ParseInvoice] and left empty on a hand-built Invoice (the builders
	// derive it). [Invoice.Validate] recomputes HeightHash(Height) and rejects a
	// parsed invoice whose "d" was forged (NIP-SN rule A7).
	HeightHash string
}

// UnpaidEvent builds the kind 35500 (unpaid invoice) event. Content is empty and
// the "d" tag is set to HeightHash(Height).
func (inv Invoice) UnpaidEvent() (*nostr.Event, error) {
	tags, err := inv.tags()
	if err != nil {
		return nil, err
	}
	return &nostr.Event{Kind: KindUnpaidInvoice, Tags: tags}, nil
}

// SettledEvent builds the kind 35501 (settled invoice) event. It requires a
// confirmed [Settlement] in full form.
func (inv Invoice) SettledEvent() (*nostr.Event, error) {
	if inv.Settlement == nil || !inv.Settlement.Confirmed {
		return nil, errors.New("event: settled invoice needs a confirmed settlement")
	}
	tags, err := inv.tags()
	if err != nil {
		return nil, err
	}
	return &nostr.Event{Kind: KindSettledInvoice, Tags: tags}, nil
}

func (inv Invoice) tags() (nostr.Tags, error) {
	if inv.Height <= 0 {
		return nil, errors.New("event: invoice height must be > 0")
	}
	if inv.BlockHash == "" {
		return nil, errors.New("event: invoice block hash is required")
	}
	tags := nostr.Tags{
		{"d", HeightHash(inv.Height)},
		{"b", inv.BlockHash},
		{"height", strconv.FormatInt(inv.Height, 10)},
		{"amount", strconv.FormatInt(inv.Amount, 10)},
		{"workers", strconv.FormatInt(inv.Workers, 10)},
	}
	if inv.Shares != "" {
		tags = append(tags, nostr.Tag{"shares", inv.Shares})
	}
	if inv.Settlement != nil {
		tags = append(tags, inv.Settlement.tag())
	}
	return tags, nil
}

// ParseInvoice extracts an [Invoice] from a kind 35500 or 35501 event, checking
// the kind and the required "d", "b", "height", "amount", and "workers" tags.
func ParseInvoice(ev *nostr.Event) (Invoice, error) {
	if ev == nil {
		return Invoice{}, ErrNilEvent
	}
	if ev.Kind != KindUnpaidInvoice && ev.Kind != KindSettledInvoice {
		return Invoice{}, fmt.Errorf("event: expected kind %d or %d, got %d", KindUnpaidInvoice, KindSettledInvoice, ev.Kind)
	}
	var (
		inv Invoice
		err error
	)
	if inv.HeightHash, err = requireString(ev.Tags, "d"); err != nil {
		return Invoice{}, err
	}
	if inv.BlockHash, err = requireString(ev.Tags, "b"); err != nil {
		return Invoice{}, err
	}
	if inv.Height, err = requireInt64(ev.Tags, "height"); err != nil {
		return Invoice{}, err
	}
	if inv.Amount, err = requireInt64(ev.Tags, "amount"); err != nil {
		return Invoice{}, err
	}
	if inv.Workers, err = requireInt64(ev.Tags, "workers"); err != nil {
		return Invoice{}, err
	}
	inv.Shares = optionalString(ev.Tags, "shares")
	if x := findTag(ev.Tags, "x"); x != nil {
		if inv.Settlement, err = parseSettlement(x); err != nil {
			return Invoice{}, err
		}
	}
	if ev.Kind == KindSettledInvoice && (inv.Settlement == nil || !inv.Settlement.Confirmed) {
		return Invoice{}, errors.New("event: settled invoice is missing a full-form \"x\" tag")
	}
	return inv, nil
}

// Validate applies NIP-SN rule A7: the "d" HeightHash must equal
// HeightHash(Height), the block hash must be 32-byte hex, Height must be
// positive, and the amount must be positive.
//
// HeightHash is compared only when set (i.e. for a parsed Invoice); a hand-built
// Invoice leaves it empty and relies on the builders deriving it. This closes
// the forged-coordinate gap: a relayed invoice whose "d" was altered is now
// rejected.
func (inv Invoice) Validate() error {
	if inv.Height <= 0 {
		return errors.New("event: invoice height must be > 0")
	}
	if !is32ByteHex(inv.BlockHash) {
		return errors.New("event: invoice block hash must be 32-byte hex")
	}
	if inv.Amount <= 0 {
		return errors.New("event: invoice amount must be > 0")
	}
	if want := HeightHash(inv.Height); inv.HeightHash != "" && inv.HeightHash != want {
		return fmt.Errorf("event: invoice HeightHash %q does not match blake2b-256(height) %q (A7)", inv.HeightHash, want)
	}
	return nil
}

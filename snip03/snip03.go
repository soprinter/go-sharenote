package snip03

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ohstr/nmilat/pkg/nip01"
	"github.com/soprinter/go-sharenote/internal/hash"
	"github.com/soprinter/go-sharenote/internal/tags"
)

const (
	KindInvoice        = 35500
	KindSettledInvoice = 35501
	KindPendingShare   = 35503
	KindFinalizedShare = 35504
	KindSharePayment   = 35505
)

// Transaction holds on-chain settlement data for the "x" tag.
type Transaction struct {
	Txid        string
	Confirmed   bool
	BlockHeight int64
	BlockHash   string
}

// Invoice represents a pool block invoice (kinds 35500/35501).
type Invoice struct {
	HeightHash string // "d": blake2b-256 of height
	BlockHash  string // "b"
	Height     int64  // "height"
	Amount     int64  // "amount": satoshis
	Workers    int64  // "workers"
	Shares     string // "shares": pool combined difficulty label
	Tx         *Transaction
}

// Share represents a miner payout slice (kinds 35503/35504/35505).
type Share struct {
	ShareID           string   // "d": computed hash
	Address           string   // "a"
	HeightHash        string   // "h"
	BlockHash         string   // "b"
	ChainID           string   // "chain": hex
	Workers           []string // "workers"
	Height            int64    // "height"
	Amount            int64    // "amount": satoshis
	Shares            string   // "shares": difficulty label
	ShareCount        uint64   // second value of "shares" tag
	TotalShares       string   // "totalshares"
	TotalShareCount   uint64   // second value of "totalshares"
	Timestamp         int64    // "timestamp": unix ms
	Fee               int64    // "fee": optional
	EstPaymentHeight  int64    // "eph": optional
	SharenoteEventIDs []string // "sn": references to kind 35510
	Tx                *Transaction
}

// --- Invoice functions ---

// NewInvoiceEvent creates an unpaid invoice event (kind 35500).
func NewInvoiceEvent(inv *Invoice) (*nip01.Event, error) {
	t, err := MarshalInvoiceTags(inv)
	if err != nil {
		return nil, err
	}
	return nip01.NewEvent(KindInvoice, "", "", t...), nil
}

// NewSettledInvoiceEvent creates a settled invoice event (kind 35501).
func NewSettledInvoiceEvent(inv *Invoice) (*nip01.Event, error) {
	if inv.Tx == nil {
		return nil, fmt.Errorf("settled invoice requires a transaction")
	}
	t, err := MarshalInvoiceTags(inv)
	if err != nil {
		return nil, err
	}
	return nip01.NewEvent(KindSettledInvoice, "", "", t...), nil
}

// MarshalInvoiceTags converts an Invoice to tags.
func MarshalInvoiceTags(inv *Invoice) ([][]string, error) {
	if inv == nil {
		return nil, fmt.Errorf("invoice is nil")
	}

	t := make([][]string, 0, 8)
	t = append(t, []string{"d", inv.HeightHash})
	t = append(t, []string{"b", inv.BlockHash})
	t = append(t, []string{"height", strconv.FormatInt(inv.Height, 10)})
	t = append(t, []string{"amount", strconv.FormatInt(inv.Amount, 10)})
	t = append(t, []string{"workers", strconv.FormatInt(inv.Workers, 10)})
	if inv.Shares != "" {
		t = append(t, []string{"shares", inv.Shares})
	}

	if inv.Tx != nil {
		t = append(t, marshalTransaction(inv.Tx))
	}

	return t, nil
}

// ParseInvoiceEvent extracts an Invoice from a nip01.Event (kind 35500 or 35501).
func ParseInvoiceEvent(ev *nip01.Event) (*Invoice, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindInvoice && ev.Kind != KindSettledInvoice {
		return nil, fmt.Errorf("expected kind %d or %d, got %d", KindInvoice, KindSettledInvoice, ev.Kind)
	}
	return UnmarshalInvoiceTags(ev.Tags)
}

// UnmarshalInvoiceTags parses tags into an Invoice.
func UnmarshalInvoiceTags(t [][]string) (*Invoice, error) {
	inv := &Invoice{}
	var err error

	inv.HeightHash, err = tags.RequireString(t, "d")
	if err != nil {
		return nil, err
	}
	inv.BlockHash, err = tags.RequireString(t, "b")
	if err != nil {
		return nil, err
	}
	inv.Height, err = tags.RequireInt64(t, "height")
	if err != nil {
		return nil, err
	}
	inv.Amount, err = tags.RequireInt64(t, "amount")
	if err != nil {
		return nil, err
	}
	inv.Workers, err = tags.RequireInt64(t, "workers")
	if err != nil {
		return nil, err
	}
	inv.Shares = tags.OptionalString(t, "shares")

	xTag := tags.Find(t, "x")
	if xTag != nil {
		inv.Tx, err = unmarshalTransaction(xTag)
		if err != nil {
			return nil, err
		}
	}

	return inv, nil
}

// --- Share functions ---

// NewPendingShareEvent creates a pending share event (kind 35503).
func NewPendingShareEvent(s *Share) (*nip01.Event, error) {
	t, err := MarshalShareTags(s)
	if err != nil {
		return nil, err
	}
	return nip01.NewEvent(KindPendingShare, "", "", t...), nil
}

// NewFinalizedShareEvent creates a finalized share event (kind 35504).
func NewFinalizedShareEvent(s *Share) (*nip01.Event, error) {
	t, err := MarshalShareTags(s)
	if err != nil {
		return nil, err
	}
	return nip01.NewEvent(KindFinalizedShare, "", "", t...), nil
}

// NewSharePaymentEvent creates a share payment event (kind 35505).
func NewSharePaymentEvent(s *Share) (*nip01.Event, error) {
	t, err := MarshalShareTags(s)
	if err != nil {
		return nil, err
	}
	// Default chain tag if absent.
	found := false
	for _, tag := range t {
		if len(tag) > 0 && tag[0] == "chain" {
			found = true
			break
		}
	}
	if !found {
		t = append(t, []string{"chain", "15"})
	}
	return nip01.NewEvent(KindSharePayment, "", "", t...), nil
}

// MarshalShareTags converts a Share to tags.
func MarshalShareTags(s *Share) ([][]string, error) {
	if s == nil {
		return nil, fmt.Errorf("share is nil")
	}

	t := make([][]string, 0, 16)
	t = append(t, []string{"d", s.ShareID})
	t = append(t, []string{"a", s.Address})
	t = append(t, []string{"h", s.HeightHash})
	t = append(t, []string{"b", s.BlockHash})

	if s.ChainID != "" {
		t = append(t, []string{"chain", s.ChainID})
	}

	workerTag := []string{"workers"}
	workerTag = append(workerTag, s.Workers...)
	t = append(t, workerTag)

	t = append(t, []string{"height", strconv.FormatInt(s.Height, 10)})
	t = append(t, []string{"amount", strconv.FormatInt(s.Amount, 10)})

	if s.Shares != "" {
		shareTag := []string{"shares", s.Shares}
		if s.ShareCount > 0 {
			shareTag = append(shareTag, strconv.FormatUint(s.ShareCount, 10))
		}
		t = append(t, shareTag)
	}

	if s.TotalShares != "" {
		totalTag := []string{"totalshares", s.TotalShares}
		if s.TotalShareCount > 0 {
			totalTag = append(totalTag, strconv.FormatUint(s.TotalShareCount, 10))
		}
		t = append(t, totalTag)
	}

	t = append(t, []string{"timestamp", strconv.FormatInt(s.Timestamp, 10)})

	if s.Fee != 0 {
		t = append(t, []string{"fee", strconv.FormatInt(s.Fee, 10)})
	}
	if s.EstPaymentHeight != 0 {
		t = append(t, []string{"eph", strconv.FormatInt(s.EstPaymentHeight, 10)})
	}

	for _, id := range s.SharenoteEventIDs {
		if strings.TrimSpace(id) != "" {
			t = append(t, []string{"sn", id})
		}
	}

	if s.Tx != nil {
		t = append(t, marshalTransaction(s.Tx))
	}

	return t, nil
}

// ParseShareEvent extracts a Share from a nip01.Event (kind 35503, 35504, or 35505).
func ParseShareEvent(ev *nip01.Event) (*Share, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindPendingShare && ev.Kind != KindFinalizedShare && ev.Kind != KindSharePayment {
		return nil, fmt.Errorf("expected kind %d, %d, or %d, got %d", KindPendingShare, KindFinalizedShare, KindSharePayment, ev.Kind)
	}
	return UnmarshalShareTags(ev.Tags)
}

// UnmarshalShareTags parses tags into a Share.
func UnmarshalShareTags(t [][]string) (*Share, error) {
	s := &Share{}
	var err error

	s.ShareID, err = tags.RequireString(t, "d")
	if err != nil {
		return nil, err
	}
	s.Address, err = tags.RequireString(t, "a")
	if err != nil {
		return nil, err
	}
	s.HeightHash, err = tags.RequireString(t, "h")
	if err != nil {
		return nil, err
	}
	s.BlockHash, err = tags.RequireString(t, "b")
	if err != nil {
		return nil, err
	}

	s.ChainID = tags.OptionalString(t, "chain")

	workersTag := tags.Find(t, "workers")
	if workersTag != nil && len(workersTag) > 1 {
		s.Workers = workersTag[1:]
	}

	s.Height, err = tags.RequireInt64(t, "height")
	if err != nil {
		return nil, err
	}
	s.Amount, err = tags.RequireInt64(t, "amount")
	if err != nil {
		return nil, err
	}

	sharesTag := tags.Find(t, "shares")
	if sharesTag != nil && len(sharesTag) >= 2 {
		s.Shares = sharesTag[1]
		if len(sharesTag) >= 3 {
			if v, e := strconv.ParseUint(sharesTag[2], 10, 64); e == nil {
				s.ShareCount = v
			}
		}
	}

	totalTag := tags.Find(t, "totalshares")
	if totalTag != nil && len(totalTag) >= 2 {
		s.TotalShares = totalTag[1]
		if len(totalTag) >= 3 {
			if v, e := strconv.ParseUint(totalTag[2], 10, 64); e == nil {
				s.TotalShareCount = v
			}
		}
	}

	s.Timestamp, err = tags.OptionalInt64(t, "timestamp")
	if err != nil {
		return nil, err
	}
	s.Fee, err = tags.OptionalInt64(t, "fee")
	if err != nil {
		return nil, err
	}
	s.EstPaymentHeight, err = tags.OptionalInt64(t, "eph")
	if err != nil {
		return nil, err
	}

	for _, snTag := range tags.FindAll(t, "sn") {
		if len(snTag) >= 2 {
			s.SharenoteEventIDs = append(s.SharenoteEventIDs, snTag[1:]...)
		}
	}

	xTag := tags.Find(t, "x")
	if xTag != nil {
		s.Tx, err = unmarshalTransaction(xTag)
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

// --- Validation ---

// ValidateInvoice checks that an Invoice's HeightHash matches its Height.
func ValidateInvoice(inv *Invoice) error {
	if inv.Height <= 0 {
		return fmt.Errorf("height must be greater than zero")
	}
	expected := ComputeHeightHash(inv.Height)
	if inv.HeightHash != expected {
		return fmt.Errorf("height hash mismatch: expected %s, got %s", expected, inv.HeightHash)
	}
	if inv.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	return nil
}

// ValidateShare checks ShareID computation and required fields.
func ValidateShare(s *Share, kind int) error {
	if s.Height <= 0 {
		return fmt.Errorf("height must be greater than zero")
	}
	expected := ComputeHeightHash(s.Height)
	if s.HeightHash != expected {
		return fmt.Errorf("height hash mismatch: expected %s, got %s", expected, s.HeightHash)
	}
	if len(s.Workers) == 0 {
		return fmt.Errorf("at least one worker is required")
	}

	switch kind {
	case KindPendingShare:
		expectedID := ComputePendingShareID(s.Height, s.Address, s.Workers[0])
		if s.ShareID != expectedID {
			return fmt.Errorf("shareID mismatch: expected %s, got %s", expectedID, s.ShareID)
		}
	case KindFinalizedShare, KindSharePayment:
		expectedID := ComputePaymentShareID(s.Height, s.Address)
		if s.ShareID != expectedID {
			return fmt.Errorf("shareID mismatch: expected %s, got %s", expectedID, s.ShareID)
		}
	default:
		return fmt.Errorf("unknown share kind: %d", kind)
	}

	return nil
}

// ComputeHeightHash returns blake2b-256(decimal(height)).
func ComputeHeightHash(height int64) string {
	return hash.Int64(height)
}

// ComputePendingShareID returns blake2b-256("{height}/{address}/{worker}").
func ComputePendingShareID(height int64, address, worker string) string {
	return hash.ShareID(height, address, worker)
}

// ComputePaymentShareID returns blake2b-256("{height}/{address}").
func ComputePaymentShareID(height int64, address string) string {
	return hash.PaymentShareID(height, address)
}

// --- Transaction helpers ---

func marshalTransaction(tx *Transaction) []string {
	if tx.Confirmed {
		return []string{"x", tx.Txid, strconv.FormatInt(tx.BlockHeight, 10), tx.BlockHash}
	}
	return []string{"x", tx.Txid}
}

func unmarshalTransaction(tag []string) (*Transaction, error) {
	if len(tag) < 2 || tag[1] == "" {
		return nil, fmt.Errorf("transaction tag missing txid")
	}
	tx := &Transaction{Txid: tag[1]}
	if len(tag) >= 4 {
		h, err := strconv.ParseInt(tag[2], 10, 64)
		if err == nil && tag[3] != "" {
			tx.BlockHeight = h
			tx.BlockHash = tag[3]
			tx.Confirmed = true
		}
	}
	return tx, nil
}

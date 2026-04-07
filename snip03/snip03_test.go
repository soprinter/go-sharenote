package snip03

import (
	"testing"

	"github.com/ohstr/nmilat/pkg/nip01"
)

func TestInvoiceRoundTrip(t *testing.T) {
	heightHash := ComputeHeightHash(100)
	original := &Invoice{
		HeightHash: heightHash,
		BlockHash:  "000000000000000000abcdef1234567890abcdef1234567890abcdef12345678",
		Height:     100,
		Amount:     5000000,
		Workers:    5,
		Shares:     "30z00",
	}

	tags, err := MarshalInvoiceTags(original)
	if err != nil {
		t.Fatalf("MarshalInvoiceTags: %v", err)
	}

	parsed, err := UnmarshalInvoiceTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalInvoiceTags: %v", err)
	}

	if parsed.HeightHash != original.HeightHash {
		t.Errorf("HeightHash: got %s, want %s", parsed.HeightHash, original.HeightHash)
	}
	if parsed.BlockHash != original.BlockHash {
		t.Errorf("BlockHash: got %s, want %s", parsed.BlockHash, original.BlockHash)
	}
	if parsed.Height != original.Height {
		t.Errorf("Height: got %d, want %d", parsed.Height, original.Height)
	}
	if parsed.Amount != original.Amount {
		t.Errorf("Amount: got %d, want %d", parsed.Amount, original.Amount)
	}
	if parsed.Workers != original.Workers {
		t.Errorf("Workers: got %d, want %d", parsed.Workers, original.Workers)
	}
	if parsed.Shares != original.Shares {
		t.Errorf("Shares: got %s, want %s", parsed.Shares, original.Shares)
	}
}

func TestInvoiceWithTransaction(t *testing.T) {
	heightHash := ComputeHeightHash(200)
	inv := &Invoice{
		HeightHash: heightHash,
		BlockHash:  "000000000000000000abcdef1234567890abcdef1234567890abcdef12345678",
		Height:     200,
		Amount:     1000000,
		Workers:    3,
		Shares:     "33z53",
		Tx: &Transaction{
			Txid:        "aabbccdd00112233445566778899aabbccddeeff00112233445566778899aabb",
			Confirmed:   true,
			BlockHeight: 201,
			BlockHash:   "111111110000000000000000000000000000000000000000000000000000ffff",
		},
	}

	tags, err := MarshalInvoiceTags(inv)
	if err != nil {
		t.Fatalf("MarshalInvoiceTags: %v", err)
	}

	parsed, err := UnmarshalInvoiceTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalInvoiceTags: %v", err)
	}

	if parsed.Tx == nil {
		t.Fatal("expected transaction")
	}
	if !parsed.Tx.Confirmed {
		t.Error("expected confirmed transaction")
	}
	if parsed.Tx.Txid != inv.Tx.Txid {
		t.Errorf("Txid: got %s, want %s", parsed.Tx.Txid, inv.Tx.Txid)
	}
	if parsed.Tx.BlockHeight != 201 {
		t.Errorf("BlockHeight: got %d, want 201", parsed.Tx.BlockHeight)
	}
}

func TestInvoiceUnconfirmedTransaction(t *testing.T) {
	heightHash := ComputeHeightHash(300)
	inv := &Invoice{
		HeightHash: heightHash,
		BlockHash:  "000000000000000000abcdef1234567890abcdef1234567890abcdef12345678",
		Height:     300,
		Amount:     2000000,
		Workers:    1,
		Tx: &Transaction{
			Txid: "aabbccdd00112233445566778899aabbccddeeff00112233445566778899aabb",
		},
	}

	tags, err := MarshalInvoiceTags(inv)
	if err != nil {
		t.Fatalf("MarshalInvoiceTags: %v", err)
	}

	parsed, err := UnmarshalInvoiceTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalInvoiceTags: %v", err)
	}

	if parsed.Tx == nil {
		t.Fatal("expected transaction")
	}
	if parsed.Tx.Confirmed {
		t.Error("expected unconfirmed transaction")
	}
	if parsed.Tx.Txid != inv.Tx.Txid {
		t.Errorf("Txid: got %s, want %s", parsed.Tx.Txid, inv.Tx.Txid)
	}
}

func TestShareRoundTrip(t *testing.T) {
	height := int64(100)
	address := "fc1qxyz123"
	worker := "worker1"
	shareID := ComputePendingShareID(height, address, worker)
	heightHash := ComputeHeightHash(height)

	original := &Share{
		ShareID:    shareID,
		Address:    address,
		HeightHash: heightHash,
		BlockHash:  "000000000000000000abcdef1234567890abcdef1234567890abcdef12345678",
		Workers:    []string{worker},
		Height:     height,
		Amount:     50000,
		Shares:     "20z40",
		ShareCount: 5,
		TotalShares:    "30z00",
		TotalShareCount: 100,
		Timestamp:  1712345678000,
	}

	tags, err := MarshalShareTags(original)
	if err != nil {
		t.Fatalf("MarshalShareTags: %v", err)
	}

	parsed, err := UnmarshalShareTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalShareTags: %v", err)
	}

	if parsed.ShareID != original.ShareID {
		t.Errorf("ShareID: got %s, want %s", parsed.ShareID, original.ShareID)
	}
	if parsed.Address != original.Address {
		t.Errorf("Address: got %s, want %s", parsed.Address, original.Address)
	}
	if parsed.Height != original.Height {
		t.Errorf("Height: got %d, want %d", parsed.Height, original.Height)
	}
	if parsed.Amount != original.Amount {
		t.Errorf("Amount: got %d, want %d", parsed.Amount, original.Amount)
	}
	if parsed.Shares != original.Shares {
		t.Errorf("Shares: got %s, want %s", parsed.Shares, original.Shares)
	}
	if parsed.ShareCount != original.ShareCount {
		t.Errorf("ShareCount: got %d, want %d", parsed.ShareCount, original.ShareCount)
	}
	if parsed.TotalShares != original.TotalShares {
		t.Errorf("TotalShares: got %s, want %s", parsed.TotalShares, original.TotalShares)
	}
	if parsed.TotalShareCount != original.TotalShareCount {
		t.Errorf("TotalShareCount: got %d, want %d", parsed.TotalShareCount, original.TotalShareCount)
	}
	if parsed.Timestamp != original.Timestamp {
		t.Errorf("Timestamp: got %d, want %d", parsed.Timestamp, original.Timestamp)
	}
	if len(parsed.Workers) != 1 || parsed.Workers[0] != worker {
		t.Errorf("Workers: got %v, want [%s]", parsed.Workers, worker)
	}
}

func TestShareWithOptionalFields(t *testing.T) {
	height := int64(100)
	address := "fc1qxyz123"
	shareID := ComputePaymentShareID(height, address)
	heightHash := ComputeHeightHash(height)

	original := &Share{
		ShareID:          shareID,
		Address:          address,
		HeightHash:       heightHash,
		BlockHash:        "000000000000000000abcdef1234567890abcdef1234567890abcdef12345678",
		ChainID:          "15",
		Workers:          []string{"w1", "w2"},
		Height:           height,
		Amount:           75000,
		Shares:           "25z00",
		TotalShares:      "30z00",
		Timestamp:        1712345678000,
		Fee:              1500,
		EstPaymentHeight: 105,
		SharenoteEventIDs: []string{"evid1", "evid2"},
		Tx: &Transaction{
			Txid:        "aabbccdd00112233445566778899aabbccddeeff00112233445566778899aabb",
			Confirmed:   true,
			BlockHeight: 101,
			BlockHash:   "111111110000000000000000000000000000000000000000000000000000ffff",
		},
	}

	tags, err := MarshalShareTags(original)
	if err != nil {
		t.Fatalf("MarshalShareTags: %v", err)
	}

	parsed, err := UnmarshalShareTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalShareTags: %v", err)
	}

	if parsed.ChainID != "15" {
		t.Errorf("ChainID: got %s, want 15", parsed.ChainID)
	}
	if parsed.Fee != 1500 {
		t.Errorf("Fee: got %d, want 1500", parsed.Fee)
	}
	if parsed.EstPaymentHeight != 105 {
		t.Errorf("EstPaymentHeight: got %d, want 105", parsed.EstPaymentHeight)
	}
	if len(parsed.SharenoteEventIDs) != 2 {
		t.Fatalf("SharenoteEventIDs: got %d, want 2", len(parsed.SharenoteEventIDs))
	}
	if parsed.Tx == nil || !parsed.Tx.Confirmed {
		t.Fatal("expected confirmed transaction")
	}
	if len(parsed.Workers) != 2 {
		t.Fatalf("Workers: got %d, want 2", len(parsed.Workers))
	}
}

func TestValidateInvoice(t *testing.T) {
	heightHash := ComputeHeightHash(100)
	inv := &Invoice{
		HeightHash: heightHash,
		Height:     100,
		Amount:     5000,
	}
	if err := ValidateInvoice(inv); err != nil {
		t.Fatalf("ValidateInvoice: %v", err)
	}

	// Wrong height hash.
	inv.HeightHash = "wrong"
	if err := ValidateInvoice(inv); err == nil {
		t.Fatal("expected error for wrong height hash")
	}

	// Zero height.
	inv.Height = 0
	if err := ValidateInvoice(inv); err == nil {
		t.Fatal("expected error for zero height")
	}
}

func TestValidateSharePending(t *testing.T) {
	height := int64(100)
	addr := "fc1qxyz"
	worker := "w1"
	s := &Share{
		ShareID:    ComputePendingShareID(height, addr, worker),
		HeightHash: ComputeHeightHash(height),
		Address:    addr,
		Height:     height,
		Workers:    []string{worker},
	}
	if err := ValidateShare(s, KindPendingShare); err != nil {
		t.Fatalf("ValidateShare: %v", err)
	}
}

func TestValidateShareFinalized(t *testing.T) {
	height := int64(200)
	addr := "fc1qaddr"
	s := &Share{
		ShareID:    ComputePaymentShareID(height, addr),
		HeightHash: ComputeHeightHash(height),
		Address:    addr,
		Height:     height,
		Workers:    []string{"w1", "w2"},
	}
	if err := ValidateShare(s, KindFinalizedShare); err != nil {
		t.Fatalf("ValidateShare: %v", err)
	}
}

func TestValidateShareIDMismatch(t *testing.T) {
	s := &Share{
		ShareID:    "wrong",
		HeightHash: ComputeHeightHash(100),
		Address:    "addr",
		Height:     100,
		Workers:    []string{"w1"},
	}
	if err := ValidateShare(s, KindPendingShare); err == nil {
		t.Fatal("expected error for ShareID mismatch")
	}
}

func TestNewPendingShareEvent(t *testing.T) {
	s := &Share{
		ShareID:    "abc",
		Address:    "addr",
		HeightHash: "def",
		BlockHash:  "ghi",
		Workers:    []string{"w1"},
		Height:     100,
		Amount:     1000,
		Timestamp:  1712345678000,
	}
	ev, err := NewPendingShareEvent(s)
	if err != nil {
		t.Fatalf("NewPendingShareEvent: %v", err)
	}
	if ev.Kind != KindPendingShare {
		t.Errorf("Kind: got %d, want %d", ev.Kind, KindPendingShare)
	}
}

func TestNewSharePaymentEventDefaultChain(t *testing.T) {
	s := &Share{
		ShareID:    "abc",
		Address:    "addr",
		HeightHash: "def",
		BlockHash:  "ghi",
		Workers:    []string{"w1"},
		Height:     100,
		Amount:     1000,
		Timestamp:  1712345678000,
	}
	ev, err := NewSharePaymentEvent(s)
	if err != nil {
		t.Fatalf("NewSharePaymentEvent: %v", err)
	}
	if ev.Kind != KindSharePayment {
		t.Errorf("Kind: got %d, want %d", ev.Kind, KindSharePayment)
	}
	// Check that chain tag was added.
	foundChain := false
	for _, tag := range ev.Tags {
		if len(tag) >= 2 && tag[0] == "chain" && tag[1] == "15" {
			foundChain = true
		}
	}
	if !foundChain {
		t.Error("expected default chain tag '15'")
	}
}

func TestParseShareEventWrongKind(t *testing.T) {
	ev := &nip01.Event{Kind: 99999, Tags: [][]string{}}
	_, err := ParseShareEvent(ev)
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestNewSettledInvoiceRequiresTransaction(t *testing.T) {
	inv := &Invoice{
		HeightHash: ComputeHeightHash(100),
		BlockHash:  "abc",
		Height:     100,
		Amount:     1000,
		Workers:    1,
	}
	_, err := NewSettledInvoiceEvent(inv)
	if err == nil {
		t.Fatal("expected error for missing transaction")
	}
}

func TestComputeHashes(t *testing.T) {
	h := ComputeHeightHash(100)
	if h == "" {
		t.Fatal("expected non-empty height hash")
	}

	sid := ComputePendingShareID(100, "addr", "w1")
	if sid == "" {
		t.Fatal("expected non-empty share ID")
	}

	pid := ComputePaymentShareID(100, "addr")
	if pid == "" {
		t.Fatal("expected non-empty payment share ID")
	}

	// Different inputs produce different hashes.
	if sid == pid {
		t.Error("pending and payment share IDs should differ")
	}
}

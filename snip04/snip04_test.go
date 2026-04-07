package snip04

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ohstr/nmilat/pkg/nip01"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	// Use empty HeaderHex to skip header hash validation during marshal.
	original := &Sharenote{
		HeaderHash:     "0000000000000000abcd123400000000000000000000000000000000deadbeef",
		Address:        "fc1qxyz123",
		Worker:         "antminer-s19",
		Agent:          "bmminer/2.0.0",
		Label:          "34z10",
		PrimaryChainID: "15",
		AuxBlocks: []AuxBlock{
			{BlockHash: "000000ab00000000000000000000000000000000000000000000000000000001", ChainID: "15", Height: 843000, Solved: true, SharenoteLabel: "40z00"},
			{BlockHash: "000001cc00000000000000000000000000000000000000000000000000000002", ChainID: "2a", Height: 110000, Solved: false, SharenoteLabel: "34z10"},
		},
	}

	tags, err := MarshalTags(original)
	if err != nil {
		t.Fatalf("MarshalTags: %v", err)
	}

	parsed, err := UnmarshalTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalTags: %v", err)
	}

	if parsed.HeaderHash != original.HeaderHash {
		t.Errorf("HeaderHash: got %s, want %s", parsed.HeaderHash, original.HeaderHash)
	}
	if parsed.Address != original.Address {
		t.Errorf("Address: got %s, want %s", parsed.Address, original.Address)
	}
	if parsed.Worker != original.Worker {
		t.Errorf("Worker: got %s, want %s", parsed.Worker, original.Worker)
	}
	if parsed.Agent != original.Agent {
		t.Errorf("Agent: got %s, want %s", parsed.Agent, original.Agent)
	}
	if parsed.Label != "34z10" {
		t.Errorf("Label: got %s, want 34z10", parsed.Label)
	}
	if parsed.PrimaryChainID != "15" {
		t.Errorf("PrimaryChainID: got %s, want 15", parsed.PrimaryChainID)
	}
	if len(parsed.AuxBlocks) != 2 {
		t.Fatalf("AuxBlocks length: got %d, want 2", len(parsed.AuxBlocks))
	}
	// Primary chain should be first.
	if parsed.AuxBlocks[0].ChainID != "15" {
		t.Errorf("first AuxBlock ChainID: got %s, want 15", parsed.AuxBlocks[0].ChainID)
	}
	if parsed.AuxBlocks[0].Height != 843000 {
		t.Errorf("first AuxBlock Height: got %d, want 843000", parsed.AuxBlocks[0].Height)
	}
	if !parsed.AuxBlocks[0].Solved {
		t.Error("first AuxBlock Solved: got false, want true")
	}
	if parsed.AuxBlocks[1].ChainID != "2a" {
		t.Errorf("second AuxBlock ChainID: got %s, want 2a", parsed.AuxBlocks[1].ChainID)
	}
	if parsed.HeaderHex != original.HeaderHex {
		t.Errorf("HeaderHex: got %s, want %s", parsed.HeaderHex, original.HeaderHex)
	}
}

func TestNewSharenoteEvent(t *testing.T) {
	sn := &Sharenote{
		HeaderHash:     "0000000000000000abcd123400000000000000000000000000000000deadbeef",
		Address:        "fc1qaddr",
		Worker:         "w1",
		Agent:          "agent/1.0",
		Label:          "33Z53",
		PrimaryChainID: "15",
		AuxBlocks: []AuxBlock{
			{BlockHash: "000000ab00000000000000000000000000000000000000000000000000000001", ChainID: "15", Height: 100, Solved: true, SharenoteLabel: "40z00"},
		},
	}

	ev, err := NewSharenoteEvent(sn)
	if err != nil {
		t.Fatalf("NewSharenoteEvent: %v", err)
	}
	if ev.Kind != KindSharenote {
		t.Errorf("Kind: got %d, want %d", ev.Kind, KindSharenote)
	}
	// d, a, z, w, dd = 5 tags
	if len(ev.Tags) != 5 {
		t.Errorf("Tags count: got %d, want 5", len(ev.Tags))
	}
}

func TestMarshalEmptyAuxBlocks(t *testing.T) {
	sn := &Sharenote{
		HeaderHash: "abc",
		Label:      "33Z53",
	}
	_, err := MarshalTags(sn)
	if err == nil {
		t.Fatal("expected error for empty AuxBlocks")
	}
}

func TestMarshalNilSharenote(t *testing.T) {
	_, err := MarshalTags(nil)
	if err == nil {
		t.Fatal("expected error for nil sharenote")
	}
}

func TestMarshalPrimaryChainNotFound(t *testing.T) {
	sn := &Sharenote{
		HeaderHash:     "abc",
		Label:          "33z53",
		PrimaryChainID: "ff",
		AuxBlocks: []AuxBlock{
			{BlockHash: "abc", ChainID: "15", Height: 100, Solved: true, SharenoteLabel: "33z53"},
		},
	}
	_, err := MarshalTags(sn)
	if err == nil {
		t.Fatal("expected error for missing primary chain")
	}
}

func TestUnmarshalMissingRequiredTags(t *testing.T) {
	tests := []struct {
		name string
		tags [][]string
	}{
		{"missing d", [][]string{{"a", "addr", "w", "ag"}, {"z", "33z53"}, {"w", "bh", "15", "100", "true", "33z53"}}},
		{"missing a", [][]string{{"d", "hash"}, {"z", "33z53"}, {"w", "bh", "15", "100", "true", "33z53"}}},
		{"missing z", [][]string{{"d", "hash"}, {"a", "addr", "w", "ag"}, {"w", "bh", "15", "100", "true", "33z53"}}},
		{"missing w", [][]string{{"d", "hash"}, {"a", "addr", "w", "ag"}, {"z", "33z53"}}},
		{"short a", [][]string{{"d", "hash"}, {"a", "addr"}, {"z", "33z53"}, {"w", "bh", "15", "100", "true", "33z53"}}},
		{"short w", [][]string{{"d", "hash"}, {"a", "addr", "w", "ag"}, {"z", "33z53"}, {"w", "bh", "15"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnmarshalTags(tt.tags)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestParseSharenoteEventWrongKind(t *testing.T) {
	ev := &nip01.Event{Kind: 99999, Tags: [][]string{}}
	_, err := ParseSharenoteEvent(ev)
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestValidateHeaderHash(t *testing.T) {
	// Create a known header and compute double-SHA256 with byte-reversal.
	headerHex := "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a29ab5f49ffff001d1dac2b7c"
	headerBytes, _ := hex.DecodeString(headerHex)
	first := sha256.Sum256(headerBytes)
	second := sha256.Sum256(first[:])
	for i, j := 0, len(second)-1; i < j; i, j = i+1, j-1 {
		second[i], second[j] = second[j], second[i]
	}
	expectedHash := hex.EncodeToString(second[:])

	sn := &Sharenote{
		HeaderHash: expectedHash,
		HeaderHex:  headerHex,
	}
	if err := ValidateHeaderHash(sn); err != nil {
		t.Fatalf("ValidateHeaderHash failed: %v", err)
	}

	// Tamper the hash.
	sn.HeaderHash = "0000000000000000000000000000000000000000000000000000000000000000"
	if err := ValidateHeaderHash(sn); err == nil {
		t.Fatal("expected error for tampered hash")
	}
}

func TestValidateHeaderHashMissingFields(t *testing.T) {
	sn := &Sharenote{HeaderHash: "", HeaderHex: ""}
	if err := ValidateHeaderHash(sn); err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestValidateLabel(t *testing.T) {
	if err := ValidateLabel("33z53"); err != nil {
		t.Fatalf("ValidateLabel: %v", err)
	}
	if err := ValidateLabel("invalid"); err == nil {
		t.Fatal("expected error for invalid label")
	}
}


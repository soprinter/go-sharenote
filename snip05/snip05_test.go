package snip05

import (
	"testing"

	"github.com/ohstr/nmilat/pkg/nip01"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	original := &Identity{
		ChainID: "15",
		Address: "fc1qxyz123abc456def789",
	}

	tags, err := MarshalTags(original)
	if err != nil {
		t.Fatalf("MarshalTags: %v", err)
	}

	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0][0] != "a" || tags[0][1] != "15" || tags[0][2] != "fc1qxyz123abc456def789" {
		t.Errorf("unexpected tag: %v", tags[0])
	}

	parsed, err := UnmarshalTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalTags: %v", err)
	}

	if parsed.ChainID != original.ChainID {
		t.Errorf("ChainID: got %s, want %s", parsed.ChainID, original.ChainID)
	}
	if parsed.Address != original.Address {
		t.Errorf("Address: got %s, want %s", parsed.Address, original.Address)
	}
}

func TestNewIdentityEvent(t *testing.T) {
	id := &Identity{
		ChainID: "15",
		Address: "fc1qaddr",
	}
	ev, err := NewIdentityEvent(id)
	if err != nil {
		t.Fatalf("NewIdentityEvent: %v", err)
	}
	if ev.Kind != KindMinerIdentity {
		t.Errorf("Kind: got %d, want %d", ev.Kind, KindMinerIdentity)
	}
	if len(ev.Tags) != 1 {
		t.Errorf("Tags count: got %d, want 1", len(ev.Tags))
	}
}

func TestMarshalNilIdentity(t *testing.T) {
	_, err := MarshalTags(nil)
	if err == nil {
		t.Fatal("expected error for nil identity")
	}
}

func TestMarshalMissingChainID(t *testing.T) {
	_, err := MarshalTags(&Identity{Address: "addr"})
	if err == nil {
		t.Fatal("expected error for missing chain ID")
	}
}

func TestMarshalMissingAddress(t *testing.T) {
	_, err := MarshalTags(&Identity{ChainID: "15"})
	if err == nil {
		t.Fatal("expected error for missing address")
	}
}

func TestUnmarshalMissingATag(t *testing.T) {
	_, err := UnmarshalTags([][]string{{"d", "something"}})
	if err == nil {
		t.Fatal("expected error for missing 'a' tag")
	}
}

func TestUnmarshalShortATag(t *testing.T) {
	_, err := UnmarshalTags([][]string{{"a", "15"}})
	if err == nil {
		t.Fatal("expected error for short 'a' tag")
	}
}

func TestParseIdentityEventWrongKind(t *testing.T) {
	ev := &nip01.Event{Kind: 99999, Tags: [][]string{{"a", "15", "addr"}}}
	_, err := ParseIdentityEvent(ev)
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestParseIdentityEventValid(t *testing.T) {
	ev := &nip01.Event{
		Kind: KindMinerIdentity,
		Tags: [][]string{{"a", "2a", "bc1qsomebitcoinaddr"}},
	}
	id, err := ParseIdentityEvent(ev)
	if err != nil {
		t.Fatalf("ParseIdentityEvent: %v", err)
	}
	if id.ChainID != "2a" {
		t.Errorf("ChainID: got %s, want 2a", id.ChainID)
	}
	if id.Address != "bc1qsomebitcoinaddr" {
		t.Errorf("Address: got %s, want bc1qsomebitcoinaddr", id.Address)
	}
}

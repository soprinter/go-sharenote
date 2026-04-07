package snip04

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestMarshalUnmarshalSharenote(t *testing.T) {
	sn := &Sharenote{
		Label:      "33z55",
		HeaderHash: "0000000000000000000000000000000000000000000000000000000000000001",
		HeaderHex:  "0102030401020304010203040102030401020304010203040102030401020304",
		Target:     "000000ffff000000000000000000000000000000000000000000000000000000",
		AuxBlocks:  []string{"aux1", "aux2"},
	}

	tgs, err := MarshalTags(sn)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	unmarshaled, err := UnmarshalTags(tgs)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if unmarshaled.Label != "33z55" || unmarshaled.HeaderHash != sn.HeaderHash || len(unmarshaled.AuxBlocks) != 2 {
		t.Errorf("fields mismatch: %+v", unmarshaled)
	}
}

func TestParseZBitEvent(t *testing.T) {
	tgs := nostr.Tags{
		nostr.Tag{"d", "hhash"},
		nostr.Tag{"l", "z-bit", "https://sharenote.xyz", "33z55"},
		nostr.Tag{"h", "hhex"},
		nostr.Tag{"aux", "aux1"},
	}
	ev := &nostr.Event{
		Kind: KindSharenote,
		Tags: tgs,
	}

	sn, err := ParseZBitEvent(ev)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if sn.Label != "33z55" || len(sn.AuxBlocks) != 1 {
		t.Errorf("parsed fields mismatch: %+v", sn)
	}
}

func TestParseZBitEventWrongKind(t *testing.T) {
	ev := &nostr.Event{Kind: 1}
	if _, err := ParseZBitEvent(ev); err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

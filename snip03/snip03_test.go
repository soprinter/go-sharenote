package snip03

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestUnmarshalInvoice(t *testing.T) {
	tags := nostr.Tags{
		nostr.Tag{"d", "hhash"},
		nostr.Tag{"b", "bhash"},
		nostr.Tag{"height", "100"},
		nostr.Tag{"amount", "1000"},
		nostr.Tag{"workers", "5"},
		nostr.Tag{"shares", "difficulty"},
		nostr.Tag{"x", "txid", "100", "bhash"},
	}

	inv, err := UnmarshalInvoiceTags(tags)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if inv.Height != 100 || inv.Amount != 1000 || inv.Workers != 5 || inv.Shares != "difficulty" {
		t.Errorf("fields mismatch: %+v", inv)
	}

	if inv.Tx == nil || inv.Tx.Txid != "txid" || inv.Tx.BlockHeight != 100 || inv.Tx.BlockHash != "bhash" || !inv.Tx.Confirmed {
		t.Errorf("tx mismatch: %+v", inv.Tx)
	}
}

func TestUnmarshalSharePending(t *testing.T) {
	tags := nostr.Tags{
		nostr.Tag{"d", "shareid"},
		nostr.Tag{"a", "addr"},
		nostr.Tag{"h", "hhash"},
		nostr.Tag{"b", "bhash"},
		nostr.Tag{"chain", "15"},
		nostr.Tag{"workers", "w1", "w2"},
		nostr.Tag{"height", "200"},
		nostr.Tag{"amount", "5000"},
		nostr.Tag{"shares", "diff", "1000"},
		nostr.Tag{"totalshares", "totdiff", "5000"},
		nostr.Tag{"timestamp", "1712345678"},
		nostr.Tag{"fee", "10"},
		nostr.Tag{"eph", "205"},
		nostr.Tag{"sn", "snid1", "snid2"},
	}

	s, err := UnmarshalShareTags(tags)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if s.Height != 200 || s.Amount != 5000 || s.ChainID != "15" || len(s.Workers) != 2 {
		t.Errorf("fields mismatch: %+v", s)
	}
	if len(s.SharenoteEventIDs) != 2 {
		t.Errorf("sn ids mismatch: %v", s.SharenoteEventIDs)
	}
}

func TestNewSharePaymentEvent(t *testing.T) {
	s := &Share{
		ShareID:    "id",
		Address:    "addr",
		HeightHash: "hhash",
		BlockHash:  "bhash",
		Height:     300,
		Amount:     10000,
	}

	ev, err := NewSharePaymentEvent(s)
	if err != nil {
		t.Fatalf("error creating event: %v", err)
	}

	if ev.Kind != KindSharePayment {
		t.Errorf("expected kind %d, got %d", KindSharePayment, ev.Kind)
	}

	// check chain tag default
	found := false
	for _, tag := range ev.Tags {
		if tag[0] == "chain" && tag[1] == "15" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("default chain tag not found")
	}
}

package snip05

import (
	"testing"
)

func TestMinerIdentity(t *testing.T) {
	m := &MinerIdentity{
		Pubkey:  "pubkey",
		Address: "addr",
	}

	ev, err := NewMinerIdentityEvent(m)
	if err != nil {
		t.Fatalf("error creating event: %v", err)
	}

	if ev.Kind != KindMinerIdentity {
		t.Errorf("expected kind %d, got %d", KindMinerIdentity, ev.Kind)
	}

	parsed, err := ParseMinerIdentityEvent(ev)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if parsed.Pubkey != m.Pubkey || parsed.Address != m.Address {
		t.Errorf("id mismatch: %+v", parsed)
	}
}

func TestPoolIdentity(t *testing.T) {
	p := &PoolIdentity{
		Name:        "Pool",
		Description: "Desc",
		Pubkey:      "poolpub",
		Address:     "pooladdr",
		Endpoints:   []string{"relay1", "relay2"},
	}

	ev, err := NewPoolIdentityEvent(p)
	if err != nil {
		t.Fatalf("error creating event: %v", err)
	}

	parsed, err := ParsePoolIdentityEvent(ev)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if parsed.Name != p.Name || parsed.Pubkey != p.Pubkey || len(parsed.Endpoints) != 2 {
		t.Errorf("pool id mismatch: %+v", parsed)
	}
}

package snip05

import (
	"encoding/json"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/soprinter/go-sharenote/internal/tags"
)

const (
	KindMinerIdentity = 35506
	KindPoolIdentity  = 35507
)

// MinerIdentity represents a user's mining profile (kind 35506).
type MinerIdentity struct {
	Pubkey  string // miners hex pubkey
	Address string // primary receiving address
}

// PoolIdentity represents a pool's metadata (kind 35507).
type PoolIdentity struct {
	Name        string   // "name"
	Description string   // "description"
	Pubkey      string   // pools hex pubkey
	Address     string   // pools treasury address
	Endpoints   []string // "relay": pool telemetry relays
}

// --- MinerIdentity functions ---

// NewMinerIdentityEvent creates a miner identity event (kind 35506).
func NewMinerIdentityEvent(m *MinerIdentity) (*nostr.Event, error) {
	t, err := MarshalMinerTags(m)
	if err != nil {
		return nil, err
	}
	return &nostr.Event{
		Kind: KindMinerIdentity,
		Tags: t,
	}, nil
}

// MarshalMinerTags converts a MinerIdentity to tags.
func MarshalMinerTags(m *MinerIdentity) (nostr.Tags, error) {
	if m == nil {
		return nil, fmt.Errorf("miner identity is nil")
	}
	if m.Pubkey == "" || m.Address == "" {
		return nil, fmt.Errorf("pubkey and address are required")
	}

	t := make(nostr.Tags, 0, 2)
	t = append(t, nostr.Tag{"p", m.Pubkey})
	t = append(t, nostr.Tag{"a", m.Address})

	return t, nil
}

// ParseMinerIdentityEvent extracts a MinerIdentity from a nostr.Event.
func ParseMinerIdentityEvent(ev *nostr.Event) (*MinerIdentity, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindMinerIdentity {
		return nil, fmt.Errorf("expected kind %d, got %d", KindMinerIdentity, ev.Kind)
	}
	return UnmarshalMinerTags(ev.Tags)
}

// UnmarshalMinerTags parses tags into a MinerIdentity.
func UnmarshalMinerTags(t nostr.Tags) (*MinerIdentity, error) {
	m := &MinerIdentity{}
	var err error

	m.Pubkey, err = tags.RequireString(t, "p")
	if err != nil {
		return nil, err
	}
	m.Address, err = tags.RequireString(t, "a")
	if err != nil {
		return nil, err
	}

	return m, nil
}

// --- PoolIdentity functions ---

// NewPoolIdentityEvent creates a pool identity event (kind 35507).
func NewPoolIdentityEvent(p *PoolIdentity) (*nostr.Event, error) {
	if p == nil {
		return nil, fmt.Errorf("pool identity is nil")
	}
	content, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	t, err := MarshalPoolTags(p)
	if err != nil {
		return nil, err
	}

	return &nostr.Event{
		Kind:    KindPoolIdentity,
		Content: string(content),
		Tags:    t,
	}, nil
}

// MarshalPoolTags converts a PoolIdentity to tags.
func MarshalPoolTags(p *PoolIdentity) (nostr.Tags, error) {
	if p == nil {
		return nil, fmt.Errorf("pool identity is nil")
	}
	if p.Pubkey == "" || p.Address == "" {
		return nil, fmt.Errorf("pubkey and address are required")
	}

	t := make(nostr.Tags, 0, len(p.Endpoints)+2)
	t = append(t, nostr.Tag{"p", p.Pubkey})
	t = append(t, nostr.Tag{"a", p.Address})

	for _, endpoint := range p.Endpoints {
		t = append(t, nostr.Tag{"relay", endpoint})
	}

	return t, nil
}

// ParsePoolIdentityEvent extracts a PoolIdentity from a nostr.Event.
func ParsePoolIdentityEvent(ev *nostr.Event) (*PoolIdentity, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindPoolIdentity {
		return nil, fmt.Errorf("expected kind %d, got %d", KindPoolIdentity, ev.Kind)
	}

	p := &PoolIdentity{}
	if ev.Content != "" {
		if err := json.Unmarshal([]byte(ev.Content), p); err != nil {
			return nil, fmt.Errorf("failed to unmarshal content: %w", err)
		}
	}

	tagsIdentity, err := UnmarshalPoolTags(ev.Tags)
	if err != nil {
		return nil, err
	}

	// Tags take precedence for identity fields
	p.Pubkey = tagsIdentity.Pubkey
	p.Address = tagsIdentity.Address
	p.Endpoints = tagsIdentity.Endpoints

	return p, nil
}

// UnmarshalPoolTags parses tags into a PoolIdentity.
func UnmarshalPoolTags(t nostr.Tags) (*PoolIdentity, error) {
	p := &PoolIdentity{}
	var err error

	p.Pubkey, err = tags.RequireString(t, "p")
	if err != nil {
		return nil, err
	}
	p.Address, err = tags.RequireString(t, "a")
	if err != nil {
		return nil, err
	}

	for _, tag := range tags.FindAll(t, "relay") {
		if len(tag) >= 2 {
			p.Endpoints = append(p.Endpoints, tag[1])
		}
	}

	return p, nil
}

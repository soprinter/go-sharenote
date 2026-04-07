package snip05

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ohstr/nmilat/pkg/nip01"
)

const (
	KindMinerIdentity = 10520
	KindPoolIdentity  = 10521
)

// --- Miner Identity (Kind 35520) ---

// ChainAddress binds a chain ID to a receiving address.
type ChainAddress struct {
	ChainID string // hex-encoded chain ID (e.g., "15")
	Address string // receiving address on the chain
}

// MinerIdentity represents a SNIP-05 miner identity event.
type MinerIdentity struct {
	Chains         []ChainAddress // one or more chain address bindings
	PreferredPayout string        // optional preferred payout scheme hint
}

// NewMinerIdentityEvent builds a nip01.Event of kind 35520.
func NewMinerIdentityEvent(m *MinerIdentity) (*nip01.Event, error) {
	t, err := MarshalMinerTags(m)
	if err != nil {
		return nil, err
	}
	return nip01.NewEvent(KindMinerIdentity, "", "", t...), nil
}

// MarshalMinerTags converts a MinerIdentity into its tag representation.
func MarshalMinerTags(m *MinerIdentity) ([][]string, error) {
	if m == nil {
		return nil, fmt.Errorf("miner identity is nil")
	}
	if len(m.Chains) == 0 {
		return nil, fmt.Errorf("at least one chain address is required")
	}

	seen := make(map[string]bool, len(m.Chains))
	t := make([][]string, 0, len(m.Chains)+1)

	for _, c := range m.Chains {
		if c.ChainID == "" {
			return nil, fmt.Errorf("chain ID is required")
		}
		if c.Address == "" {
			return nil, fmt.Errorf("address is required for chain %s", c.ChainID)
		}
		if seen[c.ChainID] {
			return nil, fmt.Errorf("duplicate chain ID: %s", c.ChainID)
		}
		seen[c.ChainID] = true
		t = append(t, []string{"a", c.ChainID, c.Address})
	}

	if m.PreferredPayout != "" {
		t = append(t, []string{"payout", m.PreferredPayout})
	}

	return t, nil
}

// ParseMinerIdentityEvent extracts a MinerIdentity from a nip01.Event.
func ParseMinerIdentityEvent(ev *nip01.Event) (*MinerIdentity, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindMinerIdentity {
		return nil, fmt.Errorf("expected kind %d, got %d", KindMinerIdentity, ev.Kind)
	}
	return UnmarshalMinerTags(ev.Tags)
}

// UnmarshalMinerTags parses tags into a MinerIdentity.
func UnmarshalMinerTags(t [][]string) (*MinerIdentity, error) {
	m := &MinerIdentity{}
	seen := make(map[string]bool)
	for _, tag := range t {
		if len(tag) < 2 {
			continue
		}
		switch tag[0] {
		case "a":
			if len(tag) >= 3 && !seen[tag[1]] {
				seen[tag[1]] = true
				m.Chains = append(m.Chains, ChainAddress{
					ChainID: tag[1],
					Address: tag[2],
				})
			}
		case "payout":
			m.PreferredPayout = tag[1]
		}
	}
	if len(m.Chains) == 0 {
		return nil, fmt.Errorf("at least one 'a' tag is required")
	}
	return m, nil
}

// --- Pool Identity (Kind 35521) ---

// PoolChain binds a chain ID to a pool payout address with a fee.
type PoolChain struct {
	ChainID string // hex-encoded chain ID
	Address string // pool's payout address for this chain
	FeeBps  int64  // fee in basis points (200 = 2.00%)
}

// PayoutScheme describes a supported payout method.
type PayoutScheme struct {
	Scheme string   // scheme ID: "pps", "fpps", "pplns", "prop", "solo"
	Params []string // key:value parameters (e.g., "n:10000", "fee:300")
}

// PayoutThreshold declares the minimum payout for a chain.
type PayoutThreshold struct {
	ChainID string
	Amount  int64 // minimum payout in satoshis
}

// PoolProfile holds the JSON content metadata.
type PoolProfile struct {
	Name    string `json:"name,omitempty"`
	About   string `json:"about,omitempty"`
	Picture string `json:"picture,omitempty"`
	Website string `json:"website,omitempty"`
}

// PoolIdentity represents a SNIP-05 pool identity event.
type PoolIdentity struct {
	Profile       PoolProfile
	Chains        []PoolChain
	Payouts       []PayoutScheme
	MinSharenote  string // minimum accepted sharenote denomination
	Thresholds    []PayoutThreshold
}

// NewPoolIdentityEvent builds a nip01.Event of kind 35521.
func NewPoolIdentityEvent(p *PoolIdentity) (*nip01.Event, error) {
	t, err := MarshalPoolTags(p)
	if err != nil {
		return nil, err
	}

	content := ""
	if p.Profile.Name != "" || p.Profile.About != "" || p.Profile.Picture != "" || p.Profile.Website != "" {
		b, err := json.Marshal(p.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal profile: %w", err)
		}
		content = string(b)
	}

	return nip01.NewEvent(KindPoolIdentity, "", content, t...), nil
}

// MarshalPoolTags converts a PoolIdentity into its tag representation.
func MarshalPoolTags(p *PoolIdentity) ([][]string, error) {
	if p == nil {
		return nil, fmt.Errorf("pool identity is nil")
	}
	if len(p.Chains) == 0 {
		return nil, fmt.Errorf("at least one chain is required")
	}
	if len(p.Payouts) == 0 {
		return nil, fmt.Errorf("at least one payout scheme is required")
	}

	seen := make(map[string]bool, len(p.Chains))
	t := make([][]string, 0, len(p.Chains)+len(p.Payouts)+3)

	for _, c := range p.Chains {
		if c.ChainID == "" {
			return nil, fmt.Errorf("chain ID is required")
		}
		if c.Address == "" {
			return nil, fmt.Errorf("address is required for chain %s", c.ChainID)
		}
		if c.FeeBps < 0 {
			return nil, fmt.Errorf("fee must be non-negative for chain %s", c.ChainID)
		}
		if seen[c.ChainID] {
			return nil, fmt.Errorf("duplicate chain ID: %s", c.ChainID)
		}
		seen[c.ChainID] = true
		t = append(t, []string{"a", c.ChainID, c.Address, strconv.FormatInt(c.FeeBps, 10)})
	}

	for _, ps := range p.Payouts {
		if ps.Scheme == "" {
			return nil, fmt.Errorf("payout scheme is required")
		}
		tag := []string{"payout", ps.Scheme}
		tag = append(tag, ps.Params...)
		t = append(t, tag)
	}

	if p.MinSharenote != "" {
		t = append(t, []string{"sharenote", p.MinSharenote})
	}

	for _, th := range p.Thresholds {
		t = append(t, []string{"threshold", th.ChainID, strconv.FormatInt(th.Amount, 10)})
	}

	return t, nil
}

// ParsePoolIdentityEvent extracts a PoolIdentity from a nip01.Event.
func ParsePoolIdentityEvent(ev *nip01.Event) (*PoolIdentity, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindPoolIdentity {
		return nil, fmt.Errorf("expected kind %d, got %d", KindPoolIdentity, ev.Kind)
	}
	p, err := UnmarshalPoolTags(ev.Tags)
	if err != nil {
		return nil, err
	}

	if ev.Content != "" {
		if err := json.Unmarshal([]byte(ev.Content), &p.Profile); err != nil {
			return nil, fmt.Errorf("invalid profile JSON: %w", err)
		}
	}

	return p, nil
}

// UnmarshalPoolTags parses tags into a PoolIdentity (without profile).
func UnmarshalPoolTags(t [][]string) (*PoolIdentity, error) {
	p := &PoolIdentity{}
	seen := make(map[string]bool)
	for _, tag := range t {
		if len(tag) < 2 {
			continue
		}
		switch tag[0] {
		case "a":
			if len(tag) >= 4 && !seen[tag[1]] {
				seen[tag[1]] = true
				fee, err := strconv.ParseInt(tag[3], 10, 64)
				if err != nil {
					fee = 0
				}
				p.Chains = append(p.Chains, PoolChain{
					ChainID: tag[1],
					Address: tag[2],
					FeeBps:  fee,
				})
			}
		case "payout":
			ps := PayoutScheme{Scheme: tag[1]}
			if len(tag) > 2 {
				ps.Params = tag[2:]
			}
			p.Payouts = append(p.Payouts, ps)
		case "sharenote":
			p.MinSharenote = tag[1]
		case "threshold":
			if len(tag) >= 3 {
				amt, err := strconv.ParseInt(tag[2], 10, 64)
				if err != nil {
					amt = 0
				}
				p.Thresholds = append(p.Thresholds, PayoutThreshold{
					ChainID: tag[1],
					Amount:  amt,
				})
			}
		}
	}
	if len(p.Chains) == 0 {
		return nil, fmt.Errorf("at least one 'a' tag is required")
	}
	return p, nil
}

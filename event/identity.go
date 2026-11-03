package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
)

// ChainAddress binds a hex chain ID to a receiving address on that chain.
type ChainAddress struct {
	// ChainID is a hex-encoded network identifier, e.g. "15".
	ChainID string
	// Address is the receiving address on that chain.
	Address string
}

// MinerIdentity is a miner's replaceable identity event (kind 10520): the chain
// addresses the miner controls, so pools and settlement systems can discover
// where to send payouts, plus an optional preferred payout scheme.
type MinerIdentity struct {
	// Addresses are the miner's receiving addresses, one per chain. At least
	// one is required and chain IDs must not repeat.
	Addresses []ChainAddress
	// Payout is an optional preferred payout scheme hint, e.g. "pplns".
	Payout string
}

// Event builds the kind 10520 event. Content is empty. It errors if no address
// is present.
func (m MinerIdentity) Event() (*nostr.Event, error) {
	if len(m.Addresses) == 0 {
		return nil, errors.New("event: miner identity needs at least one address")
	}
	tags := make(nostr.Tags, 0, len(m.Addresses)+1)
	for _, a := range m.Addresses {
		if a.ChainID == "" || a.Address == "" {
			return nil, errors.New("event: miner identity address needs chain ID and address")
		}
		tags = append(tags, nostr.Tag{"a", a.ChainID, a.Address})
	}
	if m.Payout != "" {
		tags = append(tags, nostr.Tag{"payout", m.Payout})
	}
	return &nostr.Event{Kind: KindMinerIdentity, Tags: tags}, nil
}

// ParseMinerIdentity extracts a [MinerIdentity] from a kind 10520 event,
// checking the kind and that at least one "a" tag is present.
func ParseMinerIdentity(ev *nostr.Event) (MinerIdentity, error) {
	if ev == nil {
		return MinerIdentity{}, ErrNilEvent
	}
	if ev.Kind != KindMinerIdentity {
		return MinerIdentity{}, fmt.Errorf("event: expected kind %d, got %d", KindMinerIdentity, ev.Kind)
	}
	var m MinerIdentity
	for _, t := range findAllTags(ev.Tags, "a") {
		if len(t) < 3 {
			return MinerIdentity{}, errors.New("event: miner identity \"a\" tag needs chain ID and address")
		}
		m.Addresses = append(m.Addresses, ChainAddress{ChainID: t[1], Address: t[2]})
	}
	if len(m.Addresses) == 0 {
		return MinerIdentity{}, errors.New("event: miner identity has no \"a\" tag")
	}
	m.Payout = optionalString(ev.Tags, "payout")
	return m, nil
}

// Validate applies NIP-SN rule A5: at least one address, valid hex chain IDs,
// and no duplicate chain IDs.
func (m MinerIdentity) Validate() error {
	if len(m.Addresses) == 0 {
		return errors.New("event: miner identity needs at least one address")
	}
	seen := make(map[string]struct{}, len(m.Addresses))
	for _, a := range m.Addresses {
		if !isHex(a.ChainID) {
			return fmt.Errorf("event: chain ID %q is not valid hex", a.ChainID)
		}
		if _, dup := seen[a.ChainID]; dup {
			return fmt.Errorf("event: duplicate chain ID %q", a.ChainID)
		}
		seen[a.ChainID] = struct{}{}
		if a.Address == "" {
			return errors.New("event: miner identity address is empty")
		}
	}
	return nil
}

// PoolProfile is a pool's kind-0-style metadata, serialised as the JSON content
// of a [PoolIdentity] event. All fields are optional.
type PoolProfile struct {
	Name    string `json:"name,omitempty"`
	About   string `json:"about,omitempty"`
	Picture string `json:"picture,omitempty"`
	Website string `json:"website,omitempty"`
}

func (p PoolProfile) isZero() bool {
	return p == PoolProfile{}
}

// PoolAddress is a pool's payout address on one chain, with its fee in basis
// points (200 = 2.00%, 0 = no fee).
type PoolAddress struct {
	ChainID string
	Address string
	FeeBps  int
}

// PayoutScheme is a payout scheme a pool supports, with trailing "key:value"
// parameters. A "pplns" scheme must carry "n:<window>"; a "fee:<bps>" parameter
// overrides the address fee for miners on that scheme.
type PayoutScheme struct {
	// Scheme is the scheme name, e.g. "pps", "fpps", "pplns", "prop", "solo".
	Scheme string
	// Params are the trailing "key:value" parameters, e.g. ["fee:150", "n:5000"].
	Params []string
}

// Threshold is a per-chain minimum payout threshold, in the coin's base unit
// (its smallest unit; satoshis on Bitcoin, for example).
type Threshold struct {
	ChainID string
	Amount  int64
}

// PoolIdentity is a pool operator's replaceable identity event (kind 10521): its
// profile, the chains it accepts with fee schedules, the payout schemes it
// supports, an optional minimum accepted denomination, and per-chain payout
// thresholds.
type PoolIdentity struct {
	// Profile is the optional kind-0-style metadata serialised as JSON content.
	Profile PoolProfile
	// Addresses are the pool's payout addresses, one per chain. At least one is
	// required and chain IDs must not repeat.
	Addresses []PoolAddress
	// Payouts are the supported payout schemes. At least one is required.
	Payouts []PayoutScheme
	// MinDenomination is the optional minimum accepted denomination floor, e.g.
	// "30z00".
	MinDenomination string
	// Thresholds are optional per-chain minimum payout thresholds.
	Thresholds []Threshold
}

// Event builds the kind 10521 event. Content is the profile as JSON (empty when
// the profile is the zero value). It errors if no address or no payout scheme is
// present.
func (p PoolIdentity) Event() (*nostr.Event, error) {
	if len(p.Addresses) == 0 {
		return nil, errors.New("event: pool identity needs at least one address")
	}
	if len(p.Payouts) == 0 {
		return nil, errors.New("event: pool identity needs at least one payout scheme")
	}
	tags := make(nostr.Tags, 0, len(p.Addresses)+len(p.Payouts)+len(p.Thresholds)+1)
	for _, a := range p.Addresses {
		if a.ChainID == "" || a.Address == "" {
			return nil, errors.New("event: pool identity address needs chain ID and address")
		}
		tags = append(tags, nostr.Tag{"a", a.ChainID, a.Address, strconv.Itoa(a.FeeBps)})
	}
	for _, s := range p.Payouts {
		if s.Scheme == "" {
			return nil, errors.New("event: payout scheme name is empty")
		}
		tags = append(tags, append(nostr.Tag{"payout", s.Scheme}, s.Params...))
	}
	if p.MinDenomination != "" {
		tags = append(tags, nostr.Tag{"sharenote", p.MinDenomination})
	}
	for _, th := range p.Thresholds {
		tags = append(tags, nostr.Tag{"threshold", th.ChainID, strconv.FormatInt(th.Amount, 10)})
	}

	ev := &nostr.Event{Kind: KindPoolIdentity, Tags: tags}
	if !p.Profile.isZero() {
		b, err := json.Marshal(p.Profile)
		if err != nil {
			return nil, fmt.Errorf("event: marshal pool profile: %w", err)
		}
		ev.Content = string(b)
	}
	return ev, nil
}

// ParsePoolIdentity extracts a [PoolIdentity] from a kind 10521 event, checking
// the kind, that at least one "a" and one "payout" tag are present, and that any
// non-empty content is valid JSON.
func ParsePoolIdentity(ev *nostr.Event) (PoolIdentity, error) {
	if ev == nil {
		return PoolIdentity{}, ErrNilEvent
	}
	if ev.Kind != KindPoolIdentity {
		return PoolIdentity{}, fmt.Errorf("event: expected kind %d, got %d", KindPoolIdentity, ev.Kind)
	}
	var p PoolIdentity
	if ev.Content != "" {
		if err := json.Unmarshal([]byte(ev.Content), &p.Profile); err != nil {
			return PoolIdentity{}, fmt.Errorf("event: pool profile content is not valid JSON: %w", err)
		}
	}
	for _, t := range findAllTags(ev.Tags, "a") {
		if len(t) < 3 {
			return PoolIdentity{}, errors.New("event: pool identity \"a\" tag needs chain ID and address")
		}
		addr := PoolAddress{ChainID: t[1], Address: t[2]}
		if len(t) >= 4 {
			fee, err := strconv.Atoi(t[3])
			if err != nil {
				return PoolIdentity{}, fmt.Errorf("event: pool fee %q is not an integer: %w", t[3], err)
			}
			addr.FeeBps = fee
		}
		p.Addresses = append(p.Addresses, addr)
	}
	if len(p.Addresses) == 0 {
		return PoolIdentity{}, errors.New("event: pool identity has no \"a\" tag")
	}
	for _, t := range findAllTags(ev.Tags, "payout") {
		if len(t) < 2 {
			return PoolIdentity{}, errors.New("event: pool identity \"payout\" tag needs a scheme")
		}
		p.Payouts = append(p.Payouts, PayoutScheme{Scheme: t[1], Params: append([]string(nil), t[2:]...)})
	}
	if len(p.Payouts) == 0 {
		return PoolIdentity{}, errors.New("event: pool identity has no \"payout\" tag")
	}
	p.MinDenomination = optionalString(ev.Tags, "sharenote")
	for _, t := range findAllTags(ev.Tags, "threshold") {
		if len(t) < 3 {
			return PoolIdentity{}, errors.New("event: pool identity \"threshold\" tag needs chain ID and amount")
		}
		amount, err := strconv.ParseInt(t[2], 10, 64)
		if err != nil {
			return PoolIdentity{}, fmt.Errorf("event: threshold amount %q is not an integer: %w", t[2], err)
		}
		p.Thresholds = append(p.Thresholds, Threshold{ChainID: t[1], Amount: amount})
	}
	return p, nil
}

// Validate applies NIP-SN rule A6: at least one address with a non-negative fee
// and valid non-duplicate hex chain IDs, at least one payout scheme, and a
// "pplns" scheme must carry an "n:" window parameter.
func (p PoolIdentity) Validate() error {
	if len(p.Addresses) == 0 {
		return errors.New("event: pool identity needs at least one address")
	}
	seen := make(map[string]struct{}, len(p.Addresses))
	for _, a := range p.Addresses {
		if !isHex(a.ChainID) {
			return fmt.Errorf("event: chain ID %q is not valid hex", a.ChainID)
		}
		if _, dup := seen[a.ChainID]; dup {
			return fmt.Errorf("event: duplicate chain ID %q", a.ChainID)
		}
		seen[a.ChainID] = struct{}{}
		if a.Address == "" {
			return errors.New("event: pool identity address is empty")
		}
		if a.FeeBps < 0 {
			return fmt.Errorf("event: fee %d bps is negative", a.FeeBps)
		}
	}
	if len(p.Payouts) == 0 {
		return errors.New("event: pool identity needs at least one payout scheme")
	}
	for _, s := range p.Payouts {
		if s.Scheme == "pplns" && !hasParam(s.Params, "n:") {
			return errors.New("event: pplns payout scheme must carry an n:<window> parameter")
		}
	}
	return nil
}

func hasParam(params []string, prefix string) bool {
	for _, p := range params {
		if len(p) >= len(prefix) && p[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

package snip05

import (
	"fmt"

	"github.com/ohstr/nmilat/pkg/nip01"
)

// Identity is a backwards-compatible alias for ChainAddress.
// New code should use ChainAddress, MinerIdentity, or PoolIdentity directly.
type Identity = ChainAddress

// MarshalTags is a backwards-compatible wrapper around MarshalMinerTags for
// single-chain miner identities.
func MarshalTags(id *Identity) ([][]string, error) {
	if id == nil {
		return nil, fmt.Errorf("identity is nil")
	}
	ca := ChainAddress{ChainID: id.ChainID, Address: id.Address}
	return MarshalMinerTags(&MinerIdentity{Chains: []ChainAddress{ca}})
}

// UnmarshalTags is a backwards-compatible wrapper around UnmarshalMinerTags
// for single-chain miner identities. Returns the first chain address found.
func UnmarshalTags(tags [][]string) (*Identity, error) {
	m, err := UnmarshalMinerTags(tags)
	if err != nil {
		return nil, err
	}
	id := m.Chains[0]
	return &id, nil
}

// NewIdentityEvent is a backwards-compatible wrapper around NewMinerIdentityEvent
// for single-chain miner identities.
func NewIdentityEvent(id *Identity) (*nip01.Event, error) {
	if id == nil {
		return nil, nil
	}
	ca := ChainAddress{ChainID: id.ChainID, Address: id.Address}
	return NewMinerIdentityEvent(&MinerIdentity{Chains: []ChainAddress{ca}})
}

// ParseIdentityEvent is a backwards-compatible wrapper around
// ParseMinerIdentityEvent for single-chain miner identities.
// Returns the first chain address found in the event.
func ParseIdentityEvent(ev *nip01.Event) (*Identity, error) {
	m, err := ParseMinerIdentityEvent(ev)
	if err != nil {
		return nil, err
	}
	id := m.Chains[0]
	return &id, nil
}

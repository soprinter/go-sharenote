package snip05

import (
	"github.com/nbd-wtf/go-nostr"
)

// Legacy Identity struct using [][]string for tags.
// Only for internal backwards compatibility if needed.

func MarshalTags(p *PoolIdentity) (nostr.Tags, error) {
	return MarshalPoolTags(p)
}

func UnmarshalTags(t nostr.Tags) (*PoolIdentity, error) {
	return UnmarshalPoolTags(t)
}

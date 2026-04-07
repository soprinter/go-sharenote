package snip04

import (
	"fmt"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/soprinter/go-sharenote/internal/tags"
	"github.com/soprinter/go-sharenote/snip00"
)

const KindSharenote = 35510

// Sharenote represents a proof-of-work snippet (kind 35510).
type Sharenote struct {
	Label      string   // SNIP-00 label
	HeaderHash string   // "d": hex
	HeaderHex  string   // "h": hex
	Target     string   // "t": hex
	AuxBlocks  []string // "aux"
}

// NewZBitEvent creates a Sharenote event (kind 35510).
func NewZBitEvent(sn *Sharenote) (*nostr.Event, error) {
	t, err := MarshalTags(sn)
	if err != nil {
		return nil, err
	}
	return &nostr.Event{
		Kind: KindSharenote,
		Tags: t,
	}, nil
}

// MarshalTags converts a Sharenote into its tag representation.
func MarshalTags(sn *Sharenote) (nostr.Tags, error) {
	if sn == nil {
		return nil, fmt.Errorf("sharenote is nil")
	}
	if len(sn.AuxBlocks) == 0 {
		return nil, fmt.Errorf("auxblocks must not be empty")
	}
	if sn.HeaderHex != "" && sn.HeaderHash != "" {
		if err := validateHeaderHashInternal(sn.HeaderHex, sn.HeaderHash); err != nil {
			return nil, err
		}
	}
	note, err := snip00.EnsureNote(sn.Label)
	if err != nil {
		return nil, fmt.Errorf("invalid sharenote label: %w", err)
	}
	displayLabel := strings.ToLower(note.Label())

	t := make(nostr.Tags, 0, len(sn.AuxBlocks)+4)
	t = append(t, nostr.Tag{"d", sn.HeaderHash})
	t = append(t, nostr.Tag{"l", "z-bit", "https://sharenote.xyz", displayLabel})
	t = append(t, nostr.Tag{"h", sn.HeaderHex})
	t = append(t, nostr.Tag{"t", sn.Target})

	for _, aux := range sn.AuxBlocks {
		if aux != "" {
			t = append(t, nostr.Tag{"aux", aux})
		}
	}

	return t, nil
}

// ParseZBitEvent extracts a Sharenote from a nostr.Event.
func ParseZBitEvent(ev *nostr.Event) (*Sharenote, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindSharenote {
		return nil, fmt.Errorf("expected kind %d, got %d", KindSharenote, ev.Kind)
	}
	return UnmarshalTags(ev.Tags)
}

// UnmarshalTags parses tags into a Sharenote.
func UnmarshalTags(t nostr.Tags) (*Sharenote, error) {
	headerHash, err := tags.RequireString(t, "d")
	if err != nil {
		return nil, err
	}
	lTag := tags.Find(t, "l")
	if lTag == nil || len(lTag) < 4 {
		return nil, fmt.Errorf("missing or invalid 'l' tag")
	}
	if lTag[1] != "z-bit" {
		return nil, fmt.Errorf("expected z-bit label, got %s", lTag[1])
	}
	label := lTag[3]

	headerHex, err := tags.RequireString(t, "h")
	if err != nil {
		return nil, err
	}
	target, err := tags.OptionalString(t, "t"), nil // Optional in parsing for flexibility

	var auxBlocks []string
	for _, tag := range tags.FindAll(t, "aux") {
		if len(tag) >= 2 {
			auxBlocks = append(auxBlocks, tag[1])
		}
	}

	return &Sharenote{
		Label:      label,
		HeaderHash: headerHash,
		HeaderHex:  headerHex,
		Target:     target,
		AuxBlocks:  auxBlocks,
	}, nil
}

func validateHeaderHashInternal(headerHex, headerHash string) error {
	if err := tags.Validate32Hex(headerHex); err != nil {
		return fmt.Errorf("headerHex: %w", err)
	}
	if err := tags.Validate32Hex(headerHash); err != nil {
		return fmt.Errorf("headerHash: %w", err)
	}
	return nil
}

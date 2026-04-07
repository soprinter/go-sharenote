package snip04

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/ohstr/nmilat/pkg/nip01"
	"github.com/soprinter/go-sharenote/internal/tags"
	"github.com/soprinter/go-sharenote/snip00"
)

const KindSharenote = 35510

// AuxBlock describes one merged-mining chain entry in a "w" tag.
type AuxBlock struct {
	BlockHash      string // auxiliary target hash
	ChainID        string // hex chain ID (e.g., "15")
	Height         int64  // chain block height
	Solved         bool   // true if network target reached
	SharenoteLabel string // difficulty label for this branch
}

// Sharenote represents a SNIP-04 minting event payload.
type Sharenote struct {
	HeaderHash     string     // "d" tag: block header hash
	Address        string     // "a" tag[1]: miner chain address
	Worker         string     // "a" tag[2]: worker name
	Agent          string     // "a" tag[3]: user-agent string
	Label          string     // "z" tag: sharenote denomination label
	PrimaryChainID string     // hex chain ID of the primary "w" tag
	AuxBlocks      []AuxBlock // "w" tags: primary first, then auxiliaries
	HeaderHex      string     // "dd" tag: full header hex
}

// NewSharenoteEvent builds a nip01.Event of kind 35510.
func NewSharenoteEvent(sn *Sharenote) (*nip01.Event, error) {
	t, err := MarshalTags(sn)
	if err != nil {
		return nil, err
	}
	return nip01.NewEvent(KindSharenote, "", "", t...), nil
}

// MarshalTags converts a Sharenote into its tag representation.
func MarshalTags(sn *Sharenote) ([][]string, error) {
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

	t := make([][]string, 0, len(sn.AuxBlocks)+4)
	t = append(t, []string{"d", sn.HeaderHash})
	t = append(t, []string{"a", sn.Address, sn.Worker, sn.Agent})
	t = append(t, []string{"z", displayLabel})

	// Primary chain first.
	primaryIdx := -1
	for i, block := range sn.AuxBlocks {
		if block.ChainID == sn.PrimaryChainID {
			primaryIdx = i
			break
		}
	}
	if primaryIdx == -1 {
		return nil, fmt.Errorf("primary chain %s not found in aux blocks", sn.PrimaryChainID)
	}

	t = append(t, marshalAuxBlock(sn.AuxBlocks[primaryIdx]))
	for i, block := range sn.AuxBlocks {
		if i == primaryIdx {
			continue
		}
		t = append(t, marshalAuxBlock(block))
	}

	t = append(t, []string{"dd", sn.HeaderHex})
	return t, nil
}

func marshalAuxBlock(b AuxBlock) []string {
	return []string{
		"w",
		b.BlockHash,
		b.ChainID,
		strconv.FormatInt(b.Height, 10),
		strconv.FormatBool(b.Solved),
		b.SharenoteLabel,
	}
}

// ParseSharenoteEvent extracts a Sharenote from a nip01.Event.
func ParseSharenoteEvent(ev *nip01.Event) (*Sharenote, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindSharenote {
		return nil, fmt.Errorf("expected kind %d, got %d", KindSharenote, ev.Kind)
	}
	return UnmarshalTags(ev.Tags)
}

// UnmarshalTags parses tags into a Sharenote.
func UnmarshalTags(t [][]string) (*Sharenote, error) {
	headerHash, err := tags.RequireString(t, "d")
	if err != nil {
		return nil, err
	}

	aTag := tags.Find(t, "a")
	if aTag == nil || len(aTag) < 4 {
		return nil, fmt.Errorf("required tag 'a' must have at least 3 values (address, worker, agent)")
	}

	label, err := tags.RequireString(t, "z")
	if err != nil {
		return nil, err
	}

	headerHex := tags.OptionalString(t, "dd")

	wTags := tags.FindAll(t, "w")
	if len(wTags) == 0 {
		return nil, fmt.Errorf("at least one 'w' tag is required")
	}

	auxBlocks := make([]AuxBlock, 0, len(wTags))
	for _, wTag := range wTags {
		if len(wTag) < 6 {
			return nil, fmt.Errorf("'w' tag requires 6 elements, got %d", len(wTag))
		}
		height, err := strconv.ParseInt(wTag[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid height in 'w' tag: %w", err)
		}
		solved, err := strconv.ParseBool(wTag[4])
		if err != nil {
			return nil, fmt.Errorf("invalid solved in 'w' tag: %w", err)
		}
		auxBlocks = append(auxBlocks, AuxBlock{
			BlockHash:      wTag[1],
			ChainID:        wTag[2],
			Height:         height,
			Solved:         solved,
			SharenoteLabel: wTag[5],
		})
	}

	primaryChainID := ""
	if len(auxBlocks) > 0 {
		primaryChainID = auxBlocks[0].ChainID
	}

	return &Sharenote{
		HeaderHash:     headerHash,
		Address:        aTag[1],
		Worker:         aTag[2],
		Agent:          aTag[3],
		Label:          label,
		PrimaryChainID: primaryChainID,
		AuxBlocks:      auxBlocks,
		HeaderHex:      headerHex,
	}, nil
}

// ValidateHeaderHash checks that double-SHA256 of HeaderHex matches HeaderHash.
func ValidateHeaderHash(sn *Sharenote) error {
	if sn.HeaderHex == "" || sn.HeaderHash == "" {
		return fmt.Errorf("both HeaderHex and HeaderHash are required for validation")
	}
	return validateHeaderHashInternal(sn.HeaderHex, sn.HeaderHash)
}

func validateHeaderHashInternal(headerHex, headerHash string) error {
	headerBytes, err := hex.DecodeString(headerHex)
	if err != nil {
		return fmt.Errorf("invalid header hex: %w", err)
	}
	first := sha256.Sum256(headerBytes)
	second := sha256.Sum256(first[:])
	// Byte-reverse for block hash display format.
	for i, j := 0, len(second)-1; i < j; i, j = i+1, j-1 {
		second[i], second[j] = second[j], second[i]
	}
	computed := hex.EncodeToString(second[:])
	if computed != headerHash {
		return fmt.Errorf("header hash mismatch: computed %s, got %s", computed, headerHash)
	}
	return nil
}

// ValidateLabel checks that label is a valid snip00 sharenote label.
func ValidateLabel(label string) error {
	_, err := snip00.EnsureNote(label)
	if err != nil {
		return fmt.Errorf("invalid sharenote label: %w", err)
	}
	return nil
}

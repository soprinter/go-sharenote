package event

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	sharenote "github.com/soprinter/go-sharenote"
)

// Branch is one blockchain a single hash execution solved or cleared, carried by
// a "w" tag of the Raw Sharenote. The primary chain's branch appears first; aux
// chains follow.
type Branch struct {
	// BlockHash is the target hash for this branch, hex-encoded.
	BlockHash string
	// ChainID is the hex-encoded network ID, e.g. "15" (decimal 21) or "2a".
	ChainID string
	// Height is this chain's block height.
	Height int64
	// Solved is true when the hash reached this network's subsidy target, false
	// when it only cleared the lower Sharenote boundary.
	Solved bool
	// Label is this branch's Sharenote difficulty label, lowercase.
	Label string
}

// RawSharenote is the only cryptographically verifiable artifact in the chain: a
// minting agent's raw block header and the merged-mining branches a single hash
// cleared (kind 35510). Validators hash the header hex and check it against the
// HeaderHash and the leading-zero bound implied by Denomination (NIP-SN rule
// A14); [RawSharenote.Validate] performs both checks.
type RawSharenote struct {
	// HeaderHash is the block header hash; the addressable "d" coordinate.
	HeaderHash string
	// Address, Worker, and Agent form the miner attribution ("a" tag).
	Address string
	Worker  string
	Agent   string
	// Denomination is the Sharenote denomination label, lowercase ("z" tag).
	Denomination string
	// Branches are the merged-mining branches, primary chain first ("w" tags);
	// at least one is required.
	Branches []Branch
	// HeaderHex is the full block header, hex-encoded ("dd" tag, emitted last).
	HeaderHex string
}

// Event builds the kind 35510 event. Content is empty, the denomination is
// lowercased, and the header "dd" tag is emitted last as NIP-SN requires. It
// errors if there is no branch or no header hash.
func (r RawSharenote) Event() (*nostr.Event, error) {
	if r.HeaderHash == "" {
		return nil, errors.New("event: raw sharenote header hash is required")
	}
	if len(r.Branches) == 0 {
		return nil, errors.New("event: raw sharenote needs at least one branch")
	}
	tags := make(nostr.Tags, 0, len(r.Branches)+4)
	tags = append(tags,
		nostr.Tag{"d", r.HeaderHash},
		nostr.Tag{"a", r.Address, r.Worker, r.Agent},
		nostr.Tag{"z", strings.ToLower(r.Denomination)},
	)
	for _, b := range r.Branches {
		tags = append(tags, nostr.Tag{
			"w", b.BlockHash, b.ChainID,
			strconv.FormatInt(b.Height, 10),
			strconv.FormatBool(b.Solved),
			strings.ToLower(b.Label),
		})
	}
	// "dd" MUST be the last tag (NIP-SN rule A14).
	tags = append(tags, nostr.Tag{"dd", r.HeaderHex})
	return &nostr.Event{Kind: KindRawSharenote, Tags: tags}, nil
}

// ParseRawSharenote extracts a [RawSharenote] from a kind 35510 event, checking
// the kind, the required "d", "a", "z", and "dd" tags, that at least one "w"
// branch is present, that each "w" tag has exactly six elements, and that "dd"
// is the last tag (NIP-SN rule A14).
func ParseRawSharenote(ev *nostr.Event) (RawSharenote, error) {
	if ev == nil {
		return RawSharenote{}, ErrNilEvent
	}
	if ev.Kind != KindRawSharenote {
		return RawSharenote{}, fmt.Errorf("event: expected kind %d, got %d", KindRawSharenote, ev.Kind)
	}
	var (
		r   RawSharenote
		err error
	)
	if r.HeaderHash, err = requireString(ev.Tags, "d"); err != nil {
		return RawSharenote{}, err
	}
	a := findTag(ev.Tags, "a")
	if a == nil {
		return RawSharenote{}, errors.New("event: raw sharenote is missing its \"a\" tag")
	}
	if len(a) > 1 {
		r.Address = a[1]
	}
	if len(a) > 2 {
		r.Worker = a[2]
	}
	if len(a) > 3 {
		r.Agent = a[3]
	}
	if r.Denomination, err = requireString(ev.Tags, "z"); err != nil {
		return RawSharenote{}, err
	}
	for _, w := range findAllTags(ev.Tags, "w") {
		// A14: each "w" entry MUST have exactly six elements.
		if len(w) != 6 {
			return RawSharenote{}, errors.New("event: raw sharenote \"w\" tag must have exactly six elements")
		}
		height, err := strconv.ParseInt(w[3], 10, 64)
		if err != nil {
			return RawSharenote{}, fmt.Errorf("event: branch height %q is not an integer: %w", w[3], err)
		}
		r.Branches = append(r.Branches, Branch{
			BlockHash: w[1],
			ChainID:   w[2],
			Height:    height,
			Solved:    w[4] == "true",
			Label:     w[5],
		})
	}
	if len(r.Branches) == 0 {
		return RawSharenote{}, errors.New("event: raw sharenote has no \"w\" branch")
	}
	if r.HeaderHex, err = requireString(ev.Tags, "dd"); err != nil {
		return RawSharenote{}, err
	}
	// A14: "dd" MUST be the last tag.
	if n := len(ev.Tags); n == 0 || len(ev.Tags[n-1]) == 0 || ev.Tags[n-1][0] != "dd" {
		return RawSharenote{}, errors.New("event: raw sharenote \"dd\" tag must be the last tag")
	}
	return r, nil
}

// Validate applies NIP-SN rule A14 in full.
//
//   - Clause 1: the byte-reversed double-SHA256 of the "dd" header hex MUST equal
//     the HeaderHash ("d"). This is the only cryptographic verification path —
//     the denomination label is never trusted (rule A2).
//   - Clause 2: that header hash, read as a big-endian 256-bit integer, MUST clear
//     the leading-zero bound implied by Denomination ("z"), i.e.
//     int(HeaderHash,16) <= target_for(Denomination). For an integer-cents label
//     this reduces to "at least z leading zero bits".
//
// It also checks the structural rules it can see on the parsed value: the
// denomination and every branch label are lowercase, at least one branch is
// present, and each branch carries a block hash and a hex chain ID.
//
// primaryChainID identifies the primary chain out of band (NIP-SN gives a
// received 35510 no self-describing marker — see the contract §6.1). When it is
// non-empty the primary chain MUST be present among the branches and MUST be the
// first branch; when it is empty those two checks are skipped.
func (r RawSharenote) Validate(primaryChainID string) error {
	if r.HeaderHash == "" {
		return errors.New("event: raw sharenote header hash is required")
	}
	// A14: z MUST be lowercase.
	if r.Denomination != strings.ToLower(r.Denomination) {
		return errors.New("event: raw sharenote denomination must be lowercase")
	}
	if r.HeaderHex == "" {
		return errors.New("event: raw sharenote header hex is required")
	}
	if len(r.Branches) == 0 {
		return errors.New("event: raw sharenote needs at least one branch")
	}
	for i, b := range r.Branches {
		if b.BlockHash == "" || b.ChainID == "" {
			return fmt.Errorf("event: branch %d is missing its block hash or chain ID", i)
		}
		if !isHex(b.ChainID) {
			return fmt.Errorf("event: branch %d chain ID %q is not valid hex (A16)", i, b.ChainID)
		}
		if b.Label != strings.ToLower(b.Label) {
			return fmt.Errorf("event: branch %d label must be lowercase", i)
		}
	}
	// Primary-chain presence and ordering (contract §6.1).
	if primaryChainID != "" {
		found := false
		for _, b := range r.Branches {
			if b.ChainID == primaryChainID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("event: declared primary chain %q is absent from the branches (A14)", primaryChainID)
		}
		if r.Branches[0].ChainID != primaryChainID {
			return fmt.Errorf("event: primary chain %q is not the first branch (A14)", primaryChainID)
		}
	}
	// A14 clause 1: the header hash MUST be the byte-reversed double-SHA256 of
	// the header hex.
	if err := verifyHeaderHash(r.HeaderHex, r.HeaderHash); err != nil {
		return err
	}
	// A14 clause 2: the header hash MUST clear the leading-zero bound implied by
	// the denomination label.
	return verifyHeaderZBound(r.HeaderHash, r.Denomination)
}

// ValidateRawSharenoteEvent validates a kind 35510 event against the full
// NIP-SN A14 rule set in a single call, giving Go the same raw-event validation
// entry point as the Python sharenotelib.validate_raw_event and the TypeScript
// validateRawSharenoteEvent. It runs [ParseRawSharenote] for the structural tag
// checks (required "d", "a", "z", and "dd" tags, exactly six elements per "w",
// at least one branch, and "dd" last) and then [RawSharenote.Validate] for the
// semantic checks (lowercase labels, hex branch chain IDs, primary-chain
// presence and ordering, and A14 clauses 1 and 2), returning the parsed
// [RawSharenote] on success.
//
// primaryChainID identifies the primary chain out of band (see
// [RawSharenote.Validate]); pass "" to skip the presence and ordering checks.
func ValidateRawSharenoteEvent(ev *nostr.Event, primaryChainID string) (RawSharenote, error) {
	r, err := ParseRawSharenote(ev)
	if err != nil {
		return RawSharenote{}, err
	}
	if err := r.Validate(primaryChainID); err != nil {
		return RawSharenote{}, err
	}
	return r, nil
}

// verifyHeaderHash checks NIP-SN rule A14 clause 1: the byte-reversed
// double-SHA256 of headerHex equals headerHash (Bitcoin block-hash display
// order).
func verifyHeaderHash(headerHex, headerHash string) error {
	raw, err := hex.DecodeString(strings.TrimSpace(headerHex))
	if err != nil {
		return fmt.Errorf("event: raw sharenote header hex is not valid hex: %w", err)
	}
	first := sha256.Sum256(raw)
	second := sha256.Sum256(first[:])
	// Reverse into display order.
	reversed := make([]byte, len(second))
	for i := range second {
		reversed[len(second)-1-i] = second[i]
	}
	got := hex.EncodeToString(reversed)
	if !strings.EqualFold(got, strings.TrimSpace(headerHash)) {
		return fmt.Errorf("event: raw sharenote header hash %q does not match reversed double-SHA256(dd) %q (A14 clause 1)", headerHash, got)
	}
	return nil
}

// verifyHeaderZBound checks NIP-SN rule A14 clause 2: the header hash, read as a
// big-endian 256-bit integer, is no greater than target_for(denomination).
func verifyHeaderZBound(headerHash, denomination string) error {
	note, err := sharenote.ParseLabel(denomination)
	if err != nil {
		return fmt.Errorf("event: raw sharenote denomination %q is not a valid label: %w", denomination, err)
	}
	target, err := note.Target()
	if err != nil {
		return fmt.Errorf("event: raw sharenote denomination %q has no representable target: %w", denomination, err)
	}
	hashInt, ok := new(big.Int).SetString(strings.TrimSpace(headerHash), 16)
	if !ok {
		return fmt.Errorf("event: raw sharenote header hash %q is not valid hex", headerHash)
	}
	if hashInt.Cmp(target) > 0 {
		return fmt.Errorf("event: raw sharenote header hash does not clear the %s leading-zero bound (A14 clause 2)", note.Label())
	}
	return nil
}

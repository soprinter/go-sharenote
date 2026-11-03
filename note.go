package sharenote

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// CentBitStep is the fractional Z-bit value of one centi-bit (the "cc" field of
// a denomination label), i.e. 0.01 Z-bits.
const CentBitStep = 0.01

// MinCents and MaxCents bound the centi-bit field of a denomination label. A
// label's cents are always clamped to [MinCents, MaxCents].
const (
	MinCents = 0
	MaxCents = 99
)

// centsPerBit is the number of centi-bit increments in one whole Z-bit.
const centsPerBit = 100

// Note is a Sharenote denomination: a quantity of proof-of-work measured in
// Z-bits, the base-2 logarithm of its continuous difficulty.
//
// The zero Note is valid and equal to the label "0Z00": zero bits, a continuous
// difficulty of 1, and a per-hash probability of 1. Notes are immutable values;
// the arithmetic methods return new notes rather than mutating the receiver, so
// a Note is safe to copy and share.
//
// Construct a Note with [ParseLabel], [FromBits], [FromComponents],
// [FromCentBits], [FromTarget], or [FromCompact]. Build one from a rig's
// capacity with [FromHashrate].
type Note struct {
	// bits is the authoritative Z-bit value (log2 of the continuous
	// difficulty). The label and components are derived from it.
	bits float64

	// diff caches the note's exact continuous difficulty (2**bits) as a
	// rational when it is known exactly — whole-Z notes, or results of the
	// exact-arithmetic methods. It is nil for notes whose difficulty is only
	// known as a float (e.g. built from a fractional Z-bit value), in which
	// case [Note.Difficulty] reconstructs it on demand. Treated as immutable.
	diff *big.Rat
}

var (
	reDecimal  = regexp.MustCompile(`^(\d+(?:\.\d+)?)Z$`)
	reStandard = regexp.MustCompile(`^(\d+)Z(?:(\d{1,2})(?:CZ)?)?$`)
	reDotted   = regexp.MustCompile(`^(\d+)\.(\d{1,2})Z$`)
)

// ParseLabel parses a denomination label into a Note. It accepts the canonical
// form "34Z10", the spaced centi-bit form "34Z 10CZ", the dotted form "34.10Z",
// and a bare decimal "34.1Z"; parsing is case-insensitive and ignores spaces.
// Centi-bits of more than two digits are truncated and fewer are zero-padded.
func ParseLabel(label string) (Note, error) {
	cleaned := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(label), " ", ""))

	if m := reStandard.FindStringSubmatch(cleaned); m != nil {
		z, _ := strconv.Atoi(m[1])
		cents := 0
		if m[2] != "" {
			cents, _ = strconv.Atoi(m[2])
		}
		return FromComponents(z, cents)
	}
	if m := reDotted.FindStringSubmatch(cleaned); m != nil {
		z, _ := strconv.Atoi(m[1])
		decimals := m[2]
		switch {
		case len(decimals) < 2:
			decimals += strings.Repeat("0", 2-len(decimals))
		case len(decimals) > 2:
			decimals = decimals[:2]
		}
		cents, _ := strconv.Atoi(decimals)
		return FromComponents(z, cents)
	}
	if m := reDecimal.FindStringSubmatch(cleaned); m != nil {
		bits, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return Note{}, fmt.Errorf("sharenote: parse bits: %w", err)
		}
		return FromBits(bits)
	}
	return Note{}, fmt.Errorf("sharenote: unrecognised denomination label %q", label)
}

// MustParseLabel is like [ParseLabel] but panics if the label is invalid. It is
// intended for package-level variables and test fixtures with constant labels.
func MustParseLabel(label string) Note {
	n, err := ParseLabel(label)
	if err != nil {
		panic(err)
	}
	return n
}

// FromBits returns the Note whose continuous difficulty is 2**bits. bits must be
// a finite, non-negative number.
func FromBits(bits float64) (Note, error) {
	if !isFinite(bits) {
		return Note{}, errors.New("sharenote: bits must be finite")
	}
	if bits < 0 {
		return Note{}, errors.New("sharenote: bits must be non-negative")
	}
	z, cents := componentsFromBits(bits)
	return Note{bits: bits, diff: exactDifficulty(z, cents, bits)}, nil
}

// FromComponents returns the Note for a whole-Z value z and a centi-bit value
// cents. z must be non-negative; cents is clamped to [MinCents, MaxCents].
func FromComponents(z, cents int) (Note, error) {
	if z < 0 {
		return Note{}, errors.New("sharenote: z must be non-negative")
	}
	c := clampCents(cents)
	bits := float64(z) + float64(c)*CentBitStep
	return Note{bits: bits, diff: exactDifficulty(z, c, bits)}, nil
}

// FromCentBits returns the Note for a total centi-bit count, e.g. 3410 yields
// "34Z10". The count must be non-negative.
func FromCentBits(centBits int) (Note, error) {
	if centBits < 0 {
		return Note{}, errors.New("sharenote: centi-bit value must be non-negative")
	}
	return FromComponents(centBits/centsPerBit, centBits%centsPerBit)
}

// Bits returns the note's exact Z-bit value (log2 of its continuous difficulty).
func (n Note) Bits() float64 { return n.bits }

// Z returns the whole-Z component of the note's label.
func (n Note) Z() int {
	z, _ := componentsFromBits(n.bits)
	return z
}

// Cents returns the centi-bit component of the note's label, in [0, 99].
func (n Note) Cents() int {
	_, c := componentsFromBits(n.bits)
	return c
}

// Label returns the canonical denomination label, e.g. "34Z10". The label is a
// pricing-bucket floor: it declares that the note carries at least that many
// bits of entropy and MUST NOT be used to verify a target (NIP-SN rule A2).
func (n Note) Label() string {
	z, c := componentsFromBits(n.bits)
	return fmt.Sprintf("%dZ%02d", z, c)
}

// String implements [fmt.Stringer], returning the canonical label.
func (n Note) String() string { return n.Label() }

// Difficulty returns the note's exact continuous difficulty D = 2**bits as a
// rational. The returned value is a fresh copy the caller may mutate. This is
// the authoritative quantity for accounting (NIP-SN §Z-Arithmetic); prefer it
// to [Note.DifficultyFloat] whenever values are summed or compared exactly.
func (n Note) Difficulty() *big.Rat {
	return n.difficultyRat()
}

// DifficultyFloat returns the note's continuous difficulty D = 2**bits as a
// float64, for display only. It loses precision for large or fractional notes;
// use [Note.Difficulty] on any accounting path.
func (n Note) DifficultyFloat() float64 { return math.Exp2(n.bits) }

// Probability returns the per-hash success probability 2**(-bits): the chance
// that a single random hash clears the note's target.
func (n Note) Probability() float64 { return math.Exp2(-n.bits) }

// ExpectedHashes returns the mean number of hash attempts needed to produce the
// note, 1 / [Note.Probability].
func (n Note) ExpectedHashes() float64 { return math.Exp2(n.bits) }

// Cmp orders notes by rarity. It returns -1 if n is easier than other, +1 if n
// is harder, and 0 if they share the same label. Notes are compared by whole-Z
// first, then by centi-bits, matching how denomination labels sort.
func (n Note) Cmp(other Note) int {
	nz, nc := componentsFromBits(n.bits)
	oz, oc := componentsFromBits(other.bits)
	switch {
	case nz != oz:
		if nz < oz {
			return -1
		}
		return 1
	case nc < oc:
		return -1
	case nc > oc:
		return 1
	default:
		return 0
	}
}

// Target returns the 256-bit integer target that a hash must not exceed to
// clear this note, the hash-space inverse of its continuous difficulty. It
// errors if the note is too large to fit a 256-bit target.
func (n Note) Target() (*big.Int, error) {
	integerBits := int(math.Floor(n.bits))
	baseExponent := 256 - integerBits
	if baseExponent < 0 {
		return nil, errors.New("sharenote: note too large; target underflow")
	}
	fractional := n.bits - float64(integerBits)
	scale := math.Exp2(-fractional)

	const precisionBits = 48
	scaleFactor := uint64(math.Round(scale * math.Exp2(precisionBits)))
	base := new(big.Int).Lsh(big.NewInt(1), uint(baseExponent))
	result := new(big.Int).Mul(base, new(big.Int).SetUint64(scaleFactor))
	return result.Rsh(result, precisionBits), nil
}

// Compact encodes the note's target in Bitcoin's compact "nBits" form as an
// 8-character hex string, the inverse of [FromCompact].
func (n Note) Compact() (string, error) {
	target, err := n.Target()
	if err != nil {
		return "", err
	}
	if target.Sign() <= 0 {
		return "", errors.New("sharenote: target must be positive")
	}
	compact, err := targetToCompact(target)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x", compact), nil
}

// FromTarget returns the Note for a 256-bit cryptographic target, the inverse of
// [Note.Target]. The target must be a positive integer no greater than 2**256.
func FromTarget(target *big.Int) (Note, error) {
	if target == nil || target.Sign() <= 0 {
		return Note{}, errors.New("sharenote: target must be a positive integer")
	}
	// float64 holds magnitudes up to ~2**1024, so a 256-bit target fits; take
	// its highest mantissa bits via big.Float.
	f := new(big.Float).SetInt(target)
	val, _ := f.Float64()
	if math.IsInf(val, 1) || val == 0 {
		return Note{}, errors.New("sharenote: target out of precision bounds")
	}
	// target = 2**(256 - bits) => bits = 256 - log2(target).
	bits := 256.0 - math.Log2(val)
	if bits < 0 {
		return Note{}, errors.New("sharenote: target exceeds maximum 256-bit entropy")
	}
	return FromBits(bits)
}

// FromCompact returns the Note for a Bitcoin compact "nBits" value, given as an
// 8-character hex string (an optional "0x" prefix is allowed), the inverse of
// [Note.Compact].
func FromCompact(nbits string) (Note, error) {
	cleaned := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(nbits), "0x"))
	if len(cleaned) != 8 {
		return Note{}, errors.New("sharenote: nBits must be 8 hex characters")
	}
	value, err := strconv.ParseUint(cleaned, 16, 32)
	if err != nil {
		return Note{}, fmt.Errorf("sharenote: parse nBits: %w", err)
	}
	exponent := value >> 24
	mantissa := value & 0xFFFFFF
	if mantissa == 0 {
		return Note{}, errors.New("sharenote: nBits mantissa must be non-zero")
	}
	log2Target := math.Log2(float64(mantissa)) + 8*float64(exponent-3)
	return FromBits(256 - log2Target)
}

// --- internal exact-maths helpers (shared with arith.go) ---

// exactDifficulty returns the exact continuous difficulty (2**bits) for the
// given components. Whole-Z notes have a power-of-two difficulty representable
// exactly as 2**z; fractional notes have an irrational difficulty, so the exact
// rational value of the float64 2**bits is used. Keeping the difficulty as a
// *big.Rat lets the aggregation paths sum, subtract, and scale without float64
// accumulation error and round to a label exactly once at the end.
func exactDifficulty(z, cents int, bits float64) *big.Rat {
	if z >= 0 && cents == 0 && bits == float64(z) {
		return new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(z)))
	}
	d := math.Exp2(bits)
	if !isFinite(d) || d <= 0 {
		return new(big.Rat)
	}
	if r := new(big.Rat).SetFloat64(d); r != nil {
		return r
	}
	return new(big.Rat)
}

// difficultyRat returns a mutable copy of the note's exact difficulty, using the
// cached value when present and reconstructing it otherwise.
func (n Note) difficultyRat() *big.Rat {
	if n.diff != nil {
		return new(big.Rat).Set(n.diff)
	}
	z, c := componentsFromBits(n.bits)
	return exactDifficulty(z, c, n.bits)
}

// noteFromRat returns the Note whose exact difficulty is r, carrying r forward
// on the result so later exact arithmetic stays exact. A non-positive r yields
// the zero note.
func noteFromRat(r *big.Rat) (Note, error) {
	if r == nil || r.Sign() <= 0 {
		return FromBits(0)
	}
	bits, err := bitsFromDifficultyRat(r)
	if err != nil {
		return Note{}, err
	}
	n, err := FromBits(bits)
	if err != nil {
		return Note{}, err
	}
	n.diff = new(big.Rat).Set(r)
	return n, nil
}

// bitsFromDifficultyRat converts an exact rational difficulty to float64 Z-bits.
// The difficulty is split into a binary mantissa/exponent (difficulty =
// mant * 2**exp with mant in [0.5, 1)) so that log2(difficulty) = exp +
// log2(mant) and only the small mantissa is converted to float64, avoiding
// overflow for very large but exactly representable difficulties.
func bitsFromDifficultyRat(difficulty *big.Rat) (float64, error) {
	if difficulty == nil || difficulty.Sign() <= 0 {
		return 0, errors.New("sharenote: difficulty must be > 0")
	}
	bf := new(big.Float).SetPrec(256).SetRat(difficulty)
	mant := new(big.Float).SetPrec(256)
	exp := bf.MantExp(mant)
	m, _ := mant.Float64()
	if !isFinite(m) || m <= 0 {
		return 0, errors.New("sharenote: difficulty out of representable range")
	}
	return float64(exp) + math.Log2(m), nil
}

// componentsFromBits derives the (z, cents) label components from a Z-bit value,
// applying the pricing-floor epsilon (+1e-9) and clamping cents to [0, 99].
func componentsFromBits(bits float64) (z, cents int) {
	z = int(math.Floor(bits))
	if z < 0 {
		z = 0
	}
	fractional := bits - float64(z)
	rawCents := int(math.Floor((fractional / CentBitStep) + 1e-9))
	return z, clampCents(rawCents)
}

func targetToCompact(target *big.Int) (uint32, error) {
	if target == nil || target.Sign() <= 0 {
		return 0, errors.New("sharenote: target must be positive")
	}
	bytes := target.Bytes()
	exponent := len(bytes)
	var mantissa uint32
	tmp := new(big.Int).Set(target)
	if exponent <= 3 {
		mantissa = uint32(tmp.Uint64()) << (uint(8 * (3 - exponent)))
	} else {
		mantissa = uint32(new(big.Int).Rsh(tmp, uint(8*(exponent-3))).Uint64())
	}
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}
	if exponent > 255 {
		return 0, errors.New("sharenote: target exponent overflow")
	}
	return uint32(exponent)<<24 | mantissa, nil
}

func clampCents(value int) int {
	if value < MinCents {
		return MinCents
	}
	if value > MaxCents {
		return MaxCents
	}
	return value
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

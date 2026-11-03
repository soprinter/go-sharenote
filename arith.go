package sharenote

import (
	"errors"
	"math/big"
)

// Add returns the exact serial combination of n with others: the notes'
// continuous difficulties are summed in linear space (NIP-SN §Z-Arithmetic),
// not their labels. The sum is accumulated as a [math/big.Rat] and rounded to a
// label exactly once, so the result is independent of input order and free of
// the float64 drift a running total would introduce. Called with no arguments
// it returns n unchanged.
//
// For example 1Z00 + 1Z00 == 2Z00 (difficulty 2 + 2 == 4), never "2Z00" by
// concatenation.
func (n Note) Add(others ...Note) (Note, error) {
	total := n.difficultyRat()
	for _, o := range others {
		total.Add(total, o.difficultyRat())
	}
	return noteFromRat(total)
}

// Sum returns the exact serial combination of all notes, as [Note.Add] does. It
// errors if notes is empty.
func Sum(notes ...Note) (Note, error) {
	if len(notes) == 0 {
		return Note{}, errors.New("sharenote: Sum needs at least one note")
	}
	return notes[0].Add(notes[1:]...)
}

// Sub returns the note whose continuous difficulty is n's minus other's, using
// exact rational arithmetic. The result is clamped to the zero note when the
// difference is zero or negative.
func (n Note) Sub(other Note) (Note, error) {
	diff := new(big.Rat).Sub(n.difficultyRat(), other.difficultyRat())
	return noteFromRat(diff)
}

// Scale returns the note whose continuous difficulty is n's multiplied by
// factor, using exact rational arithmetic. factor must be finite and
// non-negative. A factor of 0 yields the zero note. Scale does not clamp:
// driving the difficulty below one hash (negative Z-bits) is outside the
// representable range and is reported as an error rather than snapped to 0Z00.
func (n Note) Scale(factor float64) (Note, error) {
	if !isFinite(factor) {
		return Note{}, errors.New("sharenote: factor must be finite")
	}
	if factor < 0 {
		return Note{}, errors.New("sharenote: factor must be >= 0")
	}
	if factor == 0 {
		return FromBits(0)
	}
	fr := new(big.Rat).SetFloat64(factor)
	if fr == nil {
		return Note{}, errors.New("sharenote: factor must be finite")
	}
	scaled := new(big.Rat).Mul(n.difficultyRat(), fr)
	return noteFromRat(scaled)
}

// Ratio returns the ratio of n's continuous difficulty to other's, computed
// with exact rational arithmetic and rounded to float64 once. It errors if
// other has non-positive difficulty.
func (n Note) Ratio(other Note) (float64, error) {
	den := other.difficultyRat()
	if den.Sign() <= 0 {
		return 0, errors.New("sharenote: division by zero-difficulty note")
	}
	ratio, _ := new(big.Rat).Quo(n.difficultyRat(), den).Float64()
	return ratio, nil
}

// Work is an exact accumulator of proof-of-work in linear difficulty space,
// backed by [math/big.Rat]. It is the right tool for summing many shares — a
// block's whole pool total, or one address's slice — without the float64 drift
// that NIP-SN rule A1 forbids on accounting paths.
//
// The zero Work is an empty accumulator (difficulty 0). Add notes with
// [Work.Add], recover a denomination with [Work.Note], and slice a reward with
// [Work.Payout].
type Work struct {
	r big.Rat
}

// Add adds note's exact continuous difficulty to the accumulator and returns the
// receiver, so calls can be chained.
func (w *Work) Add(note Note) *Work {
	w.r.Add(&w.r, note.difficultyRat())
	return w
}

// AddRat adds an exact difficulty r to the accumulator and returns the receiver.
func (w *Work) AddRat(r *big.Rat) *Work {
	if r != nil {
		w.r.Add(&w.r, r)
	}
	return w
}

// Rat returns the accumulated difficulty as a fresh, mutable rational.
func (w *Work) Rat() *big.Rat {
	return new(big.Rat).Set(&w.r)
}

// IsZero reports whether no (positive) work has been accumulated.
func (w *Work) IsZero() bool { return w.r.Sign() <= 0 }

// Note converts the accumulated work into its denomination. An empty
// accumulator yields the zero note.
func (w *Work) Note() (Note, error) {
	return noteFromRat(w.Rat())
}

// Payout returns this accumulator's exact proportional slice of amount under the
// NIP-SN payout law
//
//	payout = amount * (this work / total work)
//
// computed with exact rational arithmetic and floored to a whole number of the
// coin's base unit (its smallest unit; satoshis on Bitcoin, for example). total
// must be a superset of this work and carry positive difficulty. Flooring every
// recipient's slice guarantees the slices never exceed the invoice total,
// satisfying the share-sum conservation rule (A11).
func (w *Work) Payout(total *Work, amount *big.Int) (*big.Int, error) {
	if amount == nil {
		return nil, errors.New("sharenote: amount must not be nil")
	}
	if total == nil || total.r.Sign() <= 0 {
		return nil, errors.New("sharenote: total work must be positive")
	}
	// share = amount * w / total, as an exact rational.
	share := new(big.Rat).SetInt(amount)
	share.Mul(share, &w.r)
	share.Quo(share, &total.r)
	// Floor to an integer: Num / Denom truncates toward zero, and shares are
	// non-negative, so that is the floor.
	out := new(big.Int).Quo(share.Num(), share.Denom())
	return out, nil
}

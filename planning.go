package sharenote

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Reliability is a Poisson planning preset: how confident a miner wants to be of
// producing a note within a time window. Because the time to find proof-of-work
// is exponentially distributed, reaching confidence c within a window needs
// -ln(1-c) times the mean hashrate; Multiplier is that factor. The mean case
// (confidence 0) has Multiplier 1.
type Reliability struct {
	// Name is a short human label, e.g. "Often (95%)".
	Name string
	// Confidence is the target probability in [0,1); 0 marks the mean case.
	Confidence float64
	// Multiplier is the hashrate factor -ln(1-Confidence) (1 for the mean).
	Multiplier float64
}

// The predefined reliability presets, from the mean to near-certainty.
var (
	ReliabilityMean          = Reliability{Name: "On average", Confidence: 0, Multiplier: 1}
	ReliabilityUsually       = Reliability{Name: "Usually (90%)", Confidence: 0.90, Multiplier: 2.302585092994046}
	ReliabilityOften         = Reliability{Name: "Often (95%)", Confidence: 0.95, Multiplier: 2.995732273553991}
	ReliabilityVeryLikely    = Reliability{Name: "Very likely (99%)", Confidence: 0.99, Multiplier: 4.605170185988092}
	ReliabilityAlmostCertain = Reliability{Name: "Almost certain (99.9%)", Confidence: 0.999, Multiplier: 6.907755278982137}
)

// Reliabilities returns the predefined reliability presets in ascending order of
// confidence.
func Reliabilities() []Reliability {
	return []Reliability{
		ReliabilityMean,
		ReliabilityUsually,
		ReliabilityOften,
		ReliabilityVeryLikely,
		ReliabilityAlmostCertain,
	}
}

// PlanOption customises the Poisson multiplier used by [Note.RequiredHashrate],
// [FromHashrate], and [Note.Estimate]. The zero options (none supplied) plan for
// the mean, i.e. multiplier 1.
type PlanOption func(*planConfig)

type planConfig struct {
	multiplier float64
	confidence float64 // 0 means "mean"
}

// WithMultiplier sets the Poisson multiplier directly. It must be positive.
func WithMultiplier(m float64) PlanOption {
	return func(c *planConfig) {
		c.multiplier = m
		c.confidence = 0
	}
}

// WithConfidence plans for a target confidence in the open interval (0,1),
// setting the multiplier to -ln(1-confidence). Values outside (0,1) are ignored.
func WithConfidence(confidence float64) PlanOption {
	return func(c *planConfig) {
		if confidence <= 0 || confidence >= 1 {
			return
		}
		c.multiplier = -math.Log(1 - confidence)
		c.confidence = confidence
	}
}

// WithReliability plans for one of the [Reliability] presets.
func WithReliability(r Reliability) PlanOption {
	return func(c *planConfig) {
		c.multiplier = r.Multiplier
		c.confidence = r.Confidence
	}
}

func resolvePlan(opts ...PlanOption) (planConfig, error) {
	cfg := planConfig{multiplier: 1}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if !isFinite(cfg.multiplier) || cfg.multiplier <= 0 {
		return planConfig{}, errors.New("sharenote: multiplier must be > 0")
	}
	return cfg, nil
}

func windowSeconds(window time.Duration) (float64, error) {
	s := window.Seconds()
	if !isFinite(s) || s <= 0 {
		return 0, errors.New("sharenote: window must be > 0")
	}
	return s, nil
}

// RequiredHashrate returns the hash rate, in hashes per second, needed to
// produce the note within window. By default it plans for the mean; pass
// [WithConfidence], [WithReliability], or [WithMultiplier] to plan for a
// confidence level. The value is multiplier * expected_hashes / seconds.
func (n Note) RequiredHashrate(window time.Duration, opts ...PlanOption) (float64, error) {
	seconds, err := windowSeconds(window)
	if err != nil {
		return 0, err
	}
	cfg, err := resolvePlan(opts...)
	if err != nil {
		return 0, err
	}
	return n.ExpectedHashes() * cfg.multiplier / seconds, nil
}

// MaxZBitsForHashrate returns the largest Z-bit value a rig of the given hash
// rate (hashes per second) can expect to reach within window:
// bits = log2(hashrate * seconds / multiplier). Pass planning options to size
// for a confidence level rather than the mean. It is the continuous building
// block behind [FromHashrate].
func MaxZBitsForHashrate(hashrate float64, window time.Duration, opts ...PlanOption) (float64, error) {
	if !isFinite(hashrate) || hashrate <= 0 {
		return 0, errors.New("sharenote: hashrate must be > 0")
	}
	seconds, err := windowSeconds(window)
	if err != nil {
		return 0, err
	}
	cfg, err := resolvePlan(opts...)
	if err != nil {
		return 0, err
	}
	return math.Log2(hashrate * seconds / cfg.multiplier), nil
}

// FromHashrate returns the hardest note a rig of the given hash rate (hashes per
// second) can expect to produce within window. It is the inverse of
// [Note.RequiredHashrate]: bits = log2(hashrate * seconds / multiplier). Pass
// planning options to size for a confidence level rather than the mean.
func FromHashrate(hashrate float64, window time.Duration, opts ...PlanOption) (Note, error) {
	bits, err := MaxZBitsForHashrate(hashrate, window, opts...)
	if err != nil {
		return Note{}, err
	}
	return FromBits(bits)
}

// HashrateRange is an inclusive hash-rate band, in hashes per second, whose
// notes all round to the same denomination label. Min is the rate needed for
// the note itself; Max is the rate needed for the next centi-bit up.
type HashrateRange struct {
	// Min is the lower bound (H/s) — the rate that produces the note's label.
	Min float64
	// Max is the upper bound (H/s) — the rate at one centi-bit above the note.
	Max float64
}

// HashrateRangeForNote returns the [HashrateRange] of hash rates that all map to
// n's denomination label within window, under the chosen planning options. It
// mirrors sharenotejs's hashrateRange and sharenotelib's hashrate_range.
func HashrateRangeForNote(n Note, window time.Duration, opts ...PlanOption) (HashrateRange, error) {
	lower, err := n.RequiredHashrate(window, opts...)
	if err != nil {
		return HashrateRange{}, err
	}
	next, err := FromBits(n.Bits() + CentBitStep)
	if err != nil {
		return HashrateRange{}, err
	}
	upper, err := next.RequiredHashrate(window, opts...)
	if err != nil {
		return HashrateRange{}, err
	}
	return HashrateRange{Min: lower, Max: upper}, nil
}

// Estimate is a planning report for minting a note within a time window: the
// per-hash probability, the mean number of hashes, and the hash rate required
// both on average and at the chosen confidence level. All fields are float64,
// for display and capacity planning only.
type Estimate struct {
	// Note is the denomination being planned for.
	Note Note
	// Window is the target time to produce the note.
	Window time.Duration
	// Probability is the per-hash success probability, 2**(-bits).
	Probability float64
	// ExpectedHashes is the mean number of hashes needed.
	ExpectedHashes float64
	// RequiredHashrateMean is the mean hash rate (H/s) to hit the window.
	RequiredHashrateMean float64
	// RequiredHashrate is the hash rate (H/s) at the requested confidence; it
	// equals RequiredHashrateMean when planning for the mean.
	RequiredHashrate float64
	// Multiplier is the Poisson multiplier applied (1 for the mean).
	Multiplier float64
	// Confidence is the target confidence in (0,1), or 0 for the mean.
	Confidence float64
}

// String implements [fmt.Stringer] with a compact one-line summary.
func (e Estimate) String() string {
	return fmt.Sprintf("Estimate{%s @ %s, p=%s, need %s}",
		e.Note, e.Window, FormatProbability(e.Note.Bits(), 8), FormatHashrate(e.RequiredHashrate))
}

// Estimate builds an [Estimate] for producing the note within window. By default
// the required hash rate is the mean; pass a planning option to size it for a
// confidence level.
func (n Note) Estimate(window time.Duration, opts ...PlanOption) (Estimate, error) {
	seconds, err := windowSeconds(window)
	if err != nil {
		return Estimate{}, err
	}
	cfg, err := resolvePlan(opts...)
	if err != nil {
		return Estimate{}, err
	}
	expected := n.ExpectedHashes()
	return Estimate{
		Note:                 n,
		Window:               window,
		Probability:          n.Probability(),
		ExpectedHashes:       expected,
		RequiredHashrateMean: expected / seconds,
		RequiredHashrate:     expected * cfg.multiplier / seconds,
		Multiplier:           cfg.multiplier,
		Confidence:           cfg.confidence,
	}, nil
}

// EstimateNotes builds an [Estimate] for each note over the same window and
// planning options — a batch of [Note.Estimate]. It mirrors sharenotejs's
// estimateMany and sharenotelib's estimate_many.
func EstimateNotes(notes []Note, window time.Duration, opts ...PlanOption) ([]Estimate, error) {
	out := make([]Estimate, 0, len(notes))
	for _, n := range notes {
		est, err := n.Estimate(window, opts...)
		if err != nil {
			return nil, err
		}
		out = append(out, est)
	}
	return out, nil
}

// Plan is the note a given rig can mint within a window, with its full
// [Estimate] and the input hash rate it was planned from. It pairs
// [PlanSharenoteFromHashrate]'s inputs with its derived note.
type Plan struct {
	// Note is the denomination the rig can expect to produce.
	Note Note
	// Estimate is the full planning report for Note over the window.
	Estimate Estimate
	// Window is the target time to produce the note.
	Window time.Duration
	// InputHashrate is the rig's hash rate (H/s) the plan was built from.
	InputHashrate float64
}

// PlanSharenoteFromHashrate plans the note a rig of the given hash rate can mint
// within window and returns it with a full [Estimate]. It mirrors sharenotejs's
// planFromHashrate and sharenotelib's plan_from_hashrate.
func PlanSharenoteFromHashrate(hashrate float64, window time.Duration, opts ...PlanOption) (Plan, error) {
	note, err := FromHashrate(hashrate, window, opts...)
	if err != nil {
		return Plan{}, err
	}
	est, err := note.Estimate(window, opts...)
	if err != nil {
		return Plan{}, err
	}
	return Plan{Note: note, Estimate: est, Window: window, InputHashrate: hashrate}, nil
}

// FormatProbability renders a note's per-hash probability as "1 / 2^bits" with
// the given number of fractional digits, e.g. FormatProbability(33.53, 2) is
// "1 / 2^33.53".
func FormatProbability(bits float64, digits int) string {
	if digits < 0 {
		digits = 0
	}
	return fmt.Sprintf("1 / 2^%.*f", digits, bits)
}

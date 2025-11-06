// Package sharenote implements SNIP-00 — Core Z Arithmetic and Hashrate Conversion.
package snip00

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// ImplementationMeta summarises a SNIP implementation.
type ImplementationMeta struct {
	ID            string
	Title         string
	Status        string
	Summary       string
	Specification string
}

// SNIP0000Implementation exposes metadata about this module's specification coverage.
var SNIP0000Implementation = ImplementationMeta{
	ID:            "SNIP-0000",
	Title:         "Core Z Arithmetic and Hashrate Conversion",
	Status:        "stable",
	Summary:       "Implements canonical note encoding, probability maths, and hashrate planning for Sharenote proofs.",
	Specification: "../sharenote-snip.md",
}

const (
	// CentBitStep is the per-cent fractional bit increment.
	CentBitStep            = 0.01
	ContinuousExponentStep = CentBitStep // backwards compatibility alias
	MinCents               = 0
	MaxCents               = 99
)

// ReliabilityID enumerates the supported reliability presets.
type ReliabilityID string

const (
	ReliabilityMean         ReliabilityID = "mean"
	ReliabilityUsually90    ReliabilityID = "usually_90"
	ReliabilityOften95      ReliabilityID = "often_95"
	ReliabilityVeryLikely99 ReliabilityID = "very_likely_99"
	ReliabilityAlmost999    ReliabilityID = "almost_999"
)

// PrimaryMode indicates whether bill estimates prioritise mean or quantile values.
type PrimaryMode string

const (
	PrimaryModeMean     PrimaryMode = "mean"
	PrimaryModeQuantile PrimaryMode = "quantile"
)

// HashrateUnit represents canonical hashrate units.
type HashrateUnit string

const (
	HashrateUnitHps  HashrateUnit = "H/s"
	HashrateUnitKHps HashrateUnit = "kH/s"
	HashrateUnitMHps HashrateUnit = "MH/s"
	HashrateUnitGHps HashrateUnit = "GH/s"
	HashrateUnitTHps HashrateUnit = "TH/s"
	HashrateUnitPHps HashrateUnit = "PH/s"
	HashrateUnitEHps HashrateUnit = "EH/s"
	HashrateUnitZHps HashrateUnit = "ZH/s"
)

// HashrateValue captures a numeric magnitude plus an optional canonical unit.
type HashrateValue struct {
	Value float64
	Unit  HashrateUnit
}

// ReliabilityLevel provides Poisson multiplier presets for time-to-success planning.
type ReliabilityLevel struct {
	ID         ReliabilityID
	Label      string
	Confidence *float64
	Multiplier float64
}

// Sharenote describes a note label using integer bits and cent-Z fraction.
type Sharenote struct {
	Z             int
	Cents         int
	Bits          float64
	labelOverride string
}

// HumanHashrate represents a human-readable H/s value plus metadata.
type HumanHashrate struct {
	Value    float64
	Unit     HashrateUnit
	Display  string
	Exponent int
}

// BillEstimate summarises the metrics required to mint a note within a time window.
type BillEstimate struct {
	Sharenote                Sharenote
	Label                    string
	Bits                     float64
	SecondsTarget            float64
	ProbabilityPerHash       float64
	ProbabilityDisplay       string
	ExpectedHashes           float64
	RequiredHashrateMean     float64
	RequiredHashrateQuantile float64
	RequiredHashratePrimary  float64
	RequiredHashrateHuman    HumanHashrate
	Multiplier               float64
	Quantile                 *float64
	PrimaryMode              PrimaryMode
}

// SharenotePlan summarises a computed note and its supporting bill estimate for a given rig.
type SharenotePlan struct {
	Sharenote          Sharenote
	Bill               BillEstimate
	SecondsTarget      float64
	InputHashrateHPS   float64
	InputHashrateHuman HumanHashrate
}

// Label returns the canonical Sharenote label (e.g. "33Z53").
func (n Sharenote) Label() string {
	if n.labelOverride != "" {
		return n.labelOverride
	}
	return fmt.Sprintf("%dZ%02d", n.Z, clampCents(n.Cents))
}

var reliabilityLevels = map[ReliabilityID]ReliabilityLevel{
	ReliabilityMean: {
		ID:         ReliabilityMean,
		Label:      "On average",
		Confidence: nil,
		Multiplier: 1,
	},
	ReliabilityUsually90: {
		ID:         ReliabilityUsually90,
		Label:      "Usually (90%)",
		Confidence: floatPtr(0.90),
		Multiplier: 2.302585092994046,
	},
	ReliabilityOften95: {
		ID:         ReliabilityOften95,
		Label:      "Often (95%)",
		Confidence: floatPtr(0.95),
		Multiplier: 2.995732273553991,
	},
	ReliabilityVeryLikely99: {
		ID:         ReliabilityVeryLikely99,
		Label:      "Very likely (99%)",
		Confidence: floatPtr(0.99),
		Multiplier: 4.605170185988092,
	},
	ReliabilityAlmost999: {
		ID:         ReliabilityAlmost999,
		Label:      "Almost certain (99.9%)",
		Confidence: floatPtr(0.999),
		Multiplier: 6.907755278982137,
	},
}

var hashrateUnits = []struct {
	unit     HashrateUnit
	exponent int
}{
	{HashrateUnitHps, 0},
	{HashrateUnitKHps, 1},
	{HashrateUnitMHps, 2},
	{HashrateUnitGHps, 3},
	{HashrateUnitTHps, 4},
	{HashrateUnitPHps, 5},
	{HashrateUnitEHps, 6},
	{HashrateUnitZHps, 7},
}

var (
	reDecimal             = regexp.MustCompile(`^(\d+(?:\.\d+)?)Z$`)
	reStandard            = regexp.MustCompile(`^(\d+)Z(?:(\d{1,2})(?:CZ)?)?$`)
	reDotted              = regexp.MustCompile(`^(\d+)\.(\d{1,2})Z$`)
	hashrateStringPattern = regexp.MustCompile(`^([+-]?(?:\d+(?:[_,]?\d+)*(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)\s*([A-Za-z\/\s-]+)?$`)
	hashrateUnitPattern   = regexp.MustCompile(`^([KMGTPEZ]?)(H)/S$`)
)

var hashratePrefixExponent = map[string]int{
	"":  0,
	"K": 1,
	"M": 2,
	"G": 3,
	"T": 4,
	"P": 5,
	"E": 6,
	"Z": 7,
}

var prefixToUnit = map[string]HashrateUnit{
	"":  HashrateUnitHps,
	"K": HashrateUnitKHps,
	"M": HashrateUnitMHps,
	"G": HashrateUnitGHps,
	"T": HashrateUnitTHps,
	"P": HashrateUnitPHps,
	"E": HashrateUnitEHps,
	"Z": HashrateUnitZHps,
}

func getReliabilityLevel(id ReliabilityID) (ReliabilityLevel, error) {
	if lvl, ok := reliabilityLevels[id]; ok {
		return lvl, nil
	}
	return ReliabilityLevel{}, fmt.Errorf("unknown reliability level: %s", id)
}

func normalizeHashrateUnitString(raw string) string {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	replacer := strings.NewReplacer(
		"-", "",
		"_", "",
		" ", "",
	)
	normalized = replacer.Replace(normalized)
	normalized = strings.ReplaceAll(normalized, "HPS", "H/S")
	normalized = strings.ReplaceAll(normalized, "HS", "H/S")
	if !strings.HasSuffix(normalized, "/S") && strings.Contains(normalized, "H") {
		normalized += "/S"
	}
	normalized = strings.ReplaceAll(normalized, "/S/S", "/S")
	return normalized
}

func resolveHashrateUnit(unit string) (int, HashrateUnit, error) {
	trimmed := strings.TrimSpace(unit)
	if trimmed == "" {
		return 0, HashrateUnitHps, nil
	}
	normalized := normalizeHashrateUnitString(trimmed)
	match := hashrateUnitPattern.FindStringSubmatch(normalized)
	if match == nil {
		return 0, "", fmt.Errorf("unrecognised hashrate unit: %q", unit)
	}
	prefix := match[1]
	exponent, ok := hashratePrefixExponent[prefix]
	if !ok {
		return 0, "", fmt.Errorf("unsupported hashrate prefix: %q", prefix)
	}
	canonical := prefixToUnit[prefix]
	return exponent, canonical, nil
}

// NormalizeHashrateValue converts a HashrateValue into H/s.
func NormalizeHashrateValue(value HashrateValue) (float64, error) {
	if !isFinite(value.Value) {
		return 0, errors.New("hashrate value must be finite")
	}
	if value.Value < 0 {
		return 0, errors.New("hashrate must be >= 0")
	}
	unit := value.Unit
	if unit == "" {
		unit = HashrateUnitHps
	}
	exponent, _, err := resolveHashrateUnit(string(unit))
	if err != nil {
		return 0, err
	}
	return value.Value * math.Pow(10, float64(exponent*3)), nil
}

// ParseHashrate accepts human-readable strings (e.g. "5 GH/s") and returns H/s.
func ParseHashrate(input string) (float64, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, errors.New("hashrate string must not be empty")
	}
	match := hashrateStringPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return 0, fmt.Errorf("unrecognised hashrate format: %q", input)
	}
	magnitudeStr := strings.NewReplacer("_", "", ",", "").Replace(match[1])
	value, err := strconv.ParseFloat(magnitudeStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse hashrate magnitude: %w", err)
	}
	if !isFinite(value) {
		return 0, errors.New("hashrate magnitude must be finite")
	}
	if value < 0 {
		return 0, errors.New("hashrate must be >= 0")
	}
	unitRaw := ""
	if len(match) > 2 {
		unitRaw = strings.TrimSpace(match[2])
	}
	exponent, _, err := resolveHashrateUnit(unitRaw)
	if err != nil {
		return 0, err
	}
	return value * math.Pow(10, float64(exponent*3)), nil
}

// ParseLabel converts textual labels (33Z53, 33.53Z, 33Z 53CZ) into a Sharenote.
func ParseLabel(label string) (Sharenote, error) {
	cleaned := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(label), " ", ""))

	if match := reStandard.FindStringSubmatch(cleaned); match != nil {
		z, _ := strconv.Atoi(match[1])
		cents := 0
		if match[2] != "" {
			cents, _ = strconv.Atoi(match[2])
		}
		return NoteFromComponents(z, cents)
	}

	if match := reDotted.FindStringSubmatch(cleaned); match != nil {
		z, _ := strconv.Atoi(match[1])
		decimals := match[2]
		if len(decimals) < 2 {
			decimals = decimals + strings.Repeat("0", 2-len(decimals))
		} else if len(decimals) > 2 {
			decimals = decimals[:2]
		}
		cents, _ := strconv.Atoi(decimals)
		return NoteFromComponents(z, cents)
	}

	if match := reDecimal.FindStringSubmatch(cleaned); match != nil {
		bits, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return Sharenote{}, fmt.Errorf("parse bits: %w", err)
		}
		return NoteFromBits(bits)
	}

	return Sharenote{}, fmt.Errorf("unrecognised Sharenote label %q", label)
}

// MustParseLabel wraps ParseLabel and panics on failure. Convenient for tests.
func MustParseLabel(label string) Sharenote {
	note, err := ParseLabel(label)
	if err != nil {
		panic(err)
	}
	return note
}

// NoteFromComponents normalises (Z, cents) into a Sharenote struct.
func NoteFromComponents(z, cents int) (Sharenote, error) {
	if z < 0 {
		return Sharenote{}, errors.New("z must be non-negative")
	}
	c := clampCents(cents)
	bits := float64(z) + float64(c)*CentBitStep
	return Sharenote{Z: z, Cents: c, Bits: bits}, nil
}

// NoteFromBits converts fractional bit difficulty to a Sharenote.
func NoteFromBits(bits float64) (Sharenote, error) {
	if !isFinite(bits) {
		return Sharenote{}, errors.New("bits must be finite")
	}
	if bits < 0 {
		return Sharenote{}, errors.New("bits must be non-negative")
	}
	z := int(math.Floor(bits))
	fractional := bits - float64(z)
	rawCents := int((fractional / CentBitStep) + 1e-9)
	cents := clampCents(rawCents)
	return NoteFromComponents(z, cents)
}

// EnsureNote accepts either a Sharenote or label string and returns the struct.
func EnsureNote(input any) (Sharenote, error) {
	switch v := input.(type) {
	case Sharenote:
		return v, nil
	case *Sharenote:
		if v == nil {
			return Sharenote{}, errors.New("nil note pointer")
		}
		return *v, nil
	case string:
		return ParseLabel(v)
	default:
		return Sharenote{}, fmt.Errorf("unsupported note input %T", v)
	}
}

// ProbabilityFromBits returns 2^(-bits).
func ProbabilityFromBits(bits float64) (float64, error) {
	if !isFinite(bits) {
		return 0, errors.New("bits must be finite")
	}
	return math.Exp2(-bits), nil
}

// ProbabilityPerHash returns the per-hash success probability for the note.
func ProbabilityPerHash(note any) (float64, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return 0, err
	}
	return math.Exp2(-resolved.Bits), nil
}

func difficultyFromBits(bits float64) float64 {
	return math.Exp2(bits)
}

func difficultyFromNote(note any) (float64, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return 0, err
	}
	return math.Exp2(resolved.Bits), nil
}

func bitsFromDifficulty(difficulty float64) (float64, error) {
	if !isFinite(difficulty) || difficulty <= 0 {
		return 0, errors.New("difficulty must be > 0")
	}
	return math.Log2(difficulty), nil
}

// ExpectedHashes returns 1 / probability.
func ExpectedHashes(bits float64) (float64, error) {
	p, err := ProbabilityFromBits(bits)
	if err != nil {
		return 0, err
	}
	return 1 / p, nil
}

// ExpectedHashesForNote returns expected hashes for converting the given note.
func ExpectedHashesForNote(note any) (float64, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return 0, err
	}
	return ExpectedHashes(resolved.Bits)
}

// RequiredHashrate computes multiplier * expected_hashes / seconds.
func RequiredHashrate(note any, seconds float64, opts ...HashrateOption) (float64, error) {
	if !isFinite(seconds) || seconds <= 0 {
		return 0, errors.New("seconds must be > 0")
	}
	cfg := hashrateOptions{multiplier: 1}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.multiplier <= 0 {
		return 0, errors.New("multiplier must be > 0")
	}
	resolved, err := EnsureNote(note)
	if err != nil {
		return 0, err
	}
	expected, err := ExpectedHashes(resolved.Bits)
	if err != nil {
		return 0, err
	}
	return expected * cfg.multiplier / seconds, nil
}

// RequiredHashrateMean returns the mean hashrate.
func RequiredHashrateMean(note any, seconds float64) (float64, error) {
	return RequiredHashrate(note, seconds)
}

// RequiredHashrateQuantile returns the quantile hashrate for the provided confidence.
func RequiredHashrateQuantile(note any, seconds, confidence float64) (float64, error) {
	if confidence <= 0 || confidence >= 1 {
		return 0, errors.New("confidence must be in (0,1)")
	}
	multiplier := -math.Log(1 - confidence)
	return RequiredHashrate(note, seconds, WithMultiplier(multiplier))
}

// MaxBitsForHashrate computes the maximum bit difficulty achievable with the provided parameters.
func MaxBitsForHashrate(hashrate, seconds, multiplier float64) (float64, error) {
	if !isFinite(hashrate) || hashrate <= 0 {
		return 0, errors.New("hashrate must be > 0")
	}
	if !isFinite(seconds) || seconds <= 0 {
		return 0, errors.New("seconds must be > 0")
	}
	if !isFinite(multiplier) || multiplier <= 0 {
		return 0, errors.New("multiplier must be > 0")
	}
	return math.Log2(hashrate * seconds / multiplier), nil
}

// NoteFromHashrate inverts RequiredHashrate using a structured hashrate input.
func NoteFromHashrate(hashrate HashrateValue, seconds float64, opts ...HashrateOption) (Sharenote, error) {
	numeric, err := NormalizeHashrateValue(hashrate)
	if err != nil {
		return Sharenote{}, err
	}
	cfg := hashrateOptions{multiplier: 1}
	for _, opt := range opts {
		opt(&cfg)
	}
	bits, err := MaxBitsForHashrate(numeric, seconds, cfg.multiplier)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromBits(bits)
}

// TargetFor returns the integer hash target for the note.
func TargetFor(note any) (*big.Int, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return nil, err
	}
	integerBits := int(math.Floor(resolved.Bits))
	baseExponent := 256 - integerBits
	if baseExponent < 0 {
		return nil, errors.New("z too large; target underflow")
	}
	fractional := resolved.Bits - float64(integerBits)
	scale := math.Exp2(-fractional)

	const precisionBits = 48
	scaleFactor := uint64(math.Round(scale * math.Exp2(precisionBits)))
	base := new(big.Int).Lsh(big.NewInt(1), uint(baseExponent))
	result := new(big.Int).Mul(base, new(big.Int).SetUint64(scaleFactor))
	return result.Rsh(result, precisionBits), nil
}

// CompareNotes orders notes by rarity (higher Z first, then cents).
func CompareNotes(a, b any) (int, error) {
	noteA, err := EnsureNote(a)
	if err != nil {
		return 0, err
	}
	noteB, err := EnsureNote(b)
	if err != nil {
		return 0, err
	}
	if noteA.Z != noteB.Z {
		if noteA.Z < noteB.Z {
			return -1, nil
		}
		return 1, nil
	}
	switch {
	case noteA.Cents < noteB.Cents:
		return -1, nil
	case noteA.Cents > noteB.Cents:
		return 1, nil
	default:
		return 0, nil
	}
}

// NBitsToSharenote converts compact Bitcoin difficulty to a Sharenote.
func NBitsToSharenote(hex string) (Sharenote, error) {
	cleaned := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hex), "0x"))
	if len(cleaned) != 8 {
		return Sharenote{}, errors.New("nBits must be 8 hex characters")
	}
	value, err := strconv.ParseUint(cleaned, 16, 32)
	if err != nil {
		return Sharenote{}, fmt.Errorf("parse nBits: %w", err)
	}
	exponent := value >> 24
	mantissa := value & 0xFFFFFF
	if mantissa == 0 {
		return Sharenote{}, errors.New("mantissa must be non-zero")
	}
	log2Target := math.Log2(float64(mantissa)) + 8*float64(exponent-3)
	bits := 256 - log2Target
	return NoteFromBits(bits)
}

// ReliabilityLevels returns all predefined reliability presets.
func ReliabilityLevels() []ReliabilityLevel {
	return []ReliabilityLevel{
		reliabilityLevels[ReliabilityMean],
		reliabilityLevels[ReliabilityUsually90],
		reliabilityLevels[ReliabilityOften95],
		reliabilityLevels[ReliabilityVeryLikely99],
		reliabilityLevels[ReliabilityAlmost999],
	}
}

// FormatProbabilityDisplay returns strings like "1 / 2^33.00000000".
func FormatProbabilityDisplay(bits float64, precision int) string {
	if precision < 0 {
		precision = 0
	}
	return fmt.Sprintf("1 / 2^%.*f", precision, bits)
}

// HumaniseHashrate renders a hashrate into an appropriate SI-prefixed unit.
func HumaniseHashrate(hashrate float64) HumanHashrate {
	if !isFinite(hashrate) || hashrate <= 0 {
		return HumanHashrate{Value: 0, Unit: HashrateUnitHps, Display: "0 H/s", Exponent: 0}
	}
	logValue := math.Log10(hashrate)
	index := int(math.Floor(logValue / 3))
	if index < 0 {
		index = 0
	}
	if index >= len(hashrateUnits) {
		index = len(hashrateUnits) - 1
	}
	unit := hashrateUnits[index]
	scaled := hashrate / math.Pow(10, float64(unit.exponent*3))
	if !isFinite(scaled) {
		scaled = hashrate
	}

	var display string
	switch {
	case scaled >= 100:
		display = fmt.Sprintf("%.0f %s", scaled, unit.unit)
	case scaled >= 10:
		display = fmt.Sprintf("%.1f %s", scaled, unit.unit)
	default:
		display = fmt.Sprintf("%.2f %s", scaled, unit.unit)
	}
	return HumanHashrate{
		Value:    scaled,
		Unit:     unit.unit,
		Display:  display,
		Exponent: unit.exponent,
	}
}

// EstimateOption configures EstimateNote.
type EstimateOption func(*estimateOptions)

type estimateOptions struct {
	multiplier           float64
	quantile             *float64
	primaryMode          PrimaryMode
	probabilityPrecision int
}

func defaultEstimateOptions() estimateOptions {
	return estimateOptions{
		multiplier:           1,
		quantile:             nil,
		primaryMode:          "",
		probabilityPrecision: 8,
	}
}

// WithEstimateMultiplier overrides the Poisson multiplier directly.
func WithEstimateMultiplier(multiplier float64) EstimateOption {
	return func(cfg *estimateOptions) {
		cfg.multiplier = multiplier
		cfg.quantile = nil
	}
}

// WithEstimateReliability selects a preset reliability level.
func WithEstimateReliability(id ReliabilityID) EstimateOption {
	return func(cfg *estimateOptions) {
		if lvl, ok := reliabilityLevels[id]; ok {
			cfg.multiplier = lvl.Multiplier
			cfg.quantile = lvl.Confidence
		}
	}
}

// WithEstimateConfidence configures a raw quantile in (0,1).
func WithEstimateConfidence(confidence float64) EstimateOption {
	return func(cfg *estimateOptions) {
		if confidence <= 0 || confidence >= 1 {
			return
		}
		cfg.multiplier = -math.Log(1 - confidence)
		cfg.quantile = &confidence
	}
}

// WithEstimatePrimaryMode sets the preferred primary mode ("mean" or "quantile").
func WithEstimatePrimaryMode(mode PrimaryMode) EstimateOption {
	return func(cfg *estimateOptions) {
		switch mode {
		case PrimaryModeMean, PrimaryModeQuantile:
			cfg.primaryMode = mode
		}
	}
}

// WithEstimateProbabilityPrecision adjusts the probability display precision.
func WithEstimateProbabilityPrecision(precision int) EstimateOption {
	return func(cfg *estimateOptions) {
		if precision < 0 {
			precision = 0
		}
		cfg.probabilityPrecision = precision
	}
}

// EstimateNote computes a BillEstimate for the provided note and window.
func EstimateNote(note any, seconds float64, opts ...EstimateOption) (BillEstimate, error) {
	if !isFinite(seconds) || seconds <= 0 {
		return BillEstimate{}, errors.New("seconds must be > 0")
	}

	resolved, err := EnsureNote(note)
	if err != nil {
		return BillEstimate{}, err
	}

	cfg := defaultEstimateOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.multiplier <= 0 {
		return BillEstimate{}, errors.New("multiplier must be > 0")
	}

	probability, err := ProbabilityPerHash(resolved)
	if err != nil {
		return BillEstimate{}, err
	}
	expectation, err := ExpectedHashesForNote(resolved)
	if err != nil {
		return BillEstimate{}, err
	}
	mean, err := RequiredHashrateMean(resolved, seconds)
	if err != nil {
		return BillEstimate{}, err
	}
	quantileRate, err := RequiredHashrate(resolved, seconds, WithMultiplier(cfg.multiplier))
	if err != nil {
		return BillEstimate{}, err
	}

	primaryMode := cfg.primaryMode
	if primaryMode == "" {
		if cfg.quantile != nil {
			primaryMode = PrimaryModeQuantile
		} else {
			primaryMode = PrimaryModeMean
		}
	}
	if primaryMode == PrimaryModeQuantile && cfg.quantile == nil {
		primaryMode = PrimaryModeMean
	}

	primary := mean
	if primaryMode == PrimaryModeQuantile {
		primary = quantileRate
	}

	var quantileCopy *float64
	if cfg.quantile != nil {
		val := *cfg.quantile
		quantileCopy = &val
	}

	return BillEstimate{
		Sharenote:                resolved,
		Label:                    resolved.Label(),
		Bits:                     resolved.Bits,
		SecondsTarget:            seconds,
		ProbabilityPerHash:       probability,
		ProbabilityDisplay:       FormatProbabilityDisplay(resolved.Bits, cfg.probabilityPrecision),
		ExpectedHashes:           expectation,
		RequiredHashrateMean:     mean,
		RequiredHashrateQuantile: quantileRate,
		RequiredHashratePrimary:  primary,
		RequiredHashrateHuman:    HumaniseHashrate(primary),
		Multiplier:               cfg.multiplier,
		Quantile:                 quantileCopy,
		PrimaryMode:              primaryMode,
	}, nil
}

// EstimateNotes estimates multiple notes at once.
func EstimateNotes(notes []any, seconds float64, opts ...EstimateOption) ([]BillEstimate, error) {
	results := make([]BillEstimate, len(notes))
	for i, note := range notes {
		estimate, err := EstimateNote(note, seconds, opts...)
		if err != nil {
			return nil, err
		}
		results[i] = estimate
	}
	return results, nil
}

// PlanOption configures plan execution for PlanSharenoteFromHashrate.
type PlanOption func(*planOptions)

type planOptions struct {
	hashrateOpts []HashrateOption
	estimateOpts []EstimateOption
}

// WithPlanHashrateOptions forwards hashrate options (e.g. WithConfidence) to the planner.
func WithPlanHashrateOptions(opts ...HashrateOption) PlanOption {
	return func(cfg *planOptions) {
		cfg.hashrateOpts = append(cfg.hashrateOpts, opts...)
	}
}

// WithPlanEstimateOptions forwards estimate options (e.g. WithEstimatePrimaryMode) to the planner.
func WithPlanEstimateOptions(opts ...EstimateOption) PlanOption {
	return func(cfg *planOptions) {
		cfg.estimateOpts = append(cfg.estimateOpts, opts...)
	}
}

// WithPlanReliability applies a reliability preset to both hashrate and estimate calculations.
func WithPlanReliability(id ReliabilityID) PlanOption {
	return func(cfg *planOptions) {
		cfg.hashrateOpts = append(cfg.hashrateOpts, WithReliability(id))
		cfg.estimateOpts = append(cfg.estimateOpts, WithEstimateReliability(id))
	}
}

// WithPlanConfidence applies a raw confidence to both hashrate and estimate calculations.
func WithPlanConfidence(confidence float64) PlanOption {
	return func(cfg *planOptions) {
		cfg.hashrateOpts = append(cfg.hashrateOpts, WithConfidence(confidence))
		cfg.estimateOpts = append(cfg.estimateOpts, WithEstimateConfidence(confidence))
	}
}

// PlanSharenoteFromHashrate derives a note and bill estimate from rig hashrate inputs.
func PlanSharenoteFromHashrate(hashrate HashrateValue, seconds float64, opts ...PlanOption) (SharenotePlan, error) {
	if !isFinite(seconds) || seconds <= 0 {
		return SharenotePlan{}, errors.New("seconds must be > 0")
	}
	numeric, err := NormalizeHashrateValue(hashrate)
	if err != nil {
		return SharenotePlan{}, err
	}
	if numeric <= 0 {
		return SharenotePlan{}, errors.New("hashrate must be > 0")
	}

	cfg := planOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}

	note, err := NoteFromHashrate(hashrate, seconds, cfg.hashrateOpts...)
	if err != nil {
		return SharenotePlan{}, err
	}

	bill, err := EstimateNote(note, seconds, cfg.estimateOpts...)
	if err != nil {
		return SharenotePlan{}, err
	}

	return SharenotePlan{
		Sharenote:          note,
		Bill:               bill,
		SecondsTarget:      seconds,
		InputHashrateHPS:   numeric,
		InputHashrateHuman: HumaniseHashrate(numeric),
	}, nil
}

// CombineNotesSerial adds bit difficulties (serial probability) and returns a new Sharenote.
func CombineNotesSerial(notes ...any) (Sharenote, error) {
	if len(notes) == 0 {
		return Sharenote{}, errors.New("notes slice must not be empty")
	}
	total := 0.0
	for _, note := range notes {
		diff, err := difficultyFromNote(note)
		if err != nil {
			return Sharenote{}, err
		}
		total += diff
	}
	if !isFinite(total) || total <= 0 {
		return NoteFromBits(0)
	}
	bits, err := bitsFromDifficulty(total)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromBits(bits)
}

// NoteDifference subtracts subtrahend bits from minuend bits (clamped at zero).
func NoteDifference(minuend, subtrahend any) (Sharenote, error) {
	minDifficulty, err := difficultyFromNote(minuend)
	if err != nil {
		return Sharenote{}, err
	}
	subDifficulty, err := difficultyFromNote(subtrahend)
	if err != nil {
		return Sharenote{}, err
	}
	diff := minDifficulty - subDifficulty
	if diff <= 0 {
		return NoteFromBits(0)
	}
	bits, err := bitsFromDifficulty(diff)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromBits(bits)
}

// ScaleNote multiplies a note's bit difficulty by the given factor.
func ScaleNote(note any, factor float64) (Sharenote, error) {
	if !isFinite(factor) {
		return Sharenote{}, errors.New("factor must be finite")
	}
	if factor < 0 {
		return Sharenote{}, errors.New("factor must be >= 0")
	}
	if factor == 0 {
		return NoteFromBits(0)
	}
	difficulty, err := difficultyFromNote(note)
	if err != nil {
		return Sharenote{}, err
	}
	bits, err := bitsFromDifficulty(difficulty * factor)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromBits(bits)
}

// DivideNotes returns the ratio of two note bit difficulties.
func DivideNotes(numerator, denominator any) (float64, error) {
	numDifficulty, err := difficultyFromNote(numerator)
	if err != nil {
		return 0, err
	}
	denDifficulty, err := difficultyFromNote(denominator)
	if err != nil {
		return 0, err
	}
	if denDifficulty <= 0 {
		return 0, errors.New("division by zero note")
	}
	return numDifficulty / denDifficulty, nil
}

// HashrateOption configures multiplier/reliability.
type HashrateOption func(*hashrateOptions)

type hashrateOptions struct {
	multiplier float64
}

// WithMultiplier sets the Poisson multiplier directly.
func WithMultiplier(multiplier float64) HashrateOption {
	return func(cfg *hashrateOptions) {
		cfg.multiplier = multiplier
	}
}

// WithReliability selects one of the named presets or a custom confidence (0,1).
func WithReliability(id ReliabilityID) HashrateOption {
	return func(cfg *hashrateOptions) {
		if lvl, ok := reliabilityLevels[id]; ok {
			cfg.multiplier = lvl.Multiplier
		}
	}
}

// WithConfidence configures a Poisson multiplier from a raw confidence between 0 and 1.
func WithConfidence(confidence float64) HashrateOption {
	return func(cfg *hashrateOptions) {
		if confidence <= 0 || confidence >= 1 {
			return
		}
		cfg.multiplier = -math.Log(1 - confidence)
	}
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

func floatPtr(v float64) *float64 {
	return &v
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

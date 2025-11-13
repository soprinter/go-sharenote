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

const (
	// CentZBitStep is the per-cent-Z fractional Z-bit increment.
	CentZBitStep = 0.01
	// centZUnitsPerZ holds the number of cent-Z increments per whole Z.
	centZUnitsPerZ = int(1 / CentZBitStep)
	MinCentZ       = 0
	MaxCentZ       = 99
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

// Sharenote describes a note label using integer Z value, cent-Z fraction, and derived Z-bit difficulty.
type Sharenote struct {
	Z             int
	Cents         int
	ZBits         float64
	labelOverride string
}

// HumanHashrate represents a human-readable H/s value plus metadata.
type HumanHashrate struct {
	Value    float64
	Unit     HashrateUnit
	Display  string
	Exponent int
}

// HashrateMeasurement exposes a numeric value plus helpers for human display.
type HashrateMeasurement struct {
	Value float64
}

// Float64 returns the raw H/s value.
func (h HashrateMeasurement) Float64() float64 {
	return h.Value
}

// Human converts the measurement into a HumanHashrate with optional formatting overrides.
func (h HashrateMeasurement) Human(opts ...HumanHashrateOption) HumanHashrate {
	return HumaniseHashrate(h.Value, opts...)
}

// String returns the formatted human-readable representation.
func (h HashrateMeasurement) String() string {
	return h.Human().String()
}

// HashesMeasurement exposes an expected hash count with helper methods.
type HashesMeasurement struct {
	Value float64
}

// Float64 returns the raw expected hash count.
func (h HashesMeasurement) Float64() float64 {
	return h.Value
}

// String returns a scientific-notation summary for the expected hashes.
func (h HashesMeasurement) String() string {
	if !isFinite(h.Value) || h.Value <= 0 {
		return "0 hashes"
	}
	index := 0
	if h.Value > 0 {
		index = int(math.Floor(math.Log10(h.Value) / 3))
		if index < 0 {
			index = 0
		}
		if index >= len(hashCountUnits) {
			index = len(hashCountUnits) - 1
		}
	}
	unit := hashCountUnits[index]
	scaled := h.Value / math.Pow(1000, float64(unit.exponent))
	if !isFinite(scaled) || scaled <= 0 {
		return "0 hashes"
	}

	var numeric string
	switch {
	case scaled >= 100:
		numeric = fmt.Sprintf("%.0f", scaled)
	case scaled >= 10:
		numeric = fmt.Sprintf("%.1f", scaled)
	default:
		numeric = fmt.Sprintf("%.2f", scaled)
	}

	unitLabel := fmt.Sprintf("%sH/s", unit.prefix)
	if unit.prefix == "" {
		unitLabel = "H/s"
	}
	return fmt.Sprintf("%s %s", numeric, unitLabel)
}

// HashrateRange represents the inclusive/exclusive H/s interval that maps to a Sharenote label.
type HashrateRange struct {
	Min float64
	Max float64
}

// Human renders the min/max bounds into human-readable units.
func (r HashrateRange) Human(opts ...HumanHashrateOption) (HumanHashrate, HumanHashrate) {
	return HumaniseHashrate(r.Min, opts...), HumaniseHashrate(r.Max, opts...)
}

// String implements fmt.Stringer and favours the precomputed display value.
func (h HumanHashrate) String() string {
	if h.Display != "" {
		return h.Display
	}
	if !isFinite(h.Value) || h.Value <= 0 {
		return "0 H/s"
	}
	unit := string(h.Unit)
	if unit == "" {
		unit = string(HashrateUnitHps)
	}
	return fmt.Sprintf("%.2f %s", h.Value, unit)
}

// BillEstimate summarises the metrics required to mint a note within a time window.
type BillEstimate struct {
	Sharenote                Sharenote
	Label                    string
	ZBits                    float64
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

// String implements fmt.Stringer with a compact summary for logging.
func (b BillEstimate) String() string {
	mode := string(b.PrimaryMode)
	if mode == "" {
		mode = string(PrimaryModeMean)
	}
	return fmt.Sprintf(
		"BillEstimate{%s @ %.2fs, p=%s, %s=%s}",
		b.Sharenote,
		b.SecondsTarget,
		b.ProbabilityDisplay,
		mode,
		b.RequiredHashrateHuman,
	)
}

// SharenotePlan summarises a computed note and its supporting bill estimate for a given rig.
type SharenotePlan struct {
	Sharenote          Sharenote
	Bill               BillEstimate
	SecondsTarget      float64
	InputHashrateHPS   float64
	InputHashrateHuman HumanHashrate
}

// String implements fmt.Stringer for concise plan inspection.
func (p SharenotePlan) String() string {
	return fmt.Sprintf(
		"SharenotePlan{%s -> %s @ %.2fs}",
		p.Sharenote,
		p.Bill.RequiredHashrateHuman,
		p.SecondsTarget,
	)
}

// Label returns the canonical Sharenote label (e.g. "33Z53").
func (n Sharenote) Label() string {
	if n.labelOverride != "" {
		return n.labelOverride
	}
	return fmt.Sprintf("%dZ%02d", n.Z, clampCents(n.Cents))
}

// String implements fmt.Stringer by returning the canonical label.
func (n Sharenote) String() string {
	return n.Label()
}

// ProbabilityPerHash returns 2^(-zbits) for the receiver.
func (n Sharenote) ProbabilityPerHash() (float64, error) {
	return ProbabilityFromZBits(n.ZBits)
}

// ExpectedHashes returns the expected hash attempts for the receiver.
func (n Sharenote) ExpectedHashes() (HashesMeasurement, error) {
	return ExpectedHashesMeasurement(n)
}

// RequiredHashrate returns the required H/s to hit the note within the provided window.
func (n Sharenote) RequiredHashrate(seconds float64, opts ...HashrateOption) (HashrateMeasurement, error) {
	return RequiredHashrate(n, seconds, opts...)
}

// RequiredHashrateMean returns the mean H/s requirement for the receiver.
func (n Sharenote) RequiredHashrateMean(seconds float64) (HashrateMeasurement, error) {
	return RequiredHashrateMean(n, seconds)
}

// RequiredHashrateQuantile returns the quantile H/s requirement for the receiver.
func (n Sharenote) RequiredHashrateQuantile(seconds, confidence float64) (HashrateMeasurement, error) {
	return RequiredHashrateQuantile(n, seconds, confidence)
}

// RequiredHashrateMeasurement returns a measurement struct for the required H/s.
func (n Sharenote) RequiredHashrateMeasurement(seconds float64, opts ...HashrateOption) (HashrateMeasurement, error) {
	return RequiredHashrateMeasurement(n, seconds, opts...)
}

// RequiredHashrateMeanMeasurement returns a measurement struct for the mean requirement.
func (n Sharenote) RequiredHashrateMeanMeasurement(seconds float64) (HashrateMeasurement, error) {
	return RequiredHashrateMeanMeasurement(n, seconds)
}

// RequiredHashrateQuantileMeasurement returns a measurement struct for the quantile requirement.
func (n Sharenote) RequiredHashrateQuantileMeasurement(seconds, confidence float64) (HashrateMeasurement, error) {
	return RequiredHashrateQuantileMeasurement(n, seconds, confidence)
}

// HashrateRange returns the [min,max) H/s interval that maps to the receiver's label.
func (n Sharenote) HashrateRange(seconds float64, opts ...HashrateOption) (HashrateRange, error) {
	return HashrateRangeForNote(n, seconds, opts...)
}

// Target returns the integer hash target for the receiver.
func (n Sharenote) Target() (*big.Int, error) {
	return TargetFor(n)
}

// CombineSerial returns the serial combination of the receiver with additional notes.
func (n Sharenote) CombineSerial(others ...any) (Sharenote, error) {
	inputs := make([]any, 0, len(others)+1)
	inputs = append(inputs, n)
	inputs = append(inputs, others...)
	return CombineNotesSerial(inputs...)
}

// Difference subtracts the provided note difficulty from the receiver.
func (n Sharenote) Difference(other any) (Sharenote, error) {
	return NoteDifference(n, other)
}

// Scale multiplies the receiver's difficulty by the provided factor.
func (n Sharenote) Scale(factor float64) (Sharenote, error) {
	return ScaleNote(n, factor)
}

// NBits encodes the receiver in compact nBits format.
func (n Sharenote) NBits() (string, error) {
	return SharenoteToNBits(n)
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

var hashCountUnits = []struct {
	prefix   string
	exponent int
}{
	{"", 0},
	{"K", 1},
	{"M", 2},
	{"G", 3},
	{"T", 4},
	{"P", 5},
	{"E", 6},
	{"Z", 7},
	{"Y", 8},
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

// parseLabel converts textual labels (33Z53, 33.53Z, 33Z 53CZ) into a Sharenote.
func parseLabel(label string) (Sharenote, error) {
	cleaned := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(label), " ", ""))

	if match := reStandard.FindStringSubmatch(cleaned); match != nil {
		z, _ := strconv.Atoi(match[1])
		cents := 0
		if match[2] != "" {
			cents, _ = strconv.Atoi(match[2])
		}
		return noteFromComponents(z, cents)
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
		return noteFromComponents(z, cents)
	}

	if match := reDecimal.FindStringSubmatch(cleaned); match != nil {
		zbits, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return Sharenote{}, fmt.Errorf("parse zbits: %w", err)
		}
		return NoteFromZBits(zbits)
	}

	return Sharenote{}, fmt.Errorf("unrecognised Sharenote label %q", label)
}

// noteFromComponents normalises (Z, cents) into a Sharenote struct using cent-Z precision.
func noteFromComponents(z, cents int) (Sharenote, error) {
	if z < 0 {
		return Sharenote{}, errors.New("z must be non-negative")
	}
	c := clampCents(cents)
	zbits := float64(z) + float64(c)*CentZBitStep
	return Sharenote{Z: z, Cents: c, ZBits: zbits}, nil
}

// NoteFromComponents constructs a canonical Sharenote from integer Z and cent-Z precision.
// It preserves the legacy cent-Z precision behaviour for callers that rely on (Z, CZ) inputs.
func NoteFromComponents(z, cents int) (Sharenote, error) {
	return noteFromComponents(z, cents)
}

func labelComponentsFromZBits(zbits float64) (int, int) {
	z := int(math.Floor(zbits))
	if z < 0 {
		z = 0
	}
	fractional := zbits - float64(z)
	rawCents := int(math.Floor((fractional / CentZBitStep) + 1e-9))
	return z, clampCents(rawCents)
}

// NoteFromZBits converts fractional Z-bit difficulty to a Sharenote while preserving precision.
func NoteFromZBits(zbits float64) (Sharenote, error) {
	if !isFinite(zbits) {
		return Sharenote{}, errors.New("zbits must be finite")
	}
	if zbits < 0 {
		return Sharenote{}, errors.New("zbits must be non-negative")
	}
	z, cents := labelComponentsFromZBits(zbits)
	return Sharenote{Z: z, Cents: cents, ZBits: zbits}, nil
}

// MustNoteFromZBits wraps NoteFromZBits and panics on failure. Intended for tests and fixtures.
func MustNoteFromZBits(zbits float64) Sharenote {
	note, err := NoteFromZBits(zbits)
	if err != nil {
		panic(err)
	}
	return note
}

// NoteFromCentZBits converts cent-Z units (e.g. 3353 => 33.53Z) into a Sharenote.
func NoteFromCentZBits(centZ int) (Sharenote, error) {
	if centZ < 0 {
		return Sharenote{}, errors.New("cent-z value must be non-negative")
	}
	z := centZ / centZUnitsPerZ
	cents := centZ % centZUnitsPerZ
	return noteFromComponents(z, cents)
}

// MustNoteFromCentZBits wraps NoteFromCentZBits and panics on failure. Intended for tests and fixtures.
func MustNoteFromCentZBits(centZ int) Sharenote {
	note, err := NoteFromCentZBits(centZ)
	if err != nil {
		panic(err)
	}
	return note
}

// EnsureNote accepts a Sharenote, label string, or raw Z-bit value and returns the struct.
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
		return parseLabel(v)
	case float64:
		return NoteFromZBits(v)
	case float32:
		return NoteFromZBits(float64(v))
	case int:
		return NoteFromZBits(float64(v))
	case int8:
		return NoteFromZBits(float64(v))
	case int16:
		return NoteFromZBits(float64(v))
	case int32:
		return NoteFromZBits(float64(v))
	case int64:
		return NoteFromZBits(float64(v))
	case uint:
		return NoteFromZBits(float64(v))
	case uint8:
		return NoteFromZBits(float64(v))
	case uint16:
		return NoteFromZBits(float64(v))
	case uint32:
		return NoteFromZBits(float64(v))
	case uint64:
		return NoteFromZBits(float64(v))
	default:
		return Sharenote{}, fmt.Errorf("unsupported note input %T", v)
	}
}

// ProbabilityFromZBits returns 2^(-zbits).
func ProbabilityFromZBits(zbits float64) (float64, error) {
	if !isFinite(zbits) {
		return 0, errors.New("zbits must be finite")
	}
	return math.Exp2(-zbits), nil
}

// ProbabilityPerHash returns the per-hash success probability for the note.
func ProbabilityPerHash(note any) (float64, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return 0, err
	}
	return math.Exp2(-resolved.ZBits), nil
}

func difficultyFromNote(note any) (float64, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return 0, err
	}
	return math.Exp2(resolved.ZBits), nil
}

func zBitsFromDifficulty(difficulty float64) (float64, error) {
	if !isFinite(difficulty) || difficulty <= 0 {
		return 0, errors.New("difficulty must be > 0")
	}
	return math.Log2(difficulty), nil
}

func expectedHashesValueFromZBits(zbits float64) (float64, error) {
	p, err := ProbabilityFromZBits(zbits)
	if err != nil {
		return 0, err
	}
	return 1 / p, nil
}

// ExpectedHashesForZBits returns 1 / probability.
func ExpectedHashesForZBits(zbits float64) (HashesMeasurement, error) {
	value, err := expectedHashesValueFromZBits(zbits)
	if err != nil {
		return HashesMeasurement{}, err
	}
	return HashesMeasurement{Value: value}, nil
}

// ExpectedHashesForNote returns expected hashes for converting the given note.
func ExpectedHashesForNote(note any) (HashesMeasurement, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return HashesMeasurement{}, err
	}
	return ExpectedHashesForZBits(resolved.ZBits)
}

// ExpectedHashesMeasurement returns an expected hash count with helpers.
func ExpectedHashesMeasurement(note any) (HashesMeasurement, error) {
	return ExpectedHashesForNote(note)
}

func requiredHashrateValue(note any, seconds float64, opts ...HashrateOption) (float64, error) {
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
	expected, err := expectedHashesValueFromZBits(resolved.ZBits)
	if err != nil {
		return 0, err
	}
	return expected * cfg.multiplier / seconds, nil
}

// RequiredHashrate computes multiplier * expected_hashes / seconds and returns a measurement.
func RequiredHashrate(note any, seconds float64, opts ...HashrateOption) (HashrateMeasurement, error) {
	value, err := requiredHashrateValue(note, seconds, opts...)
	if err != nil {
		return HashrateMeasurement{}, err
	}
	return HashrateMeasurement{Value: value}, nil
}

// RequiredHashrateMean returns the mean hashrate.
func RequiredHashrateMean(note any, seconds float64) (HashrateMeasurement, error) {
	return RequiredHashrate(note, seconds)
}

// RequiredHashrateQuantile returns the quantile hashrate for the provided confidence.
func RequiredHashrateQuantile(note any, seconds, confidence float64) (HashrateMeasurement, error) {
	if confidence <= 0 || confidence >= 1 {
		return HashrateMeasurement{}, errors.New("confidence must be in (0,1)")
	}
	multiplier := -math.Log(1 - confidence)
	return RequiredHashrate(note, seconds, WithMultiplier(multiplier))
}

// RequiredHashrateMeasurement returns a structured measurement for the required H/s.
func RequiredHashrateMeasurement(note any, seconds float64, opts ...HashrateOption) (HashrateMeasurement, error) {
	return RequiredHashrate(note, seconds, opts...)
}

// RequiredHashrateMeanMeasurement returns a structured measurement for the mean requirement.
func RequiredHashrateMeanMeasurement(note any, seconds float64) (HashrateMeasurement, error) {
	return RequiredHashrateMean(note, seconds)
}

// RequiredHashrateQuantileMeasurement returns a structured measurement for the provided confidence.
func RequiredHashrateQuantileMeasurement(note any, seconds, confidence float64) (HashrateMeasurement, error) {
	return RequiredHashrateQuantile(note, seconds, confidence)
}

// HashrateRangeForNote returns the [min,max) hashrate interval corresponding to the provided note label.
func HashrateRangeForNote(note any, seconds float64, opts ...HashrateOption) (HashrateRange, error) {
	if !isFinite(seconds) || seconds <= 0 {
		return HashrateRange{}, errors.New("seconds must be > 0")
	}
	cfg := hashrateOptions{multiplier: 1}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.multiplier <= 0 {
		return HashrateRange{}, errors.New("multiplier must be > 0")
	}
	resolved, err := EnsureNote(note)
	if err != nil {
		return HashrateRange{}, err
	}
	lowerExpected, err := expectedHashesValueFromZBits(resolved.ZBits)
	if err != nil {
		return HashrateRange{}, err
	}
	upperExpected, err := expectedHashesValueFromZBits(resolved.ZBits + CentZBitStep)
	if err != nil {
		return HashrateRange{}, err
	}
	lower := lowerExpected * cfg.multiplier / seconds
	upper := upperExpected * cfg.multiplier / seconds
	if upper < lower {
		upper = lower
	}
	return HashrateRange{Min: lower, Max: upper}, nil
}

// MaxZBitsForHashrate computes the maximum bit difficulty achievable with the provided parameters.
func MaxZBitsForHashrate(hashrate, seconds, multiplier float64) (float64, error) {
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
	zbits, err := MaxZBitsForHashrate(numeric, seconds, cfg.multiplier)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromZBits(zbits)
}

// TargetFor returns the integer hash target for the note.
func TargetFor(note any) (*big.Int, error) {
	resolved, err := EnsureNote(note)
	if err != nil {
		return nil, err
	}
	integerBits := int(math.Floor(resolved.ZBits))
	baseExponent := 256 - integerBits
	if baseExponent < 0 {
		return nil, errors.New("z too large; target underflow")
	}
	fractional := resolved.ZBits - float64(integerBits)
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
	zbits := 256 - log2Target
	return NoteFromZBits(zbits)
}

func targetToCompact(target *big.Int) (uint32, error) {
	if target == nil || target.Sign() <= 0 {
		return 0, errors.New("target must be positive")
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
		return 0, errors.New("target exponent overflow")
	}
	return uint32(exponent)<<24 | mantissa, nil
}

// SharenoteToNBits encodes a note into compact nBits hex representation.
func SharenoteToNBits(note any) (string, error) {
	target, err := TargetFor(note)
	if err != nil {
		return "", err
	}
	if target.Sign() <= 0 {
		return "", errors.New("target must be positive")
	}
	compact, err := targetToCompact(target)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x", compact), nil
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
func FormatProbabilityDisplay(zbits float64, precision int) string {
	if precision < 0 {
		precision = 0
	}
	return fmt.Sprintf("1 / 2^%.*f", precision, zbits)
}

// HumanHashrateOption customises display formatting when humanising H/s values.
type HumanHashrateOption func(*humanHashrateOptions)

type humanHashrateOptions struct {
	precision *int
}

// WithHumanHashratePrecision forces a fixed number of decimal places in the display string.
func WithHumanHashratePrecision(precision int) HumanHashrateOption {
	if precision < 0 {
		precision = 0
	}
	return func(cfg *humanHashrateOptions) {
		cfg.precision = &precision
	}
}

// HumaniseHashrate renders a hashrate into an appropriate SI-prefixed unit.
func HumaniseHashrate(hashrate float64, opts ...HumanHashrateOption) HumanHashrate {
	cfg := humanHashrateOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if !isFinite(hashrate) || hashrate <= 0 {
		return HumanHashrate{Value: 0, Unit: HashrateUnitHps, Display: "0 H/s", Exponent: 0}
	}
	logValue := math.Log10(hashrate)
	index := int(math.Max(0, math.Floor(logValue/3)))
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
	case cfg.precision != nil:
		display = fmt.Sprintf("%.*f %s", *cfg.precision, scaled, unit.unit)
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
	meanRate, err := RequiredHashrateMean(resolved, seconds)
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

	primary := meanRate
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
		ZBits:                    resolved.ZBits,
		SecondsTarget:            seconds,
		ProbabilityPerHash:       probability,
		ProbabilityDisplay:       FormatProbabilityDisplay(resolved.ZBits, cfg.probabilityPrecision),
		ExpectedHashes:           expectation.Float64(),
		RequiredHashrateMean:     meanRate.Float64(),
		RequiredHashrateQuantile: quantileRate.Float64(),
		RequiredHashratePrimary:  primary.Float64(),
		RequiredHashrateHuman:    primary.Human(),
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

// CombineNotesSerial adds Z-bit difficulties (serial probability) and returns a new Sharenote.
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
		return NoteFromZBits(0)
	}
	zbits, err := zBitsFromDifficulty(total)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromZBits(zbits)
}

// NoteDifference subtracts subtrahend Z-bit difficulty from the minuend (clamped at zero).
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
		return NoteFromZBits(0)
	}
	zbits, err := zBitsFromDifficulty(diff)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromZBits(zbits)
}

// ScaleNote multiplies a note's Z-bit difficulty by the given factor.
func ScaleNote(note any, factor float64) (Sharenote, error) {
	if !isFinite(factor) {
		return Sharenote{}, errors.New("factor must be finite")
	}
	if factor < 0 {
		return Sharenote{}, errors.New("factor must be >= 0")
	}
	if factor == 0 {
		return NoteFromZBits(0)
	}
	difficulty, err := difficultyFromNote(note)
	if err != nil {
		return Sharenote{}, err
	}
	zbits, err := zBitsFromDifficulty(difficulty * factor)
	if err != nil {
		return Sharenote{}, err
	}
	return NoteFromZBits(zbits)
}

// DivideNotes returns the ratio of two note Z-bit difficulties.
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
	if value < MinCentZ {
		return MinCentZ
	}
	if value > MaxCentZ {
		return MaxCentZ
	}
	return value
}

func floatPtr(v float64) *float64 {
	return &v
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

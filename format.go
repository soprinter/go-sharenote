package sharenote

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// siPrefixes lists the SI prefixes for hash-rate magnitudes, indexed by the
// power of 1000 they scale (""=10^0, "k"=10^3, … "Z"=10^21).
var siPrefixes = []string{"", "k", "M", "G", "T", "P", "E", "Z"}

var hashratePrefixExponent = map[string]int{
	"": 0, "K": 1, "M": 2, "G": 3, "T": 4, "P": 5, "E": 6, "Z": 7,
}

var (
	hashrateStringPattern = regexp.MustCompile(`^([+-]?(?:\d+(?:[_,]?\d+)*(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)\s*([A-Za-z\/\s-]+)?$`)
	hashrateUnitPattern   = regexp.MustCompile(`^([KMGTPEZ]?)(H)/S$`)
)

// FormatHashrate renders a hash rate in hashes per second into a compact,
// SI-prefixed string such as "7.43 GH/s". Non-positive or non-finite inputs
// render as "0 H/s". It is a display helper; it does not round-trip exactly.
func FormatHashrate(hps float64) string {
	if !isFinite(hps) || hps <= 0 {
		return "0 H/s"
	}
	index := int(math.Max(0, math.Floor(math.Log10(hps)/3)))
	if index >= len(siPrefixes) {
		index = len(siPrefixes) - 1
	}
	scaled := hps / math.Pow(10, float64(index*3))
	if !isFinite(scaled) {
		scaled = hps
	}
	var digits int
	switch {
	case scaled >= 100:
		digits = 0
	case scaled >= 10:
		digits = 1
	default:
		digits = 2
	}
	return fmt.Sprintf("%.*f %sH/s", digits, scaled, siPrefixes[index])
}

// ParseHashrate parses a human-readable hash rate such as "5 GH/s", "12.5MH/s",
// or a bare "1000" (taken as H/s) into hashes per second. Thousands separators
// ("," or "_") are accepted. It errors on an empty string, a negative or
// non-finite magnitude, or an unrecognised unit.
func ParseHashrate(input string) (float64, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, errors.New("sharenote: hashrate string must not be empty")
	}
	match := hashrateStringPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return 0, fmt.Errorf("sharenote: unrecognised hashrate format: %q", input)
	}
	magnitude := strings.NewReplacer("_", "", ",", "").Replace(match[1])
	value, err := strconv.ParseFloat(magnitude, 64)
	if err != nil {
		return 0, fmt.Errorf("sharenote: parse hashrate magnitude: %w", err)
	}
	if !isFinite(value) {
		return 0, errors.New("sharenote: hashrate magnitude must be finite")
	}
	if value < 0 {
		return 0, errors.New("sharenote: hashrate must be >= 0")
	}
	exponent, err := resolveHashrateUnit(strings.TrimSpace(match[2]))
	if err != nil {
		return 0, err
	}
	return value * math.Pow(10, float64(exponent*3)), nil
}

func resolveHashrateUnit(unit string) (int, error) {
	if strings.TrimSpace(unit) == "" {
		return 0, nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(unit))
	normalized = strings.NewReplacer("-", "", "_", "", " ", "").Replace(normalized)
	normalized = strings.ReplaceAll(normalized, "HPS", "H/S")
	normalized = strings.ReplaceAll(normalized, "HS", "H/S")
	if !strings.HasSuffix(normalized, "/S") && strings.Contains(normalized, "H") {
		normalized += "/S"
	}
	normalized = strings.ReplaceAll(normalized, "/S/S", "/S")

	m := hashrateUnitPattern.FindStringSubmatch(normalized)
	if m == nil {
		return 0, fmt.Errorf("sharenote: unrecognised hashrate unit: %q", unit)
	}
	exponent, ok := hashratePrefixExponent[m[1]]
	if !ok {
		return 0, fmt.Errorf("sharenote: unsupported hashrate prefix: %q", m[1])
	}
	return exponent, nil
}

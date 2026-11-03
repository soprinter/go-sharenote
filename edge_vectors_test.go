package sharenote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The frozen sha256 of each shared fixture (VALIDATION-CONTRACT.md §5). The
// three SDKs vendor byte-identical copies; this guard fails loudly if any copy
// drifts, so the fixtures stay the single source of truth.
const (
	snip00SHA256     = "757e00bcb92c8db6d26d1ee8fc0df94bd06ff9887d20a8baf1e1c7a460a4379e"
	sdkEdgeSHA256    = "dde47c42dd2292171d2f0ef6529fcf59370595456a52f1a5cc5e695605b48cea"
	validationSHA256 = "36d915d63756c9fd8eaa05a9400581b8c442a5b819a33e76ec91763b5bb7d445"
)

// TestFixtureChecksums pins the three shared fixtures so an accidental edit to a
// frozen file is caught immediately (keeps the snip00 guard and adds the two new
// fixtures).
func TestFixtureChecksums(t *testing.T) {
	cases := []struct{ file, want string }{
		{"testdata/snip00_tests.json", snip00SHA256},
		{"testdata/sdk_edge_tests.json", sdkEdgeSHA256},
		{"testdata/validation_tests.json", validationSHA256},
	}
	for _, tc := range cases {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != tc.want {
			t.Fatalf("%s sha256 = %s, want %s (frozen fixture changed?)", tc.file, got, tc.want)
		}
	}
}

// --- sdk_edge_tests.json (D1-D4) ---

type edgeOperand struct {
	Label *string  `json:"label"`
	ZBits *float64 `json:"z_bits"`
}

func (o edgeOperand) note(t *testing.T) Note {
	t.Helper()
	switch {
	case o.Label != nil:
		return MustParseLabel(*o.Label)
	case o.ZBits != nil:
		n, err := FromBits(*o.ZBits)
		if err != nil {
			t.Fatalf("FromBits(%v): %v", *o.ZBits, err)
		}
		return n
	default:
		t.Fatal("edge operand has neither label nor z_bits")
		return Note{}
	}
}

type edgeCase struct {
	Name      string          `json:"name"`
	Decision  string          `json:"decision"`
	Operation string          `json:"operation"`
	Context   json.RawMessage `json:"context"`
	Inputs    json.RawMessage `json:"inputs"`
	Expected  json.RawMessage `json:"expected"`
}

func TestEdgeVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/sdk_edge_tests.json")
	if err != nil {
		t.Fatalf("read edge fixture: %v", err)
	}
	var file struct {
		Cases []edgeCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("unmarshal edge fixture: %v", err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("no edge cases loaded")
	}
	for _, tc := range file.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			switch tc.Operation {
			case "compare":
				runCompareVector(t, tc)
			case "label_from_zbits":
				runLabelFromZBitsVector(t, tc)
			case "combine":
				runCombineVector(t, tc)
			case "integer_roundtrip":
				runIntegerRoundtripVector(t, tc)
			default:
				t.Fatalf("unknown edge operation %q", tc.Operation)
			}
		})
	}
}

// D1: compare by label components (z, then cents); same label => equal.
func runCompareVector(t *testing.T, tc edgeCase) {
	var in struct {
		A edgeOperand `json:"a"`
		B edgeOperand `json:"b"`
	}
	var want struct {
		Cmp int `json:"cmp"`
	}
	mustJSON(t, tc.Inputs, &in)
	mustJSON(t, tc.Expected, &want)
	if got := in.A.note(t).Cmp(in.B.note(t)); got != want.Cmp {
		t.Fatalf("%s: Cmp = %d, want %d", tc.Name, got, want.Cmp)
	}
}

// D2: near-integer z-bits clamp cents to 99 (no carry to the next whole Z).
func runLabelFromZBitsVector(t *testing.T, tc edgeCase) {
	var in struct {
		ZBits float64 `json:"z_bits"`
	}
	var want struct {
		Label string `json:"label"`
		Z     int    `json:"z"`
		Cents int    `json:"cents"`
	}
	mustJSON(t, tc.Inputs, &in)
	mustJSON(t, tc.Expected, &want)
	n, err := FromBits(in.ZBits)
	if err != nil {
		t.Fatalf("FromBits(%v): %v", in.ZBits, err)
	}
	if n.Label() != want.Label || n.Z() != want.Z || n.Cents() != want.Cents {
		t.Fatalf("%s: got %s (z=%d cents=%d), want %s (z=%d cents=%d)",
			tc.Name, n.Label(), n.Z(), n.Cents(), want.Label, want.Z, want.Cents)
	}
}

// D3: folding a combined note into another combine must carry the exact stored
// difficulty, never a value re-derived from the display label.
func runCombineVector(t *testing.T, tc edgeCase) {
	var in struct {
		Steps []struct {
			Combine []string `json:"combine"`
		} `json:"steps"`
		ResultStep int `json:"result_step"`
	}
	var want struct {
		Label                  string `json:"label"`
		ExactDifficultyDecimal string `json:"exact_difficulty_decimal"`
		ExactDyadic            struct {
			Sig string `json:"sig"`
			Exp int    `json:"exp"`
		} `json:"exact_dyadic"`
		Difficulty               float64 `json:"difficulty"`
		IntermediateStep0Decimal string  `json:"intermediate_step0_decimal"`
	}
	mustJSON(t, tc.Inputs, &in)
	mustJSON(t, tc.Expected, &want)

	results := make([]Note, 0, len(in.Steps))
	for i, step := range in.Steps {
		notes := make([]Note, 0, len(step.Combine))
		for _, tok := range step.Combine {
			if strings.HasPrefix(tok, "$") {
				idx, err := strconv.Atoi(tok[1:])
				if err != nil || idx < 0 || idx >= len(results) {
					t.Fatalf("%s: step %d bad token %q", tc.Name, i, tok)
				}
				notes = append(notes, results[idx])
			} else {
				notes = append(notes, MustParseLabel(tok))
			}
		}
		res, err := Sum(notes...)
		if err != nil {
			t.Fatalf("%s: step %d Sum: %v", tc.Name, i, err)
		}
		results = append(results, res)
	}
	if in.ResultStep < 0 || in.ResultStep >= len(results) {
		t.Fatalf("%s: result_step %d out of range", tc.Name, in.ResultStep)
	}
	final := results[in.ResultStep]

	if final.Label() != want.Label {
		t.Fatalf("%s: label = %s, want %s", tc.Name, final.Label(), want.Label)
	}
	// The discriminating assertion (contract §6.6): the exact difficulty.
	wantExact := mustRat(t, want.ExactDifficultyDecimal)
	if final.Difficulty().Cmp(wantExact) != 0 {
		t.Fatalf("%s: exact difficulty = %s, want %s", tc.Name, final.Difficulty().RatString(), wantExact.RatString())
	}
	// The dyadic form sig * 2^exp must agree with the decimal.
	dyadic := dyadicRat(t, want.ExactDyadic.Sig, want.ExactDyadic.Exp)
	if final.Difficulty().Cmp(dyadic) != 0 {
		t.Fatalf("%s: dyadic mismatch: %s vs %s", tc.Name, final.Difficulty().RatString(), dyadic.RatString())
	}
	// The intermediate (step 0) difficulty is pinned too.
	if want.IntermediateStep0Decimal != "" {
		wantStep0 := mustRat(t, want.IntermediateStep0Decimal)
		if results[0].Difficulty().Cmp(wantStep0) != 0 {
			t.Fatalf("%s: step0 difficulty = %s, want %s", tc.Name, results[0].Difficulty().RatString(), wantStep0.RatString())
		}
	}
	// The float display projection matches within tolerance.
	if want.Difficulty > 0 && !roughlyEqual(final.DifficultyFloat(), want.Difficulty) {
		t.Fatalf("%s: display difficulty = %v, want %v", tc.Name, final.DifficultyFloat(), want.Difficulty)
	}
}

// D4: integer tags round-trip exactly across the int64 range; Go parses with
// strconv.ParseInt (the same path requireInt64 uses in the event layer), so no
// float64 precision is lost.
func runIntegerRoundtripVector(t *testing.T, tc edgeCase) {
	var in struct {
		Wire string `json:"wire"`
	}
	var want struct {
		Value string `json:"value"`
	}
	mustJSON(t, tc.Inputs, &in)
	mustJSON(t, tc.Expected, &want)
	got, err := strconv.ParseInt(in.Wire, 10, 64)
	if err != nil {
		t.Fatalf("%s: ParseInt(%q): %v", tc.Name, in.Wire, err)
	}
	if strconv.FormatInt(got, 10) != want.Value {
		t.Fatalf("%s: round-trip = %d, want %s", tc.Name, got, want.Value)
	}
}

func mustRat(t *testing.T, decimal string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(decimal)
	if !ok {
		t.Fatalf("bad decimal %q", decimal)
	}
	return r
}

// dyadicRat builds sig * 2^exp as an exact rational.
func dyadicRat(t *testing.T, sig string, exp int) *big.Rat {
	t.Helper()
	n, ok := new(big.Int).SetString(sig, 10)
	if !ok {
		t.Fatalf("bad dyadic sig %q", sig)
	}
	r := new(big.Rat).SetInt(n)
	if exp >= 0 {
		r.Mul(r, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(exp))))
	} else {
		r.Quo(r, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(-exp))))
	}
	return r
}

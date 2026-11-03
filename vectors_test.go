package sharenote

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// These are the cross-SDK conformance vectors shared by the Go, JS, and Python
// Sharenote implementations (NIP-SN §Z-Arithmetic). They are the behavioural
// contract: the arithmetic here must produce byte-identical labels and
// difficulties. The fixture file is consumed verbatim and never modified.

const vectorTolerance = 1e-6

func roughlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= vectorTolerance*math.Abs(b)
}

type vectorNote struct {
	Label      string  `json:"label"`
	ZBits      float64 `json:"z_bits"`
	Difficulty float64 `json:"difficulty"`
}

type vectorCase struct {
	Name      string          `json:"name"`
	Operation string          `json:"operation"`
	Inputs    json.RawMessage `json:"inputs"`
	Expected  json.RawMessage `json:"expected"`
}

func loadVectors(t *testing.T) []vectorCase {
	t.Helper()
	data, err := os.ReadFile("testdata/snip00_tests.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var file struct {
		Cases []vectorCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("unmarshal vectors: %v", err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("no vector cases loaded")
	}
	return file.Cases
}

func assertNote(t *testing.T, name string, got Note, want vectorNote) {
	t.Helper()
	if got.Label() != want.Label {
		t.Fatalf("%s: label = %s, want %s", name, got.Label(), want.Label)
	}
	if !roughlyEqual(got.Bits(), want.ZBits) {
		t.Fatalf("%s: bits = %v, want %v", name, got.Bits(), want.ZBits)
	}
	if want.Difficulty > 0 && !roughlyEqual(got.DifficultyFloat(), want.Difficulty) {
		t.Fatalf("%s: difficulty = %v, want %v", name, got.DifficultyFloat(), want.Difficulty)
	}
}

func TestConformanceVectors(t *testing.T) {
	for _, tc := range loadVectors(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			switch tc.Operation {
			case "add":
				var inputs []vectorNote
				var want vectorNote
				mustJSON(t, tc.Inputs, &inputs)
				mustJSON(t, tc.Expected, &want)
				notes := make([]Note, len(inputs))
				for i, in := range inputs {
					notes[i] = MustParseLabel(in.Label)
				}
				got, err := Sum(notes...)
				if err != nil {
					t.Fatalf("Sum: %v", err)
				}
				assertNote(t, tc.Name, got, want)

			case "subtract":
				var inputs struct {
					Minuend    vectorNote `json:"minuend"`
					Subtrahend vectorNote `json:"subtrahend"`
				}
				var want vectorNote
				mustJSON(t, tc.Inputs, &inputs)
				mustJSON(t, tc.Expected, &want)
				got, err := MustParseLabel(inputs.Minuend.Label).Sub(MustParseLabel(inputs.Subtrahend.Label))
				if err != nil {
					t.Fatalf("Sub: %v", err)
				}
				assertNote(t, tc.Name, got, want)

			case "scale":
				var inputs struct {
					Note   vectorNote `json:"note"`
					Factor float64    `json:"factor"`
				}
				var want vectorNote
				mustJSON(t, tc.Inputs, &inputs)
				mustJSON(t, tc.Expected, &want)
				got, err := MustParseLabel(inputs.Note.Label).Scale(inputs.Factor)
				if err != nil {
					t.Fatalf("Scale: %v", err)
				}
				assertNote(t, tc.Name, got, want)

			case "divide":
				var inputs struct {
					Numerator   vectorNote `json:"numerator"`
					Denominator vectorNote `json:"denominator"`
				}
				var want struct {
					Ratio float64 `json:"ratio"`
				}
				mustJSON(t, tc.Inputs, &inputs)
				mustJSON(t, tc.Expected, &want)
				got, err := MustParseLabel(inputs.Numerator.Label).Ratio(MustParseLabel(inputs.Denominator.Label))
				if err != nil {
					t.Fatalf("Ratio: %v", err)
				}
				if !roughlyEqual(got, want.Ratio) {
					t.Fatalf("%s: ratio = %v, want %v", tc.Name, got, want.Ratio)
				}

			default:
				t.Fatalf("unknown operation %q", tc.Operation)
			}
		})
	}
}

func mustJSON(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

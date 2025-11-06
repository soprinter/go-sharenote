package sharenote

import (
	"math"
	"strings"
	"testing"
)

const tolerance = 1e-6

func roughlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= tolerance*math.Abs(b)
}

func TestNoteConstruction(t *testing.T) {
	note, err := NoteFromComponents(33, 53)
	if err != nil {
		t.Fatalf("NoteFromComponents: %v", err)
	}
	if note.Z != 33 || note.Cents != 53 {
		t.Fatalf("unexpected note: %+v", note)
	}
	expectedBits := float64(33) + float64(53)*ContinuousExponentStep
	if !roughlyEqual(note.Bits, expectedBits) {
		t.Fatalf("bits mismatch: got %f want %f", note.Bits, expectedBits)
	}
	if note.Label() != "33Z53" {
		t.Fatalf("label mismatch: %s", note.Label())
	}
}

func TestParseLabelVariants(t *testing.T) {
	for _, label := range []string{"33Z53", "33Z 53CZ", "33.53Z"} {
		if _, err := ParseLabel(label); err != nil {
			t.Fatalf("ParseLabel(%s): %v", label, err)
		}
	}
	if note, err := ParseLabel("33z"); err != nil || note.Cents != 0 {
		t.Fatalf("ParseLabel lower-case: %+v, %v", note, err)
	}
}

func TestProbabilityMath(t *testing.T) {
	note := MustParseLabel("33Z53")
	p, err := ProbabilityPerHash(note)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(p, math.Exp2(-note.Bits)) {
		t.Fatalf("unexpected probability: %f", p)
	}
	expected, err := ExpectedHashesForNote(note)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(expected, 1/p) {
		t.Fatalf("expected hashes mismatch: %f vs %f", expected, 1/p)
	}
}

func TestHashrateRequirements(t *testing.T) {
	note := MustParseLabel("33Z53")
	mean, err := RequiredHashrateMean(note, 5)
	if err != nil {
		t.Fatal(err)
	}
 if !roughlyEqual(mean, 2.480651469e9) {
		t.Fatalf("mean hashrate mismatch: %f", mean)
	}
	q95, err := RequiredHashrateQuantile(note, 5, 0.95)
	if err != nil {
		t.Fatal(err)
	}
 if !roughlyEqual(q95, 7.431367665e9) {
		t.Fatalf("quantile mismatch: %f", q95)
	}
}

func TestNoteFromHashrate(t *testing.T) {
 note, err := NoteFromHashrate(HashrateValue{Value: 2.480651469e9, Unit: HashrateUnitHps}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if note.Label() != "33Z53" {
		t.Fatalf("unexpected label: %s", note.Label())
	}
}

func TestParseHashrate(t *testing.T) {
	value, err := ParseHashrate("5 GH/s")
	if err != nil {
		t.Fatalf("ParseHashrate: %v", err)
	}
	if !roughlyEqual(value, 5e9) {
		t.Fatalf("unexpected parsed value: %f", value)
	}
	value, err = ParseHashrate("12.5 MH/s")
	if err != nil {
		t.Fatalf("ParseHashrate words: %v", err)
	}
	if !roughlyEqual(value, 12.5e6) {
		t.Fatalf("unexpected parsed value: %f", value)
	}
	if _, err := ParseHashrate("12 foo/s"); err == nil {
		t.Fatal("expected error for invalid unit")
	}
}

func TestTargetFor(t *testing.T) {
	target, err := TargetFor("33Z00")
	if err != nil {
		t.Fatal(err)
	}
	if target.BitLen() < 222 || target.BitLen() > 224 {
		t.Fatalf("unexpected bit length %d", target.BitLen())
	}
}

func TestCompareNotes(t *testing.T) {
	cmp, err := CompareNotes("32Z00", "33Z00")
	if err != nil {
		t.Fatal(err)
	}
	if cmp >= 0 {
		t.Fatal("expected 32Z00 < 33Z00")
	}
	cmp, err = CompareNotes("33Z54", "33Z53")
	if err != nil {
		t.Fatal(err)
	}
	if cmp <= 0 {
		t.Fatal("expected 33Z54 > 33Z53")
	}
}

func TestNBitsConversion(t *testing.T) {
	note, err := NBitsToSharenote("19752b59")
	if err != nil {
		t.Fatal(err)
	}
 if !roughlyEqual(note.Bits, 57.12) {
  t.Fatalf("unexpected bits: %f", note.Bits)
 }
 if note.Label() != "57Z12" {
  t.Fatalf("unexpected label: %s", note.Label())
 }
}

func TestReliabilityLevels(t *testing.T) {
	levels := ReliabilityLevels()
	if len(levels) == 0 {
		t.Fatal("expected reliability levels")
	}
}

func TestEnsureNote(t *testing.T) {
	note := MustParseLabel("33Z53")
	resolved, err := EnsureNote(note)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Label() != "33Z53" {
		t.Fatalf("ensure note mismatch: %s", resolved.Label())
	}
	if _, err := EnsureNote(123); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestTargetDeterministic(t *testing.T) {
	note := MustParseLabel("57Z12")
	target, err := TargetFor(note)
	if err != nil {
		t.Fatal(err)
	}
	if target.Sign() <= 0 {
		t.Fatal("target should be positive")
	}
	// Validate monotonicity
	next, err := TargetFor("57Z13")
	if err != nil {
		t.Fatal(err)
	}
	if next.Cmp(target) >= 0 {
		t.Fatal("harder note should yield smaller target")
	}

 if FormatProbabilityDisplay(note.Bits, 5) != "1 / 2^57.12000" {
		t.Fatalf("unexpected probability display")
	}
}

func TestHumaniseHashrate(t *testing.T) {
	human := HumaniseHashrate(3.2e9)
	if human.Unit != HashrateUnitGHps {
		t.Fatalf("unexpected unit: %s", human.Unit)
	}
	if !strings.HasPrefix(human.Display, "3.20") {
		t.Fatalf("unexpected display: %s", human.Display)
	}
}

func TestEstimateNote(t *testing.T) {
	estimate, err := EstimateNote("33Z53", 5, WithEstimateConfidence(0.95))
	if err != nil {
		t.Fatal(err)
	}
	if estimate.Label != "33Z53" {
		t.Fatalf("unexpected label: %s", estimate.Label)
	}
    if !roughlyEqual(estimate.RequiredHashratePrimary, 7.431367665e9) {
        t.Fatalf("primary mismatch: %f", estimate.RequiredHashratePrimary)
    }
    if estimate.RequiredHashrateHuman.Unit != HashrateUnitGHps {
        t.Fatalf("unexpected human unit: %s", estimate.RequiredHashrateHuman.Unit)
    }
    if !strings.HasSuffix(estimate.RequiredHashrateHuman.Display, " GH/s") {
        t.Fatalf("unexpected human display: %s", estimate.RequiredHashrateHuman.Display)
    }
    if !strings.HasPrefix(estimate.RequiredHashrateHuman.Display, "7.43") {
        t.Fatalf("unexpected human display: %s", estimate.RequiredHashrateHuman.Display)
    }
}

func TestPlanSharenoteFromHashrate(t *testing.T) {
	plan, err := PlanSharenoteFromHashrate(
		HashrateValue{Value: 5, Unit: HashrateUnitGHps},
		5,
		WithPlanReliability(ReliabilityOften95),
	)
	if err != nil {
		t.Fatalf("PlanSharenoteFromHashrate: %v", err)
	}
	expected, err := NoteFromHashrate(HashrateValue{Value: 5, Unit: HashrateUnitGHps}, 5, WithReliability(ReliabilityOften95))
	if err != nil {
		t.Fatalf("NoteFromHashrate: %v", err)
	}
	if plan.Sharenote.Label() != expected.Label() {
		t.Fatalf("unexpected sharenote label: %s", plan.Sharenote.Label())
	}
	diff := math.Abs(plan.Bill.RequiredHashratePrimary - plan.InputHashrateHPS)
	if diff/plan.InputHashrateHPS > 0.02 {
		t.Fatalf("primary mismatch: %f", plan.Bill.RequiredHashratePrimary)
	}
	if plan.InputHashrateHuman.Unit != HashrateUnitGHps {
		t.Fatalf("unexpected input unit: %s", plan.InputHashrateHuman.Unit)
	}
}

func TestArithmeticHelpers(t *testing.T) {
	combined, err := CombineNotesSerial("33Z53", "20Z10")
	if err != nil {
		t.Fatal(err)
	}
	if combined.Label() != "33Z53" {
		t.Fatalf("unexpected combined label: %s", combined.Label())
	}
	if !roughlyEqual(combined.Bits, 33.53) {
		t.Fatalf("unexpected combined bits: %f", combined.Bits)
	}

	delta, err := NoteDifference("33Z53", "20Z10")
	if err != nil {
		t.Fatal(err)
	}
	if delta.Label() != "33Z52" {
		t.Fatalf("unexpected delta label: %s", delta.Label())
	}
	if !roughlyEqual(delta.Bits, 33.52) {
		t.Fatalf("unexpected delta bits: %f", delta.Bits)
	}

	scaled, err := ScaleNote("20Z10", 1.5)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(scaled.Bits, 20.68) {
		t.Fatalf("unexpected scaled bits: %f", scaled.Bits)
	}
	if scaled.Label() != "20Z68" {
		t.Fatalf("unexpected scaled label: %s", scaled.Label())
	}

	ratio, err := DivideNotes("33Z53", "20Z10")
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(ratio, 11036.537462) {
		t.Fatalf("unexpected ratio: %f", ratio)
	}
}

package snip00

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
)

const tolerance = 1e-6

func roughlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= tolerance*math.Abs(b)
}

func mustParseLabel(label string) Sharenote {
	note, err := parseLabel(label)
	if err != nil {
		panic(err)
	}
	return note
}

func TestNoteConstruction(t *testing.T) {
	note, err := noteFromComponents(33, 53)
	if err != nil {
		t.Fatalf("noteFromComponents: %v", err)
	}
	if note.Z != 33 || note.Cents != 53 {
		t.Fatalf("unexpected note: %+v", note)
	}
	expectedZBits := float64(33) + float64(53)*CentZBitStep
	if !roughlyEqual(note.ZBits, expectedZBits) {
		t.Fatalf("zbits mismatch: got %f want %f", note.ZBits, expectedZBits)
	}
	if note.Label() != "33Z53" {
		t.Fatalf("label mismatch: %s", note.Label())
	}
}

func TestNoteFromZBitsPreservesPrecision(t *testing.T) {
	const raw = 33.537812
	note, err := NoteFromZBits(raw)
	if err != nil {
		t.Fatalf("NoteFromZBits: %v", err)
	}
	if !roughlyEqual(note.ZBits, raw) {
		t.Fatalf("expected zbits %.6f preserved, got %.6f", raw, note.ZBits)
	}
	if note.Label() != "33Z53" {
		t.Fatalf("unexpected label for precise zbits: %s", note.Label())
	}
	if note.Z != 33 || note.Cents != 53 {
		t.Fatalf("unexpected components: %+v", note)
	}
}

func TestParseLabelVariants(t *testing.T) {
	for _, label := range []string{"33Z53", "33Z 53CZ", "33.53Z"} {
		if _, err := parseLabel(label); err != nil {
			t.Fatalf("parseLabel(%s): %v", label, err)
		}
	}
	if note, err := parseLabel("33z"); err != nil || note.Cents != 0 {
		t.Fatalf("parseLabel lower-case: %+v, %v", note, err)
	}
}

func TestProbabilityMath(t *testing.T) {
	note := mustParseLabel("33Z53")
	p, err := ProbabilityPerHash(note)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(p, math.Exp2(-note.ZBits)) {
		t.Fatalf("unexpected probability: %f", p)
	}
	expected, err := ExpectedHashesForNote(note)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(expected.Float64(), 1/p) {
		t.Fatalf("expected hashes mismatch: %f vs %f", expected.Float64(), 1/p)
	}
}

func TestHashesMeasurementString(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{0, "0 hashes"},
		{1, "1.00 H/s"},
		{12.34, "12.3 H/s"},
		{123.4, "123 H/s"},
		{12_340, "12.3 KH/s"},
		{12_340_000, "12.3 MH/s"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%f", tc.value), func(t *testing.T) {
			got := HashesMeasurement{Value: tc.value}.String()
			if got != tc.want {
				t.Fatalf("String() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestHashrateRequirements(t *testing.T) {
	note := mustParseLabel("33Z53")
	mean, err := RequiredHashrateMean(note, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(mean.Float64(), 2.480651469e9) {
		t.Fatalf("mean hashrate mismatch: %f", mean.Float64())
	}
	q95, err := RequiredHashrateQuantile(note, 5, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(q95.Float64(), 7.431367665e9) {
		t.Fatalf("quantile mismatch: %f", q95.Float64())
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

func TestHashrateRangeForNote(t *testing.T) {
	const seconds = 5.0
	const input = 1e12
	note, err := NoteFromHashrate(HashrateValue{Value: input, Unit: HashrateUnitHps}, seconds)
	if err != nil {
		t.Fatal(err)
	}
	rng, err := HashrateRangeForNote(note, seconds)
	if err != nil {
		t.Fatal(err)
	}
	if rng.Min > input || rng.Max <= input {
		t.Fatalf("range [%f, %f) does not contain %f", rng.Min, rng.Max, input)
	}
	lowNote, err := NoteFromHashrate(HashrateValue{Value: rng.Min, Unit: HashrateUnitHps}, seconds)
	if err != nil {
		t.Fatal(err)
	}
	if lowNote.Label() != note.Label() {
		t.Fatalf("expected min bound to map to %s, got %s", note.Label(), lowNote.Label())
	}
}

func TestHashrateRangeReliabilityScaling(t *testing.T) {
	note := mustParseLabel("33Z53")
	base, err := HashrateRangeForNote(note, 5)
	if err != nil {
		t.Fatal(err)
	}
	often, err := HashrateRangeForNote(note, 5, WithReliability(ReliabilityOften95))
	if err != nil {
		t.Fatal(err)
	}
	if often.Min <= base.Min {
		t.Fatalf("expected reliability range min > base min, got %f <= %f", often.Min, base.Min)
	}
	if often.Max <= base.Max {
		t.Fatalf("expected reliability range max > base max, got %f <= %f", often.Max, base.Max)
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
	value, _ := strconv.ParseUint("19752b59", 16, 32)
	exponent := value >> 24
	mantissa := value & 0xFFFFFF
	expected := 256 - (math.Log2(float64(mantissa)) + 8*float64(exponent-3))
	if !roughlyEqual(note.ZBits, expected) {
		t.Fatalf("unexpected zbits: got %f want %f", note.ZBits, expected)
	}
	if note.Label() != "57Z12" {
		t.Fatalf("unexpected label: %s", note.Label())
	}
}

func TestNBitsRoundTrip(t *testing.T) {
	label := "57Z12"
	note := mustParseLabel(label)
	nbits, err := SharenoteToNBits(note)
	if err != nil {
		t.Fatalf("SharenoteToNBits: %v", err)
	}
	rtNote, err := NBitsToSharenote(nbits)
	if err != nil {
		t.Fatalf("NBitsToSharenote: %v", err)
	}
	if rtNote.Label() != label {
		t.Fatalf("round trip mismatch: got %s want %s", rtNote.Label(), label)
	}
}

func TestHumaniseHashratePrecision(t *testing.T) {
	human := HumaniseHashrate(12.34e9, WithHumanHashratePrecision(5))
	expected := fmt.Sprintf("%.5f %s", human.Value, human.Unit)
	if human.Display != expected {
		t.Fatalf("precision formatting mismatch: got %s want %s", human.Display, expected)
	}
}

func TestHumaniseHashrateTinyInputs(t *testing.T) {
	human := HumaniseHashrate(0.25, WithHumanHashratePrecision(2))
	if human.Unit != HashrateUnitHps {
		t.Fatalf("expected H/s unit for tiny hashrates, got %s", human.Unit)
	}
	if human.Display != "0.25 H/s" {
		t.Fatalf("unexpected display for tiny hashrate: %s", human.Display)
	}
}

func TestReliabilityLevels(t *testing.T) {
	levels := ReliabilityLevels()
	if len(levels) == 0 {
		t.Fatal("expected reliability levels")
	}
}

func TestSharenoteConvenienceMethods(t *testing.T) {
	note := mustParseLabel("33Z53")

	probNote, err := note.ProbabilityPerHash()
	if err != nil {
		t.Fatalf("note ProbabilityPerHash: %v", err)
	}
	probFunc, err := ProbabilityPerHash(note)
	if err != nil {
		t.Fatalf("func ProbabilityPerHash: %v", err)
	}
	if !roughlyEqual(probNote, probFunc) {
		t.Fatalf("prob mismatch: note=%f func=%f", probNote, probFunc)
	}

	expectedNote, err := note.ExpectedHashes()
	if err != nil {
		t.Fatalf("note ExpectedHashes: %v", err)
	}
	expectedFunc, err := ExpectedHashesForNote(note)
	if err != nil {
		t.Fatalf("func ExpectedHashesForNote: %v", err)
	}
	if !roughlyEqual(expectedNote.Float64(), expectedFunc.Float64()) {
		t.Fatalf("expected hashes mismatch: note=%f func=%f", expectedNote.Float64(), expectedFunc.Float64())
	}

	const seconds = 5.0
	meanNote, err := note.RequiredHashrateMean(seconds)
	if err != nil {
		t.Fatalf("note RequiredHashrateMean: %v", err)
	}
	meanFunc, err := RequiredHashrateMean(note, seconds)
	if err != nil {
		t.Fatalf("func RequiredHashrateMean: %v", err)
	}
	if !roughlyEqual(meanNote.Float64(), meanFunc.Float64()) {
		t.Fatalf("mean mismatch: note=%f func=%f", meanNote.Float64(), meanFunc.Float64())
	}

	const confidence = 0.95
	quantNote, err := note.RequiredHashrateQuantile(seconds, confidence)
	if err != nil {
		t.Fatalf("note RequiredHashrateQuantile: %v", err)
	}
	quantFunc, err := RequiredHashrateQuantile(note, seconds, confidence)
	if err != nil {
		t.Fatalf("func RequiredHashrateQuantile: %v", err)
	}
	if !roughlyEqual(quantNote.Float64(), quantFunc.Float64()) {
		t.Fatalf("quantile mismatch: note=%f func=%f", quantNote.Float64(), quantFunc.Float64())
	}

	generalNote, err := note.RequiredHashrate(seconds, WithMultiplier(2))
	if err != nil {
		t.Fatalf("note RequiredHashrate: %v", err)
	}
	generalFunc, err := RequiredHashrate(note, seconds, WithMultiplier(2))
	if err != nil {
		t.Fatalf("func RequiredHashrate: %v", err)
	}
	if !roughlyEqual(generalNote.Float64(), generalFunc.Float64()) {
		t.Fatalf("general mismatch: note=%f func=%f", generalNote.Float64(), generalFunc.Float64())
	}

	measurement, err := note.RequiredHashrateMeasurement(seconds)
	if err != nil {
		t.Fatalf("note RequiredHashrateMeasurement: %v", err)
	}
	if !roughlyEqual(measurement.Float64(), meanFunc.Float64()) {
		t.Fatalf("measurement mismatch: %f vs %f", measurement.Float64(), meanFunc.Float64())
	}
	if measurement.String() != measurement.Human().String() {
		t.Fatalf("measurement String mismatch: %s vs %s", measurement.String(), measurement.Human())
	}
	customHuman := measurement.Human(WithHumanHashratePrecision(4))
	expectedDisplay := fmt.Sprintf("%.4f %s", customHuman.Value, customHuman.Unit)
	if customHuman.Display != expectedDisplay {
		t.Fatalf("custom precision mismatch: got %s want %s", customHuman.Display, expectedDisplay)
	}

	meanMeasurement, err := note.RequiredHashrateMeanMeasurement(seconds)
	if err != nil {
		t.Fatalf("note RequiredHashrateMeanMeasurement: %v", err)
	}
	if !roughlyEqual(meanMeasurement.Float64(), meanFunc.Float64()) {
		t.Fatalf("mean measurement mismatch: %f vs %f", meanMeasurement.Float64(), meanFunc.Float64())
	}

	quantMeasurement, err := note.RequiredHashrateQuantileMeasurement(seconds, confidence)
	if err != nil {
		t.Fatalf("note RequiredHashrateQuantileMeasurement: %v", err)
	}
	if !roughlyEqual(quantMeasurement.Float64(), quantFunc.Float64()) {
		t.Fatalf("quant measurement mismatch: %f vs %f", quantMeasurement.Float64(), quantFunc.Float64())
	}

	scaledNote, err := note.Scale(1.5)
	if err != nil {
		t.Fatalf("note Scale: %v", err)
	}
	scaledFunc, err := ScaleNote(note, 1.5)
	if err != nil {
		t.Fatalf("func ScaleNote: %v", err)
	}
	if scaledNote.Label() != scaledFunc.Label() {
		t.Fatalf("scale mismatch: note=%s func=%s", scaledNote.Label(), scaledFunc.Label())
	}

	combinedNote, err := note.CombineSerial("33Z53")
	if err != nil {
		t.Fatalf("note CombineSerial: %v", err)
	}
	combinedFunc, err := CombineNotesSerial(note, "33Z53")
	if err != nil {
		t.Fatalf("func CombineNotesSerial: %v", err)
	}
	if combinedNote.Label() != combinedFunc.Label() {
		t.Fatalf("combine mismatch: note=%s func=%s", combinedNote.Label(), combinedFunc.Label())
	}

	diffNote, err := note.Difference("32Z00")
	if err != nil {
		t.Fatalf("note Difference: %v", err)
	}
	diffFunc, err := NoteDifference(note, "32Z00")
	if err != nil {
		t.Fatalf("func NoteDifference: %v", err)
	}
	if diffNote.Label() != diffFunc.Label() {
		t.Fatalf("difference mismatch: note=%s func=%s", diffNote.Label(), diffFunc.Label())
	}

	targetNote, err := note.Target()
	if err != nil {
		t.Fatalf("note Target: %v", err)
	}
	targetFunc, err := TargetFor(note)
	if err != nil {
		t.Fatalf("func TargetFor: %v", err)
	}
	if targetNote.Cmp(targetFunc) != 0 {
		t.Fatalf("target mismatch: note=%s func=%s", targetNote, targetFunc)
	}

	nbitsNote, err := note.NBits()
	if err != nil {
		t.Fatalf("note NBits: %v", err)
	}
	nbitsFunc, err := SharenoteToNBits(note)
	if err != nil {
		t.Fatalf("func SharenoteToNBits: %v", err)
	}
	if nbitsNote != nbitsFunc {
		t.Fatalf("nbits mismatch: note=%s func=%s", nbitsNote, nbitsFunc)
	}
}

func TestEnsureNote(t *testing.T) {
	note := mustParseLabel("33Z53")
	resolved, err := EnsureNote(note)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Label() != "33Z53" {
		t.Fatalf("ensure note mismatch: %s", resolved.Label())
	}
	resolved, err = EnsureNote(33.53)
	if err != nil {
		t.Fatalf("EnsureNote zbits: %v", err)
	}
	if resolved.Label() != "33Z53" {
		t.Fatalf("unexpected label from zbits: %s", resolved.Label())
	}
	resolved, err = EnsureNote(33)
	if err != nil {
		t.Fatalf("EnsureNote integer: %v", err)
	}
	if resolved.Label() != "33Z00" {
		t.Fatalf("unexpected label from integer zbits: %s", resolved.Label())
	}
	resolved, err = EnsureNote(uint32(1))
	if err != nil {
		t.Fatalf("EnsureNote uint32: %v", err)
	}
	if resolved.Label() != "1Z00" {
		t.Fatalf("unexpected label from uint32 zbits: %s", resolved.Label())
	}
	if _, err := EnsureNote(-1.0); err == nil {
		t.Fatal("expected error for negative zbits input")
	}
	if _, err := EnsureNote(int(-1)); err == nil {
		t.Fatal("expected error for negative integer zbits input")
	}
	if _, err := EnsureNote(true); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestNoteFromCentZBits(t *testing.T) {
	note, err := NoteFromCentZBits(3353)
	if err != nil {
		t.Fatalf("NoteFromCentZBits: %v", err)
	}
	if note.Label() != "33Z53" {
		t.Fatalf("unexpected label: %s", note.Label())
	}
	if note.Z != 33 || note.Cents != 53 {
		t.Fatalf("unexpected components: %+v", note)
	}
	note = MustNoteFromCentZBits(centZUnitsPerZ + 1)
	if note.Label() != "1Z01" {
		t.Fatalf("unexpected label from MustNoteFromCentZBits: %s", note.Label())
	}
	if _, err := NoteFromCentZBits(-1); err == nil {
		t.Fatal("expected error for negative cent-z input")
	}
}

func TestTargetDeterministic(t *testing.T) {
	note := mustParseLabel("57Z12")
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

	if FormatProbabilityDisplay(note.ZBits, 5) != "1 / 2^57.12000" {
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

func TestStringers(t *testing.T) {
	note := mustParseLabel("33Z53")
	if got := fmt.Sprint(note); got != "33Z53" {
		t.Fatalf("unexpected note string: %s", got)
	}

	human := HumaniseHashrate(3.2e9)
	if got := fmt.Sprint(human); got != human.Display {
		t.Fatalf("unexpected hashrate string: %s", got)
	}

	estimate, err := EstimateNote(note, 5)
	if err != nil {
		t.Fatalf("EstimateNote: %v", err)
	}
	summary := fmt.Sprint(estimate)
	for _, want := range []string{"BillEstimate{", "33Z53", "p=1 / 2^"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("estimate string missing %q: %s", want, summary)
		}
	}

	plan := SharenotePlan{
		Sharenote:          note,
		Bill:               estimate,
		SecondsTarget:      5,
		InputHashrateHPS:   estimate.RequiredHashratePrimary,
		InputHashrateHuman: estimate.RequiredHashrateHuman,
	}
	planSummary := fmt.Sprint(plan)
	for _, want := range []string{"SharenotePlan{", "33Z53", "5.00s"} {
		if !strings.Contains(planSummary, want) {
			t.Fatalf("plan string missing %q: %s", want, planSummary)
		}
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
	noteA := mustParseLabel("33Z53")
	noteB := mustParseLabel("20Z10")

	combined, err := CombineNotesSerial("33Z53", "20Z10")
	if err != nil {
		t.Fatal(err)
	}
	if combined.Label() != "33Z53" {
		t.Fatalf("unexpected combined label: %s", combined.Label())
	}
	expectedCombined := math.Log2(math.Pow(2, noteA.ZBits) + math.Pow(2, noteB.ZBits))
	if !roughlyEqual(combined.ZBits, expectedCombined) {
		t.Fatalf("unexpected combined zbits: got %f want %f", combined.ZBits, expectedCombined)
	}

	delta, err := NoteDifference("33Z53", "20Z10")
	if err != nil {
		t.Fatal(err)
	}
	if delta.Label() != "33Z52" {
		t.Fatalf("unexpected delta label: %s", delta.Label())
	}
	expectedDelta := math.Log2(math.Pow(2, noteA.ZBits) - math.Pow(2, noteB.ZBits))
	if !roughlyEqual(delta.ZBits, expectedDelta) {
		t.Fatalf("unexpected delta zbits: got %f want %f", delta.ZBits, expectedDelta)
	}

	scaled, err := ScaleNote("20Z10", 1.5)
	if err != nil {
		t.Fatal(err)
	}
	expectedScaled := math.Log2(math.Pow(2, noteB.ZBits) * 1.5)
	if !roughlyEqual(scaled.ZBits, expectedScaled) {
		t.Fatalf("unexpected scaled zbits: got %f want %f", scaled.ZBits, expectedScaled)
	}
	if scaled.Label() != "20Z68" {
		t.Fatalf("unexpected scaled label: %s", scaled.Label())
	}

	ratio, err := DivideNotes("33Z53", "20Z10")
	if err != nil {
		t.Fatal(err)
	}
	expectedRatio := math.Pow(2, noteA.ZBits) / math.Pow(2, noteB.ZBits)
	if !roughlyEqual(ratio, expectedRatio) {
		t.Fatalf("unexpected ratio: got %f want %f", ratio, expectedRatio)
	}
}

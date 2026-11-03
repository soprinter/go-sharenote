package sharenote

import (
	"math"
	"math/big"
	"testing"
	"time"
)

func TestParseLabelVariants(t *testing.T) {
	cases := []struct {
		in        string
		wantZ     int
		wantCents int
		wantLabel string
	}{
		{"34Z10", 34, 10, "34Z10"},
		{"34Z 10CZ", 34, 10, "34Z10"},
		{"34.10Z", 34, 10, "34Z10"},
		{"34.1Z", 34, 10, "34Z10"},
		{"34z", 34, 0, "34Z00"},
		{"34Z", 34, 0, "34Z00"},
		{"33.537Z", 33, 53, "33Z53"},
	}
	for _, tc := range cases {
		n, err := ParseLabel(tc.in)
		if err != nil {
			t.Fatalf("ParseLabel(%q): %v", tc.in, err)
		}
		if n.Z() != tc.wantZ || n.Cents() != tc.wantCents || n.Label() != tc.wantLabel {
			t.Fatalf("ParseLabel(%q) = Z%d cents%d %q, want Z%d cents%d %q",
				tc.in, n.Z(), n.Cents(), n.Label(), tc.wantZ, tc.wantCents, tc.wantLabel)
		}
	}
	for _, bad := range []string{"", "not-a-note", "zz"} {
		if _, err := ParseLabel(bad); err == nil {
			t.Fatalf("ParseLabel(%q): expected error", bad)
		}
	}
}

func TestZeroNoteIsValid(t *testing.T) {
	var n Note
	if n.Label() != "0Z00" {
		t.Fatalf("zero note label = %q, want 0Z00", n.Label())
	}
	if n.Probability() != 1 {
		t.Fatalf("zero note probability = %v, want 1", n.Probability())
	}
	if got := n.Difficulty(); got.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("zero note difficulty = %s, want 1", got.RatString())
	}
}

func TestFromBitsPreservesPrecision(t *testing.T) {
	const raw = 33.537812
	n, err := FromBits(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(n.Bits(), raw) {
		t.Fatalf("bits = %v, want %v", n.Bits(), raw)
	}
	if n.Label() != "33Z53" {
		t.Fatalf("label = %q, want 33Z53", n.Label())
	}
}

func TestFromBitsErrors(t *testing.T) {
	for _, bad := range []float64{-1, math.Inf(1), math.NaN()} {
		if _, err := FromBits(bad); err == nil {
			t.Fatalf("FromBits(%v): expected error", bad)
		}
	}
}

func TestFromComponentsAndCentBits(t *testing.T) {
	n, err := FromComponents(34, 10)
	if err != nil {
		t.Fatal(err)
	}
	if n.Label() != "34Z10" {
		t.Fatalf("label = %q, want 34Z10", n.Label())
	}
	if _, err := FromComponents(-1, 0); err == nil {
		t.Fatal("expected error for negative Z")
	}
	// cents clamp
	hi, _ := FromComponents(1, 250)
	if hi.Cents() != MaxCents {
		t.Fatalf("cents = %d, want clamp to %d", hi.Cents(), MaxCents)
	}
	cb, err := FromCentBits(3410)
	if err != nil {
		t.Fatal(err)
	}
	if cb.Label() != "34Z10" {
		t.Fatalf("FromCentBits label = %q, want 34Z10", cb.Label())
	}
	if _, err := FromCentBits(-1); err == nil {
		t.Fatal("expected error for negative cent-bits")
	}
}

func TestMustParseLabelPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	MustParseLabel("garbage")
}

func TestProbabilityAndExpectedHashes(t *testing.T) {
	n := MustParseLabel("33Z53")
	if !roughlyEqual(n.Probability(), math.Exp2(-n.Bits())) {
		t.Fatalf("probability mismatch")
	}
	if !roughlyEqual(n.ExpectedHashes(), 1/n.Probability()) {
		t.Fatalf("expected hashes mismatch")
	}
}

func TestTargetMonotonicAndRoundTrip(t *testing.T) {
	a := MustParseLabel("57Z12")
	b := MustParseLabel("57Z13")
	ta, err := a.Target()
	if err != nil {
		t.Fatal(err)
	}
	tb, err := b.Target()
	if err != nil {
		t.Fatal(err)
	}
	if tb.Cmp(ta) >= 0 {
		t.Fatal("harder note should yield a smaller target")
	}
	// Target -> Note round-trip recovers the bits within fixed-point tolerance.
	for _, bits := range []float64{0.5, 10, 33.53, 57.12, 100, 200, 255} {
		n, _ := FromBits(bits)
		target, err := n.Target()
		if err != nil {
			t.Fatalf("Target(%v): %v", bits, err)
		}
		back, err := FromTarget(target)
		if err != nil {
			t.Fatalf("FromTarget(%v): %v", bits, err)
		}
		if math.Abs(back.Bits()-bits) > 0.01 {
			t.Fatalf("round trip bits=%v recovered=%v", bits, back.Bits())
		}
	}
	if _, err := MustNote(t, 257).Target(); err == nil {
		t.Fatal("expected target underflow error for huge note")
	}
	for _, bad := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1)} {
		if _, err := FromTarget(bad); err == nil {
			t.Fatalf("FromTarget(%v): expected error", bad)
		}
	}
}

func TestCompactRoundTrip(t *testing.T) {
	n, err := FromCompact("19752b59")
	if err != nil {
		t.Fatal(err)
	}
	if n.Label() != "57Z12" {
		t.Fatalf("FromCompact label = %q, want 57Z12", n.Label())
	}
	compact, err := MustParseLabel("57Z12").Compact()
	if err != nil {
		t.Fatal(err)
	}
	back, err := FromCompact(compact)
	if err != nil {
		t.Fatal(err)
	}
	if back.Label() != "57Z12" {
		t.Fatalf("compact round trip = %q, want 57Z12", back.Label())
	}
	for _, bad := range []string{"abc", "zzzzzzzz", "19000000"} {
		if _, err := FromCompact(bad); err == nil {
			t.Fatalf("FromCompact(%q): expected error", bad)
		}
	}
}

func TestCmp(t *testing.T) {
	if MustParseLabel("32Z00").Cmp(MustParseLabel("33Z00")) >= 0 {
		t.Fatal("32Z00 should be easier than 33Z00")
	}
	if MustParseLabel("33Z54").Cmp(MustParseLabel("33Z53")) <= 0 {
		t.Fatal("33Z54 should be harder than 33Z53")
	}
	if MustParseLabel("33Z53").Cmp(MustParseLabel("33Z53")) != 0 {
		t.Fatal("equal notes should compare 0")
	}
}

func TestExactArithmeticNoFloatDrift(t *testing.T) {
	// 2Z00 + 2Z00 == 3Z00 exactly (4 + 4 = 8 = 2^3).
	sum, err := Sum(MustParseLabel("2Z00"), MustParseLabel("2Z00"))
	if err != nil {
		t.Fatal(err)
	}
	if sum.Label() != "3Z00" || sum.Difficulty().Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatalf("2Z00+2Z00 = %s (%s), want 3Z00 (8)", sum.Label(), sum.Difficulty().RatString())
	}

	// Adding a tiny note to a huge one keeps the tiny contribution; a float64
	// accumulator would absorb it.
	huge := new(big.Int).Lsh(big.NewInt(1), 60) // 2^60
	if float64(huge.Int64())+2.0 != float64(huge.Int64()) {
		t.Fatal("precondition: float64 should absorb +2 at 2^60")
	}
	big60, _ := FromBits(60)
	one := MustParseLabel("1Z00")
	combined, err := big60.Add(one)
	if err != nil {
		t.Fatal(err)
	}
	want := new(big.Rat).SetInt(new(big.Int).Add(huge, big.NewInt(2)))
	if combined.Difficulty().Cmp(want) != 0 {
		t.Fatalf("exact sum = %s, want %s", combined.Difficulty().RatString(), want.RatString())
	}
}

func TestSumOrderIndependentAndEmpty(t *testing.T) {
	labels := []string{"60Z99", "1Z00", "42Z76", "33Z53", "35Z14", "34Z76"}
	fwd := make([]Note, len(labels))
	rev := make([]Note, len(labels))
	for i, l := range labels {
		fwd[i] = MustParseLabel(l)
		rev[len(labels)-1-i] = MustParseLabel(l)
	}
	a, _ := Sum(fwd...)
	b, _ := Sum(rev...)
	if a.Difficulty().Cmp(b.Difficulty()) != 0 || a.Label() != b.Label() {
		t.Fatalf("Sum not order-independent: %s vs %s", a.Label(), b.Label())
	}
	if _, err := Sum(); err == nil {
		t.Fatal("expected error for empty Sum")
	}
}

func TestSubClampsToZero(t *testing.T) {
	n, err := MustParseLabel("1Z00").Sub(MustParseLabel("2Z00"))
	if err != nil {
		t.Fatal(err)
	}
	if n.Label() != "0Z00" {
		t.Fatalf("clamp label = %q, want 0Z00", n.Label())
	}
}

func TestScaleSemantics(t *testing.T) {
	doubled, err := MustParseLabel("2Z00").Scale(2)
	if err != nil {
		t.Fatal(err)
	}
	if doubled.Label() != "3Z00" || doubled.Difficulty().Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatalf("scale by 2 = %s, want 3Z00", doubled.Label())
	}
	zero, err := MustParseLabel("10Z00").Scale(0)
	if err != nil {
		t.Fatal(err)
	}
	if zero.Label() != "0Z00" {
		t.Fatalf("scale by 0 = %q, want 0Z00", zero.Label())
	}
	// Sub-unit difficulty is outside the representable range (no clamp).
	if _, err := MustParseLabel("1Z00").Scale(0.25); err == nil {
		t.Fatal("expected error for sub-unit scaled difficulty")
	}
	for _, bad := range []float64{-1, math.Inf(1), math.NaN()} {
		if _, err := MustParseLabel("5Z00").Scale(bad); err == nil {
			t.Fatalf("Scale(%v): expected error", bad)
		}
	}
}

func TestRatio(t *testing.T) {
	r, err := MustParseLabel("2Z00").Ratio(MustParseLabel("1Z00"))
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(r, 2) {
		t.Fatalf("ratio = %v, want 2", r)
	}
}

func TestWorkAndPayout(t *testing.T) {
	// Three miners, exact proportional payout, slices never exceed the pool.
	var total Work
	alpha := new(Work).Add(MustParseLabel("30Z00"))
	beta := new(Work).Add(MustParseLabel("30Z00"))
	gamma := new(Work).Add(MustParseLabel("29Z00")) // half of a 30Z note
	total.AddRat(alpha.Rat()).AddRat(beta.Rat()).AddRat(gamma.Rat())

	amount := big.NewInt(1_000_000)
	var paidOut int64
	for _, w := range []*Work{alpha, beta, gamma} {
		p, err := w.Payout(&total, amount)
		if err != nil {
			t.Fatal(err)
		}
		paidOut += p.Int64()
	}
	if paidOut > amount.Int64() {
		t.Fatalf("payouts %d exceed pool total %d", paidOut, amount.Int64())
	}
	// alpha and beta are equal and each should take 2/5 of the pool (400000).
	pa, _ := alpha.Payout(&total, amount)
	if pa.Int64() != 400_000 {
		t.Fatalf("alpha payout = %d, want 400000", pa.Int64())
	}
	// Combined to a note, the total matches Sum of the three.
	tn, err := total.Note()
	if err != nil {
		t.Fatal(err)
	}
	sum, _ := Sum(MustParseLabel("30Z00"), MustParseLabel("30Z00"), MustParseLabel("29Z00"))
	if tn.Difficulty().Cmp(sum.Difficulty()) != 0 {
		t.Fatalf("Work.Note difficulty != Sum: %s vs %s", tn.Difficulty().RatString(), sum.Difficulty().RatString())
	}
	// Payout guards.
	var empty Work
	if _, err := alpha.Payout(&empty, amount); err == nil {
		t.Fatal("expected error for zero total work")
	}
	if _, err := alpha.Payout(&total, nil); err == nil {
		t.Fatal("expected error for nil amount")
	}
}

func TestPlanning(t *testing.T) {
	n := MustParseLabel("33Z53")
	mean, err := n.RequiredHashrate(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(mean, 2.480651469e9) {
		t.Fatalf("mean hashrate = %v, want ~2.48e9", mean)
	}
	q95, err := n.RequiredHashrate(5*time.Second, WithConfidence(0.95))
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(q95, 7.431367665e9) {
		t.Fatalf("q95 hashrate = %v, want ~7.43e9", q95)
	}
	// Reliability preset matches the raw confidence.
	often, err := n.RequiredHashrate(5*time.Second, WithReliability(ReliabilityOften))
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(often, q95) {
		t.Fatalf("reliability preset %v != confidence %v", often, q95)
	}
	// FromHashrate inverts RequiredHashrate.
	back, err := FromHashrate(mean, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if back.Label() != "33Z53" {
		t.Fatalf("FromHashrate label = %q, want 33Z53", back.Label())
	}
	// Error paths.
	if _, err := n.RequiredHashrate(0); err == nil {
		t.Fatal("expected error for zero window")
	}
	if _, err := n.RequiredHashrate(5*time.Second, WithMultiplier(-1)); err == nil {
		t.Fatal("expected error for negative multiplier")
	}
	if _, err := FromHashrate(0, 5*time.Second); err == nil {
		t.Fatal("expected error for zero hashrate")
	}
}

func TestEstimate(t *testing.T) {
	est, err := MustParseLabel("33Z53").Estimate(5*time.Second, WithConfidence(0.95))
	if err != nil {
		t.Fatal(err)
	}
	if est.Confidence != 0.95 {
		t.Fatalf("confidence = %v, want 0.95", est.Confidence)
	}
	if !roughlyEqual(est.RequiredHashrate, 7.431367665e9) {
		t.Fatalf("required hashrate = %v", est.RequiredHashrate)
	}
	if est.String() == "" {
		t.Fatal("expected non-empty estimate string")
	}
}

func TestD7PlanningHelpers(t *testing.T) {
	// MaxZBitsForHashrate is the continuous building block behind FromHashrate.
	bits, err := MaxZBitsForHashrate(1e12, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fh, err := FromHashrate(1e12, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !roughlyEqual(bits, fh.Bits()) {
		t.Fatalf("MaxZBitsForHashrate %v != FromHashrate bits %v", bits, fh.Bits())
	}
	if _, err := MaxZBitsForHashrate(0, 5*time.Second); err == nil {
		t.Fatal("expected error for zero hashrate")
	}

	// HashrateRangeForNote spans the note up to one centi-bit above it.
	n := MustParseLabel("33Z53")
	rng, err := HashrateRangeForNote(n, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	lower, _ := n.RequiredHashrate(5 * time.Second)
	if !roughlyEqual(rng.Min, lower) || rng.Max <= rng.Min {
		t.Fatalf("hashrate range = %+v, want min=%v and max>min", rng, lower)
	}

	// EstimateNotes is a batch of Note.Estimate.
	ests, err := EstimateNotes([]Note{MustParseLabel("30Z00"), MustParseLabel("33Z53")}, 5*time.Second, WithConfidence(0.95))
	if err != nil {
		t.Fatal(err)
	}
	if len(ests) != 2 || ests[1].Confidence != 0.95 {
		t.Fatalf("EstimateNotes = %+v", ests)
	}

	// PlanSharenoteFromHashrate pairs the derived note with a full Estimate.
	plan, err := PlanSharenoteFromHashrate(1e12, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Note.Label() != fh.Label() || plan.InputHashrate != 1e12 || plan.Estimate.Note.Label() != fh.Label() {
		t.Fatalf("plan mismatch: %+v", plan)
	}
}

func TestFormatAndParseHashrate(t *testing.T) {
	if got := FormatHashrate(3.2e9); got != "3.20 GH/s" {
		t.Fatalf("FormatHashrate = %q, want 3.20 GH/s", got)
	}
	if got := FormatHashrate(0); got != "0 H/s" {
		t.Fatalf("FormatHashrate(0) = %q, want 0 H/s", got)
	}
	if got := FormatHashrate(1e24); got != "1000 ZH/s" {
		t.Fatalf("FormatHashrate(1e24) = %q", got)
	}
	cases := map[string]float64{
		"5 GH/s":     5e9,
		"12.5MH/s":   12.5e6,
		"1,000 GH/s": 1000e9,
		"1_000 H/s":  1000,
		"1000":       1000,
	}
	for in, want := range cases {
		got, err := ParseHashrate(in)
		if err != nil {
			t.Fatalf("ParseHashrate(%q): %v", in, err)
		}
		if !roughlyEqual(got, want) {
			t.Fatalf("ParseHashrate(%q) = %v, want %v", in, got, want)
		}
	}
	for _, bad := range []string{"", "12 foo/s", "-5 GH/s", "1e999 GH/s"} {
		if _, err := ParseHashrate(bad); err == nil {
			t.Fatalf("ParseHashrate(%q): expected error", bad)
		}
	}
}

func TestFormatProbability(t *testing.T) {
	if got := FormatProbability(33.53, 5); got != "1 / 2^33.53000" {
		t.Fatalf("FormatProbability = %q", got)
	}
	if FormatProbability(10, -1) == "" {
		t.Fatal("negative digits should clamp, not blank")
	}
}

// MustNote builds a note from bits for tests, failing the test on error.
func MustNote(t *testing.T, bits float64) Note {
	t.Helper()
	n, err := FromBits(bits)
	if err != nil {
		t.Fatalf("FromBits(%v): %v", bits, err)
	}
	return n
}

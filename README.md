# go-sharenote

go-sharenote is the reference Go toolkit for [Sharenote](https://snip00.xyz) clients.

> Compatible with Go 1.20+ (no external dependencies).

## Installation

```bash
go get github.com/soprinter/go-sharenote
```

---

## Quick Start

```go
package main

import (
	"fmt"

	"github.com/soprinter/go-sharenote/snip00"
)

func main() {
	note := snip00.MustParseLabel("33Z53")
	fmt.Println(note.Label(), note.Bits) // 33Z53 33.53

	// Probability & planning
	p, _ := snip00.ProbabilityPerHash(note)
	fmt.Printf("P = %.12f\n", p) // 0.000000000081

	hashrate95, _ := snip00.RequiredHashrateQuantile(note, 5, 0.95)
	fmt.Printf("Quantile 95%%: %.3f GH/s\n", hashrate95/1e9) // 7.431 GH/s

	rig := snip00.HashrateValue{Value: 2, Unit: snip00.HashrateUnitGHps}
	fmt.Println(snip00.NoteFromHashrate(rig, 5).Label()) // 33Z21

	planned, _ := snip00.PlanSharenoteFromHashrate(
		snip00.HashrateValue{Value: 5, Unit: snip00.HashrateUnitGHps},
		5,
		snip00.WithPlanReliability(snip00.ReliabilityOften95),
	)
	fmt.Println(planned.snip00.Label())                  // "32Z95"
	fmt.Println(planned.Bill.RequiredHashrateHuman.Display) // "5.00 GH/s"

	// Compact difficulty
	fmt.Println(snip00.NBitsToSharenote("19752b59").Label()) // 57Z12

	// Report-ready output
	report, _ := snip00.EstimateNote(note, 5, snip00.WithEstimateConfidence(0.95))
	fmt.Println(report.ProbabilityDisplay)            // "1 / 2^33.53000"
	fmt.Println(report.RequiredHashrateHuman.Display) // "7.43 GH/s"

	// Arithmetic helpers
	serial, _ := snip00.CombineNotesSerial("33Z53", "20Z10")
	fmt.Println(serial.Label()) // 33Z53

	diff, _ := snip00.NoteDifference("33Z53", "20Z10")
	fmt.Println(diff.Label()) // 33Z52

	scaled, _ := snip00.ScaleNote("20Z10", 1.5)
	fmt.Printf("%s %.9f\n", scaled.Label(), scaled.Bits) // 20Z68 20.680000000

	ratio, _ := snip00.DivideNotes("33Z53", "20Z10")
	fmt.Printf("ratio: %.4f\n", ratio) // 11036.5375

	// Human-friendly formatting & parsing helpers
	fmt.Println(snip00.HumaniseHashrate(hashrate95).Display) // "7.43 GH/s"
	parsed, _ := snip00.ParseHashrate("12.5 MH/s")
	fmt.Printf("parsed: %.0f H/s\n", parsed)
}
```

---

## Feature Matrix

| Theme | Functions | Notes |
|-------|-----------|-------|
| Conversions | `ParseLabel`, `NoteFromComponents`, `NoteFromBits`, `BitsFromComponents`, `NBitsToSharenote` | Strict validation, cent clamping (`0–99`). |
| Probability | `ProbabilityPerHash`, `ExpectedHashesForNote` | Deterministic doubles. |
| Planning | `ParseHashrate`, `NoteFromHashrate`, `PlanSharenoteFromHashrate`, `RequiredHashrate*`, `MaxBitsForHashrate` | Accept raw confidence, enum presets (e.g. `ReliabilityOften95`), or explicit multipliers. |
| Reports | `EstimateNote`, `EstimateNotes`, `FormatProbabilityDisplay`, `HumaniseHashrate` | Produce `BillEstimate` structs with machine and human fields. |
| Arithmetic | `CombineNotesSerial`, `NoteDifference`, `ScaleNote`, `DivideNotes` | Compose serial probability, compute gaps, apply scalars, compare ratios. |
| Utilities | `ReliabilityLevels`, `CompareNotes`, `TargetFor` | Enumerate presets, sort by rarity, or emit `*big.Int` targets. |

All functions return `(value, error)` to make failure modes explicit. Use `MustParseLabel` for test fixtures or pre-validated data.

---

## Recipes

```go
// Sequential proofs (add bit difficulty)
serial, _ := snip00.CombineNotesSerial("33Z53", "20Z10")
fmt.Println(serial.Label()) // 33Z53

// Difference between notes
gap, _ := snip00.NoteDifference("33Z53", "20Z10")
fmt.Println(gap.Label()) // 33Z52

// Tooling for dashboards
rows, _ := snip00.EstimateNotes(
	[]any{"33Z53", "30Z00"},
	5,
	snip00.WithEstimateReliability(snip00.ReliabilityVeryLikely99),
)
for _, row := range rows {
	fmt.Println(row.Label, row.RequiredHashrateHuman.Display)
}
```

---

## Testing

```bash
go test ./...
```

The repository is gofmt/go vet clean.

---

## License

Creative Commons CC0 1.0 Universal

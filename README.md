# go-sharenote

The reference Go SDK for [Sharenote](https://sharenote.xyz), the NIP-SN
proof-of-work bearer-note protocol. It gives you the exact Z-arithmetic that
makes notes comparable and summable, and builders and parsers for every NIP-SN
Nostr event.

> Pure standard library plus `golang.org/x/crypto` and `github.com/nbd-wtf/go-nostr`.

## Install

```bash
go get github.com/soprinter/go-sharenote
```

## Packages

| Import | What it does |
|--------|--------------|
| `github.com/soprinter/go-sharenote` | The Z-arithmetic: the `Note` denomination, exact difficulty/target maths, aggregation, and the `Work` accumulator for pool accounting. |
| `github.com/soprinter/go-sharenote/event` | Builders and parsers for the NIP-SN event kinds (identity, telemetry, invoices, shares, raw sharenotes, operator messaging). |

## Z-arithmetic (`sharenote`)

A [`Note`](https://pkg.go.dev/github.com/soprinter/go-sharenote#Note) is a
denomination of proof-of-work in Z-bits — the base-2 log of its continuous
difficulty. Its label (e.g. `34Z10`) is a pricing-bucket floor, never a
verification value.

```go
import "github.com/soprinter/go-sharenote"

n, _ := sharenote.ParseLabel("34.10Z")
fmt.Println(n.Label(), n.Z(), n.Cents()) // 34Z10 34 10

// Aggregation happens in linear difficulty space, not by label.
total, _ := sharenote.Sum(
    sharenote.MustParseLabel("29Z50"),
    sharenote.MustParseLabel("29Z50"),
)
fmt.Println(total.Label()) // 30Z50

// Planning helpers are explicitly float (display only).
hr, _ := n.RequiredHashrate(5*time.Second, sharenote.WithConfidence(0.95))
fmt.Println(sharenote.FormatHashrate(hr))
```

### Exact accounting, never `float64`

NIP-SN rule A1 forbids using `float64` as a sequential accumulator: summing
millions of micro-shares in IEEE-754 silently drops fractional hashes. Every
`Note` keeps its exact difficulty as a `math/big.Rat`, and all aggregation
(`Add`, `Sum`, `Sub`, `Scale`, and the `Work` accumulator) stays exact — a label
is derived only for display. Use `Work` to sum shares and slice a reward by the
proportional law:

```go
alpha := new(sharenote.Work).Add(minerNote)
var pool sharenote.Work
pool.AddRat(alpha.Rat()).AddRat(betaWork.Rat())

payout, _ := alpha.Payout(&pool, big.NewInt(312_500_000)) // exact base units
```

## Building NIP-SN events (`event`)

Each domain type builds a `*nostr.Event` (you set `CreatedAt` and sign it), and
each `Parse*` function validates the kind and required tags. Deeper
§Validation-Constraints checks live on the `Validate` methods.

```go
import "github.com/soprinter/go-sharenote/event"

inv := event.Invoice{
    Height:    843000, // HeightHash ("d" tag) is derived for you
    BlockHash: blockHash,
    Amount:    312_500_000,
    Workers:   128,
    Shares:    "58Z42",
}
ev, _ := inv.UnpaidEvent() // kind 35500, ready to sign

got, _ := event.ParseShare(relayEvent) // kind + required-tag checked
_ = got.Validate(event.KindPendingShare)
```

Event kinds covered: miner identity `10520`, pool identity `10521`, unpaid /
settled invoice `35500` / `35501`, hashrate telemetry `35502`, pending /
finalized / payment share `35503` / `35504` / `35505`, raw sharenote / AuxPoW
`35510`, and operator messaging `35515`.

## Migration from the `snipXX` packages (0.1.x → 0.2.0)

The spec-numbered packages are gone. The old numbering mapped to NIP-SN domain
concepts as follows.

| Old (`0.1.x`) | New (`0.2.0`) |
|---------------|---------------|
| `snip00.Sharenote` | `sharenote.Note` |
| `snip00.EnsureNote(x)` / `NoteFromZBits` | `sharenote.ParseLabel` / `FromBits` / `FromComponents` |
| `snip00.MustNoteFromZBits` | `sharenote.MustParseLabel` / `FromBits` |
| `snip00.NoteFromCentZBits` | `sharenote.FromCentBits` |
| `snip00.NBitsToSharenote` / `SharenoteToNBits` | `sharenote.FromCompact` / `Note.Compact` |
| `snip00.TargetFor` / `ContinuousDifficulty` | `Note.Target` / `sharenote.FromTarget` |
| `snip00.CombineNotesSerial(...)` | `sharenote.Sum(...)` / `Note.Add` |
| `snip00.NoteDifference` / `ScaleNote` / `DivideNotes` | `Note.Sub` / `Note.Scale` / `Note.Ratio` |
| `note.DifficultyRat()` | `Note.Difficulty()` |
| `snip00.CompareNotes` | `Note.Cmp` |
| `snip00.ProbabilityPerHash` / `ExpectedHashesForNote` | `Note.Probability` / `Note.ExpectedHashes` |
| `snip00.RequiredHashrate*` | `Note.RequiredHashrate` (opts: `WithConfidence`, `WithReliability`, `WithMultiplier`) |
| `snip00.MaxZBitsForHashrate` | `sharenote.MaxZBitsForHashrate` |
| `snip00.NoteFromHashrate` | `sharenote.FromHashrate` |
| `snip00.HashrateRangeForNote` | `sharenote.HashrateRangeForNote` (returns `sharenote.HashrateRange`) |
| `snip00.PlanSharenoteFromHashrate` | `sharenote.PlanSharenoteFromHashrate` (returns `sharenote.Plan`) |
| `snip00.EstimateNote` / `BillEstimate` | `Note.Estimate` / `sharenote.EstimateNotes` (batch) |
| `snip00.HumaniseHashrate` / `ParseHashrate` | `sharenote.FormatHashrate` / `ParseHashrate` |
| `snip00.ReliabilityOften95` (enum) | `sharenote.ReliabilityOften` (value) |
| `snip02.Hashrate` / `NewHashrateEvent` | `event.Telemetry` / `Telemetry.Event` |
| `snip03.Invoice` / `NewInvoiceEvent` / `NewSettledInvoiceEvent` | `event.Invoice` / `Invoice.UnpaidEvent` / `Invoice.SettledEvent` |
| `snip03.Share` / `New{Pending,Finalized,SharePayment}Event` | `event.Share` / `Share.{Pending,Finalized,Payment}Event` |
| `snip03.Transaction` | `event.Settlement` |
| `snip03.ComputeHeightHash` / `ComputePendingShareID` / `ComputePaymentShareID` | `event.HeightHash` / `PendingShareID` / `FinalizedShareID` |
| `snip04.Sharenote` / `NewZBitEvent` (kind 35510) | `event.RawSharenote` / `RawSharenote.Event` (NIP-SN tag format) |
| `snip05.MinerIdentity` / `PoolIdentity` (kinds 35506/35507) | `event.MinerIdentity` / `event.PoolIdentity` (kinds 10520/10521) |

## Testing

```bash
go build ./... && go test ./... && go vet ./...
```

The cross-SDK conformance fixtures live in `testdata/` and are asserted against
the public API, byte-for-byte identical across the Go, JS, and Python SDKs (a
sha256 guard pins each one):

- `snip00_tests.json` — §Z-Arithmetic vectors (labels, combine, difference,
  scale, ratio).
- `sdk_edge_tests.json` — the D1–D4 edge vectors (comparison ordering,
  near-integer carry, exact fold composition, `int64` round-trip).
- `validation_tests.json` — forged/invalid events every `Validate` MUST reject
  plus genuine controls it MUST accept (coordinate equality A7/A9/A10, raw
  header hash + leading-zero bound A14, 35505 `chain` A13, identity/telemetry).

## License

Creative Commons CC0 1.0 Universal.

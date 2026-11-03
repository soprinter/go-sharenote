// Package sharenote implements the Z-arithmetic of the NIP-SN Sharenote
// protocol: the exact difficulty, density, denomination, and target maths that
// make proof-of-work notes comparable, summable, and verifiable.
//
// A [Note] is a denomination of proof-of-work measured in Z-bits — the base-2
// logarithm of its continuous difficulty D (the hash-space inverse of a 256-bit
// target). A note carries a human-readable label such as "34Z10" (34 whole
// bits, 10 centi-bits) that acts as a pricing-bucket floor, never as a
// verification value.
//
// # Exact arithmetic
//
// Aggregating notes happens in linear difficulty space, not in log space:
// 1Z00 + 1Z00 == 2Z00, not "2Z00" by string concatenation. The NIP-SN spec
// (§Z-Arithmetic, rule A1) forbids using float64 as a sequential accumulator,
// because IEEE-754 precision loss silently drops fractional hashes when summing
// millions of micro-shares and reopens the halt-early attack.
//
// This package therefore does all accounting arithmetic with math/big: every
// [Note] keeps its exact difficulty as a [math/big.Rat], and [Note.Add],
// [Note.Sub], [Note.Scale], [Sum], and the [Work] accumulator operate on those
// rationals and round to a float label only for display. float64 appears only
// on explicit display and planning helpers ([Note.Probability],
// [Note.RequiredHashrate], [FormatHashrate], …), never on the accounting path.
//
// # Building events
//
// The Nostr event kinds that carry Sharenote identity, telemetry, pool
// accounting, and raw merged-mining proofs live in the sub-package
// github.com/soprinter/go-sharenote/event.
package sharenote

# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

SDK remediation to the frozen cross-SDK validation contract — behaviour parity
with the JS and Python SDKs on every input, not just the conformance vectors.

### Added

- Coordinate-equality validation (NIP-SN A7/A9/A10): `Invoice` and `Share` now
  retain the on-wire `HeightHash`/`ShareID`, and `Validate` recomputes and
  rejects a forged `d`/`h` coordinate that an earlier build silently discarded.
- Full raw-sharenote verification (A14): `RawSharenote.Validate(primaryChainID)`
  checks the byte-reversed double-SHA256 of `dd` against `d` (clause 1) **and**
  that the hash clears the leading-zero bound implied by `z` (clause 2, the
  previously-missing check), enforces the declared primary chain is present and
  first, and `ParseRawSharenote` now rejects a `w` tag without exactly six
  elements and a `dd` that is not the last tag.
- Re-added planning helpers for surface parity with JS/Python:
  `MaxZBitsForHashrate`, `HashrateRangeForNote` (+ `HashrateRange`),
  `PlanSharenoteFromHashrate` (+ `Plan`), and `EstimateNotes`.
- The shared edge (`sdk_edge_tests.json`, D1–D4) and negative
  (`validation_tests.json`) fixtures are wired into `go test`, with a sha256
  guard pinning all three fixtures and a forged-coordinate regression test.

### Changed

- Kind 35505 (Share Payment) now **requires** an explicit `chain` tag (A13 /
  decision D5): the silent auto-injection of `"15"` is dropped and a missing
  chain is an error. The `"15"` default remains only for the optional `chain`
  on 35503/35504.
- Telemetry worker (`w:<name>`) tags are sorted alphabetically by the builder
  (decision D6).
- Amount/fee/threshold documentation now says "the coin's base unit" (satoshi
  only as an example), per decision D8; on-wire tag names are unchanged.
- README: removed the phantom `sharenote.Estimate`, completed the migration
  table for the re-added planning helpers, and documented all three fixtures.

## [0.2.0] - 2026-11-02

Baseline of the redesigned, domain-oriented public API. The SDK is now
organised around what Sharenote users do — exact Z-arithmetic and building or
parsing NIP-SN Nostr events — instead of the old spec-numbered `snipXX`
packages. This release replaces the previous surface wholesale.

### Added

- Cohesive top-level package `sharenote` for the NIP-SN Z-arithmetic: the
  strongly typed, zero-value-friendly `Note` denomination, exact `math/big.Rat`
  aggregation (`Note.Add`, `Sum`, `Note.Sub`, `Note.Scale`, `Note.Ratio`), the
  `Work` accumulator, and `Work.Payout` implementing the proportional payout law
  with exact integer rounding.
- Strongly typed constructors (`ParseLabel`, `FromBits`, `FromComponents`,
  `FromCentBits`, `FromTarget`, `FromCompact`, `FromHashrate`) and target /
  compact-nBits conversion on `Note`.
- Display- and planning-only float helpers kept off the accounting path:
  `Note.Probability`, `Note.RequiredHashrate`, `Note.Estimate`, the
  `Reliability` presets, `FormatHashrate`, `ParseHashrate`, and
  `FormatProbability`.
- Sub-package `sharenote/event` with builders and parsers for every NIP-SN kind
  — miner identity (10520), pool identity (10521), unpaid/settled invoices
  (35500/35501), hashrate telemetry (35502), pending/finalized/payment shares
  (35503/35504/35505), raw sharenote / AuxPoW (35510), and operator messaging
  (35515) — plus `HeightHash`, `PendingShareID`, and `FinalizedShareID` helpers
  and per-type `Validate` methods for the §Validation Constraints rules.
- Runnable `Example` tests for the core and event packages, and the cross-SDK
  §Z-Arithmetic conformance vectors are asserted against the new API.

### Changed

- Event builders and parsers now follow the consolidated **NIP-SN** spec
  exactly. In particular the raw sharenote (35510) and identity (10520/10521)
  events were rebuilt around the NIP-SN tag formats; the identity kinds moved
  from the former 35506/35507 to 10520/10521.

### Removed

- The spec-numbered packages `snip00`, `snip02`, `snip03`, `snip04`, `snip05`
  and the `internal/hash` and `internal/tags` packages. See the migration table
  in `README.md` for old-to-new name mappings.
- The `any`-typed note helpers (`EnsureNote`, variadic `CombineNotesSerial`,
  etc.) in favour of the strongly typed `Note` API.

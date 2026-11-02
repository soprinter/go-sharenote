# Changelog

## [0.1.3]

### Changed

- Reworks difficulty aggregation to be exact: total work is accumulated as
  `math/big.Rat` instead of floating-point estimates, and `DifficultyRat()` is
  added to return the exact rational difficulty. (#2)

module github.com/soprinter/go-sharenote

go 1.25.5

require (
	github.com/ohstr/nmilat v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.45.0
)

require (
	github.com/decred/dcrd/crypto/blake256 v1.1.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/flokiorg/go-flokicoin v0.25.12-alpha // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/ohstr/nmilat => /u/flzpace/xgit/orgs/ohstr/nmilat

retract v0.1.0 // Retired version.

package hash

import (
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/blake2b"
)

// Int64 returns the blake2b-256 hex digest of the decimal string of v.
// Used for HeightHash: the "d" tag on invoices.
func Int64(v int64) string {
	data := []byte(fmt.Sprintf("%d", v))
	sum := blake2b.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ShareID returns blake2b-256 hex of "{height}/{address}/{worker}".
// Used for pending share (35503) "d" tags.
func ShareID(height int64, address, worker string) string {
	data := []byte(fmt.Sprintf("%d/%s/%s", height, address, worker))
	sum := blake2b.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// PaymentShareID returns blake2b-256 hex of "{height}/{address}".
// Used for finalized share (35504) and share payment (35505) "d" tags.
func PaymentShareID(height int64, address string) string {
	data := []byte(fmt.Sprintf("%d/%s", height, address))
	sum := blake2b.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Hex returns the blake2b-256 hex digest of arbitrary data.
func Hex(data []byte) string {
	sum := blake2b.Sum256(data)
	return hex.EncodeToString(sum[:])
}

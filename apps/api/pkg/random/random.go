// Package random provides cryptographically secure random identifiers,
// used for JWT `jti` claims and anywhere else a guessable ID would be a
// security bug.
package random

import (
	"crypto/rand"
	"encoding/hex"
)

// Hex returns a random hex string encoding n random bytes (so length
// 2n), suitable as a JWT jti or a one-off token.
func Hex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

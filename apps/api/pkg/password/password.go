// Package password wraps bcrypt so callers never touch a raw hashing
// algorithm or cost factor directly.
package password

import "golang.org/x/crypto/bcrypt"

// Hash bcrypt-hashes a plaintext password at the library default cost.
func Hash(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// Verify reports whether plain matches the given bcrypt hash. It never
// distinguishes "wrong password" from "malformed hash" to the caller —
// both are just "no".
func Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

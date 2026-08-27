// Package jwt issues and verifies signed access/refresh tokens.
//
// Access and refresh tokens are both JWTs, signed with separate secrets
// (JWT_SECRET / JWT_REFRESH_SECRET) via separate Manager instances — see
// cmd/api/main.go. Access tokens are stateless and never touch the
// database. Refresh tokens carry a `jti` that internal/auth's repository
// tracks in the refresh_tokens table, which is what makes rotation and
// revocation possible: verifying the signature only proves the token
// hasn't been tampered with, not that it hasn't already been rotated out
// or explicitly revoked.
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"satelit-parfume-api/pkg/random"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Claims is deliberately minimal: enough to authorize a request without a
// database round-trip, nothing that changes often enough to go stale
// inside a token's lifetime.
type Claims struct {
	SubjectType string   `json:"typ"` // "user" (staff) or "customer"
	Roles       []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Issue signs a new token for subjectID/subjectType (and roles, for
// staff). It returns the signed token, its jti (needed by refresh tokens
// so the caller can persist it for later revocation), and its expiry.
func (m *Manager) Issue(subjectID, subjectType string, roles []string) (signed, jti string, expiresAt time.Time, err error) {
	jti, err = random.Hex(16)
	if err != nil {
		return "", "", time.Time{}, err
	}

	expiresAt = time.Now().Add(m.ttl)
	claims := Claims{
		SubjectType: subjectType,
		Roles:       roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subjectID,
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err = token.SignedString(m.secret)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return signed, jti, expiresAt, nil
}

// Verify checks signature and expiry and returns the parsed claims. It
// does NOT check revocation — that requires the database and is the
// caller's job (see internal/auth.Repository.IsRefreshTokenValid) when
// verifying a refresh token specifically. Access tokens are never checked
// against the database at all; that's the point of them being stateless.
func (m *Manager) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

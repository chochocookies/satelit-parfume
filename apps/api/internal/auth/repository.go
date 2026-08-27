package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRefreshTokenInvalid = errors.New("refresh token invalid, expired, or already used")

// Repository tracks issued refresh tokens by their JWT `jti`. The token
// itself is never stored — only enough to answer "is this jti still
// valid" and to revoke it.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) StoreRefreshToken(ctx context.Context, jti, subjectType, subjectID string, expiresAt time.Time) error {
	const q = `
		INSERT INTO refresh_tokens (id, subject_type, subject_id, expires_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.db.Exec(ctx, q, jti, subjectType, subjectID, expiresAt)
	return err
}

// ConsumeRefreshToken checks that jti is currently valid (exists, not
// revoked, not expired) and revokes it in the same statement. This is
// what makes rotation work: a given refresh token can be exchanged for a
// new pair exactly once, so a copied/leaked token that gets used by
// someone else will fail the *legitimate* client's next refresh —
// visibly, instead of silently letting both keep working.
func (r *Repository) ConsumeRefreshToken(ctx context.Context, jti string) error {
	const q = `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()
	`
	tag, err := r.db.Exec(ctx, q, jti)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRefreshTokenInvalid
	}
	return nil
}

// RevokeRefreshToken revokes a token unconditionally. Used by logout,
// where a token that's already expired or already used should still
// result in a successful logout from the client's perspective.
func (r *Repository) RevokeRefreshToken(ctx context.Context, jti string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.db.Exec(ctx, q, jti)
	return err
}

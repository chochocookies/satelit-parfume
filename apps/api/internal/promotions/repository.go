package promotions

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("promo code not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const promoColumns = `
	id, code, description, discount_type, discount_value, min_purchase,
	max_discount, max_uses, used_count, valid_from, valid_until, active, created_at
`

func scanPromo(row interface{ Scan(...any) error }) (*PromoCode, error) {
	var p PromoCode
	var description *string
	err := row.Scan(
		&p.ID, &p.Code, &description, &p.DiscountType, &p.DiscountValue, &p.MinPurchase,
		&p.MaxDiscount, &p.MaxUses, &p.UsedCount, &p.ValidFrom, &p.ValidUntil, &p.Active, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if description != nil {
		p.Description = *description
	}
	return &p, nil
}

// GetByCode is case-insensitive — "satelit10" and "SATELIT10" are the
// same code from a shopper's side, and requiring exact-case entry on a
// touchscreen or phone keyboard is a good way to make a real discount
// look broken. Read-only, no locking — see Service.Validate's own doc
// comment on when this is the right one to call instead of
// GetByCodeForUpdate below.
func (r *Repository) GetByCode(ctx context.Context, code string) (*PromoCode, error) {
	row := r.db.QueryRow(ctx, `SELECT `+promoColumns+` FROM promo_codes WHERE UPPER(code) = UPPER($1)`, code)
	p, err := scanPromo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetByCodeForUpdate is GetByCode's row-locking twin, used only inside
// Checkout's own transaction (internal/orders' Service.Checkout):
// SELECT ... FOR UPDATE holds this row locked until that transaction
// commits or rolls back, so two checkouts racing for the last use of a
// max_uses-limited code can't both read "1 use left" and both succeed —
// the second has to wait for the first to finish, then sees the
// updated count.
func (r *Repository) GetByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (*PromoCode, error) {
	row := tx.QueryRow(ctx, `SELECT `+promoColumns+` FROM promo_codes WHERE UPPER(code) = UPPER($1) FOR UPDATE`, code)
	p, err := scanPromo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// IncrementUsage runs inside Checkout's own transaction — see
// GetByCodeForUpdate's doc comment on why that matters.
func (r *Repository) IncrementUsage(ctx context.Context, tx pgx.Tx, promoID string) error {
	_, err := tx.Exec(ctx, `UPDATE promo_codes SET used_count = used_count + 1 WHERE id = $1`, promoID)
	return err
}

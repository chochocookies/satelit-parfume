package memberships

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// TotalSpent sums every COMPLETED order's total for customerID — the
// same terminal "actually received it" status internal/reviews' own
// eligibility check keys off, for the same reason: a still-open or
// cancelled order shouldn't count toward a loyalty tier any more than
// it should earn a review.
func (r *Repository) TotalSpent(ctx context.Context, customerID string) (int64, error) {
	const q = `SELECT COALESCE(SUM(total), 0) FROM orders WHERE customer_id = $1 AND status = 'COMPLETED'`
	var total int64
	err := r.db.QueryRow(ctx, q, customerID).Scan(&total)
	return total, err
}

package reviews

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("review not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// LatestCompletedOrder returns the most recent COMPLETED order this
// customer has that actually contained productID — via order_items'
// product_variant_id, the same join path stock bookkeeping already
// relies on (order_items' own migration comment: "kept for stock
// release/deduction bookkeeping only", now with a second use). Returns
// ("", false, nil) rather than an error when there's no such order —
// "not eligible to review this" is an ordinary outcome, not a failure.
func (r *Repository) LatestCompletedOrder(ctx context.Context, customerID, productID string) (string, bool, error) {
	const q = `
		SELECT o.id
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		JOIN product_variants pv ON pv.id = oi.product_variant_id
		WHERE o.customer_id = $1 AND pv.product_id = $2 AND o.status = 'COMPLETED'
		ORDER BY o.created_at DESC
		LIMIT 1
	`
	var orderID string
	err := r.db.QueryRow(ctx, q, customerID, productID).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return orderID, true, nil
}

// Upsert is how both "leave a review" and "edit my review" happen — see
// migration 000012's own comment on why a second review just overwrites
// the first rather than being rejected outright.
func (r *Repository) Upsert(ctx context.Context, customerID, productID, orderID string, rating int, comment string) error {
	const q = `
		INSERT INTO product_reviews (customer_id, product_id, order_id, rating, comment)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT (customer_id, product_id) DO UPDATE SET
			order_id   = EXCLUDED.order_id,
			rating     = EXCLUDED.rating,
			comment    = EXCLUDED.comment,
			updated_at = now()
	`
	_, err := r.db.Exec(ctx, q, customerID, productID, orderID, rating, comment)
	return err
}

func (r *Repository) Delete(ctx context.Context, customerID, productID string) error {
	const q = `DELETE FROM product_reviews WHERE customer_id = $1 AND product_id = $2`
	tag, err := r.db.Exec(ctx, q, customerID, productID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListForProduct returns every review on productID, most recent first,
// with the reviewer's display name joined in (see Review's own doc
// comment on why that's denormalized into the read rather than looked
// up separately).
func (r *Repository) ListForProduct(ctx context.Context, productID string) ([]Review, error) {
	const q = `
		SELECT pr.id, c.name, pr.rating, pr.comment, pr.created_at, pr.updated_at
		FROM product_reviews pr
		JOIN customers c ON c.id = pr.customer_id
		WHERE pr.product_id = $1
		ORDER BY pr.created_at DESC
	`
	rows, err := r.db.Query(ctx, q, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Review, 0)
	for rows.Next() {
		var rv Review
		var comment *string
		if err := rows.Scan(&rv.ID, &rv.CustomerName, &rv.Rating, &comment, &rv.CreatedAt, &rv.UpdatedAt); err != nil {
			return nil, err
		}
		if comment != nil {
			rv.Comment = *comment
		}
		list = append(list, rv)
	}
	return list, rows.Err()
}

func (r *Repository) SummaryForProduct(ctx context.Context, productID string) (Summary, error) {
	const q = `SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM product_reviews WHERE product_id = $1`
	var s Summary
	err := r.db.QueryRow(ctx, q, productID).Scan(&s.Average, &s.Count)
	return s, err
}

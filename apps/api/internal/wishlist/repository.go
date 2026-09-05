package wishlist

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"satelit-parfume-api/internal/products"
)

// ErrNotFound backs Remove only — Add is intentionally idempotent (see
// its own doc comment), so it never has a not-found case of its own.
var ErrNotFound = errors.New("product is not in your wishlist")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Add is idempotent — wishlisting something already saved just confirms
// it's there rather than erroring. From the shopper's side, tapping an
// already-filled heart icon twice (a slow connection, an accidental
// double-tap on a touchscreen) isn't a mistake worth surfacing as one.
func (r *Repository) Add(ctx context.Context, customerID, productID string) error {
	const q = `
		INSERT INTO wishlist_items (customer_id, product_id)
		VALUES ($1, $2)
		ON CONFLICT (customer_id, product_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, q, customerID, productID)
	return err
}

func (r *Repository) Remove(ctx context.Context, customerID, productID string) error {
	const q = `DELETE FROM wishlist_items WHERE customer_id = $1 AND product_id = $2`
	tag, err := r.db.Exec(ctx, q, customerID, productID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns the customer's saved products, most recently added
// first. Same SELECT-list shape (and the same primary-image correlated
// subquery) as products.Repository's own List — see listSelectColumns —
// just joined off wishlist_items and without the branch-stock subquery,
// since a wishlist page isn't scoped to one branch the way the shop
// listing can be. p.deleted_at IS NULL is carried over from the same
// query for the same reason: the FK's ON DELETE CASCADE only cleans up
// a wishlist_items row on a hard delete, and this schema soft-deletes
// products instead.
func (r *Repository) List(ctx context.Context, customerID string) ([]Entry, error) {
	const q = `
		SELECT
			p.id, p.name, p.slug, p.status, p.is_featured, p.is_bestseller,
			b.id, b.name, b.slug,
			COALESCE(MIN(v.base_price) FILTER (WHERE v.status = 'active'), 0) AS price_from,
			(
				SELECT pi.image_url FROM product_images pi
				WHERE pi.product_id = p.id
				ORDER BY pi.is_primary DESC, pi.sort_order ASC
				LIMIT 1
			) AS primary_image,
			wi.created_at
		FROM wishlist_items wi
		JOIN products p ON p.id = wi.product_id AND p.deleted_at IS NULL
		LEFT JOIN brands b ON b.id = p.brand_id
		LEFT JOIN product_variants v ON v.product_id = p.id
		WHERE wi.customer_id = $1
		GROUP BY p.id, p.name, p.slug, p.status, p.is_featured, p.is_bestseller, b.id, b.name, b.slug, wi.created_at
		ORDER BY wi.created_at DESC
	`
	rows, err := r.db.Query(ctx, q, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		var brandID, brandName, brandSlug *string
		var primaryImage *string

		err := rows.Scan(
			&e.Product.ID, &e.Product.Name, &e.Product.Slug, &e.Product.Status,
			&e.Product.IsFeatured, &e.Product.IsBestseller,
			&brandID, &brandName, &brandSlug,
			&e.Product.PriceFrom,
			&primaryImage,
			&e.AddedAt,
		)
		if err != nil {
			return nil, err
		}

		if brandID != nil {
			e.Product.Brand = &products.Brand{ID: *brandID, Name: *brandName, Slug: *brandSlug}
		}
		if primaryImage != nil {
			e.Product.PrimaryImage = *primaryImage
		}

		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// IsWishlisted backs a single product detail page's heart-icon initial
// state (section-84-style product detail, one product at a time) —
// List's full join is overkill just to answer "is this one already
// saved".
func (r *Repository) IsWishlisted(ctx context.Context, customerID, productID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM wishlist_items WHERE customer_id = $1 AND product_id = $2)`
	var exists bool
	err := r.db.QueryRow(ctx, q, customerID, productID).Scan(&exists)
	return exists, err
}

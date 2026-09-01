package cart

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"satelit-parfume-api/pkg/random"
)

var (
	ErrNotFound     = errors.New("cart not found")
	ErrItemNotFound = errors.New("cart item not found")
)

// shell is the bare cart row — id, branch, and (for guests) the token
// identifying it. The full Cart (with items and computed totals) is
// assembled by Service, which is why this stays unexported: nothing
// outside this package should work with a cart that has no items loaded.
type shell struct {
	ID           string
	BranchID     *string
	SessionToken *string
	UpdatedAt    time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const shellColumns = `id, branch_id, session_token, updated_at`

func (r *Repository) FindByCustomerID(ctx context.Context, customerID string) (*shell, error) {
	return r.scanShell(ctx, `SELECT `+shellColumns+` FROM carts WHERE customer_id = $1`, customerID)
}

func (r *Repository) FindBySessionToken(ctx context.Context, token string) (*shell, error) {
	return r.scanShell(ctx, `SELECT `+shellColumns+` FROM carts WHERE session_token = $1`, token)
}

func (r *Repository) scanShell(ctx context.Context, query, arg string) (*shell, error) {
	var s shell
	err := r.db.QueryRow(ctx, query, arg).Scan(&s.ID, &s.BranchID, &s.SessionToken, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repository) CreateForCustomer(ctx context.Context, customerID string) (*shell, error) {
	const q = `INSERT INTO carts (customer_id) VALUES ($1) RETURNING ` + shellColumns
	var s shell
	if err := r.db.QueryRow(ctx, q, customerID).Scan(&s.ID, &s.BranchID, &s.SessionToken, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateGuestCart generates its own opaque token (the same crypto/rand
// helper JWT jtis use) so a first-time guest always gets a real cart to
// add to, not an empty response with nothing to identify it by next time.
func (r *Repository) CreateGuestCart(ctx context.Context) (*shell, error) {
	token, err := random.Hex(16)
	if err != nil {
		return nil, err
	}

	const q = `INSERT INTO carts (session_token) VALUES ($1) RETURNING ` + shellColumns
	var s shell
	if err := r.db.QueryRow(ctx, q, token).Scan(&s.ID, &s.BranchID, &s.SessionToken, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) SetBranch(ctx context.Context, cartID, branchID string) error {
	_, err := r.db.Exec(ctx, `UPDATE carts SET branch_id = $2, updated_at = now() WHERE id = $1`, cartID, branchID)
	return err
}

const itemColumns = `
	ci.id, ci.product_variant_id, p.name, p.slug, pv.name, ci.branch_id,
	ci.quantity, ci.unit_price, bi_stock.available_stock
`

// itemsFrom LEFT JOINs each item's *live* available stock (not the
// unit_price snapshot, which intentionally stays frozen at add-time) so
// callers can flag "only N left" without a second round trip.
const itemsFrom = `
	FROM cart_items ci
	JOIN product_variants pv ON pv.id = ci.product_variant_id
	JOIN products p ON p.id = pv.product_id
	LEFT JOIN branch_inventory bi_stock
		ON bi_stock.branch_id = ci.branch_id AND bi_stock.product_variant_id = ci.product_variant_id
`

func (r *Repository) ItemsFor(ctx context.Context, cartID string) ([]Item, error) {
	query := `SELECT ` + itemColumns + `, ci.created_at ` + itemsFrom + ` WHERE ci.cart_id = $1 ORDER BY ci.created_at ASC`

	rows, err := r.db.Query(ctx, query, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *Repository) GetItem(ctx context.Context, cartID, itemID string) (*Item, error) {
	query := `SELECT ` + itemColumns + `, ci.created_at ` + itemsFrom + ` WHERE ci.id = $1 AND ci.cart_id = $2`

	row := r.db.QueryRow(ctx, query, itemID, cartID)
	it, err := scanItem(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotFound
		}
		return nil, err
	}
	return &it, nil
}

func scanItem(row interface{ Scan(...any) error }) (Item, error) {
	var it Item
	var availableStock *int
	err := row.Scan(
		&it.ID, &it.ProductVariantID, &it.ProductName, &it.ProductSlug, &it.VariantName, &it.BranchID,
		&it.Quantity, &it.UnitPrice, &availableStock, &it.CreatedAt,
	)
	if err != nil {
		return Item{}, err
	}
	if availableStock != nil {
		it.AvailableStock = *availableStock
	}
	it.LineTotal = it.UnitPrice * int64(it.Quantity)
	return it, nil
}

// ExistingQuantity returns the quantity already in the cart for this
// variant, or 0 if it isn't there yet — Service uses this to validate the
// COMBINED quantity (existing + requested) against available stock, since
// adding again increases quantity rather than replacing it.
func (r *Repository) ExistingQuantity(ctx context.Context, cartID, variantID string) (int, error) {
	var qty int
	err := r.db.QueryRow(ctx,
		`SELECT quantity FROM cart_items WHERE cart_id = $1 AND product_variant_id = $2`,
		cartID, variantID,
	).Scan(&qty)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return qty, nil
}

// UpsertItem inserts a new cart line, or adds to an existing one's
// quantity if this exact (cart, variant) pair is already in the cart —
// see the ON CONFLICT clause, matching cart_items' own UNIQUE (cart_id,
// product_variant_id) constraint (migration 000005_cart). unitPrice only
// takes effect on first insert — an existing line keeps its original
// snapshot price; adding more of something already in the cart doesn't
// re-price the earlier units to today's price.
//
// Bug fix (found while helping debug a real "every add-to-cart fails
// with a foreign-key violation" report, after Phase 11): this INSERT's
// column list read (cart_id, product_variant_id, branch_id, ...) while
// its VALUES were bound positionally as (cartID, branchID, variantID,
// ...) — branchID and variantID were silently swapped into each other's
// columns. Every add-to-cart call sent a real, valid variant id, and it
// landed in the branch_id column instead of product_variant_id; a real
// branch id landed in product_variant_id, which almost never happens to
// also be a valid row in product_variants, so the INSERT failed its own
// foreign-key check on a value that was never actually wrong — just in
// the wrong column. Fixed by reordering the column list to match the
// existing positional args instead of the other way around: cartID,
// branchID, and variantID here are unchanged from every existing call
// site (internal/cart/service.go), so nothing that calls this needed to
// change.
func (r *Repository) UpsertItem(ctx context.Context, cartID, branchID, variantID string, quantity int, unitPrice int64) error {
	const q = `
		INSERT INTO cart_items (cart_id, branch_id, product_variant_id, quantity, unit_price)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cart_id, product_variant_id) DO UPDATE SET
			quantity = cart_items.quantity + EXCLUDED.quantity,
			updated_at = now()
	`
	_, err := r.db.Exec(ctx, q, cartID, branchID, variantID, quantity, unitPrice)
	return err
}

func (r *Repository) UpdateItemQuantity(ctx context.Context, cartID, itemID string, quantity int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE cart_items SET quantity = $3, updated_at = now() WHERE id = $1 AND cart_id = $2`,
		itemID, cartID, quantity,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrItemNotFound
	}
	return nil
}

func (r *Repository) RemoveItem(ctx context.Context, cartID, itemID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM cart_items WHERE id = $1 AND cart_id = $2`, itemID, cartID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrItemNotFound
	}
	return nil
}

// Clear removes every item and releases the cart's branch lock — an
// empty cart shouldn't stay pinned to a branch the next item might not
// even be sold at.
func (r *Repository) Clear(ctx context.Context, cartID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed below

	if _, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE carts SET branch_id = NULL, updated_at = now() WHERE id = $1`, cartID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

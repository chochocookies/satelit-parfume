package inventory

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("inventory item not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const itemColumns = `
	bi.id, bi.branch_id, bi.product_variant_id,
	p.name, p.slug, pv.name,
	bi.stock_quantity, bi.reserved_quantity, bi.available_stock, bi.minimum_stock,
	bi.price, bi.status, bi.updated_at
`

const itemFrom = `
	FROM branch_inventory bi
	JOIN product_variants pv ON pv.id = bi.product_variant_id
	JOIN products p ON p.id = pv.product_id
`

// ListForBranch returns every inventory row for one branch — the
// staff/admin dashboard view (section 37).
func (r *Repository) ListForBranch(ctx context.Context, branchID string) ([]Item, error) {
	query := `SELECT ` + itemColumns + itemFrom + ` WHERE bi.branch_id = $1 ORDER BY p.name ASC`

	rows, err := r.db.Query(ctx, query, branchID)
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

func scanItem(row interface{ Scan(...any) error }) (Item, error) {
	var it Item
	err := row.Scan(
		&it.ID, &it.BranchID, &it.ProductVariantID,
		&it.ProductName, &it.ProductSlug, &it.VariantName,
		&it.StockQuantity, &it.ReservedQuantity, &it.AvailableStock, &it.MinimumStock,
		&it.Price, &it.Status, &it.UpdatedAt,
	)
	return it, err
}

// SetStock upserts the (branchID, variantID) row — see the package doc
// comment for why this is a direct write, not a movement-logged one.
func (r *Repository) SetStock(ctx context.Context, branchID, variantID string, req SetStockRequest) (*Item, error) {
	const q = `
		INSERT INTO branch_inventory (branch_id, product_variant_id, stock_quantity, minimum_stock, price)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (branch_id, product_variant_id) DO UPDATE SET
			stock_quantity = EXCLUDED.stock_quantity,
			minimum_stock  = EXCLUDED.minimum_stock,
			price          = EXCLUDED.price,
			updated_at     = now()
		RETURNING id
	`
	var id string
	err := r.db.QueryRow(ctx, q, branchID, variantID, req.StockQuantity, req.MinimumStock, req.Price).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.getByID(ctx, id)
}

func (r *Repository) getByID(ctx context.Context, id string) (*Item, error) {
	query := `SELECT ` + itemColumns + itemFrom + ` WHERE bi.id = $1`
	row := r.db.QueryRow(ctx, query, id)

	it, err := scanItem(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &it, nil
}

// AvailabilityForProduct backs the public ?branch= param on
// GET /products/:slug (internal/products calls this directly). It resolves
// the product's cheapest active variant, then that branch's stock for
// that specific variant — branch.price overrides the variant's base_price
// when set, same rule SetStock/branch_inventory.price documents.
func (r *Repository) AvailabilityForProduct(ctx context.Context, branchSlug, productSlug string) (*Availability, error) {
	const q = `
		WITH variant AS (
			SELECT pv.id, pv.base_price
			FROM products p
			JOIN product_variants pv ON pv.product_id = p.id
			WHERE p.slug = $2 AND p.deleted_at IS NULL AND pv.status = 'active'
			ORDER BY pv.base_price ASC
			LIMIT 1
		)
		SELECT b.slug, b.name,
		       COALESCE(bi.available_stock, 0),
		       COALESCE(bi.price, variant.base_price)
		FROM branches b
		CROSS JOIN variant
		LEFT JOIN branch_inventory bi ON bi.branch_id = b.id AND bi.product_variant_id = variant.id
		WHERE b.slug = $1 AND b.deleted_at IS NULL
	`
	var a Availability
	err := r.db.QueryRow(ctx, q, branchSlug, productSlug).Scan(&a.BranchSlug, &a.BranchName, &a.AvailableStock, &a.Price)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// AvailableStockForVariant returns stock and unit price for one specific
// (branchID, variantID) pair — unlike AvailabilityForProduct, which picks
// "the cheapest variant" for a product-detail page with no variant
// selected yet, cart operations already know exactly which variant
// they're adding. price is the branch's override if set, else the
// variant's own base_price — same rule as everywhere else this matters.
func (r *Repository) AvailableStockForVariant(ctx context.Context, branchID, variantID string) (available int, unitPrice int64, err error) {
	const q = `
		SELECT COALESCE(bi.available_stock, 0), COALESCE(bi.price, pv.base_price)
		FROM product_variants pv
		LEFT JOIN branch_inventory bi ON bi.branch_id = $1 AND bi.product_variant_id = pv.id
		WHERE pv.id = $2 AND pv.status = 'active'
	`
	err = r.db.QueryRow(ctx, q, branchID, variantID).Scan(&available, &unitPrice)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, err
	}
	return available, unitPrice, nil
}

// ── Reservation (section 21) — all three run inside a caller-supplied
// transaction, since checkout reserves several lines atomically and a
// status transition deducts/releases them the same way. ──

var ErrInsufficientStock = errors.New("insufficient stock")

// ReserveStock locks the branch_inventory row (SELECT ... FOR UPDATE) so
// two concurrent checkouts racing for the last unit can't both succeed —
// the second transaction blocks until the first commits or rolls back,
// then re-reads the now-current available_stock. Returns
// ErrInsufficientStock (not a generic error) when there isn't enough,
// including when no branch_inventory row exists at all for this variant.
func (r *Repository) ReserveStock(ctx context.Context, tx pgx.Tx, branchID, variantID string, quantity int) error {
	var available int
	err := tx.QueryRow(ctx,
		`SELECT available_stock FROM branch_inventory WHERE branch_id = $1 AND product_variant_id = $2 FOR UPDATE`,
		branchID, variantID,
	).Scan(&available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientStock
		}
		return err
	}
	if available < quantity {
		return ErrInsufficientStock
	}

	_, err = tx.Exec(ctx,
		`UPDATE branch_inventory SET reserved_quantity = reserved_quantity + $3, updated_at = now()
		 WHERE branch_id = $1 AND product_variant_id = $2`,
		branchID, variantID, quantity,
	)
	return err
}

// ReleaseStock gives reserved_quantity back without touching
// stock_quantity — nothing was ever actually sold, so there's nothing to
// restock, just a hold to let go of. Used when an order is cancelled or
// expires before payment. GREATEST(0, ...) is a defensive floor: correctly
// guarded callers should never drive this negative, but it costs nothing
// to make the invariant hold at the SQL level too.
func (r *Repository) ReleaseStock(ctx context.Context, tx pgx.Tx, branchID, variantID string, quantity int) error {
	_, err := tx.Exec(ctx,
		`UPDATE branch_inventory SET reserved_quantity = GREATEST(0, reserved_quantity - $3), updated_at = now()
		 WHERE branch_id = $1 AND product_variant_id = $2`,
		branchID, variantID, quantity,
	)
	return err
}

// DeductStock converts a reservation into an actual sale: stock_quantity
// and reserved_quantity both drop by quantity, so available_stock (their
// difference) doesn't change — the unit was already excluded from
// "available" the moment it was reserved; this just makes that permanent.
// Called when an order transitions PENDING_PAYMENT → PAID.
func (r *Repository) DeductStock(ctx context.Context, tx pgx.Tx, branchID, variantID string, quantity int) error {
	_, err := tx.Exec(ctx,
		`UPDATE branch_inventory
		 SET stock_quantity = GREATEST(0, stock_quantity - $3),
		     reserved_quantity = GREATEST(0, reserved_quantity - $3),
		     updated_at = now()
		 WHERE branch_id = $1 AND product_variant_id = $2`,
		branchID, variantID, quantity,
	)
	return err
}

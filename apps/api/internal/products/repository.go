package products

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"satelit-parfume-api/internal/categories"
	"satelit-parfume-api/pkg/slug"
)

var (
	ErrNotFound = errors.New("product not found")
	// ErrDuplicateName/ErrDuplicateSKU back the admin manual create/update
	// path (Phase 9) — CSV import already had this same duplicate-name
	// check (see importRow) but only ever surfaced it as a per-row skip
	// reason string, never a real error, since a batch import shouldn't
	// abort over one duplicate. A single admin form submission is
	// different: the staff member submitting it needs to know right away.
	ErrDuplicateName = errors.New("a product with this name already exists")
	ErrDuplicateSKU  = errors.New("that SKU or barcode is already used by another product")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Reads ──────────────────────────────────────────────────────────────

// listSelectColumns takes the arg position of the (possibly empty)
// branch-slug parameter and returns the full SELECT list, including a
// per-product correlated subquery that resolves that branch's stock for
// this product's cheapest active variant. When branchSlug is "" (no
// branch selected), the subquery's br.slug = $N simply matches no row —
// no real branch has an empty slug — so branch_stock comes back NULL,
// same as if the column weren't requested at all. This keeps the query
// text static regardless of whether a branch was selected, rather than
// conditionally assembling a different SELECT list per case.
func listSelectColumns(branchArgPos int) string {
	return fmt.Sprintf(`
		p.id, p.name, p.slug, p.status, p.is_featured, p.is_bestseller,
		b.id, b.name, b.slug,
		c.id, c.name, c.slug,
		COALESCE(MIN(v.base_price) FILTER (WHERE v.status = 'active'), 0) AS price_from,
		(
			SELECT pi.image_url FROM product_images pi
			WHERE pi.product_id = p.id
			ORDER BY pi.is_primary DESC, pi.sort_order ASC
			LIMIT 1
		) AS primary_image,
		(
			SELECT bi.available_stock
			FROM branch_inventory bi
			JOIN branches br ON br.id = bi.branch_id
			WHERE br.slug = $%d
			  AND bi.product_variant_id = (
			      SELECT pv2.id FROM product_variants pv2
			      WHERE pv2.product_id = p.id AND pv2.status = 'active'
			      ORDER BY pv2.base_price ASC LIMIT 1
			  )
		) AS branch_stock
	`, branchArgPos)
}

const listFrom = `
	FROM products p
	LEFT JOIN brands b ON b.id = p.brand_id
	LEFT JOIN categories c ON c.id = p.category_id
	LEFT JOIN product_variants v ON v.product_id = p.id
`

func (r *Repository) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 24
	}

	where, args := buildWhere(f)
	whereSQL := strings.Join(where, " AND ")

	total, err := r.count(ctx, whereSQL, args)
	if err != nil {
		return ListResult{}, fmt.Errorf("count products: %w", err)
	}

	items, err := r.selectPage(ctx, whereSQL, args, f)
	if err != nil {
		return ListResult{}, fmt.Errorf("select products: %w", err)
	}

	totalPages := (total + f.Limit - 1) / f.Limit
	if totalPages < 1 {
		totalPages = 1
	}

	return ListResult{
		Items:      items,
		Total:      total,
		Page:       f.Page,
		Limit:      f.Limit,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) count(ctx context.Context, whereSQL string, args []any) (int, error) {
	q := fmt.Sprintf(`
		SELECT COUNT(DISTINCT p.id)
		%s
		WHERE %s
	`, listFrom, whereSQL)

	var total int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *Repository) selectPage(ctx context.Context, whereSQL string, args []any, f ListFilter) ([]ListItem, error) {
	pageArgs := make([]any, len(args))
	copy(pageArgs, args)

	pageArgs = append(pageArgs, f.BranchSlug)
	branchArgPos := len(pageArgs)

	pageArgs = append(pageArgs, f.Limit, (f.Page-1)*f.Limit)
	limitPos := len(pageArgs) - 1
	offsetPos := len(pageArgs)

	q := fmt.Sprintf(`
		SELECT %s
		%s
		WHERE %s
		GROUP BY p.id, b.id, c.id
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, listSelectColumns(branchArgPos), listFrom, whereSQL, sortClause(f.Sort), limitPos, offsetPos)

	rows, err := r.db.Query(ctx, q, pageArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ListItem, 0)
	for rows.Next() {
		var item ListItem
		var brandID, brandName, brandSlug *string
		var catID, catName, catSlug *string
		var primaryImage *string

		err := rows.Scan(
			&item.ID, &item.Name, &item.Slug, &item.Status, &item.IsFeatured, &item.IsBestseller,
			&brandID, &brandName, &brandSlug,
			&catID, &catName, &catSlug,
			&item.PriceFrom,
			&primaryImage,
			&item.BranchStock,
		)
		if err != nil {
			return nil, err
		}

		if brandID != nil {
			item.Brand = &Brand{ID: *brandID, Name: *brandName, Slug: *brandSlug}
		}
		if catID != nil {
			item.Category = &CategoryRef{ID: *catID, Name: *catName, Slug: *catSlug}
		}
		if primaryImage != nil {
			item.PrimaryImage = *primaryImage
		}

		items = append(items, item)
	}
	return items, rows.Err()
}

// buildWhere turns a ListFilter into parameterized SQL fragments. Every
// user-supplied value goes through $N placeholders — string formatting is
// only ever used to place the placeholder syntax itself, never a value.
func buildWhere(f ListFilter) (clauses []string, args []any) {
	if f.AdminView {
		clauses = []string{"p.deleted_at IS NULL"}
	} else {
		clauses = []string{"p.deleted_at IS NULL", "p.status = 'active'"}
	}

	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.Search != "" {
		p := next("%" + f.Search + "%")
		clauses = append(clauses, fmt.Sprintf(
			"(p.name ILIKE %s OR p.sku ILIKE %s OR p.barcode ILIKE %s OR b.name ILIKE %s OR c.name ILIKE %s)",
			p, p, p, p, p,
		))
	}
	if f.CategorySlug != "" {
		clauses = append(clauses, fmt.Sprintf("c.slug = %s", next(f.CategorySlug)))
	}
	if f.BrandSlug != "" {
		clauses = append(clauses, fmt.Sprintf("b.slug = %s", next(f.BrandSlug)))
	}
	if f.Gender != "" {
		clauses = append(clauses, fmt.Sprintf("p.gender = %s", next(f.Gender)))
	}
	if f.AdminView && f.Status != "" {
		clauses = append(clauses, fmt.Sprintf("p.status = %s", next(f.Status)))
	}

	return clauses, args
}

// sortClause is a fixed whitelist, never string-built from user input —
// ORDER BY can't be parameterized with $N the way values can, so this is
// the one place a naive implementation could turn into SQL injection.
func sortClause(sort string) string {
	switch sort {
	case "newest":
		return "p.created_at DESC"
	case "price_asc":
		return "price_from ASC"
	case "price_desc":
		return "price_from DESC"
	default:
		return "p.name ASC"
	}
}

const detailColumns = `
	p.id, p.name, p.slug, p.sku, p.barcode, p.description, p.short_description,
	p.size, p.gender, p.fragrance_family, p.status, p.is_featured, p.is_bestseller,
	p.created_at, p.updated_at,
	b.id, b.name, b.slug,
	c.id, c.name, c.slug
`

const detailFrom = `
	FROM products p
	LEFT JOIN brands b ON b.id = p.brand_id
	LEFT JOIN categories c ON c.id = p.category_id
`

func (r *Repository) GetBySlug(ctx context.Context, productSlug string) (*Detail, error) {
	return r.getDetail(ctx, "p.slug = $1", productSlug)
}

// GetByID is GetBySlug's by-id counterpart, added for Phase 9's admin
// product forms — they navigate and update by id, not slug. Shares
// getDetail with GetBySlug so the two read paths can never quietly
// drift apart.
func (r *Repository) GetByID(ctx context.Context, id string) (*Detail, error) {
	return r.getDetail(ctx, "p.id = $1", id)
}

func (r *Repository) getDetail(ctx context.Context, whereClause, arg string) (*Detail, error) {
	q := fmt.Sprintf(`SELECT %s %s WHERE %s AND p.deleted_at IS NULL`, detailColumns, detailFrom, whereClause)

	var d Detail
	var sku, barcode, description, shortDesc, size, gender, family *string
	var brandID, brandName, brandSlug *string
	var catID, catName, catSlug *string

	err := r.db.QueryRow(ctx, q, arg).Scan(
		&d.ID, &d.Name, &d.Slug, &sku, &barcode, &description, &shortDesc,
		&size, &gender, &family, &d.Status, &d.IsFeatured, &d.IsBestseller,
		&d.CreatedAt, &d.UpdatedAt,
		&brandID, &brandName, &brandSlug,
		&catID, &catName, &catSlug,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	d.SKU = deref(sku)
	d.Barcode = deref(barcode)
	d.Description = deref(description)
	d.ShortDescription = deref(shortDesc)
	d.Size = deref(size)
	d.Gender = deref(gender)
	d.FragranceFamily = deref(family)
	if brandID != nil {
		d.Brand = &Brand{ID: *brandID, Name: *brandName, Slug: *brandSlug}
	}
	if catID != nil {
		d.Category = &CategoryRef{ID: *catID, Name: *catName, Slug: *catSlug}
	}

	variants, err := r.variantsFor(ctx, d.ID)
	if err != nil {
		return nil, fmt.Errorf("load variants: %w", err)
	}
	d.Variants = variants

	images, err := r.imagesFor(ctx, d.ID)
	if err != nil {
		return nil, fmt.Errorf("load images: %w", err)
	}
	d.Images = images

	return &d, nil
}

func (r *Repository) variantsFor(ctx context.Context, productID string) ([]Variant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, sku, barcode, size, base_price, status
		FROM product_variants
		WHERE product_id = $1
		ORDER BY base_price ASC
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]Variant, 0)
	for rows.Next() {
		var v Variant
		var sku, barcode, size *string
		if err := rows.Scan(&v.ID, &v.Name, &sku, &barcode, &size, &v.BasePrice, &v.Status); err != nil {
			return nil, err
		}
		v.SKU, v.Barcode, v.Size = deref(sku), deref(barcode), deref(size)
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func (r *Repository) imagesFor(ctx context.Context, productID string) ([]Image, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, image_url, alt_text, sort_order, is_primary
		FROM product_images
		WHERE product_id = $1
		ORDER BY is_primary DESC, sort_order ASC
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]Image, 0)
	for rows.Next() {
		var img Image
		var alt *string
		if err := rows.Scan(&img.ID, &img.ImageURL, &alt, &img.SortOrder, &img.IsPrimary); err != nil {
			return nil, err
		}
		img.AltText = deref(alt)
		images = append(images, img)
	}
	return images, rows.Err()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ── Import (see import.go for CSV parsing/row validation) ──────────────

func (r *Repository) nameExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM products WHERE lower(name) = lower($1) AND deleted_at IS NULL)`,
		name,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := slug.Generate(name)
	candidate := base

	for i := 2; ; i++ {
		var exists bool
		err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE slug = $1)`, candidate).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

func (r *Repository) ensureBrand(ctx context.Context, tx pgx.Tx, name string) (string, error) {
	s := slug.Generate(name)

	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM brands WHERE slug = $1`, s).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	err = tx.QueryRow(ctx, `INSERT INTO brands (name, slug) VALUES ($1, $2) RETURNING id`, name, s).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Import inserts every valid, non-duplicate row from rows and reports
// what happened to each one. See importRow for why each row gets its own
// transaction rather than the whole batch sharing one.
func (r *Repository) Import(ctx context.Context, categoriesRepo *categories.Repository, rows []ImportRow) (ImportResult, error) {
	result := ImportResult{}
	seenInFile := make(map[string]bool)

	for _, row := range rows {
		price, problems := validateRow(row)
		if len(problems) > 0 {
			result.Skipped++
			result.Errors = append(result.Errors, RowError{Row: row.RowNumber, Reason: strings.Join(problems, "; ")})
			continue
		}

		nameKey := strings.ToLower(row.Name)
		if seenInFile[nameKey] {
			result.Skipped++
			result.Errors = append(result.Errors, RowError{Row: row.RowNumber, Reason: "duplicate name within this file"})
			continue
		}

		imported, skipReason, err := r.importRow(ctx, categoriesRepo, row, price)
		if err != nil {
			return result, fmt.Errorf("row %d: %w", row.RowNumber, err)
		}
		if !imported {
			result.Skipped++
			result.Errors = append(result.Errors, RowError{Row: row.RowNumber, Reason: skipReason})
			continue
		}

		seenInFile[nameKey] = true
		result.Imported++
	}

	return result, nil
}

// importRow inserts one product + its default variant + optional image
// inside a single transaction, so a mid-row failure can never leave a
// product with no variant behind, and one bad row can't roll back rows
// already committed earlier in the same import.
func (r *Repository) importRow(ctx context.Context, categoriesRepo *categories.Repository, row ImportRow, price int64) (imported bool, skipReason string, err error) {
	exists, err := r.nameExists(ctx, row.Name)
	if err != nil {
		return false, "", fmt.Errorf("check existing product: %w", err)
	}
	if exists {
		return false, "a product with this name already exists — import is idempotent, so it was left alone", nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed below

	var categoryID *string
	if row.Category != "" {
		id, err := categoriesRepo.EnsureByName(ctx, tx, row.Category)
		if err != nil {
			return false, "", fmt.Errorf("resolve category: %w", err)
		}
		categoryID = &id
	}

	var brandID *string
	if row.Brand != "" {
		id, err := r.ensureBrand(ctx, tx, row.Brand)
		if err != nil {
			return false, "", fmt.Errorf("resolve brand: %w", err)
		}
		brandID = &id
	}

	productSlug, err := r.uniqueSlug(ctx, row.Name)
	if err != nil {
		return false, "", fmt.Errorf("generate slug: %w", err)
	}

	var productID string
	const insertProduct = `
		INSERT INTO products (brand_id, category_id, name, slug, sku, barcode, description, size, gender, status)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), 'active')
		RETURNING id
	`
	err = tx.QueryRow(ctx, insertProduct,
		brandID, categoryID, row.Name, productSlug, row.SKU, row.Barcode, row.Description, row.Size, row.Gender,
	).Scan(&productID)
	if err != nil {
		return false, "", fmt.Errorf("insert product: %w", err)
	}

	const insertVariant = `
		INSERT INTO product_variants (product_id, name, base_price, status)
		VALUES ($1, 'Default', $2, 'active')
	`
	if _, err := tx.Exec(ctx, insertVariant, productID, price); err != nil {
		return false, "", fmt.Errorf("insert default variant: %w", err)
	}

	if row.ImageURL != "" {
		const insertImage = `
			INSERT INTO product_images (product_id, image_url, sort_order, is_primary)
			VALUES ($1, $2, 0, true)
		`
		if _, err := tx.Exec(ctx, insertImage, productID, row.ImageURL); err != nil {
			return false, "", fmt.Errorf("insert image: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, "", fmt.Errorf("commit: %w", err)
	}

	return true, "", nil
}

// ── Admin manual CRUD (Phase 9) ─────────────────────────────────────────
//
// Phase 3 deliberately shipped CSV import without a manual single-product
// create/edit path — see that phase's own note on why. These three
// methods are that path, and they lean on importRow's exact resolution
// helpers (ensureBrand, categoriesRepo.EnsureByName, uniqueSlug,
// nameExists) so a product typed into the admin form and a product that
// came from a CSV row can never resolve "the same" category or brand
// name to two different rows.

// AdminCreate inserts a new product plus its single 'Default' variant
// (and optional primary image) inside one transaction — the same
// atomicity importRow uses, for the same reason: a product should never
// be able to exist with zero variants, even for a moment.
func (r *Repository) AdminCreate(ctx context.Context, categoriesRepo *categories.Repository, req ProductUpsertRequest) (*Detail, error) {
	exists, err := r.nameExists(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check existing product: %w", err)
	}
	if exists {
		return nil, ErrDuplicateName
	}

	status := req.Status
	if status == "" {
		status = "active"
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed below

	var categoryID *string
	if req.Category != "" {
		id, err := categoriesRepo.EnsureByName(ctx, tx, req.Category)
		if err != nil {
			return nil, fmt.Errorf("resolve category: %w", err)
		}
		categoryID = &id
	}

	var brandID *string
	if req.Brand != "" {
		id, err := r.ensureBrand(ctx, tx, req.Brand)
		if err != nil {
			return nil, fmt.Errorf("resolve brand: %w", err)
		}
		brandID = &id
	}

	productSlug, err := r.uniqueSlug(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("generate slug: %w", err)
	}

	var productID string
	const insertProduct = `
		INSERT INTO products (
			brand_id, category_id, name, slug, sku, barcode, description, short_description,
			size, gender, fragrance_family, status, is_featured, is_bestseller
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
			NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), $12, $13, $14
		)
		RETURNING id
	`
	err = tx.QueryRow(ctx, insertProduct,
		brandID, categoryID, req.Name, productSlug, req.SKU, req.Barcode, req.Description, req.ShortDescription,
		req.Size, req.Gender, req.FragranceFamily, status, req.IsFeatured, req.IsBestseller,
	).Scan(&productID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateSKU
		}
		return nil, fmt.Errorf("insert product: %w", err)
	}

	const insertVariant = `
		INSERT INTO product_variants (product_id, name, base_price, status)
		VALUES ($1, 'Default', $2, 'active')
	`
	if _, err := tx.Exec(ctx, insertVariant, productID, req.Price); err != nil {
		return nil, fmt.Errorf("insert default variant: %w", err)
	}

	if req.ImageURL != "" {
		const insertImage = `
			INSERT INTO product_images (product_id, image_url, sort_order, is_primary)
			VALUES ($1, $2, 0, true)
		`
		if _, err := tx.Exec(ctx, insertImage, productID, req.ImageURL); err != nil {
			return nil, fmt.Errorf("insert image: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.getDetail(ctx, "p.id = $1", productID)
}

// AdminUpdate replaces every editable field on an existing product — the
// same full-replace shape as branches.Update, and for the same reason:
// no partial-PATCH semantics to reason about. The slug is deliberately
// never touched here, matching branches.Update's own convention —
// renaming a product shouldn't silently break a link someone bookmarked
// or shared. The Default variant's price updates alongside it.
func (r *Repository) AdminUpdate(ctx context.Context, categoriesRepo *categories.Repository, id string, req ProductUpsertRequest) (*Detail, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var categoryID *string
	if req.Category != "" {
		cid, err := categoriesRepo.EnsureByName(ctx, tx, req.Category)
		if err != nil {
			return nil, fmt.Errorf("resolve category: %w", err)
		}
		categoryID = &cid
	}

	var brandID *string
	if req.Brand != "" {
		bid, err := r.ensureBrand(ctx, tx, req.Brand)
		if err != nil {
			return nil, fmt.Errorf("resolve brand: %w", err)
		}
		brandID = &bid
	}

	const updateProduct = `
		UPDATE products SET
			brand_id = $2, category_id = $3, name = $4, sku = NULLIF($5, ''), barcode = NULLIF($6, ''),
			description = NULLIF($7, ''), short_description = NULLIF($8, ''), size = NULLIF($9, ''),
			gender = NULLIF($10, ''), fragrance_family = NULLIF($11, ''), status = $12,
			is_featured = $13, is_bestseller = $14, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`
	tag, err := tx.Exec(ctx, updateProduct,
		id, brandID, categoryID, req.Name, req.SKU, req.Barcode, req.Description, req.ShortDescription,
		req.Size, req.Gender, req.FragranceFamily, status, req.IsFeatured, req.IsBestseller,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateSKU
		}
		return nil, fmt.Errorf("update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	const updateVariant = `
		UPDATE product_variants SET base_price = $2, updated_at = now()
		WHERE product_id = $1 AND name = 'Default'
	`
	if _, err := tx.Exec(ctx, updateVariant, id, req.Price); err != nil {
		return nil, fmt.Errorf("update default variant price: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return r.getDetail(ctx, "p.id = $1", id)
}

// AdminSoftDelete archives a product via deleted_at, the same convention
// branches.SoftDelete already uses. It never hard-deletes: order_items
// snapshot their own product_name/sku at checkout time (section 84)
// rather than reading live from products, but product_variants still
// carries a real foreign key back to this row, so the row needs to keep
// existing — archived, not gone — for that reference to stay meaningful.
func (r *Repository) AdminSoftDelete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE products SET deleted_at = now(), status = 'archived', updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

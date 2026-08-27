package branches

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"satelit-parfume-api/pkg/slug"
)

var ErrNotFound = errors.New("branch not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const branchColumns = `
	id, name, code, slug, address, city, province, postal_code,
	latitude, longitude, phone, whatsapp, opening_time, closing_time, status,
	created_at, updated_at
`

// List returns active branches. When lat and lng are both non-nil, results
// are sorted nearest-first (section 11) via a standard Haversine formula —
// no PostGIS dependency needed for "which branch is closest". The
// LEAST/GREATEST clamp guards against acos() receiving a value fractionally
// outside [-1, 1] from floating-point rounding, which would otherwise
// return NULL (Postgres's NaN-equivalent for acos) instead of ~0.
func (r *Repository) List(ctx context.Context, lat, lng *float64) ([]Branch, error) {
	var (
		query string
		args  []any
	)

	if lat != nil && lng != nil {
		query = fmt.Sprintf(`
			SELECT %s,
				6371 * acos(
					LEAST(1.0, GREATEST(-1.0,
						cos(radians($1)) * cos(radians(latitude)) * cos(radians(longitude) - radians($2))
						+ sin(radians($1)) * sin(radians(latitude))
					))
				) AS distance_km
			FROM branches
			WHERE deleted_at IS NULL AND status = 'active'
			ORDER BY (latitude IS NULL OR longitude IS NULL), distance_km ASC NULLS LAST, name ASC
		`, branchColumns)
		args = []any{*lat, *lng}
	} else {
		query = fmt.Sprintf(`
			SELECT %s, NULL::DOUBLE PRECISION AS distance_km
			FROM branches
			WHERE deleted_at IS NULL AND status = 'active'
			ORDER BY name ASC
		`, branchColumns)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Branch, 0)
	for rows.Next() {
		b, err := scanBranch(rows, true)
		if err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (r *Repository) GetBySlug(ctx context.Context, s string) (*Branch, error) {
	query := fmt.Sprintf(`SELECT %s FROM branches WHERE slug = $1 AND deleted_at IS NULL`, branchColumns)
	return r.scanOne(ctx, query, s)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Branch, error) {
	query := fmt.Sprintf(`SELECT %s FROM branches WHERE id = $1 AND deleted_at IS NULL`, branchColumns)
	return r.scanOne(ctx, query, id)
}

func (r *Repository) scanOne(ctx context.Context, query string, arg string) (*Branch, error) {
	row := r.db.QueryRow(ctx, query, arg)
	b, err := scanBranch(row, false)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query) —
// both expose Scan with this signature, so scanBranch works for either.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanBranch(row rowScanner, withDistance bool) (Branch, error) {
	var b Branch
	var address, city, province, postalCode, phone, whatsapp, openingTime, closingTime *string

	dest := []any{
		&b.ID, &b.Name, &b.Code, &b.Slug, &address, &city, &province, &postalCode,
		&b.Latitude, &b.Longitude, &phone, &whatsapp, &openingTime, &closingTime, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
	}
	if withDistance {
		dest = append(dest, &b.DistanceKM)
	}

	if err := row.Scan(dest...); err != nil {
		return Branch{}, err
	}

	b.Address, b.City, b.Province, b.PostalCode = deref(address), deref(city), deref(province), deref(postalCode)
	b.Phone, b.WhatsApp = deref(phone), deref(whatsapp)
	b.OpeningTime, b.ClosingTime = deref(openingTime), deref(closingTime)

	return b, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *Repository) Create(ctx context.Context, req UpsertRequest) (*Branch, error) {
	branchSlug, err := r.uniqueSlug(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("generate slug: %w", err)
	}

	const q = `
		INSERT INTO branches (name, code, slug, address, city, province, postal_code,
			latitude, longitude, phone, whatsapp, opening_time, closing_time)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), NULLIF($7,''),
			$8, $9, NULLIF($10,''), NULLIF($11,''), NULLIF($12,''), NULLIF($13,''))
		RETURNING id
	`
	var id string
	err = r.db.QueryRow(ctx, q,
		req.Name, req.Code, branchSlug, req.Address, req.City, req.Province, req.PostalCode,
		req.Latitude, req.Longitude, req.Phone, req.WhatsApp, req.OpeningTime, req.ClosingTime,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id string, req UpsertRequest) (*Branch, error) {
	const q = `
		UPDATE branches SET
			name = $2, code = $3,
			address = NULLIF($4,''), city = NULLIF($5,''), province = NULLIF($6,''), postal_code = NULLIF($7,''),
			latitude = $8, longitude = $9,
			phone = NULLIF($10,''), whatsapp = NULLIF($11,''),
			opening_time = NULLIF($12,''), closing_time = NULLIF($13,''),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`
	tag, err := r.db.Exec(ctx, q,
		id, req.Name, req.Code, req.Address, req.City, req.Province, req.PostalCode,
		req.Latitude, req.Longitude, req.Phone, req.WhatsApp, req.OpeningTime, req.ClosingTime,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *Repository) SoftDelete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE branches SET deleted_at = now(), status = 'inactive' WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := slug.Generate(name)
	candidate := base
	for i := 2; ; i++ {
		var exists bool
		err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM branches WHERE slug = $1)`, candidate).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

// ── Branch staffing ──────────────────────────────────────────────────

func (r *Repository) AssignStaff(ctx context.Context, branchID, userID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO branch_staff (user_id, branch_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, branchID,
	)
	return err
}

func (r *Repository) UnassignStaff(ctx context.Context, branchID, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM branch_staff WHERE user_id = $1 AND branch_id = $2`,
		userID, branchID,
	)
	return err
}

func (r *Repository) ListStaff(ctx context.Context, branchID string) ([]StaffMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.name, u.email,
		       COALESCE(array_agg(r.name) FILTER (WHERE r.name IS NOT NULL), '{}')
		FROM branch_staff bs
		JOIN users u ON u.id = bs.user_id
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE bs.branch_id = $1 AND u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY u.name ASC
	`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	staff := make([]StaffMember, 0)
	for rows.Next() {
		var s StaffMember
		if err := rows.Scan(&s.UserID, &s.Name, &s.Email, &s.Roles); err != nil {
			return nil, err
		}
		staff = append(staff, s)
	}
	return staff, rows.Err()
}

// IsStaffAssignedToBranch backs RequireBranchAccess (middleware.go) — the
// access check hits this table fresh on every request rather than trusting
// anything cached in a JWT, so revoking an assignment takes effect
// immediately, not at next login.
func (r *Repository) IsStaffAssignedToBranch(ctx context.Context, userID, branchID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM branch_staff WHERE user_id = $1 AND branch_id = $2)`,
		userID, branchID,
	).Scan(&exists)
	return exists, err
}

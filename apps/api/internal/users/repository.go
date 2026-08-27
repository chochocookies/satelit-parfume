package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound    = errors.New("user not found")
	ErrEmailTaken  = errors.New("an account with this email already exists")
	ErrUnknownRole = errors.New("unknown role")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const selectUserTemplate = `
	SELECT u.id, u.name, u.email, u.password_hash, u.status,
	       COALESCE(array_agg(r.name) FILTER (WHERE r.name IS NOT NULL), '{}'),
	       u.created_at, u.updated_at
	FROM users u
	LEFT JOIN user_roles ur ON ur.user_id = u.id
	LEFT JOIN roles r ON r.id = ur.role_id
	WHERE u.deleted_at IS NULL AND %s
	GROUP BY u.id
`

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return r.scanOne(ctx, "u.email = $1", email)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	return r.scanOne(ctx, "u.id = $1", id)
}

// whereClause is always one of the two fixed literals above — never
// caller/user-supplied — so building the query with Sprintf is safe; the
// actual variable input (arg) still goes through pgx as a bound $1
// parameter, never string-concatenated.
func (r *Repository) scanOne(ctx context.Context, whereClause, arg string) (*User, error) {
	query := fmt.Sprintf(selectUserTemplate, whereClause)
	row := r.db.QueryRow(ctx, query, arg)

	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Status, &u.Roles, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// ── Admin staff management (Phase 9) ────────────────────────────────────
//
// GetByEmail/GetByID/scanOne above are login-critical — every StaffLogin
// and every authenticated request's RequireAuth check runs through them —
// so they're left completely untouched here. List/Create/Update below
// are new, additive methods with their own queries instead of reusing
// selectUserTemplate/scanOne, deliberately: minimizing changes to
// already-relied-upon code matters more here than avoiding a little
// duplication in the SELECT list.

// List returns every staff account, roles included, ordered by name.
// Unlike products/orders this stays unpaginated — a perfume retailer's
// staff headcount is naturally small, nothing like its product or order
// count.
func (r *Repository) List(ctx context.Context) ([]User, error) {
	const q = `
		SELECT u.id, u.name, u.email, u.password_hash, u.status,
		       COALESCE(array_agg(r.name) FILTER (WHERE r.name IS NOT NULL), '{}'),
		       u.created_at, u.updated_at
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY u.name ASC
	`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Status, &u.Roles, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// Create inserts a new staff account and its role assignments inside one
// transaction, then re-reads it through GetByID — same "insert, then
// read back the canonical row" shape AdminCreate uses elsewhere. roleNames
// must all already exist in the roles table (seeded by migration
// 000002_auth); an unrecognized name rolls the whole transaction back
// rather than silently creating an account with fewer roles than asked
// for.
func (r *Repository) Create(ctx context.Context, name, email, passwordHash string, roleNames []string) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed below

	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		name, email, passwordHash,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	if err := r.setRoles(ctx, tx, id, roleNames); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

// Update replaces a staff account's name, status, and role assignments —
// full replace, same convention as branches.Update/AdminUpdate elsewhere.
// There's no email or password field: changing either of those is a more
// sensitive, separate flow this phase doesn't build (see the root
// README's "Deliberately not in Phase 9" note).
func (r *Repository) Update(ctx context.Context, id, name, status string, roleNames []string) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx,
		`UPDATE users SET name = $2, status = $3, updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		id, name, status,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, id); err != nil {
		return nil, err
	}
	if err := r.setRoles(ctx, tx, id, roleNames); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

// setRoles inserts one user_roles row per name, resolving each name to
// its role id server-side (SELECT ... FROM roles WHERE name = $2) rather
// than trusting a client-supplied role id — the only thing the caller
// ever hands us is a role *name*. Zero rows affected means that name
// doesn't exist in the roles table, which the caller (Create/Update)
// surfaces as ErrUnknownRole and rolls the whole transaction back for,
// rather than leaving a staff account with fewer roles than the request
// asked for.
func (r *Repository) setRoles(ctx context.Context, tx pgx.Tx, userID string, roleNames []string) error {
	for _, roleName := range roleNames {
		tag, err := tx.Exec(ctx,
			`INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = $2`,
			userID, roleName,
		)
		if err != nil {
			return fmt.Errorf("assign role %s: %w", roleName, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: %s", ErrUnknownRole, roleName)
		}
	}
	return nil
}

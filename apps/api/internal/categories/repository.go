package categories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"satelit-parfume-api/pkg/slug"
)

var ErrNotFound = errors.New("category not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]Category, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, slug FROM categories ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (r *Repository) GetBySlug(ctx context.Context, s string) (*Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `SELECT id, name, slug FROM categories WHERE slug = $1`, s).
		Scan(&c.ID, &c.Name, &c.Slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// EnsureByName returns the id of the category matching name (by its
// slugified form), creating it if it doesn't exist yet. It takes an
// explicit pgx.Tx rather than using the pool directly so callers (product
// import) can run it inside their own transaction — a partially-imported
// row should never leave behind an orphaned category no product ended up
// using.
func (r *Repository) EnsureByName(ctx context.Context, tx pgx.Tx, name string) (string, error) {
	s := slug.Generate(name)

	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, s).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	err = tx.QueryRow(ctx, `INSERT INTO categories (name, slug) VALUES ($1, $2) RETURNING id`, name, s).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

package customers

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("customer not found")
	ErrEmailTaken = errors.New("email already registered")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, name, email, phone, passwordHash string) (*Customer, error) {
	const q = `
		INSERT INTO customers (name, email, phone, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, email, phone, password_hash, status, created_at, updated_at
	`
	var c Customer
	err := r.db.QueryRow(ctx, q, name, email, phone, passwordHash).Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.PasswordHash, &c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return &c, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*Customer, error) {
	const q = `
		SELECT id, name, email, phone, password_hash, status, created_at, updated_at
		FROM customers
		WHERE email = $1 AND deleted_at IS NULL
	`
	return r.scanOne(ctx, q, email)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Customer, error) {
	const q = `
		SELECT id, name, email, phone, password_hash, status, created_at, updated_at
		FROM customers
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanOne(ctx, q, id)
}

func (r *Repository) scanOne(ctx context.Context, query, arg string) (*Customer, error) {
	row := r.db.QueryRow(ctx, query, arg)
	var c Customer
	err := row.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.PasswordHash, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

package payments

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("payment not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type CreateParams struct {
	OrderID       string
	Provider      string
	PaymentMethod string
	Reference     string
	Amount        int64
	QRString      string
	VANumber      string
	PaymentURL    string
}

const paymentColumns = `
	id, order_id, provider, payment_method, reference, amount, status,
	qr_string, va_number, payment_url, created_at, updated_at
`

func (r *Repository) Create(ctx context.Context, p CreateParams) (*Payment, error) {
	const q = `
		INSERT INTO payments (order_id, provider, payment_method, reference, amount, qr_string, va_number, payment_url)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), NULLIF($7,''), NULLIF($8,''))
		RETURNING id
	`
	var id string
	err := r.db.QueryRow(ctx, q,
		p.OrderID, p.Provider, p.PaymentMethod, p.Reference, p.Amount, p.QRString, p.VANumber, p.PaymentURL,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Payment, error) {
	return r.scanOne(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id = $1`, id)
}

func (r *Repository) GetLatestForOrder(ctx context.Context, orderID string) (*Payment, error) {
	return r.scanOne(ctx,
		`SELECT `+paymentColumns+` FROM payments WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`,
		orderID,
	)
}

func (r *Repository) scanOne(ctx context.Context, query, arg string) (*Payment, error) {
	var p Payment
	var qrString, vaNumber, paymentURL *string
	err := r.db.QueryRow(ctx, query, arg).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.PaymentMethod, &p.Reference, &p.Amount, &p.Status,
		&qrString, &vaNumber, &paymentURL, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.QRString, p.VANumber, p.PaymentURL = deref(qrString), deref(vaNumber), deref(paymentURL)
	return &p, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// MarkPaid/MarkFailed are looked up by (provider, reference) — the pair
// CreatePaymentForOrder's Create call was uniqued on — not by order_id,
// since a webhook only ever tells us the provider's own reference.
func (r *Repository) MarkPaid(ctx context.Context, provider, reference string) error {
	return r.setStatus(ctx, provider, reference, StatusPaid)
}

func (r *Repository) MarkFailed(ctx context.Context, provider, reference string) error {
	return r.setStatus(ctx, provider, reference, StatusFailed)
}

func (r *Repository) setStatus(ctx context.Context, provider, reference, status string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE payments SET status = $3, updated_at = now() WHERE provider = $1 AND reference = $2`,
		provider, reference, status,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type LogParams struct {
	Provider       string
	Reference      string
	RawPayload     string
	SignatureValid bool
}

// LogTransaction records a webhook delivery verbatim, valid or not —
// see the migration comment on payment_transactions for why this is an
// audit log, not a dedup mechanism.
func (r *Repository) LogTransaction(ctx context.Context, p LogParams) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO payment_transactions (provider, reference, raw_payload, signature_valid) VALUES ($1, NULLIF($2,''), $3, $4)`,
		p.Provider, p.Reference, p.RawPayload, p.SignatureValid,
	)
	return err
}

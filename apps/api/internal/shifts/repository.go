package shifts

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const shiftColumns = `
	id, branch_id, user_id, opening_balance, closing_balance, expected_balance,
	discrepancy, status, notes, opened_at, closed_at, created_at, updated_at
`

func scanShift(row interface{ Scan(...any) error }) (*Shift, error) {
	var s Shift
	var notes *string
	err := row.Scan(
		&s.ID, &s.BranchID, &s.UserID, &s.OpeningBalance, &s.ClosingBalance, &s.ExpectedBalance,
		&s.Discrepancy, &s.Status, &notes, &s.OpenedAt, &s.ClosedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if notes != nil {
		s.Notes = *notes
	}
	return &s, nil
}

// Open starts a new shift. The partial unique index from migration
// 000008 (one open shift per user) is what actually enforces "you can't
// open two at once" — this just translates that constraint violation
// into ErrAlreadyOpen instead of a raw pg error leaking out.
func (r *Repository) Open(ctx context.Context, branchID, userID string, openingBalance int64, notes string) (*Shift, error) {
	const q = `
		INSERT INTO cashier_shifts (branch_id, user_id, opening_balance, notes)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING ` + shiftColumns

	shift, err := scanShift(r.db.QueryRow(ctx, q, branchID, userID, openingBalance, notes))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyOpen
		}
		return nil, err
	}
	return shift, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Shift, error) {
	shift, err := scanShift(r.db.QueryRow(ctx, `SELECT `+shiftColumns+` FROM cashier_shifts WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return shift, nil
}

// GetOpenForUser is how the POS checkout flow decides which shift a sale
// belongs to — always derived server-side from the authenticated caller,
// never accepted as a client-supplied shift id. See
// orders.Handler.POSCheckout.
func (r *Repository) GetOpenForUser(ctx context.Context, userID string) (*Shift, error) {
	shift, err := scanShift(r.db.QueryRow(ctx,
		`SELECT `+shiftColumns+` FROM cashier_shifts WHERE user_id = $1 AND status = 'open'`, userID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return shift, nil
}

// Close computes expected_balance from cash sales recorded against this
// shift (orders.payment_method = 'cash', excluding anything refunded —
// cash physically handed back doesn't belong in the drawer count
// anymore), then records the cashier's own counted closing_balance and
// the discrepancy between the two, all in one statement — so "compute
// expected" and "write the close" can't drift apart from a second query
// racing a sale that lands in between them.
func (r *Repository) Close(ctx context.Context, id string, closingBalance int64, notes string) (*Shift, error) {
	const q = `
		WITH expected AS (
			SELECT cs.opening_balance + COALESCE((
				SELECT SUM(o.total) FROM orders o
				WHERE o.cashier_shift_id = cs.id
				  AND o.payment_method = 'cash'
				  AND o.status != 'REFUNDED'
			), 0) AS amount
			FROM cashier_shifts cs
			WHERE cs.id = $1
		)
		UPDATE cashier_shifts SET
			closing_balance = $2,
			expected_balance = (SELECT amount FROM expected),
			discrepancy = $2 - (SELECT amount FROM expected),
			notes = COALESCE(NULLIF($3, ''), notes),
			status = 'closed',
			closed_at = now(),
			updated_at = now()
		WHERE id = $1 AND status = 'open'
		RETURNING ` + shiftColumns

	shift, err := scanShift(r.db.QueryRow(ctx, q, id, closingBalance, notes))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either the id doesn't exist, or it does but is already
			// closed — tell those apart with one more lookup rather than
			// guessing, so the error actually matches what happened.
			if _, getErr := r.GetByID(ctx, id); errors.Is(getErr, ErrNotFound) {
				return nil, ErrNotFound
			}
			return nil, ErrAlreadyClosed
		}
		return nil, err
	}
	return shift, nil
}

// ListForBranch returns shift history for a branch, most recent first —
// the admin/branch-manager shift history view.
func (r *Repository) ListForBranch(ctx context.Context, branchID string) ([]Shift, error) {
	rows, err := r.db.Query(ctx, `SELECT `+shiftColumns+` FROM cashier_shifts WHERE branch_id = $1 ORDER BY opened_at DESC`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Shift, 0)
	for rows.Next() {
		s, err := scanShift(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *s)
	}
	return list, rows.Err()
}

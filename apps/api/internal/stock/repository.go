package stock

import (
	"context"
	"errors"
	"fmt"

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

// ── Movements ────────────────────────────────────────────────────────

type LogMovementParams struct {
	BranchID       string
	VariantID      string
	QuantityChange int
	Reason         string
	ReferenceType  string
	ReferenceID    string
	Note           string
	ActorUserID    string
}

// LogMovement inserts one ledger line and returns its id. Takes a
// caller-supplied tx so a movement is always recorded in the same
// transaction as the stock_quantity change it describes — this package's
// own Receive/Adjust/transfer/opname methods pass their own tx;
// internal/orders' applyTransition (Phase 7-9, unmodified otherwise)
// passes its own for the 'sale' reason.
func (r *Repository) LogMovement(ctx context.Context, tx pgx.Tx, p LogMovementParams) (string, error) {
	const q = `
		INSERT INTO stock_movements (
			branch_id, product_variant_id, quantity_change, reason,
			reference_type, reference_id, note, actor_user_id
		) VALUES (
			$1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, '')::uuid, NULLIF($7, ''), NULLIF($8, '')::uuid
		)
		RETURNING id
	`
	var id string
	err := tx.QueryRow(ctx, q,
		p.BranchID, p.VariantID, p.QuantityChange, p.Reason,
		p.ReferenceType, p.ReferenceID, p.Note, p.ActorUserID,
	).Scan(&id)
	return id, err
}

const movementDetailQuery = `
	SELECT sm.id, sm.branch_id, sm.product_variant_id, p.name, pv.name,
	       sm.quantity_change, sm.reason, sm.reference_type, sm.reference_id,
	       sm.note, sm.actor_user_id, u.name, sm.created_at
	FROM stock_movements sm
	JOIN product_variants pv ON pv.id = sm.product_variant_id
	JOIN products p ON p.id = pv.product_id
	LEFT JOIN users u ON u.id = sm.actor_user_id
`

func scanMovement(row interface{ Scan(...any) error }) (*Movement, error) {
	var m Movement
	var referenceType, referenceID, note, actorUserID, actorName *string
	err := row.Scan(
		&m.ID, &m.BranchID, &m.ProductVariantID, &m.ProductName, &m.VariantName,
		&m.QuantityChange, &m.Reason, &referenceType, &referenceID,
		&note, &actorUserID, &actorName, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if referenceType != nil {
		m.ReferenceType = *referenceType
	}
	if referenceID != nil {
		m.ReferenceID = *referenceID
	}
	if note != nil {
		m.Note = *note
	}
	if actorUserID != nil {
		m.ActorUserID = *actorUserID
	}
	if actorName != nil {
		m.ActorName = *actorName
	}
	return &m, nil
}

func (r *Repository) getMovement(ctx context.Context, id string) (*Movement, error) {
	m, err := scanMovement(r.db.QueryRow(ctx, movementDetailQuery+` WHERE sm.id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return m, nil
}

// MovementHistory backs the "movement history" view for one variant at
// one branch — every ledger line, most recent first.
func (r *Repository) MovementHistory(ctx context.Context, branchID, variantID string, limit int) ([]Movement, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		movementDetailQuery+` WHERE sm.branch_id = $1 AND sm.product_variant_id = $2 ORDER BY sm.created_at DESC LIMIT $3`,
		branchID, variantID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Movement, 0)
	for rows.Next() {
		m, err := scanMovement(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *m)
	}
	return list, rows.Err()
}

// Receive adds newly delivered stock — always additive (ReceiveRequest's
// binding rules out zero/negative). Creates the branch_inventory row if
// this branch has never stocked this variant before.
func (r *Repository) Receive(ctx context.Context, branchID, variantID string, quantity int, note, actorUserID string) (*Movement, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const upsert = `
		INSERT INTO branch_inventory (branch_id, product_variant_id, stock_quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (branch_id, product_variant_id) DO UPDATE SET
			stock_quantity = branch_inventory.stock_quantity + EXCLUDED.stock_quantity,
			updated_at = now()
	`
	if _, err := tx.Exec(ctx, upsert, branchID, variantID, quantity); err != nil {
		return nil, fmt.Errorf("update stock: %w", err)
	}

	movementID, err := r.LogMovement(ctx, tx, LogMovementParams{
		BranchID: branchID, VariantID: variantID, QuantityChange: quantity,
		Reason: "receive", Note: note, ActorUserID: actorUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("log movement: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.getMovement(ctx, movementID)
}

// Adjust applies a signed correction — a positive number found more
// stock than expected, a negative one writes off damage or loss. Unlike
// Receive, this can drive the branch to zero but never below it: a
// resulting negative total is rejected outright (ErrNegativeResult)
// rather than silently floored, since floor-and-continue would apply a
// smaller adjustment than the admin actually asked for without them
// necessarily noticing.
func (r *Repository) Adjust(ctx context.Context, branchID, variantID string, quantityChange int, note, actorUserID string) (*Movement, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var current int
	err = tx.QueryRow(ctx,
		`SELECT stock_quantity FROM branch_inventory WHERE branch_id = $1 AND product_variant_id = $2 FOR UPDATE`,
		branchID, variantID,
	).Scan(&current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if current+quantityChange < 0 {
		return nil, ErrNegativeResult
	}

	const upsert = `
		INSERT INTO branch_inventory (branch_id, product_variant_id, stock_quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (branch_id, product_variant_id) DO UPDATE SET
			stock_quantity = branch_inventory.stock_quantity + EXCLUDED.stock_quantity,
			updated_at = now()
	`
	if _, err := tx.Exec(ctx, upsert, branchID, variantID, quantityChange); err != nil {
		return nil, fmt.Errorf("update stock: %w", err)
	}

	movementID, err := r.LogMovement(ctx, tx, LogMovementParams{
		BranchID: branchID, VariantID: variantID, QuantityChange: quantityChange,
		Reason: "adjust", Note: note, ActorUserID: actorUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("log movement: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.getMovement(ctx, movementID)
}

// ── Transfers ────────────────────────────────────────────────────────

const transferColumns = `id, from_branch_id, to_branch_id, status, requested_by, completed_by, notes, completed_at, created_at, updated_at`

func scanTransfer(row interface{ Scan(...any) error }) (*Transfer, error) {
	var t Transfer
	var completedBy, notes *string
	err := row.Scan(
		&t.ID, &t.FromBranchID, &t.ToBranchID, &t.Status, &t.RequestedBy, &completedBy,
		&notes, &t.CompletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if completedBy != nil {
		t.CompletedBy = *completedBy
	}
	if notes != nil {
		t.Notes = *notes
	}
	return &t, nil
}

func (r *Repository) itemsForTransfer(ctx context.Context, transferID string) ([]TransferItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT sti.product_variant_id, p.name, pv.name, sti.quantity
		FROM stock_transfer_items sti
		JOIN product_variants pv ON pv.id = sti.product_variant_id
		JOIN products p ON p.id = pv.product_id
		WHERE sti.transfer_id = $1
	`, transferID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]TransferItem, 0)
	for rows.Next() {
		var it TransferItem
		if err := rows.Scan(&it.ProductVariantID, &it.ProductName, &it.VariantName, &it.Quantity); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *Repository) GetTransfer(ctx context.Context, id string) (*Transfer, error) {
	t, err := scanTransfer(r.db.QueryRow(ctx, `SELECT `+transferColumns+` FROM stock_transfers WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	items, err := r.itemsForTransfer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	t.Items = items
	return t, nil
}

// ListForBranch returns transfers where branchID is either side —
// outgoing or incoming — since a branch manager cares about both.
func (r *Repository) ListForBranch(ctx context.Context, branchID string) ([]Transfer, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+transferColumns+` FROM stock_transfers WHERE from_branch_id = $1 OR to_branch_id = $1 ORDER BY created_at DESC`,
		branchID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Transfer, 0)
	for rows.Next() {
		t, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		items, err := r.itemsForTransfer(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		t.Items = items
		list = append(list, *t)
	}
	return list, rows.Err()
}

// CreateTransfer deducts the source branch's stock immediately — see
// migration 000009's table comment on why a pending transfer can't be
// oversold from the source. Every item is checked and deducted inside
// one transaction with the same SELECT ... FOR UPDATE pattern
// inventory.ReserveStock uses, so two transfers racing for the same
// last units can't both succeed.
func (r *Repository) CreateTransfer(ctx context.Context, fromBranchID, requestedBy string, req CreateTransferRequest) (*Transfer, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var transferID string
	err = tx.QueryRow(ctx,
		`INSERT INTO stock_transfers (from_branch_id, to_branch_id, requested_by, notes) VALUES ($1, $2, $3, NULLIF($4, '')) RETURNING id`,
		fromBranchID, req.ToBranchID, requestedBy, req.Notes,
	).Scan(&transferID)
	if err != nil {
		return nil, fmt.Errorf("create transfer: %w", err)
	}

	for _, item := range req.Items {
		var available int
		err := tx.QueryRow(ctx,
			`SELECT available_stock FROM branch_inventory WHERE branch_id = $1 AND product_variant_id = $2 FOR UPDATE`,
			fromBranchID, item.ProductVariantID,
		).Scan(&available)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInsufficientStock
			}
			return nil, err
		}
		if available < item.Quantity {
			return nil, ErrInsufficientStock
		}

		if _, err := tx.Exec(ctx,
			`UPDATE branch_inventory SET stock_quantity = stock_quantity - $3, updated_at = now() WHERE branch_id = $1 AND product_variant_id = $2`,
			fromBranchID, item.ProductVariantID, item.Quantity,
		); err != nil {
			return nil, fmt.Errorf("deduct source stock: %w", err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO stock_transfer_items (transfer_id, product_variant_id, quantity) VALUES ($1, $2, $3)`,
			transferID, item.ProductVariantID, item.Quantity,
		); err != nil {
			return nil, fmt.Errorf("record transfer item: %w", err)
		}

		if _, err := r.LogMovement(ctx, tx, LogMovementParams{
			BranchID: fromBranchID, VariantID: item.ProductVariantID, QuantityChange: -item.Quantity,
			Reason: "transfer_out", ReferenceType: "transfer", ReferenceID: transferID, ActorUserID: requestedBy,
		}); err != nil {
			return nil, fmt.Errorf("log movement: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.GetTransfer(ctx, transferID)
}

// CompleteTransfer adds the transferred stock to the destination branch
// — the moment it stops being "in transit" and becomes real, sellable
// stock there. toBranchID must match the transfer's actual to_branch_id
// (the handler passes whatever :id was in the URL); a mismatch means
// someone's trying to complete a transfer through the wrong branch.
func (r *Repository) CompleteTransfer(ctx context.Context, transferID, toBranchID, completedBy string) (*Transfer, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var status, actualToBranch string
	err = tx.QueryRow(ctx, `SELECT status, to_branch_id FROM stock_transfers WHERE id = $1 FOR UPDATE`, transferID).Scan(&status, &actualToBranch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if actualToBranch != toBranchID {
		return nil, ErrTransferWrongBranch
	}
	if status != "pending" {
		return nil, ErrTransferNotPending
	}

	items, err := r.itemsForTransfer(ctx, transferID)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}

	for _, item := range items {
		const upsert = `
			INSERT INTO branch_inventory (branch_id, product_variant_id, stock_quantity)
			VALUES ($1, $2, $3)
			ON CONFLICT (branch_id, product_variant_id) DO UPDATE SET
				stock_quantity = branch_inventory.stock_quantity + EXCLUDED.stock_quantity,
				updated_at = now()
		`
		if _, err := tx.Exec(ctx, upsert, toBranchID, item.ProductVariantID, item.Quantity); err != nil {
			return nil, fmt.Errorf("add destination stock: %w", err)
		}
		if _, err := r.LogMovement(ctx, tx, LogMovementParams{
			BranchID: toBranchID, VariantID: item.ProductVariantID, QuantityChange: item.Quantity,
			Reason: "transfer_in", ReferenceType: "transfer", ReferenceID: transferID, ActorUserID: completedBy,
		}); err != nil {
			return nil, fmt.Errorf("log movement: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE stock_transfers SET status = 'completed', completed_by = $2, completed_at = now(), updated_at = now() WHERE id = $1`,
		transferID, completedBy,
	); err != nil {
		return nil, fmt.Errorf("update transfer: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.GetTransfer(ctx, transferID)
}

// CancelTransfer reverses the deduction CreateTransfer made — the stock
// never actually left, so it goes back to being ordinary sellable stock
// at the source branch. Only the source branch can cancel (fromBranchID
// must match), the same way only the destination can complete.
func (r *Repository) CancelTransfer(ctx context.Context, transferID, fromBranchID, cancelledBy string) (*Transfer, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var status, actualFromBranch string
	err = tx.QueryRow(ctx, `SELECT status, from_branch_id FROM stock_transfers WHERE id = $1 FOR UPDATE`, transferID).Scan(&status, &actualFromBranch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if actualFromBranch != fromBranchID {
		return nil, ErrTransferWrongBranch
	}
	if status != "pending" {
		return nil, ErrTransferNotPending
	}

	items, err := r.itemsForTransfer(ctx, transferID)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx,
			`UPDATE branch_inventory SET stock_quantity = stock_quantity + $3, updated_at = now() WHERE branch_id = $1 AND product_variant_id = $2`,
			fromBranchID, item.ProductVariantID, item.Quantity,
		); err != nil {
			return nil, fmt.Errorf("restore source stock: %w", err)
		}
		if _, err := r.LogMovement(ctx, tx, LogMovementParams{
			BranchID: fromBranchID, VariantID: item.ProductVariantID, QuantityChange: item.Quantity,
			Reason: "transfer_cancelled", ReferenceType: "transfer", ReferenceID: transferID, ActorUserID: cancelledBy,
		}); err != nil {
			return nil, fmt.Errorf("log movement: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE stock_transfers SET status = 'cancelled', updated_at = now() WHERE id = $1`, transferID); err != nil {
		return nil, fmt.Errorf("update transfer: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.GetTransfer(ctx, transferID)
}

// ── Opname (physical stock count) ───────────────────────────────────

const opnameColumns = `id, branch_id, status, started_by, completed_by, notes, completed_at, created_at, updated_at`

func scanOpname(row interface{ Scan(...any) error }) (*Opname, error) {
	var o Opname
	var completedBy, notes *string
	err := row.Scan(
		&o.ID, &o.BranchID, &o.Status, &o.StartedBy, &completedBy,
		&notes, &o.CompletedAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if completedBy != nil {
		o.CompletedBy = *completedBy
	}
	if notes != nil {
		o.Notes = *notes
	}
	return &o, nil
}

func (r *Repository) itemsForOpname(ctx context.Context, opnameID string) ([]OpnameItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT oi.id, oi.product_variant_id, p.name, pv.name, oi.system_quantity, oi.counted_quantity
		FROM stock_opname_items oi
		JOIN product_variants pv ON pv.id = oi.product_variant_id
		JOIN products p ON p.id = pv.product_id
		WHERE oi.opname_id = $1
		ORDER BY p.name ASC
	`, opnameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]OpnameItem, 0)
	for rows.Next() {
		var it OpnameItem
		if err := rows.Scan(&it.ID, &it.ProductVariantID, &it.ProductName, &it.VariantName, &it.SystemQuantity, &it.CountedQuantity); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *Repository) GetOpname(ctx context.Context, id string) (*Opname, error) {
	o, err := scanOpname(r.db.QueryRow(ctx, `SELECT `+opnameColumns+` FROM stock_opnames WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	items, err := r.itemsForOpname(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	o.Items = items
	return o, nil
}

func (r *Repository) GetOpenForBranch(ctx context.Context, branchID string) (*Opname, error) {
	o, err := scanOpname(r.db.QueryRow(ctx, `SELECT `+opnameColumns+` FROM stock_opnames WHERE branch_id = $1 AND status = 'open'`, branchID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	items, err := r.itemsForOpname(ctx, o.ID)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	o.Items = items
	return o, nil
}

func (r *Repository) ListForBranchOpnames(ctx context.Context, branchID string) ([]Opname, error) {
	rows, err := r.db.Query(ctx, `SELECT `+opnameColumns+` FROM stock_opnames WHERE branch_id = $1 ORDER BY created_at DESC`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Opname, 0)
	for rows.Next() {
		o, err := scanOpname(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *o)
	}
	return list, rows.Err()
}

// StartOpname snapshots every current branch_inventory row for this
// branch into stock_opname_items in one INSERT ... SELECT — system_quantity
// freezes at this moment (see migration 000009's table comment) rather
// than being read live when the count later completes.
func (r *Repository) StartOpname(ctx context.Context, branchID, startedBy string) (*Opname, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var opnameID string
	err = tx.QueryRow(ctx, `INSERT INTO stock_opnames (branch_id, started_by) VALUES ($1, $2) RETURNING id`, branchID, startedBy).Scan(&opnameID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrOpnameAlreadyOpen
		}
		return nil, fmt.Errorf("create opname: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_opname_items (opname_id, product_variant_id, system_quantity)
		SELECT $1, product_variant_id, stock_quantity FROM branch_inventory WHERE branch_id = $2
	`, opnameID, branchID); err != nil {
		return nil, fmt.Errorf("snapshot items: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.GetOpname(ctx, opnameID)
}

// CountItem records what staff actually counted for one line. Only
// allowed while the opname is still open — the WHERE clause's join back
// to stock_opnames enforces that at the SQL level, and the two possible
// zero-row causes (wrong item id vs. an opname that's already completed)
// are told apart with one follow-up lookup rather than guessing, so the
// error actually matches what happened.
func (r *Repository) CountItem(ctx context.Context, opnameID, itemID string, counted int) (*OpnameItem, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE stock_opname_items oi SET counted_quantity = $3
		FROM stock_opnames o
		WHERE oi.id = $2 AND oi.opname_id = $1 AND o.id = oi.opname_id AND o.status = 'open'
	`, opnameID, itemID, counted)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var status string
		if statusErr := r.db.QueryRow(ctx, `SELECT status FROM stock_opnames WHERE id = $1`, opnameID).Scan(&status); statusErr == nil && status != "open" {
			return nil, ErrOpnameNotOpen
		}
		return nil, ErrOpnameItemNotFound
	}
	return r.getOpnameItem(ctx, itemID)
}

func (r *Repository) getOpnameItem(ctx context.Context, itemID string) (*OpnameItem, error) {
	row := r.db.QueryRow(ctx, `
		SELECT oi.id, oi.product_variant_id, p.name, pv.name, oi.system_quantity, oi.counted_quantity
		FROM stock_opname_items oi
		JOIN product_variants pv ON pv.id = oi.product_variant_id
		JOIN products p ON p.id = pv.product_id
		WHERE oi.id = $1
	`, itemID)
	var it OpnameItem
	err := row.Scan(&it.ID, &it.ProductVariantID, &it.ProductName, &it.VariantName, &it.SystemQuantity, &it.CountedQuantity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOpnameItemNotFound
		}
		return nil, err
	}
	return &it, nil
}

// CompleteOpname applies every counted line that differs from what the
// system expected, each as its own 'opname' movement, then closes the
// session. Lines never counted (counted_quantity still null) are left
// alone entirely — an uncounted item is unknown, not zero, so it would
// be wrong to treat silence as "nothing here".
func (r *Repository) CompleteOpname(ctx context.Context, opnameID, branchID, completedBy, notes string) (*Opname, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM stock_opnames WHERE id = $1 AND branch_id = $2 FOR UPDATE`, opnameID, branchID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != "open" {
		return nil, ErrOpnameNotOpen
	}

	rows, err := tx.Query(ctx, `
		SELECT product_variant_id, system_quantity, counted_quantity
		FROM stock_opname_items
		WHERE opname_id = $1 AND counted_quantity IS NOT NULL AND counted_quantity != system_quantity
	`, opnameID)
	if err != nil {
		return nil, fmt.Errorf("load discrepancies: %w", err)
	}
	type discrepancy struct {
		VariantID string
		Delta     int
	}
	var diffs []discrepancy
	for rows.Next() {
		var variantID string
		var systemQty, countedQty int
		if err := rows.Scan(&variantID, &systemQty, &countedQty); err != nil {
			rows.Close()
			return nil, err
		}
		diffs = append(diffs, discrepancy{VariantID: variantID, Delta: countedQty - systemQty})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for _, d := range diffs {
		if _, err := tx.Exec(ctx,
			`UPDATE branch_inventory SET stock_quantity = GREATEST(0, stock_quantity + $3), updated_at = now() WHERE branch_id = $1 AND product_variant_id = $2`,
			branchID, d.VariantID, d.Delta,
		); err != nil {
			return nil, fmt.Errorf("apply correction: %w", err)
		}
		if _, err := r.LogMovement(ctx, tx, LogMovementParams{
			BranchID: branchID, VariantID: d.VariantID, QuantityChange: d.Delta,
			Reason: "opname", ReferenceType: "opname", ReferenceID: opnameID, ActorUserID: completedBy,
		}); err != nil {
			return nil, fmt.Errorf("log movement: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE stock_opnames SET status = 'completed', completed_by = $2, notes = COALESCE(NULLIF($3, ''), notes), completed_at = now(), updated_at = now() WHERE id = $1`,
		opnameID, completedBy, notes,
	); err != nil {
		return nil, fmt.Errorf("update opname: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.GetOpname(ctx, opnameID)
}

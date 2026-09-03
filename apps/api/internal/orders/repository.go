package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("order not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Begin exposes a transaction to Service, which orchestrates checkout and
// status transitions across this repository AND inventory's
// Reserve/Release/Deduct — those need to commit or roll back together.
func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// NextOrderNumber is a single atomic statement (INSERT ... ON CONFLICT
// DO UPDATE ... RETURNING), safe under concurrent checkouts without any
// extra locking — two simultaneous callers each get a distinct,
// correctly-incrementing count for today, never the same number twice.
func (r *Repository) NextOrderNumber(ctx context.Context, tx pgx.Tx) (string, error) {
	today := time.Now().Format("20060102")
	var count int
	err := tx.QueryRow(ctx, `
		INSERT INTO order_number_counters (counter_date, count) VALUES (CURRENT_DATE, 1)
		ON CONFLICT (counter_date) DO UPDATE SET count = order_number_counters.count + 1
		RETURNING count
	`).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SP-%s-%04d", today, count), nil
}

type CreateOrderParams struct {
	OrderNumber    string
	CustomerID     *string
	GuestName      string
	GuestPhone     string
	GuestEmail     string
	BranchID       string
	OrderType      string
	Subtotal       int64
	Total          int64
	RecipientName  string
	RecipientPhone string
	AddressLine    string
	City           string
	Province       string
	PostalCode     string
	Notes          string
	ExpiresAt      time.Time
	// CashierShiftID is set only for a POS sale (Phase 10) — nil for
	// every ordinary customer/guest checkout, exactly matching the
	// column's own nullability. See CheckoutRequest.CashierShiftID.
	CashierShiftID *string
}

func (r *Repository) CreateOrder(ctx context.Context, tx pgx.Tx, p CreateOrderParams) (string, error) {
	const q = `
		INSERT INTO orders (
			order_number, customer_id, guest_name, guest_phone, guest_email,
			branch_id, order_type, subtotal, total,
			recipient_name, recipient_phone, address_line, city, province, postal_code, notes,
			expires_at, cashier_shift_id
		) VALUES (
			$1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''),
			$6, $7, $8, $9,
			NULLIF($10,''), NULLIF($11,''), NULLIF($12,''), NULLIF($13,''), NULLIF($14,''), NULLIF($15,''), NULLIF($16,''),
			$17, $18
		)
		RETURNING id
	`
	var id string
	err := tx.QueryRow(ctx, q,
		p.OrderNumber, p.CustomerID, p.GuestName, p.GuestPhone, p.GuestEmail,
		p.BranchID, p.OrderType, p.Subtotal, p.Total,
		p.RecipientName, p.RecipientPhone, p.AddressLine, p.City, p.Province, p.PostalCode, p.Notes,
		p.ExpiresAt, p.CashierShiftID,
	).Scan(&id)
	return id, err
}

// SetPaymentMethod records how an order was actually paid ('cash' or
// 'qris') after the fact, once payment is confirmed. Deliberately
// separate from UpdateStatus: this touches exactly one column, never the
// guarded status transition or status_history, and is purely additive —
// every existing UpdateStatus call site (Phase 7-9) is completely
// unaffected by this method's existence.
func (r *Repository) SetPaymentMethod(ctx context.Context, orderID, method string) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET payment_method = $2, updated_at = now() WHERE id = $1`, orderID, method)
	return err
}

type CreateOrderItemParams struct {
	OrderID          string
	ProductVariantID string
	BranchID         string
	ProductName      string
	VariantName      string
	SKU              string
	UnitPrice        int64
	Quantity         int
	Subtotal         int64
}

func (r *Repository) CreateOrderItem(ctx context.Context, tx pgx.Tx, p CreateOrderItemParams) error {
	const q = `
		INSERT INTO order_items (order_id, product_variant_id, branch_id, product_name, variant_name, sku, unit_price, quantity, subtotal)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, NULLIF($6,''), $7, $8, $9)
	`
	_, err := tx.Exec(ctx, q,
		p.OrderID, p.ProductVariantID, p.BranchID, p.ProductName, p.VariantName, p.SKU,
		p.UnitPrice, p.Quantity, p.Subtotal,
	)
	return err
}

func (r *Repository) AddStatusHistory(ctx context.Context, tx pgx.Tx, orderID, status, note string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO order_status_histories (order_id, status, note) VALUES ($1, $2, NULLIF($3,''))`,
		orderID, status, note,
	)
	return err
}

// UpdateStatus atomically transitions the order, guarded by a WHERE on
// the *current* status: if it's already moved on (someone else got there
// first, or Service's own pre-check read a status that's now stale),
// this affects zero rows and returns ErrNotFound — which Service reads
// as "don't double-apply the stock side-effect for this transition,"
// not as a real failure. This is what makes a concurrent expiry-sweep
// and a manual cancel racing each other safe without extra locking.
func (r *Repository) UpdateStatus(ctx context.Context, tx pgx.Tx, orderID, from, to string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE orders SET status = $3, updated_at = now() WHERE id = $1 AND status = $2`,
		orderID, from, to,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const orderColumns = `
	id, order_number, customer_id, guest_name, guest_phone, guest_email,
	branch_id, order_type, status, subtotal, total,
	recipient_name, recipient_phone, address_line, city, province, postal_code, notes,
	expires_at, created_at, updated_at, payment_method, cashier_shift_id
`

func scanOrder(row interface{ Scan(...any) error }) (*Order, error) {
	var o Order
	var customerID *string
	var guestName, guestPhone, guestEmail *string
	var recipientName, recipientPhone, addressLine, city, province, postalCode, notes *string
	var paymentMethod, cashierShiftID *string

	err := row.Scan(
		&o.ID, &o.OrderNumber, &customerID, &guestName, &guestPhone, &guestEmail,
		&o.BranchID, &o.OrderType, &o.Status, &o.Subtotal, &o.Total,
		&recipientName, &recipientPhone, &addressLine, &city, &province, &postalCode, &notes,
		&o.ExpiresAt, &o.CreatedAt, &o.UpdatedAt, &paymentMethod, &cashierShiftID,
	)
	if err != nil {
		return nil, err
	}

	if customerID != nil {
		o.CustomerID = *customerID
	}
	o.GuestName, o.GuestPhone, o.GuestEmail = deref(guestName), deref(guestPhone), deref(guestEmail)
	o.RecipientName, o.RecipientPhone = deref(recipientName), deref(recipientPhone)
	o.AddressLine, o.City, o.Province, o.PostalCode = deref(addressLine), deref(city), deref(province), deref(postalCode)
	o.Notes = deref(notes)
	o.PaymentMethod = deref(paymentMethod)
	o.CashierShiftID = deref(cashierShiftID)

	return &o, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *Repository) scanOne(ctx context.Context, query string, args ...any) (*Order, error) {
	row := r.db.QueryRow(ctx, query, args...)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return o, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Order, error) {
	o, err := r.scanOne(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return r.hydrate(ctx, o)
}

// GetByOrderNumber is used by payments' webhook handler — the provider
// only ever echoes back the order_number it was given (Duitku calls it
// merchantOrderId), never our internal UUID. No phone/ownership check
// here, unlike GetByOrderNumberAndPhone: this path is authenticated by
// the webhook's own signature verification, not by the caller proving
// they know the order.
func (r *Repository) GetByOrderNumber(ctx context.Context, orderNumber string) (*Order, error) {
	o, err := r.scanOne(ctx, `SELECT `+orderColumns+` FROM orders WHERE order_number = $1`, orderNumber)
	if err != nil {
		return nil, err
	}
	return r.hydrate(ctx, o)
}

// GetByOrderNumberAndPhone backs guest order lookup — matches against
// either guest_phone or recipient_phone, since a guest may have only
// filled in one depending on pickup vs delivery.
func (r *Repository) GetByOrderNumberAndPhone(ctx context.Context, orderNumber, phone string) (*Order, error) {
	o, err := r.scanOne(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE order_number = $1 AND (guest_phone = $2 OR recipient_phone = $2)`,
		orderNumber, phone,
	)
	if err != nil {
		return nil, err
	}
	return r.hydrate(ctx, o)
}

// ListForCustomer/ListForBranch intentionally return un-hydrated orders
// (no items/status history loaded) — same light-list-vs-full-detail
// split as products.ListItem vs Detail. Fetch by ID for the full picture.
func (r *Repository) ListForCustomer(ctx context.Context, customerID string) ([]Order, error) {
	return r.list(ctx, `SELECT `+orderColumns+` FROM orders WHERE customer_id = $1 ORDER BY created_at DESC`, customerID)
}

func (r *Repository) ListForBranch(ctx context.Context, branchID string) ([]Order, error) {
	return r.list(ctx, `SELECT `+orderColumns+` FROM orders WHERE branch_id = $1 ORDER BY created_at DESC`, branchID)
}

func (r *Repository) list(ctx context.Context, query string, arg string) ([]Order, error) {
	rows, err := r.db.Query(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *o)
	}
	return orders, rows.Err()
}

func (r *Repository) hydrate(ctx context.Context, o *Order) (*Order, error) {
	items, err := r.itemsFor(ctx, o.ID)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	o.Items = items

	history, err := r.statusHistoryFor(ctx, o.ID)
	if err != nil {
		return nil, fmt.Errorf("load status history: %w", err)
	}
	o.StatusHistory = history

	return o, nil
}

func (r *Repository) itemsFor(ctx context.Context, orderID string) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, product_variant_id, product_name, variant_name, sku, unit_price, quantity, subtotal
		FROM order_items WHERE order_id = $1 ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		var it Item
		var variantID, sku *string
		err := rows.Scan(&it.ID, &variantID, &it.ProductName, &it.VariantName, &sku, &it.UnitPrice, &it.Quantity, &it.Subtotal)
		if err != nil {
			return nil, err
		}
		it.ProductVariantID, it.SKU = deref(variantID), deref(sku)
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *Repository) statusHistoryFor(ctx context.Context, orderID string) ([]StatusEvent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT status, note, created_at FROM order_status_histories WHERE order_id = $1 ORDER BY created_at ASC`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]StatusEvent, 0)
	for rows.Next() {
		var e StatusEvent
		var note *string
		if err := rows.Scan(&e.Status, &note, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Note = deref(note)
		events = append(events, e)
	}
	return events, rows.Err()
}

// ListAdmin backs the SUPER_ADMIN/ADMIN cross-branch order view (Phase
// 9) — filtered and paginated, unlike ListForBranch/ListForCustomer,
// since "every order on the platform" has no natural bound the way one
// branch's or one customer's orders do. Reuses orderColumns/scanOrder
// completely unchanged from the rest of this file: this is a new,
// separate query, not a modification of anything the checkout/status
// paths already depend on.
func (r *Repository) ListAdmin(ctx context.Context, f AdminListFilter) (AdminListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}

	clauses := []string{"1=1"}
	var args []any
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = %s", next(f.Status)))
	}
	if f.BranchID != "" {
		clauses = append(clauses, fmt.Sprintf("branch_id = %s", next(f.BranchID)))
	}
	if f.Search != "" {
		p := next("%" + f.Search + "%")
		clauses = append(clauses, fmt.Sprintf(
			"(order_number ILIKE %s OR guest_name ILIKE %s OR guest_phone ILIKE %s OR recipient_name ILIKE %s OR recipient_phone ILIKE %s)",
			p, p, p, p, p,
		))
	}
	whereSQL := strings.Join(clauses, " AND ")

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM orders WHERE %s`, whereSQL)
	if err := r.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return AdminListResult{}, fmt.Errorf("count orders: %w", err)
	}

	pageArgs := append(append([]any{}, args...), f.Limit, (f.Page-1)*f.Limit)
	listQ := fmt.Sprintf(
		`SELECT `+orderColumns+` FROM orders WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereSQL, len(pageArgs)-1, len(pageArgs),
	)
	rows, err := r.db.Query(ctx, listQ, pageArgs...)
	if err != nil {
		return AdminListResult{}, fmt.Errorf("select orders: %w", err)
	}
	defer rows.Close()

	items := make([]Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return AdminListResult{}, err
		}
		items = append(items, *o)
	}
	if err := rows.Err(); err != nil {
		return AdminListResult{}, err
	}

	totalPages := (total + f.Limit - 1) / f.Limit
	if totalPages < 1 {
		totalPages = 1
	}

	return AdminListResult{
		Items:      items,
		Total:      total,
		Page:       f.Page,
		Limit:      f.Limit,
		TotalPages: totalPages,
	}, nil
}

// ExpiredCandidateIDs returns PENDING_PAYMENT order IDs past their
// expires_at — used both by Service's lazy per-order check and by
// SweepExpired's bulk pass.
func (r *Repository) ExpiredCandidateIDs(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id FROM orders WHERE status = $1 AND expires_at < now() ORDER BY expires_at ASC LIMIT $2`,
		StatusPendingPayment, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Package orders implements checkout and order management: turning a
// cart into a real order with reserved stock (section 21), snapshotted
// line items that never change even if the product they came from does
// (section 84), and a validated status lifecycle (section 24).
//
// PENDING_PAYMENT orders are lazily expired: GetByID checks expires_at
// on every read and expires the order right there if it's overdue, so a
// customer or staff member never sees a stale "pay now" order that has
// actually timed out. SweepExpired does the same thing in bulk for
// orders nobody happens to look at — meant to be called periodically
// (a cron hitting the wrapping admin endpoint), since without it a
// reservation could sit held forever if nobody ever fetches that order
// again. Real payment processing (what actually moves PENDING_PAYMENT
// forward under normal circumstances) is Phase 8 — this phase's
// PENDING_PAYMENT -> PAID transition exists for the cash/POS path
// (section 28: a cashier confirms payment directly), not a gateway.
package orders

import "time"

type Item struct {
	ID               string `json:"id"`
	ProductVariantID string `json:"product_variant_id,omitempty"`
	ProductName      string `json:"product_name"`
	VariantName      string `json:"variant_name"`
	SKU              string `json:"sku,omitempty"`
	UnitPrice        int64  `json:"unit_price"`
	Quantity         int    `json:"quantity"`
	Subtotal         int64  `json:"subtotal"`
}

type StatusEvent struct {
	Status    string    `json:"status"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Order struct {
	ID             string     `json:"id"`
	OrderNumber    string     `json:"order_number"`
	CustomerID     string     `json:"-"` // internal only — ownership checks, never serialized
	BranchID       string     `json:"branch_id"`
	OrderType      string     `json:"order_type"`
	Status         string     `json:"status"`
	Subtotal       int64      `json:"subtotal"`
	Total          int64      `json:"total"`
	GuestName      string     `json:"guest_name,omitempty"`
	GuestPhone     string     `json:"guest_phone,omitempty"`
	GuestEmail     string     `json:"guest_email,omitempty"`
	RecipientName  string     `json:"recipient_name,omitempty"`
	RecipientPhone string     `json:"recipient_phone,omitempty"`
	AddressLine    string     `json:"address_line,omitempty"`
	City           string     `json:"city,omitempty"`
	Province       string     `json:"province,omitempty"`
	PostalCode     string     `json:"postal_code,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	// PaymentMethod and CashierShiftID are Phase 10 additions — 'cash' |
	// 'qris' | "" (online orders not yet paid, or paid before Phase 10's
	// column existed), and the POS shift a sale was rung up under, if
	// any. See migration 000008_pos's column comments.
	PaymentMethod  string        `json:"payment_method,omitempty"`
	CashierShiftID string        `json:"cashier_shift_id,omitempty"`
	Items          []Item        `json:"items"`
	StatusHistory  []StatusEvent `json:"status_history"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type CheckoutRequest struct {
	OrderType      string `json:"order_type" binding:"required,oneof=pickup delivery"`
	GuestName      string `json:"guest_name"`
	GuestPhone     string `json:"guest_phone"`
	GuestEmail     string `json:"guest_email" binding:"omitempty,email"`
	RecipientName  string `json:"recipient_name"`
	RecipientPhone string `json:"recipient_phone"`
	AddressLine    string `json:"address_line"`
	City           string `json:"city"`
	Province       string `json:"province"`
	PostalCode     string `json:"postal_code"`
	Notes          string `json:"notes"`
	// CashierShiftID marks this checkout as a POS sale (Phase 10) rather
	// than a customer/guest one. `json:"-"` is deliberate: the public
	// POST /orders route binds this struct straight from the request
	// body, so this field must never be settable by a client — it's set
	// server-side, field-by-field, only by Handler.POSCheckout, derived
	// from the authenticated cashier's own open shift.
	CashierShiftID string `json:"-"`
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
	// PaymentMethod optionally records how an order was paid ('cash' or
	// 'qris') in the same call that moves it to PAID. Empty for every
	// caller before Phase 10 — the branch-scoped cash-confirm action
	// (see Handler.UpdateStatus) is the first thing that sets it.
	PaymentMethod string `json:"payment_method"`
}

type LookupRequest struct {
	OrderNumber string `json:"order_number" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
}

// AdminListFilter/AdminListResult back Handler.AdminList (Phase 9) — the
// SUPER_ADMIN/ADMIN-only cross-branch order view. ListForBranch already
// covers "orders at one branch" for branch staff; this is the admin
// dashboard's need to see everything, optionally narrowed, and paged —
// unlike ListForBranch/ListForCustomer, which stay unpaginated since
// they're each naturally bounded (one branch, one customer).
type AdminListFilter struct {
	Status   string
	BranchID string
	Search   string // matches order_number, guest_name, guest_phone, recipient_name, recipient_phone
	Page     int
	Limit    int
}

type AdminListResult struct {
	Items      []Order `json:"items"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	TotalPages int     `json:"total_pages"`
}

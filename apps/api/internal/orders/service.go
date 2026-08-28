package orders

import (
	"context"
	"errors"
	"fmt"
	"time"

	"satelit-parfume-api/internal/cart"
	"satelit-parfume-api/internal/inventory"
	"satelit-parfume-api/internal/shifts"
	"satelit-parfume-api/internal/stock"
)

var (
	ErrEmptyCart         = errors.New("cart is empty")
	ErrNoBranchSelected  = errors.New("cart has no branch selected")
	ErrGuestInfoRequired = errors.New("name and phone are required for guest checkout")
	ErrAddressRequired   = errors.New("delivery orders require a recipient name, phone, and address")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrInvalidStatus     = errors.New("unrecognized status")
	ErrOrderTypeMismatch = errors.New("this status doesn't apply to this order's type")
	ErrWrongBranch       = errors.New("order does not belong to this branch")
	ErrNotOwner          = errors.New("this order does not belong to you")
	// ErrShiftNotOpen/ErrShiftWrongBranch back Phase 10's POS checkout —
	// see Checkout's CashierShiftID handling below.
	ErrShiftNotOpen     = errors.New("cashier shift is not open")
	ErrShiftWrongBranch = errors.New("cashier shift belongs to a different branch")
)

// reservationWindow is how long a PENDING_PAYMENT order holds its stock
// before GetByID/SweepExpired consider it stale. 30 minutes is a
// reasonable default for a manual/cash-leaning checkout; revisit once
// Phase 8 adds a real payment gateway with its own typical completion
// time.
const reservationWindow = 30 * time.Minute

type Service struct {
	repo      *Repository
	cart      *cart.Service
	inventory *inventory.Repository
	shifts    *shifts.Repository
	stock     *stock.Repository
}

func NewService(repo *Repository, cartService *cart.Service, inventoryRepo *inventory.Repository, shiftsRepo *shifts.Repository, stockRepo *stock.Repository) *Service {
	return &Service{repo: repo, cart: cartService, inventory: inventoryRepo, shifts: shiftsRepo, stock: stockRepo}
}

// Checkout re-validates and reserves stock for every cart line inside one
// transaction (section 21), snapshots each line into order_items
// (section 84), and only clears the cart after that transaction commits.
// Stock is re-checked here rather than trusted from whatever the cart
// last confirmed — time has passed since items were added, and someone
// else may have bought the same units in the meantime.
//
// req.CashierShiftID (Phase 10) marks this as a POS sale rather than a
// customer/guest one: Handler.POSCheckout sets it from the authenticated
// cashier's own open shift (never from a client-supplied value — see
// CheckoutRequest's doc comment), and when it's set, the ordinary
// guest-info requirement below is skipped, since a walk-in POS sale has
// no name or phone to collect the way Phase 7's guest checkout does.
func (s *Service) Checkout(ctx context.Context, identity cart.Identity, req CheckoutRequest) (*Order, error) {
	c, err := s.cart.Get(ctx, identity)
	if err != nil {
		return nil, fmt.Errorf("load cart: %w", err)
	}
	if len(c.Items) == 0 {
		return nil, ErrEmptyCart
	}
	if c.BranchID == nil {
		return nil, ErrNoBranchSelected
	}

	var shiftID *string
	if req.CashierShiftID != "" {
		shift, err := s.shifts.GetByID(ctx, req.CashierShiftID)
		if err != nil {
			return nil, fmt.Errorf("load cashier shift: %w", err)
		}
		if shift.Status != "open" {
			return nil, ErrShiftNotOpen
		}
		if shift.BranchID != *c.BranchID {
			return nil, ErrShiftWrongBranch
		}
		shiftID = &req.CashierShiftID
	} else if identity.CustomerID == "" && (req.GuestName == "" || req.GuestPhone == "") {
		return nil, ErrGuestInfoRequired
	}

	if req.OrderType == "delivery" {
		if req.RecipientName == "" || req.RecipientPhone == "" || req.AddressLine == "" || req.City == "" {
			return nil, ErrAddressRequired
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed below

	for _, item := range c.Items {
		if err := s.inventory.ReserveStock(ctx, tx, *c.BranchID, item.ProductVariantID, item.Quantity); err != nil {
			if errors.Is(err, inventory.ErrInsufficientStock) {
				return nil, fmt.Errorf("%w: %s", inventory.ErrInsufficientStock, item.ProductName)
			}
			return nil, fmt.Errorf("reserve stock: %w", err)
		}
	}

	orderNumber, err := s.repo.NextOrderNumber(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("generate order number: %w", err)
	}

	var customerID *string
	if identity.CustomerID != "" {
		customerID = &identity.CustomerID
	}

	orderID, err := s.repo.CreateOrder(ctx, tx, CreateOrderParams{
		OrderNumber:    orderNumber,
		CustomerID:     customerID,
		GuestName:      req.GuestName,
		GuestPhone:     req.GuestPhone,
		GuestEmail:     req.GuestEmail,
		BranchID:       *c.BranchID,
		OrderType:      req.OrderType,
		Subtotal:       c.Subtotal,
		Total:          c.Subtotal, // no promotions to discount it yet — Phase 12
		RecipientName:  req.RecipientName,
		RecipientPhone: req.RecipientPhone,
		AddressLine:    req.AddressLine,
		City:           req.City,
		Province:       req.Province,
		PostalCode:     req.PostalCode,
		Notes:          req.Notes,
		ExpiresAt:      time.Now().Add(reservationWindow),
		CashierShiftID: shiftID,
	})
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	for _, item := range c.Items {
		err := s.repo.CreateOrderItem(ctx, tx, CreateOrderItemParams{
			OrderID:          orderID,
			ProductVariantID: item.ProductVariantID,
			BranchID:         *c.BranchID,
			ProductName:      item.ProductName,
			VariantName:      item.VariantName,
			// No SKU snapshot yet: none of the real imported products
			// have one verified (seeds/products_verified.csv left it
			// blank), so cart.Item doesn't carry it either — nothing to
			// snapshot that would be more than an empty string today.
			UnitPrice: item.UnitPrice,
			Quantity:  item.Quantity,
			Subtotal:  item.LineTotal,
		})
		if err != nil {
			return nil, fmt.Errorf("create order item: %w", err)
		}
	}

	if err := s.repo.AddStatusHistory(ctx, tx, orderID, StatusPendingPayment, "order created"); err != nil {
		return nil, fmt.Errorf("record status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Best-effort, outside the transaction and after commit: the order
	// is already correct and complete at this point, so a failure here
	// is a UX nuisance (stale cart shown briefly), never a data
	// integrity problem the way a half-committed order would be.
	_ = s.cart.Clear(ctx, identity)

	return s.repo.GetByID(ctx, orderID)
}

// GetByID performs the lazy expiry check described in the package doc
// comment before returning: a PENDING_PAYMENT order past its
// expires_at gets expired right here, so nobody is ever shown a "pay
// now" order that has actually already timed out.
func (s *Service) GetByID(ctx context.Context, orderID string) (*Order, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return s.expireIfDue(ctx, order, func() (*Order, error) { return s.repo.GetByID(ctx, orderID) })
}

// expireIfDue is the lazy-expiry check shared by every order-fetch path
// (GetByID, GetByOrderNumber): a PENDING_PAYMENT order past its window
// gets expired — and its stock reservation released — right here, rather
// than showing stale "still pending" data. refetch is only called if the
// expiry transition loses a race (someone else already moved the order
// between the caller's initial fetch and this check).
func (s *Service) expireIfDue(ctx context.Context, order *Order, refetch func() (*Order, error)) (*Order, error) {
	if !s.isExpired(order) {
		return order, nil
	}

	expired, err := s.applyTransition(ctx, order, StatusExpired, "reservation window elapsed")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return refetch()
		}
		return nil, err
	}
	return expired, nil
}

func (s *Service) isExpired(o *Order) bool {
	return o.Status == StatusPendingPayment && o.ExpiresAt != nil && o.ExpiresAt.Before(time.Now())
}

func (s *Service) ListForCustomer(ctx context.Context, customerID string) ([]Order, error) {
	return s.repo.ListForCustomer(ctx, customerID)
}

func (s *Service) ListForBranch(ctx context.Context, branchID string) ([]Order, error) {
	return s.repo.ListForBranch(ctx, branchID)
}

// SetPaymentMethod is Repository.SetPaymentMethod's pass-through — see
// that method's doc comment for why it's deliberately separate from
// UpdateStatus.
func (s *Service) SetPaymentMethod(ctx context.Context, orderID, method string) error {
	return s.repo.SetPaymentMethod(ctx, orderID, method)
}

// GetOpenShiftForUser is shifts.Repository.GetOpenForUser's pass-through
// — Handler.POSCheckout's way of finding "which shift is this sale
// under" without Handler needing its own separate dependency on
// internal/shifts alongside Service.
func (s *Service) GetOpenShiftForUser(ctx context.Context, userID string) (*shifts.Shift, error) {
	return s.shifts.GetOpenForUser(ctx, userID)
}

// ListAdmin is the SUPER_ADMIN/ADMIN cross-branch equivalent of
// ListForBranch — see AdminListFilter's doc comment in model.go.
func (s *Service) ListAdmin(ctx context.Context, f AdminListFilter) (AdminListResult, error) {
	return s.repo.ListAdmin(ctx, f)
}

// GetByOrderNumber is payments' entry point for its webhook handler —
// routed through this Service method (not repo.GetByOrderNumber
// directly) so it gets the same lazy-expiry check GetByID applies: a
// webhook arriving for an order that's actually already timed out
// should see it as EXPIRED, not stale PENDING_PAYMENT data.
func (s *Service) GetByOrderNumber(ctx context.Context, orderNumber string) (*Order, error) {
	order, err := s.repo.GetByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, err
	}
	return s.expireIfDue(ctx, order, func() (*Order, error) { return s.repo.GetByOrderNumber(ctx, orderNumber) })
}

func (s *Service) LookupGuestOrder(ctx context.Context, orderNumber, phone string) (*Order, error) {
	return s.repo.GetByOrderNumberAndPhone(ctx, orderNumber, phone)
}

// Cancel is the customer/staff-facing shortcut for the one transition a
// non-staff caller should ever trigger directly by choice (expiry
// happens on its own; the rest of the lifecycle is staff/POS-driven).
// Goes through GetByID (not repo.GetByID) so an order that's actually
// already past its reservation window gets recognized as EXPIRED first,
// rather than jumping straight to CANCELLED on stale data. UpdateStatus's
// own CanTransition check is what actually rejects cancelling anything
// past PENDING_PAYMENT — no need to duplicate that here.
func (s *Service) Cancel(ctx context.Context, orderID string) (*Order, error) {
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return s.UpdateStatus(ctx, order, StatusCancelled, "cancelled by request")
}

// UpdateStatus validates the transition (graph + order-type match), then
// applies it and whatever stock side-effect that specific transition
// implies (deduct on -> PAID, release on -> CANCELLED/EXPIRED), all
// inside one transaction — a stock change can never happen without the
// status change that justified it, or vice versa.
func (s *Service) UpdateStatus(ctx context.Context, order *Order, newStatus, note string) (*Order, error) {
	if !isValidStatus(newStatus) {
		return nil, ErrInvalidStatus
	}
	if !CanTransition(order.Status, newStatus) {
		return nil, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, order.Status, newStatus)
	}
	if err := validateOrderTypeForStatus(order.OrderType, newStatus); err != nil {
		return nil, err
	}
	return s.applyTransition(ctx, order, newStatus, note)
}

// applyTransition is the shared, already-validated core both
// UpdateStatus and GetByID's lazy expiry use — GetByID's expiry doesn't
// go through the public validation (it's driven by a timestamp, not a
// caller's choice) but still needs the exact same atomic
// status-change + stock-side-effect + history-entry behavior.
func (s *Service) applyTransition(ctx context.Context, order *Order, newStatus, note string) (*Order, error) {
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := s.repo.UpdateStatus(ctx, tx, order.ID, order.Status, newStatus); err != nil {
		return nil, err
	}

	switch newStatus {
	case StatusPaid:
		for _, item := range order.Items {
			if item.ProductVariantID == "" {
				continue
			}
			if err := s.inventory.DeductStock(ctx, tx, order.BranchID, item.ProductVariantID, item.Quantity); err != nil {
				return nil, fmt.Errorf("deduct stock: %w", err)
			}
			// Phase 11: every real stock_quantity change gets a ledger
			// line — see internal/stock's package doc comment. Best-effort
			// in the sense that a logging failure here still fails the
			// whole transaction (unlike Phase 10's post-commit
			// SetPaymentMethod calls): this INSERT runs inside the exact
			// same tx as DeductStock, so either both the stock change and
			// its ledger line commit together, or neither does — there's
			// no risk of a stock change existing with no record of why.
			if _, err := s.stock.LogMovement(ctx, tx, stock.LogMovementParams{
				BranchID: order.BranchID, VariantID: item.ProductVariantID, QuantityChange: -item.Quantity,
				Reason: "sale", ReferenceType: "order", ReferenceID: order.ID,
			}); err != nil {
				return nil, fmt.Errorf("log stock movement: %w", err)
			}
		}
	case StatusCancelled, StatusExpired:
		for _, item := range order.Items {
			if item.ProductVariantID == "" {
				continue
			}
			if err := s.inventory.ReleaseStock(ctx, tx, order.BranchID, item.ProductVariantID, item.Quantity); err != nil {
				return nil, fmt.Errorf("release stock: %w", err)
			}
		}
	}

	if err := s.repo.AddStatusHistory(ctx, tx, order.ID, newStatus, note); err != nil {
		return nil, fmt.Errorf("record status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.repo.GetByID(ctx, order.ID)
}

func validateOrderTypeForStatus(orderType, status string) error {
	switch status {
	case StatusReadyForPickup:
		if orderType != "pickup" {
			return fmt.Errorf("%w: %s is a pickup-only status", ErrOrderTypeMismatch, status)
		}
	case StatusShipped, StatusDelivered:
		if orderType != "delivery" {
			return fmt.Errorf("%w: %s is a delivery-only status", ErrOrderTypeMismatch, status)
		}
	}
	return nil
}

// SweepExpired finds PENDING_PAYMENT orders past their expires_at and
// expires them in bulk. GetByID's lazy check handles the common case (a
// customer or staff member actually looks at the order); this exists for
// the orders nobody looks at, so their reservation doesn't sit held
// forever. Meant to be called periodically — see the root README for how
// to wire up a cron against the admin endpoint that wraps this.
func (s *Service) SweepExpired(ctx context.Context, limit int) (expiredCount int, err error) {
	ids, err := s.repo.ExpiredCandidateIDs(ctx, limit)
	if err != nil {
		return 0, err
	}

	for _, id := range ids {
		order, err := s.repo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return expiredCount, err
		}

		if _, err := s.applyTransition(ctx, order, StatusExpired, "reservation window elapsed"); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue // someone else already moved it — not a failure
			}
			return expiredCount, err
		}
		expiredCount++
	}
	return expiredCount, nil
}

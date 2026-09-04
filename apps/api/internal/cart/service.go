package cart

import (
	"context"
	"errors"
	"fmt"

	"satelit-parfume-api/internal/inventory"
)

var (
	ErrDifferentBranch   = errors.New("cart is already associated with a different branch")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Service struct {
	repo      *Repository
	inventory *inventory.Repository
}

func NewService(repo *Repository, inventoryRepo *inventory.Repository) *Service {
	return &Service{repo: repo, inventory: inventoryRepo}
}

// resolve finds or creates the cart for identity. A customer always gets
// exactly one cart, created on first use. A guest with no token yet (or
// a token that no longer matches anything — stale localStorage, wiped
// data) gets a brand new one; the caller is responsible for persisting
// the SessionToken that comes back on it.
//
// When identity carries both a CustomerID and a SessionToken — a
// customer who added items before logging in, tracked under the guest
// X-Cart-Token the whole time — the guest cart behind that token is
// folded into the customer's own cart first. Without this, a customer
// cart is always found-or-created by CustomerID alone, and a first-time
// customer's own cart is *always* freshly created and therefore empty:
// their items, added and still sitting under the old guest token, would
// simply never be looked at again. Checkout is where this mattered in
// practice — a checkout attempt right after logging in mid-flow got
// ErrEmptyCart from what looked, to the person checking out, like a
// cart that had items in it seconds earlier.
func (s *Service) resolve(ctx context.Context, identity Identity) (*shell, error) {
	if identity.CustomerID != "" {
		if identity.SessionToken != "" {
			// Best-effort: a hiccup while merging shouldn't block a
			// customer from reaching their own, already-established
			// cart — worst case here is a repeat customer's older,
			// unrelated guest token failing to fold in this one time.
			_ = s.mergeGuestIntoCustomer(ctx, identity.CustomerID, identity.SessionToken)
		}
		sh, err := s.repo.FindByCustomerID(ctx, identity.CustomerID)
		if errors.Is(err, ErrNotFound) {
			return s.repo.CreateForCustomer(ctx, identity.CustomerID)
		}
		return sh, err
	}

	if identity.SessionToken != "" {
		sh, err := s.repo.FindBySessionToken(ctx, identity.SessionToken)
		if err == nil {
			return sh, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		// Falls through to creating a fresh cart — from the shopper's
		// side, "start a new cart" is the only sensible recovery from a
		// token that doesn't match anything anymore.
	}

	return s.repo.CreateGuestCart(ctx)
}

// mergeGuestIntoCustomer folds guestSessionToken's cart into customerID's
// own cart: same one-branch-per-cart rule AddItem enforces (an empty
// customer cart just adopts the guest cart's branch), same
// availability-aware quantity logic (clamped rather than failing
// outright if stock moved in the meantime), and the same UpsertItem the
// guest cart's own additions went through — a merge is just each of
// those items being "added" again, to a different cart. No-ops quietly
// (nil, no error) whenever there's nothing to merge: an unknown or
// already-guest-only token, or a real one with an empty cart behind it.
// The guest cart is cleared, not deleted — same as a normal checkout
// leaves it — so a stale reference to its id elsewhere still resolves to
// a valid, simply-empty cart rather than a dangling one.
func (s *Service) mergeGuestIntoCustomer(ctx context.Context, customerID, guestSessionToken string) error {
	guest, err := s.repo.FindBySessionToken(ctx, guestSessionToken)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}

	items, err := s.repo.ItemsFor(ctx, guest.ID)
	if err != nil {
		return fmt.Errorf("load guest cart items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	mine, err := s.repo.FindByCustomerID(ctx, customerID)
	if errors.Is(err, ErrNotFound) {
		mine, err = s.repo.CreateForCustomer(ctx, customerID)
	}
	if err != nil {
		return err
	}

	if mine.BranchID == nil && guest.BranchID != nil {
		if err := s.repo.SetBranch(ctx, mine.ID, *guest.BranchID); err != nil {
			return err
		}
	}

	for _, item := range items {
		existingQty, err := s.repo.ExistingQuantity(ctx, mine.ID, item.ProductVariantID)
		if err != nil {
			return err
		}
		available, _, err := s.inventory.AvailableStockForVariant(ctx, item.BranchID, item.ProductVariantID)
		if err != nil {
			// Best-effort here too: Checkout re-validates stock for
			// real (ReserveStock, inside its own transaction) right
			// after this — an availability lookup hiccup during the
			// merge just means this one line doesn't carry over,
			// not that the whole checkout should fail on the spot.
			continue
		}
		qty := item.Quantity
		if existingQty+qty > available {
			qty = available - existingQty
		}
		if qty <= 0 {
			continue
		}
		// item.UnitPrice, not today's price from AvailableStockForVariant
		// above: a merge isn't a new add-to-cart decision, so it keeps
		// whatever price the guest cart already snapshotted, the same
		// way an existing line surviving a repeat AddItem call does
		// (see UpsertItem's own doc comment).
		if err := s.repo.UpsertItem(ctx, mine.ID, item.BranchID, item.ProductVariantID, qty, item.UnitPrice); err != nil {
			return err
		}
	}

	return s.repo.Clear(ctx, guest.ID)
}

func (s *Service) Get(ctx context.Context, identity Identity) (*Cart, error) {
	sh, err := s.resolve(ctx, identity)
	if err != nil {
		return nil, err
	}
	return s.assemble(ctx, sh)
}

// AddItem enforces the two rules that are actually this phase's scope
// (see package doc comment): one branch per cart, and the combined
// quantity (already-in-cart + this request) can't exceed what's
// available — not a reservation, a check.
func (s *Service) AddItem(ctx context.Context, identity Identity, req AddItemRequest) (*Cart, error) {
	sh, err := s.resolve(ctx, identity)
	if err != nil {
		return nil, err
	}

	if sh.BranchID != nil && *sh.BranchID != req.BranchID {
		return nil, ErrDifferentBranch
	}

	available, unitPrice, err := s.inventory.AvailableStockForVariant(ctx, req.BranchID, req.ProductVariantID)
	if err != nil {
		return nil, fmt.Errorf("check availability: %w", err)
	}

	existingQty, err := s.repo.ExistingQuantity(ctx, sh.ID, req.ProductVariantID)
	if err != nil {
		return nil, err
	}

	if existingQty+req.Quantity > available {
		return nil, fmt.Errorf("%w: only %d available", ErrInsufficientStock, available)
	}

	if err := s.repo.UpsertItem(ctx, sh.ID, req.BranchID, req.ProductVariantID, req.Quantity, unitPrice); err != nil {
		return nil, err
	}

	if sh.BranchID == nil {
		if err := s.repo.SetBranch(ctx, sh.ID, req.BranchID); err != nil {
			return nil, err
		}
		// Without this, assemble() below would still read the
		// pre-update sh.BranchID (nil) and report branch_id: null in
		// the response, even though the database was just updated —
		// the caller would see their own just-selected branch vanish.
		sh.BranchID = &req.BranchID
	}

	return s.assemble(ctx, sh)
}

func (s *Service) UpdateItemQuantity(ctx context.Context, identity Identity, itemID string, quantity int) (*Cart, error) {
	sh, err := s.resolve(ctx, identity)
	if err != nil {
		return nil, err
	}

	item, err := s.repo.GetItem(ctx, sh.ID, itemID)
	if err != nil {
		return nil, err
	}

	available, _, err := s.inventory.AvailableStockForVariant(ctx, item.BranchID, item.ProductVariantID)
	if err != nil {
		return nil, fmt.Errorf("check availability: %w", err)
	}
	if quantity > available {
		return nil, fmt.Errorf("%w: only %d available", ErrInsufficientStock, available)
	}

	if err := s.repo.UpdateItemQuantity(ctx, sh.ID, itemID, quantity); err != nil {
		return nil, err
	}

	return s.assemble(ctx, sh)
}

func (s *Service) RemoveItem(ctx context.Context, identity Identity, itemID string) (*Cart, error) {
	sh, err := s.resolve(ctx, identity)
	if err != nil {
		return nil, err
	}
	if err := s.repo.RemoveItem(ctx, sh.ID, itemID); err != nil {
		return nil, err
	}
	return s.assemble(ctx, sh)
}

func (s *Service) Clear(ctx context.Context, identity Identity) error {
	sh, err := s.resolve(ctx, identity)
	if err != nil {
		return err
	}
	return s.repo.Clear(ctx, sh.ID)
}

func (s *Service) assemble(ctx context.Context, sh *shell) (*Cart, error) {
	items, err := s.repo.ItemsFor(ctx, sh.ID)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}

	var subtotal int64
	for _, it := range items {
		subtotal += it.LineTotal
	}

	cart := &Cart{
		ID:        sh.ID,
		BranchID:  sh.BranchID,
		Items:     items,
		ItemCount: len(items),
		Subtotal:  subtotal,
		UpdatedAt: sh.UpdatedAt,
	}
	if sh.SessionToken != nil {
		cart.SessionToken = *sh.SessionToken
	}
	return cart, nil
}

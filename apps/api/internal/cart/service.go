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
func (s *Service) resolve(ctx context.Context, identity Identity) (*shell, error) {
	if identity.CustomerID != "" {
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

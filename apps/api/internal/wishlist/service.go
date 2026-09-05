package wishlist

import (
	"context"
	"errors"

	"satelit-parfume-api/internal/products"
)

type Service struct {
	repo     *Repository
	products *products.Repository
}

func NewService(repo *Repository, productsRepo *products.Repository) *Service {
	return &Service{repo: repo, products: productsRepo}
}

// Add checks the product actually exists first — without this, a bad
// product_id would only ever surface once it hit the database, as a raw
// foreign-key violation. Same class of unhelpful, hard-to-read 500 that
// migration 000010 exists to fix on the orders side; cheaper to just not
// have it here in the first place.
func (s *Service) Add(ctx context.Context, customerID, productID string) error {
	if _, err := s.products.GetByID(ctx, productID); err != nil {
		if errors.Is(err, products.ErrNotFound) {
			return products.ErrNotFound
		}
		return err
	}
	return s.repo.Add(ctx, customerID, productID)
}

func (s *Service) Remove(ctx context.Context, customerID, productID string) error {
	return s.repo.Remove(ctx, customerID, productID)
}

func (s *Service) List(ctx context.Context, customerID string) ([]Entry, error) {
	return s.repo.List(ctx, customerID)
}

func (s *Service) IsWishlisted(ctx context.Context, customerID, productID string) (bool, error) {
	return s.repo.IsWishlisted(ctx, customerID, productID)
}

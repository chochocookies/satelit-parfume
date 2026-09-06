package reviews

import (
	"context"
	"errors"
)

var (
	ErrNotEligible   = errors.New("you can only review a product from a completed order")
	ErrInvalidRating = errors.New("rating must be between 1 and 5")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Submit is both "leave a review" and "edit my review" — see
// Repository.Upsert's own doc comment. Rating is re-checked here (not
// just left to the table's CHECK constraint) for the same reason
// migration 000010 exists at all: a bad value should come back as a
// clean, readable error from the application layer, not a raw
// "violates check constraint" from Postgres.
func (s *Service) Submit(ctx context.Context, customerID, productID string, rating int, comment string) error {
	if rating < 1 || rating > 5 {
		return ErrInvalidRating
	}
	orderID, eligible, err := s.repo.LatestCompletedOrder(ctx, customerID, productID)
	if err != nil {
		return err
	}
	if !eligible {
		return ErrNotEligible
	}
	return s.repo.Upsert(ctx, customerID, productID, orderID, rating, comment)
}

func (s *Service) Delete(ctx context.Context, customerID, productID string) error {
	return s.repo.Delete(ctx, customerID, productID)
}

func (s *Service) ListForProduct(ctx context.Context, productID string) ([]Review, error) {
	return s.repo.ListForProduct(ctx, productID)
}

func (s *Service) SummaryForProduct(ctx context.Context, productID string) (Summary, error) {
	return s.repo.SummaryForProduct(ctx, productID)
}

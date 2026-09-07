package promotions

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrCodeInactive = errors.New("promo code is not active")
	ErrCodeExpired  = errors.New("promo code has expired")
	ErrCodeNotYet   = errors.New("promo code is not valid yet")
	ErrUsesExceeded = errors.New("promo code has reached its usage limit")
	ErrBelowMinimum = errors.New("subtotal is below this code's minimum purchase")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// checkEligibility is the ruleset both Validate and ApplyInTx run their
// fetched row through — same rules either way, just fetched two
// different ways for two different concurrency needs (see
// GetByCodeForUpdate's own doc comment on why Checkout needs the
// locking one).
func checkEligibility(p *PromoCode, subtotal int64) error {
	if !p.Active {
		return ErrCodeInactive
	}
	now := time.Now()
	if now.Before(p.ValidFrom) {
		return ErrCodeNotYet
	}
	if p.ValidUntil != nil && now.After(*p.ValidUntil) {
		return ErrCodeExpired
	}
	if p.MaxUses != nil && p.UsedCount >= *p.MaxUses {
		return ErrUsesExceeded
	}
	if subtotal < p.MinPurchase {
		return ErrBelowMinimum
	}
	return nil
}

// Validate is the checkout form's on-demand "apply this code" check —
// read-only, no locking, safe to call as many times as someone edits
// the code field or their cart changes. Returns the discount subtotal
// would get right now; Checkout re-validates and re-locks for real at
// the moment it actually matters (ApplyInTx) rather than trusting this
// result, since time — and other shoppers using up a limited code —
// passes in between.
func (s *Service) Validate(ctx context.Context, code string, subtotal int64) (*PromoCode, int64, error) {
	promo, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, 0, err
	}
	if err := checkEligibility(promo, subtotal); err != nil {
		return nil, 0, err
	}
	return promo, promo.DiscountFor(subtotal), nil
}

// ApplyInTx is Checkout's own entry point (internal/orders' own
// Service.Checkout): locks the row, re-runs the same eligibility check
// Validate does, increments used_count, and returns the discount — all
// inside Checkout's existing transaction, so a code's use is atomic
// with the order it discounted.
func (s *Service) ApplyInTx(ctx context.Context, tx pgx.Tx, code string, subtotal int64) (*PromoCode, int64, error) {
	promo, err := s.repo.GetByCodeForUpdate(ctx, tx, code)
	if err != nil {
		return nil, 0, err
	}
	if err := checkEligibility(promo, subtotal); err != nil {
		return nil, 0, err
	}
	if err := s.repo.IncrementUsage(ctx, tx, promo.ID); err != nil {
		return nil, 0, err
	}
	return promo, promo.DiscountFor(subtotal), nil
}

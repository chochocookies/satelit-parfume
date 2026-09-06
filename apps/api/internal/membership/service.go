// Package membership computes a customer's loyalty tier from their
// total COMPLETED-order spending — Phase 12, part 3 (see the root
// README's roadmap: wishlist, reviews, membership, promotions, in that
// order — "promotions" is deliberately its own later part, not this
// one; see this package's own note on perks below).
//
// Deliberately read-only and derived, not a stored, mutable balance:
// tier is recalculated from orders.total every time it's asked for,
// rather than tracked as its own running total that could drift out of
// sync with the orders it's supposed to reflect. That also means this
// package never needs to touch Checkout at all — same scope discipline
// internal/reviews' own doc comment explains for why it doesn't touch
// products.Repository.List either.
package membership

import "context"

type Tier string

const (
	TierBronze Tier = "BRONZE"
	TierSilver Tier = "SILVER"
	TierGold   Tier = "GOLD"
)

// thresholds is this package's tier ladder, highest first. Phase 12's
// original spec detail for exact cutoffs isn't available here (unlike
// e.g. the 23 verified products, which came with real prices) — these
// are reasonable defaults sized to this catalog's own price range
// (roughly Rp25,000-55,000 per item), not lifted from an original spec.
// Easy to move to config/env once real numbers are decided.
var thresholds = []struct {
	tier Tier
	min  int64
}{
	{TierGold, 1_000_000},
	{TierSilver, 300_000},
	{TierBronze, 0},
}

func tierFor(totalSpent int64) Tier {
	for _, t := range thresholds {
		if totalSpent >= t.min {
			return t.tier
		}
	}
	return TierBronze
}

// perks is informational copy only — nothing here is mechanically
// enforced. A Gold member isn't actually charged less by any pricing
// code that exists; an automatic tier discount is a checkout-side
// change (recalculating totals, most likely alongside a real discount
// code system) that belongs with Phase 12's own separate "promotions"
// part, not bundled into this one.
var perks = map[Tier][]string{
	TierBronze: {"Member terdaftar Satelit Parfume"},
	TierSilver: {"Member terdaftar Satelit Parfume", "Info promo & produk baru lebih dulu"},
	TierGold: {
		"Member terdaftar Satelit Parfume",
		"Info promo & produk baru lebih dulu",
		"Akses awal ke rilis produk baru",
	},
}

// Status is what GET /api/v1/membership/me returns.
type Status struct {
	Tier             Tier     `json:"tier"`
	TotalSpent       int64    `json:"total_spent"`
	Perks            []string `json:"perks"`
	NextTier         Tier     `json:"next_tier,omitempty"`
	AmountToNextTier int64    `json:"amount_to_next_tier,omitempty"`
}

// statusFor fills in NextTier/AmountToNextTier by walking thresholds
// (highest-first) to find the entry just above the current tier, if
// any — Gold has none, so those two fields stay at their zero value
// (omitted by Status's own omitempty) rather than pointing at nothing.
func statusFor(totalSpent int64) Status {
	tier := tierFor(totalSpent)
	status := Status{Tier: tier, TotalSpent: totalSpent, Perks: perks[tier]}

	for i, t := range thresholds {
		if t.tier == tier && i > 0 {
			next := thresholds[i-1]
			status.NextTier = next.tier
			status.AmountToNextTier = next.min - totalSpent
			break
		}
	}
	return status
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) StatusFor(ctx context.Context, customerID string) (Status, error) {
	total, err := s.repo.TotalSpent(ctx, customerID)
	if err != nil {
		return Status{}, err
	}
	return statusFor(total), nil
}

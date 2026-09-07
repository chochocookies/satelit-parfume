// Package promotions is Phase 12's last part: promo codes a shopper can
// apply at checkout (see the root README's roadmap: wishlist, reviews,
// membership, promotions). The one Phase 12 slice that actually reaches
// into Checkout — internal/orders' own Service.go already had "no
// promotions to discount it yet — Phase 12" on its Total line,
// anticipating exactly this.
//
// Deliberately not gated to logged-in customers the way
// wishlist/reviews/membership are: a guest checkout can use a code too,
// the same way it can pay with cash — there's no reason a promo needs
// an account behind it.
package promotions

import "time"

type DiscountType string

const (
	DiscountPercentage DiscountType = "PERCENTAGE"
	DiscountFixed      DiscountType = "FIXED"
)

type PromoCode struct {
	ID            string       `json:"id"`
	Code          string       `json:"code"`
	Description   string       `json:"description,omitempty"`
	DiscountType  DiscountType `json:"discount_type"`
	DiscountValue int64        `json:"discount_value"`
	MinPurchase   int64        `json:"min_purchase"`
	MaxDiscount   *int64       `json:"max_discount,omitempty"`
	MaxUses       *int         `json:"max_uses,omitempty"`
	UsedCount     int          `json:"used_count"`
	ValidFrom     time.Time    `json:"valid_from"`
	ValidUntil    *time.Time   `json:"valid_until,omitempty"`
	Active        bool         `json:"active"`
	CreatedAt     time.Time    `json:"created_at"`
}

// DiscountFor is the pure arithmetic half of applying a code — "is this
// code even usable right now" (active, dates, max_uses, min_purchase)
// is a separate check in Service, since that half needs a database read
// this doesn't.
func (p PromoCode) DiscountFor(subtotal int64) int64 {
	var discount int64
	switch p.DiscountType {
	case DiscountFixed:
		discount = p.DiscountValue
	case DiscountPercentage:
		discount = subtotal * p.DiscountValue / 100
	}
	if p.MaxDiscount != nil && discount > *p.MaxDiscount {
		discount = *p.MaxDiscount
	}
	if discount > subtotal {
		discount = subtotal // never discount past zero
	}
	return discount
}

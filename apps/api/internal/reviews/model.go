// Package reviews lets a customer rate and comment on a product they've
// actually received — Phase 12, part 2 (see the root README's roadmap:
// wishlist, reviews, membership, promotions). Reading reviews is public;
// writing one requires a COMPLETED order containing the product, checked
// in Service.Submit — see migration 000012's own comment on why that
// status specifically, and why order_id is kept rather than just
// checked-then-discarded.
package reviews

import "time"

// Review is one customer's rating (and optional comment) on a product.
// CustomerName is joined in for display — a reviews list is read far
// more often than written, so denormalizing the join into every read is
// the cheaper trade here, same reasoning products.ListItem's own
// brand/image joins already make.
type Review struct {
	ID           string    `json:"id"`
	CustomerName string    `json:"customer_name"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Summary is the aggregate a product page needs alongside the list —
// kept separate from ListItem so today's shop grid and POS grid stay
// exactly as they are; see this package's own doc comment for why this
// round doesn't also touch products.Repository's list query.
type Summary struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

// ProductReviews is GET /api/v1/products/:slug/reviews' whole response
// body — summary and list together, since a product page always wants
// both at once and this saves it a second round trip.
type ProductReviews struct {
	Summary Summary  `json:"summary"`
	Reviews []Review `json:"reviews"`
}

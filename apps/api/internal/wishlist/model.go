// Package wishlist lets a logged-in customer save products for later —
// Phase 12's first slice (see the root README's roadmap: wishlist,
// reviews, membership, promotions, in that order). Customer-only by
// design, same reasoning as the migration's own comment: there's no
// guest wishlist the way there's a guest cart.
package wishlist

import (
	"time"

	"satelit-parfume-api/internal/products"
)

// Entry pairs a saved product's catalog summary with when it was added.
// Product reuses products.ListItem rather than a parallel type — it's
// already exactly the "light enough for a grid of cards" shape a
// wishlist page needs (name, slug, price_from, primary image, brand),
// so the frontend's existing product-card rendering works here
// unchanged instead of needing a second, near-identical type to parse.
type Entry struct {
	Product products.ListItem `json:"product"`
	AddedAt time.Time         `json:"added_at"`
}

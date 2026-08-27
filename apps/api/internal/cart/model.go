// Package cart implements the shopping cart: guest or customer, one
// branch per cart, and stock *validation* on every add/update. Actual
// stock *reservation* (incrementing reserved_quantity, releasing on
// payment failure/expiry — section 21) is Phase 7/8's job, once there's
// a checkout flow for a reservation to protect. A cart can sit untouched
// for days; locking inventory against it the whole time would be wrong.
package cart

import "time"

type Item struct {
	ID               string    `json:"id"`
	ProductVariantID string    `json:"product_variant_id"`
	ProductName      string    `json:"product_name"`
	ProductSlug      string    `json:"product_slug"`
	VariantName      string    `json:"variant_name"`
	BranchID         string    `json:"branch_id"`
	Quantity         int       `json:"quantity"`
	UnitPrice        int64     `json:"unit_price"` // snapshot from when it was added — section 20
	LineTotal        int64     `json:"line_total"`
	AvailableStock   int       `json:"available_stock"` // live, for "only N left" warnings
	CreatedAt        time.Time `json:"created_at"`
}

type Cart struct {
	ID           string    `json:"id"`
	BranchID     *string   `json:"branch_id,omitempty"`
	Items        []Item    `json:"items"`
	ItemCount    int       `json:"item_count"`
	Subtotal     int64     `json:"subtotal"`
	SessionToken string    `json:"session_token,omitempty"` // only set for a newly created guest cart
	UpdatedAt    time.Time `json:"updated_at"`
}

type AddItemRequest struct {
	ProductVariantID string `json:"product_variant_id" binding:"required,uuid"`
	BranchID         string `json:"branch_id" binding:"required,uuid"`
	Quantity         int    `json:"quantity" binding:"required,min=1"`
}

type UpdateItemRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

// Identity resolves a cart to exactly one of a logged-in customer or a
// guest session, mirroring the carts table's own CHECK constraint.
type Identity struct {
	CustomerID   string
	SessionToken string
}

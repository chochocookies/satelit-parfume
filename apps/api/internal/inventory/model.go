// Package inventory covers branch-scoped stock (branch_inventory) — how
// much of a product variant a specific branch has, and at what price if
// that branch overrides the variant's base price.
//
// This is a direct "set the absolute quantity" API, not an auditable
// movement log — stock_movements with a reason/actor/history trail
// (receive, adjust, transfer, opname) is Phase 11's job (section 32).
// Phase 4's job is making branch-scoped stock exist and be queryable.
package inventory

import "time"

// Item is the staff/admin-facing shape — includes reserved_quantity and
// minimum_stock, which customers never see. See Availability for the
// public-facing shape.
type Item struct {
	ID               string    `json:"id"`
	BranchID         string    `json:"branch_id"`
	ProductVariantID string    `json:"product_variant_id"`
	ProductName      string    `json:"product_name"`
	ProductSlug      string    `json:"product_slug"`
	VariantName      string    `json:"variant_name"`
	StockQuantity    int       `json:"stock_quantity"`
	ReservedQuantity int       `json:"reserved_quantity"`
	AvailableStock   int       `json:"available_stock"`
	MinimumStock     int       `json:"minimum_stock"`
	Price            *int64    `json:"price,omitempty"` // branch override; omitted = uses the variant's base_price
	Status           string    `json:"status"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type SetStockRequest struct {
	StockQuantity int    `json:"stock_quantity" binding:"min=0"`
	MinimumStock  int    `json:"minimum_stock" binding:"min=0"`
	Price         *int64 `json:"price" binding:"omitempty,min=0"`
}

// Availability is the public shape — GET /products/:slug?branch=<slug>
// embeds this. No reserved_quantity, no minimum_stock: a customer needs
// "can I buy this here", not the branch's internal stock-management detail.
type Availability struct {
	BranchSlug     string `json:"branch_slug"`
	BranchName     string `json:"branch_name"`
	AvailableStock int    `json:"available_stock"`
	Price          int64  `json:"price"`
}

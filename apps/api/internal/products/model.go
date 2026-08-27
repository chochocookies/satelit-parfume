// Package products holds the product catalog: products, their variants,
// images, brands, and the CSV import path that gets real verified data
// (seeds/products_verified.csv) into the database. Categories live in
// their own package (internal/categories) since they're shared reference
// data; brands stay here since products are their only consumer so far.
package products

import (
	"time"

	"satelit-parfume-api/internal/inventory"
)

type Brand struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Variant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SKU       string `json:"sku,omitempty"`
	Barcode   string `json:"barcode,omitempty"`
	Size      string `json:"size,omitempty"`
	BasePrice int64  `json:"base_price"`
	Status    string `json:"status"`
}

type Image struct {
	ID        string `json:"id"`
	ImageURL  string `json:"image_url"`
	AltText   string `json:"alt_text,omitempty"`
	SortOrder int    `json:"sort_order"`
	IsPrimary bool   `json:"is_primary"`
}

type CategoryRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ListItem is the shape returned by GET /products — light enough for a
// grid of cards. PriceFrom is the cheapest active variant's price; almost
// every product currently has exactly one variant (see the migration
// comment on product_variants — no fake size lineups), so this is
// usually just that variant's price. BranchStock is only non-nil when the
// request included ?branch=<slug> — section 16's product card shows
// availability, but only once a branch is actually selected.
type ListItem struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Slug         string       `json:"slug"`
	Status       string       `json:"status"`
	IsFeatured   bool         `json:"is_featured"`
	IsBestseller bool         `json:"is_bestseller"`
	Brand        *Brand       `json:"brand,omitempty"`
	Category     *CategoryRef `json:"category,omitempty"`
	PriceFrom    int64        `json:"price_from"`
	PrimaryImage string       `json:"primary_image_url,omitempty"`
	BranchStock  *int         `json:"branch_stock,omitempty"`
}

// Detail is the full payload for GET /products/:slug. Availability is
// only populated when the request includes ?branch=<slug> — see
// Handler.GetBySlug — since without a selected branch there's no
// meaningful single stock number to show (section 9: stock is per branch,
// never global).
type Detail struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	Slug             string                  `json:"slug"`
	SKU              string                  `json:"sku,omitempty"`
	Barcode          string                  `json:"barcode,omitempty"`
	Description      string                  `json:"description,omitempty"`
	ShortDescription string                  `json:"short_description,omitempty"`
	Size             string                  `json:"size,omitempty"`
	Gender           string                  `json:"gender,omitempty"`
	FragranceFamily  string                  `json:"fragrance_family,omitempty"`
	Status           string                  `json:"status"`
	IsFeatured       bool                    `json:"is_featured"`
	IsBestseller     bool                    `json:"is_bestseller"`
	Brand            *Brand                  `json:"brand,omitempty"`
	Category         *CategoryRef            `json:"category,omitempty"`
	Variants         []Variant               `json:"variants"`
	Images           []Image                 `json:"images"`
	Availability     *inventory.Availability `json:"availability,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

// ListFilter captures every query param GET /products accepts. Empty
// string fields mean "no filter" — see repository.go's buildWhere.
// BranchSlug doesn't filter which products are returned — it only
// resolves BranchStock on each item (see ListItem) — because a customer
// browsing without a selected branch should still see the catalog, just
// without a stock number attached to it yet.
type ListFilter struct {
	Search       string
	CategorySlug string
	BrandSlug    string
	Gender       string
	BranchSlug   string
	Sort         string // "newest" | "price_asc" | "price_desc" | "" (name A-Z)
	Page         int
	Limit        int

	// AdminView and Status back Handler.AdminList (Phase 9) only — the
	// public List handler never sets them. AdminView drops buildWhere's
	// "must be active" default so drafts/archived products show up in
	// the admin table too; Status then optionally narrows to one
	// specific status. Because the public path always leaves both at
	// their zero value, buildWhere's public-facing clause is completely
	// unchanged from before these fields existed.
	AdminView bool
	Status    string
}

type ListResult struct {
	Items      []ListItem `json:"items"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	TotalPages int        `json:"total_pages"`
}

// ProductUpsertRequest is shared by AdminCreate and AdminUpdate — every
// field is a full replace on update, the same "no partial PATCH" shape
// branches.UpsertRequest already uses. Category/Brand are names, not
// IDs: they resolve through the exact same ensureBrand/EnsureByName
// helpers the CSV import path uses (see repository.go), so a manually
// created product and an imported one can never resolve "Perfume" to
// two different category rows. Price sets the product's single
// 'Default' variant — see the package doc comment on ListItem for why
// that's almost always the only variant a real product has.
type ProductUpsertRequest struct {
	Name             string `json:"name" binding:"required,min=2,max=200"`
	Category         string `json:"category"`
	Brand            string `json:"brand"`
	SKU              string `json:"sku"`
	Barcode          string `json:"barcode"`
	Description      string `json:"description"`
	ShortDescription string `json:"short_description"`
	Size             string `json:"size"`
	Gender           string `json:"gender"`
	FragranceFamily  string `json:"fragrance_family"`
	Status           string `json:"status" binding:"omitempty,oneof=active draft archived"`
	IsFeatured       bool   `json:"is_featured"`
	IsBestseller     bool   `json:"is_bestseller"`
	Price            int64  `json:"price" binding:"required,min=1"`
	ImageURL         string `json:"image_url" binding:"omitempty,url"`
}

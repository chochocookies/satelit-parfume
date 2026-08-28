// Package stock holds Phase 11's advanced inventory workflows: an
// auditable movement log, branch-to-branch transfers, and stock opname
// (physical count reconciliation) — see this package's own Phase 1
// placeholder doc comment, now filled in.
//
// Deliberately independent of internal/inventory at the Go level in
// both directions: this package writes branch_inventory.stock_quantity
// directly via its own SQL rather than calling back into
// inventory.Repository, the same reasoning internal/dashboard (Phase 9)
// used for querying other packages' tables directly — a transfer or an
// opname completion isn't really "inventory's" operation any more than
// a sale is, and giving each stock-changing feature its own queries
// against a well-known table beats threading repositories through each
// other for one call apiece. inventory.SetStock (Phase 4) is untouched
// and still doesn't log a movement — see the root README's "Deliberately
// not in Phase 11" note on why the two now deliberately overlap instead
// of one replacing the other.
package stock

import (
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrInsufficientStock   = errors.New("insufficient stock for this transfer")
	ErrTransferNotPending  = errors.New("transfer is not pending")
	ErrTransferWrongBranch = errors.New("transfer does not involve this branch")
	ErrOpnameAlreadyOpen   = errors.New("this branch already has an open stock count — complete it before starting another")
	ErrOpnameNotOpen       = errors.New("this stock count is not open")
	ErrOpnameItemNotFound  = errors.New("that item is not part of this stock count")
	ErrNegativeResult      = errors.New("this adjustment would take stock below zero")
)

// Movement is one line in the stock ledger — see migration 000009's
// table comment for why reservation holds/releases never appear here.
type Movement struct {
	ID               string    `json:"id"`
	BranchID         string    `json:"branch_id"`
	ProductVariantID string    `json:"product_variant_id"`
	ProductName      string    `json:"product_name"`
	VariantName      string    `json:"variant_name"`
	QuantityChange   int       `json:"quantity_change"`
	Reason           string    `json:"reason"`
	ReferenceType    string    `json:"reference_type,omitempty"`
	ReferenceID      string    `json:"reference_id,omitempty"`
	Note             string    `json:"note,omitempty"`
	ActorUserID      string    `json:"actor_user_id,omitempty"`
	ActorName        string    `json:"actor_name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type ReceiveRequest struct {
	Quantity int    `json:"quantity" binding:"required,min=1"`
	Note     string `json:"note"`
}

// AdjustRequest's QuantityChange can be negative (writing off damaged or
// lost stock) — unlike ReceiveRequest, which is always additive, this is
// the "correct the number to whatever it should be" path, so `min=1`
// would wrongly rule out the negative case entirely.
type AdjustRequest struct {
	QuantityChange int    `json:"quantity_change" binding:"required"`
	Note           string `json:"note"`
}

// Transfer and TransferItem — see migration 000009's table comment on
// why creating a transfer deducts source stock immediately rather than
// waiting for completion.
type TransferItem struct {
	ProductVariantID string `json:"product_variant_id"`
	ProductName      string `json:"product_name,omitempty"`
	VariantName      string `json:"variant_name,omitempty"`
	Quantity         int    `json:"quantity"`
}

type Transfer struct {
	ID           string         `json:"id"`
	FromBranchID string         `json:"from_branch_id"`
	ToBranchID   string         `json:"to_branch_id"`
	Status       string         `json:"status"`
	RequestedBy  string         `json:"requested_by"`
	CompletedBy  string         `json:"completed_by,omitempty"`
	Notes        string         `json:"notes,omitempty"`
	Items        []TransferItem `json:"items"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateTransferRequest struct {
	ToBranchID string         `json:"to_branch_id" binding:"required,uuid"`
	Items      []TransferItem `json:"items" binding:"required,min=1,dive"`
	Notes      string         `json:"notes"`
}

// Opname and OpnameItem — a physical stock count session. SystemQuantity
// is frozen at the moment the session starts (see migration 000009);
// CountedQuantity stays null until staff actually count that line.
type OpnameItem struct {
	ID               string `json:"id"`
	ProductVariantID string `json:"product_variant_id"`
	ProductName      string `json:"product_name"`
	VariantName      string `json:"variant_name"`
	SystemQuantity   int    `json:"system_quantity"`
	CountedQuantity  *int   `json:"counted_quantity,omitempty"`
}

type Opname struct {
	ID          string       `json:"id"`
	BranchID    string       `json:"branch_id"`
	Status      string       `json:"status"`
	StartedBy   string       `json:"started_by"`
	CompletedBy string       `json:"completed_by,omitempty"`
	Notes       string       `json:"notes,omitempty"`
	Items       []OpnameItem `json:"items"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type CountItemRequest struct {
	CountedQuantity int `json:"counted_quantity" binding:"min=0"`
}

type CompleteOpnameRequest struct {
	Notes string `json:"notes"`
}

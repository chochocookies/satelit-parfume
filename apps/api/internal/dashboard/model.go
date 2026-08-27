// Package dashboard aggregates read-only summary statistics for the
// staff admin dashboard's overview page (Phase 9) — order counts by
// status, revenue, and catalog/branch/staff/low-stock counts. Every
// number here comes from a live query against tables other packages
// already own (orders, products, branches, users, branch_inventory);
// nothing is cached, precomputed, or fabricated. No new tables or
// columns exist for this — every field below reads something the
// schema already tracks (see Repository.Stats for exactly which query
// backs which field).
package dashboard

// Stats is the response shape for GET /api/v1/admin/dashboard/stats.
type Stats struct {
	Revenue       int64          `json:"revenue"`
	TotalOrders   int            `json:"total_orders"`
	OrderCounts   map[string]int `json:"order_counts"`
	TotalProducts int            `json:"total_products"`
	TotalBranches int            `json:"total_branches"`
	TotalStaff    int            `json:"total_staff"`
	LowStockCount int            `json:"low_stock_count"`
}

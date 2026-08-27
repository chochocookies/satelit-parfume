package dashboard

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"satelit-parfume-api/internal/orders"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Stats runs a handful of small aggregate queries against tables owned
// by other packages. This package deliberately doesn't thread five
// repositories (products, orders, branches, users, inventory) through
// itself just to call one counting method on each — "count everything"
// isn't really any single domain's responsibility, so a dashboard
// repository with its own read-only queries against well-known
// table/column names is simpler than the alternative.
func (r *Repository) Stats(ctx context.Context) (*Stats, error) {
	s := &Stats{OrderCounts: map[string]int{}}

	rows, err := r.db.Query(ctx, `SELECT status, COUNT(*), COALESCE(SUM(total), 0) FROM orders GROUP BY status`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var status string
		var count int
		var sum int64
		if err := rows.Scan(&status, &count, &sum); err != nil {
			rows.Close()
			return nil, err
		}
		s.OrderCounts[status] = count
		s.TotalOrders += count
		if isRevenueStatus(status) {
			s.Revenue += sum
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE deleted_at IS NULL`).Scan(&s.TotalProducts); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM branches WHERE deleted_at IS NULL`).Scan(&s.TotalBranches); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`).Scan(&s.TotalStaff); err != nil {
		return nil, err
	}
	// minimum_stock defaults to 0 (see migration 000004_branches), which
	// only means a branch never set a real threshold — counting those as
	// "low stock" would flag nearly every row and tell staff nothing
	// useful, so this only counts rows where a branch has actually set a
	// minimum above zero and current stock has fallen to or below it.
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM branch_inventory WHERE minimum_stock > 0 AND available_stock <= minimum_stock`,
	).Scan(&s.LowStockCount); err != nil {
		return nil, err
	}

	return s, nil
}

// isRevenueStatus reports whether an order in this status represents
// money that was collected and stayed collected — PAID onward, except
// REFUNDED. PENDING_PAYMENT/CANCELLED/EXPIRED never collected anything;
// REFUNDED collected it and then gave it back.
func isRevenueStatus(status string) bool {
	switch status {
	case orders.StatusPaid, orders.StatusProcessing, orders.StatusPacked,
		orders.StatusReadyForPickup, orders.StatusShipped, orders.StatusDelivered, orders.StatusCompleted:
		return true
	default:
		return false
	}
}

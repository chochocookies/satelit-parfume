package orders

const (
	StatusPendingPayment = "PENDING_PAYMENT"
	StatusPaid           = "PAID"
	StatusProcessing     = "PROCESSING"
	StatusPacked         = "PACKED"
	StatusReadyForPickup = "READY_FOR_PICKUP"
	StatusShipped        = "SHIPPED"
	StatusDelivered      = "DELIVERED"
	StatusCompleted      = "COMPLETED"
	StatusCancelled      = "CANCELLED"
	StatusRefunded       = "REFUNDED"
	StatusExpired        = "EXPIRED"
)

// validTransitions is section 24's status list turned into the graph
// section 24 also requires ("status transitions must be validated").
// PACKED branches on order type (pickup -> READY_FOR_PICKUP, delivery ->
// SHIPPED) — validateOrderTypeForStatus in service.go checks that half,
// since a plain from/to map can't express "only for this order's type."
var validTransitions = map[string][]string{
	StatusPendingPayment: {StatusPaid, StatusCancelled, StatusExpired},
	StatusPaid:           {StatusProcessing, StatusRefunded},
	StatusProcessing:     {StatusPacked, StatusRefunded},
	StatusPacked:         {StatusReadyForPickup, StatusShipped, StatusRefunded},
	StatusReadyForPickup: {StatusCompleted, StatusRefunded},
	StatusShipped:        {StatusDelivered, StatusRefunded},
	StatusDelivered:      {StatusCompleted, StatusRefunded},
	StatusCompleted:      {},
	StatusCancelled:      {},
	StatusRefunded:       {},
	StatusExpired:        {},
}

// CanTransition reports whether from -> to is allowed. An unrecognized
// `from` (which shouldn't happen — it only ever comes from a row already
// in the database) is never valid, rather than panicking on a missing
// map key.
func CanTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// isValidStatus checks a status string is one of the known constants —
// used to reject typos/garbage in UpdateStatusRequest before it ever
// reaches CanTransition, so an unrecognized status fails with a clear
// "not a real status" rather than a confusing "no transition exists."
func isValidStatus(s string) bool {
	_, ok := validTransitions[s]
	return ok
}

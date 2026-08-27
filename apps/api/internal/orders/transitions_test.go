package orders

import "testing"

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{StatusPendingPayment, StatusPaid, true},
		{StatusPendingPayment, StatusCancelled, true},
		{StatusPendingPayment, StatusExpired, true},
		{StatusPendingPayment, StatusShipped, false}, // can't skip straight to shipped
		{StatusPaid, StatusProcessing, true},
		{StatusPaid, StatusPendingPayment, false}, // no going backward
		{StatusPacked, StatusReadyForPickup, true},
		{StatusPacked, StatusShipped, true}, // graph itself allows both; order-type match is a separate check
		{StatusCompleted, StatusProcessing, false},
		{StatusCompleted, StatusCompleted, false}, // terminal states have no outgoing edges, not even to themselves
		{StatusCancelled, StatusPaid, false},
		{"NOT_A_REAL_STATUS", StatusPaid, false},
		{StatusPendingPayment, "NOT_A_REAL_STATUS", false},
	}

	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestIsValidStatus(t *testing.T) {
	for _, s := range []string{
		StatusPendingPayment, StatusPaid, StatusProcessing, StatusPacked,
		StatusReadyForPickup, StatusShipped, StatusDelivered, StatusCompleted,
		StatusCancelled, StatusRefunded, StatusExpired,
	} {
		if !isValidStatus(s) {
			t.Errorf("isValidStatus(%q) = false, want true — every declared status constant should be recognized", s)
		}
	}

	for _, s := range []string{"", "paid", "PENDING", "SHIPPED "} {
		if isValidStatus(s) {
			t.Errorf("isValidStatus(%q) = true, want false", s)
		}
	}
}

func TestValidateOrderTypeForStatus(t *testing.T) {
	cases := []struct {
		orderType, status string
		wantErr           bool
	}{
		{"pickup", StatusReadyForPickup, false},
		{"delivery", StatusReadyForPickup, true},
		{"delivery", StatusShipped, false},
		{"pickup", StatusShipped, true},
		{"delivery", StatusDelivered, false},
		{"pickup", StatusDelivered, true},
		// Statuses that apply regardless of order type shouldn't error either way.
		{"pickup", StatusPaid, false},
		{"delivery", StatusPaid, false},
		{"pickup", StatusCompleted, false},
		{"delivery", StatusCompleted, false},
	}

	for _, c := range cases {
		err := validateOrderTypeForStatus(c.orderType, c.status)
		if (err != nil) != c.wantErr {
			t.Errorf("validateOrderTypeForStatus(%q, %q) error = %v, wantErr %v", c.orderType, c.status, err, c.wantErr)
		}
	}
}

// Package payments implements the payment abstraction section 26 asks
// for: business logic (this package's own Service, and orders' checkout)
// talks to the Provider interface below, never to a specific gateway's
// request/response shapes directly. Adding a second provider means
// writing a new adapter package (see duitku/ for the pattern), not
// touching this package or internal/orders.
//
// Cash payments never reach this package at all — a cashier confirming
// cash at the counter goes straight through orders' own
// PENDING_PAYMENT -> PAID transition (Phase 7, section 28's POS flow).
// This package exists specifically for the *online* methods: QRIS,
// virtual accounts, e-wallets, cards.
package payments

import (
	"context"
	"errors"
	"net/http"
)

var ErrNotImplemented = errors.New("not implemented by this provider")

type CreatePaymentRequest struct {
	OrderID       string
	OrderNumber   string // human-friendly, becomes the provider's merchant order id
	Amount        int64  // whole Rupiah
	PaymentMethod string // provider-specific code, e.g. Duitku's "SP" for QRIS
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	ProductDetail string
	CallbackURL   string
	ReturnURL     string
	ExpiryMinutes int
}

type CreatePaymentResult struct {
	Reference   string // the provider's transaction reference
	PaymentURL  string // where to redirect the customer, if applicable
	QRString    string // raw QRIS payload, if the method is a QRIS variant
	VANumber    string // virtual account number, if the method is a VA
	RawResponse string // the provider's raw response body, kept for the audit log — never parsed by callers
}

type VerifyPaymentResult struct {
	Reference string
	Amount    int64
	Paid      bool
}

type WebhookResult struct {
	OrderNumber string // matches CreatePaymentRequest.OrderNumber — how the webhook is tied back to an order
	Reference   string
	Amount      int64
	Paid        bool
}

// Provider is the interface section 26 names directly: CreatePayment,
// VerifyPayment, HandleWebhook, RefundPayment.
type Provider interface {
	// Name identifies the provider for storage (payments.provider column)
	// and logging — never used for branching logic outside this package.
	Name() string

	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResult, error)

	// VerifyPayment asks the provider directly for an order's current
	// status, keyed by *our* order number (not the provider's own
	// reference) — Duitku's actual status-check API takes the merchant
	// order id directly, and a caller checking "has this order been
	// paid" already has that value without an extra lookup. A fallback
	// for when a webhook never arrives (section 27: never mark an order
	// paid just because the customer returned to a success page; this is
	// the honest way to check instead of trusting that redirect).
	VerifyPayment(ctx context.Context, orderNumber string) (*VerifyPaymentResult, error)

	// HandleWebhook verifies the signature and parses the callback body.
	// A non-nil error here (bad signature, malformed body) means the
	// webhook is rejected before Service ever looks at its contents —
	// see service.go's HandleWebhook.
	HandleWebhook(ctx context.Context, rawBody []byte, headers http.Header) (*WebhookResult, error)

	// RefundPayment is part of the interface because section 26 declares
	// it as part of the abstraction, even though no provider implements
	// it yet. Duitku's adapter returns ErrNotImplemented — see
	// duitku/adapter.go's doc comment for why, rather than this being a
	// silent no-op.
	RefundPayment(ctx context.Context, reference string, amount int64) error
}

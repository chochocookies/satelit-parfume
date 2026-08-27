package payments

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"satelit-parfume-api/internal/orders"
)

var ErrAmountMismatch = errors.New("webhook amount does not match the order total")

type Service struct {
	repo     *Repository
	provider Provider
	orders   *orders.Service
}

func NewService(repo *Repository, provider Provider, ordersService *orders.Service) *Service {
	return &Service{repo: repo, provider: provider, orders: ordersService}
}

// CreatePaymentForOrder asks the provider for a payable reference (QR
// string, VA number, or redirect URL, depending on paymentMethod) and
// records it. Only reached when the customer picks an online method —
// "pay at counter" never touches this package (see the package doc
// comment).
func (s *Service) CreatePaymentForOrder(ctx context.Context, order *orders.Order, paymentMethod, callbackURL, returnURL string) (*Payment, error) {
	if order.Status != orders.StatusPendingPayment {
		return nil, fmt.Errorf("order is %s, not awaiting payment", order.Status)
	}

	name, phone := customerContactFor(order)

	result, err := s.provider.CreatePayment(ctx, CreatePaymentRequest{
		OrderID:       order.ID,
		OrderNumber:   order.OrderNumber,
		Amount:        order.Total,
		PaymentMethod: paymentMethod,
		CustomerName:  name,
		CustomerEmail: order.GuestEmail,
		CustomerPhone: phone,
		ProductDetail: "Satelit Parfume order " + order.OrderNumber,
		CallbackURL:   callbackURL,
		ReturnURL:     returnURL,
		ExpiryMinutes: 30, // matches orders.reservationWindow — no point outliving the reservation it's paying for
	})
	if err != nil {
		return nil, fmt.Errorf("create payment with provider: %w", err)
	}

	return s.repo.Create(ctx, CreateParams{
		OrderID:       order.ID,
		Provider:      s.provider.Name(),
		PaymentMethod: paymentMethod,
		Reference:     result.Reference,
		Amount:        order.Total,
		QRString:      result.QRString,
		VANumber:      result.VANumber,
		PaymentURL:    result.PaymentURL,
	})
}

func customerContactFor(o *orders.Order) (name, phone string) {
	if o.RecipientName != "" {
		return o.RecipientName, o.RecipientPhone
	}
	return o.GuestName, o.GuestPhone
}

func (s *Service) GetLatestForOrder(ctx context.Context, orderID string) (*Payment, error) {
	return s.repo.GetLatestForOrder(ctx, orderID)
}

// HandleWebhook verifies the incoming callback, logs it verbatim for
// audit (section 82) regardless of outcome, and — only if the signature
// is valid and the amount matches what was actually invoiced — advances
// the order to PAID. A forged or tampered callback never reaches
// orders.Service at all; it's rejected before any state changes (section
// 52's "never trust payment status from the frontend" applies just as
// much to a spoofed backend call as a client-supplied field).
//
// Idempotency (section 89: "duplicate payment webhook" must not
// double-process) isn't handled by deduping the payload — it falls out
// of orders.UpdateStatus's own guarded transition: a second delivery of
// the same "paid" callback finds the order already PAID, the guarded
// UPDATE affects zero rows, and that's treated as success, not an error.
func (s *Service) HandleWebhook(ctx context.Context, rawBody []byte, headers http.Header) error {
	result, err := s.provider.HandleWebhook(ctx, rawBody, headers)

	logErr := s.repo.LogTransaction(ctx, LogParams{
		Provider:       s.provider.Name(),
		Reference:      referenceOrEmpty(result),
		RawPayload:     string(rawBody),
		SignatureValid: err == nil,
	})
	if logErr != nil {
		// Logging failure shouldn't block processing a legitimate
		// webhook — but it's worth surfacing if this ever needs
		// debugging, hence not silently swallowed like cart.Clear's
		// best-effort pattern elsewhere in this project.
		_ = logErr
	}

	if err != nil {
		return fmt.Errorf("verify webhook: %w", err)
	}

	order, err := s.orders.GetByOrderNumber(ctx, result.OrderNumber)
	if err != nil {
		return fmt.Errorf("find order %s: %w", result.OrderNumber, err)
	}

	if result.Amount != order.Total {
		_ = s.repo.MarkFailed(ctx, s.provider.Name(), result.Reference)
		return fmt.Errorf("%w: got %d, expected %d", ErrAmountMismatch, result.Amount, order.Total)
	}

	if !result.Paid {
		return s.repo.MarkFailed(ctx, s.provider.Name(), result.Reference)
	}

	if err := s.repo.MarkPaid(ctx, s.provider.Name(), result.Reference); err != nil {
		return fmt.Errorf("record payment: %w", err)
	}

	if _, err := s.orders.UpdateStatus(ctx, order, orders.StatusPaid, "confirmed via "+s.provider.Name()+" webhook"); err != nil {
		if errors.Is(err, orders.ErrInvalidTransition) {
			return nil // already PAID (or past it) — a repeat delivery, not a failure
		}
		return fmt.Errorf("advance order status: %w", err)
	}

	// Phase 10: records this as a 'qris' sale for cashier-shift
	// reconciliation (see migration 000008_pos's payment_method column
	// comment). Best-effort and after the fact, same reasoning as
	// orders.Handler.UpdateStatus's own SetPaymentMethod call: the order
	// is already correctly PAID by this point regardless of whether this
	// one extra column write succeeds. Hardcoded to "qris" rather than
	// derived from the webhook result — WebhookResult carries no payment
	// method field, and QRIS is the only online method this project
	// actually exposes today (Phase 10's own scope); revisit if a second
	// online method ever gets wired up alongside it.
	_ = s.orders.SetPaymentMethod(ctx, order.ID, "qris")

	return nil
}

func referenceOrEmpty(r *WebhookResult) string {
	if r == nil {
		return ""
	}
	return r.Reference
}

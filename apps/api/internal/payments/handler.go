package payments

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/internal/orders"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service     *Service
	ordersSvc   *orders.Service
	callbackURL string
	returnURL   string
}

func NewHandler(service *Service, ordersSvc *orders.Service, callbackURL, returnURL string) *Handler {
	return &Handler{service: service, ordersSvc: ordersSvc, callbackURL: callbackURL, returnURL: returnURL}
}

// Pay backs POST /api/v1/orders/:id/pay — same ownership check as
// orders.CancelMine (this order must belong to the calling customer),
// duplicated rather than shared across packages for the same reason
// orders/cart's identity() helpers are each their own few lines.
func (h *Handler) Pay(c *gin.Context) {
	subjectType, _ := c.Get(auth.ContextSubjectType)
	if subjectType != "customer" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to pay for an order")
		return
	}
	subjectID, _ := c.Get(auth.ContextSubjectID)
	customerID, _ := subjectID.(string)

	var req PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	order, err := h.ordersSvc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, orders.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "order not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load order")
		return
	}
	if order.CustomerID != customerID {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "this order does not belong to you")
		return
	}

	payment, err := h.service.CreatePaymentForOrder(c.Request.Context(), order, req.PaymentMethod, h.callbackURL, h.returnURL)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR", err.Error())
		return
	}
	response.OK(c, http.StatusCreated, "payment created", payment)
}

// GetForOrder backs GET /api/v1/orders/:id/payment — for the frontend to
// poll while a QR code is on screen. Same ownership check as Pay.
func (h *Handler) GetForOrder(c *gin.Context) {
	subjectType, _ := c.Get(auth.ContextSubjectType)
	if subjectType != "customer" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to view this")
		return
	}
	subjectID, _ := c.Get(auth.ContextSubjectID)
	customerID, _ := subjectID.(string)

	order, err := h.ordersSvc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "order not found")
		return
	}
	if order.CustomerID != customerID {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "this order does not belong to you")
		return
	}

	payment, err := h.service.GetLatestForOrder(c.Request.Context(), order.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "no payment has been started for this order yet")
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load payment")
		return
	}
	response.OK(c, http.StatusOK, "payment", payment)
}

// AdminPay backs POST /api/v1/admin/branches/:id/orders/:orderId/pay
// (Phase 10) — a cashier creating a QRIS-at-counter payment for a POS
// sale. There's no customer-ownership check here the way Pay has one:
// a POS sale has no customer at all (see
// orders.CheckoutRequest.CashierShiftID's doc comment) — RequireAuth +
// RequireBranchAccess in main.go is what actually authorizes this
// instead, the same as every other branch-scoped staff action.
func (h *Handler) AdminPay(c *gin.Context) {
	var req PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	order, err := h.ordersSvc.GetByID(c.Request.Context(), c.Param("orderId"))
	if err != nil {
		if errors.Is(err, orders.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "order not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load order")
		return
	}
	if order.BranchID != c.Param("id") {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "order not found at this branch")
		return
	}

	payment, err := h.service.CreatePaymentForOrder(c.Request.Context(), order, req.PaymentMethod, h.callbackURL, h.returnURL)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR", err.Error())
		return
	}
	response.OK(c, http.StatusCreated, "payment created", payment)
}

// Webhook backs POST /api/v1/payments/webhook/duitku — deliberately
// public (no auth middleware): a payment gateway calling back has no way
// to present a Bearer token or session, and section 26 already requires
// this be trustworthy by *signature* verification (done inside
// Service.HandleWebhook), not by network-level auth. Always returns 200
// once the body is read and handed off, matching how these gateways
// generally treat "not 200" as "retry me" — a webhook that's invalid or
// unprocessable shouldn't trigger endless retries of something that will
// never succeed.
func (h *Handler) Webhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_BODY", "could not read request body")
		return
	}

	if err := h.service.HandleWebhook(c.Request.Context(), body, c.Request.Header); err != nil {
		// Logged server-side inside HandleWebhook already (the audit
		// log write happens before this error is even returned) — still
		// 200 here, per the comment above, rather than inviting retries.
		response.OK(c, http.StatusOK, "received", gin.H{"processed": false, "reason": err.Error()})
		return
	}

	response.OK(c, http.StatusOK, "received", gin.H{"processed": true})
}

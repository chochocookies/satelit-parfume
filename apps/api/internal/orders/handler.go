package orders

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/internal/cart"
	"satelit-parfume-api/internal/inventory"
	"satelit-parfume-api/internal/shifts"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// identity mirrors cart's own — duplicated rather than imported/exported,
// since it's a few lines reading gin.Context, not shared logic worth a
// cross-package dependency for. Reads both fields whenever both are
// present (see cart.identity's own doc comment on why): this is the path
// that actually matters for it — Checkout is where a merged-away guest
// cart would otherwise turn into a hard ErrEmptyCart failure the instant
// a customer logs in with items already sitting in it.
func identity(c *gin.Context) cart.Identity {
	id := cart.Identity{SessionToken: c.GetHeader("X-Cart-Token")}
	if subjectType, ok := c.Get(auth.ContextSubjectType); ok && subjectType == "customer" {
		subjectID, _ := c.Get(auth.ContextSubjectID)
		id.CustomerID = subjectID.(string)
	}
	return id
}

func (h *Handler) Checkout(c *gin.Context) {
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	order, err := h.service.Checkout(c.Request.Context(), identity(c), req)
	if err != nil {
		respondCheckoutError(c, err)
		return
	}
	response.OK(c, http.StatusCreated, "order created", order)
}

// POSCheckout backs POST /admin/branches/:id/pos/checkout (Phase 10) — a
// cashier ringing up a walk-in sale. Reuses Checkout entirely: the POS
// frontend manages its own cart via the same X-Cart-Token mechanism a
// guest customer's cart already uses (see identity() above), scoped to
// this branch. This handler's only real job is finding the caller's own
// open shift server-side and stamping its id onto the checkout request —
// never trusting a shift id the client might supply (see
// CheckoutRequest.CashierShiftID's doc comment on why that field has no
// json tag at all).
func (h *Handler) POSCheckout(c *gin.Context) {
	subjectID, _ := c.Get(auth.ContextSubjectID)
	userID, _ := subjectID.(string)

	shift, err := h.service.GetOpenShiftForUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, shifts.ErrNotFound) {
			response.Error(c, http.StatusConflict, "NO_OPEN_SHIFT", "open a shift before ringing up a sale")
			return
		}
		response.InternalError(c, err, "could not load your shift")
		return
	}
	if shift.BranchID != c.Param("id") {
		response.Error(c, http.StatusConflict, "WRONG_BRANCH", "your open shift is at a different branch")
		return
	}

	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	// A POS sale is always in-person — see migration 000008_pos's
	// comment on why there's deliberately no separate order_type for it.
	// Forced here regardless of what the request body said, the same
	// "never trust the client for this" reasoning as CashierShiftID.
	req.OrderType = "pickup"
	req.CashierShiftID = shift.ID

	order, err := h.service.Checkout(c.Request.Context(), identity(c), req)
	if err != nil {
		respondCheckoutError(c, err)
		return
	}
	response.OK(c, http.StatusCreated, "sale created", order)
}

func respondCheckoutError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmptyCart):
		response.Error(c, http.StatusBadRequest, "EMPTY_CART", err.Error())
	case errors.Is(err, ErrNoBranchSelected):
		response.Error(c, http.StatusBadRequest, "NO_BRANCH", err.Error())
	case errors.Is(err, ErrGuestInfoRequired):
		response.Error(c, http.StatusBadRequest, "GUEST_INFO_REQUIRED", err.Error())
	case errors.Is(err, ErrAddressRequired):
		response.Error(c, http.StatusBadRequest, "ADDRESS_REQUIRED", err.Error())
	case errors.Is(err, inventory.ErrInsufficientStock):
		response.Error(c, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error())
	default:
		response.InternalError(c, err, "checkout failed")
	}
}

// requireCustomer reads the authenticated customer's id, or writes a 401
// and returns ok=false if the caller isn't one — every customer-facing
// order route (list/get/cancel) needs this, guests use Lookup instead.
func requireCustomer(c *gin.Context) (customerID string, ok bool) {
	subjectType, _ := c.Get(auth.ContextSubjectType)
	if subjectType != "customer" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to view your orders")
		return "", false
	}
	subjectID, _ := c.Get(auth.ContextSubjectID)
	return subjectID.(string), true
}

func (h *Handler) ListMine(c *gin.Context) {
	customerID, ok := requireCustomer(c)
	if !ok {
		return
	}

	list, err := h.service.ListForCustomer(c.Request.Context(), customerID)
	if err != nil {
		response.InternalError(c, err, "could not load orders")
		return
	}
	response.OK(c, http.StatusOK, "orders", list)
}

func (h *Handler) GetMine(c *gin.Context) {
	customerID, ok := requireCustomer(c)
	if !ok {
		return
	}

	order, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondOrderLookupError(c, err)
		return
	}
	if order.CustomerID != customerID {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "this order does not belong to you")
		return
	}
	response.OK(c, http.StatusOK, "order", order)
}

func (h *Handler) CancelMine(c *gin.Context) {
	customerID, ok := requireCustomer(c)
	if !ok {
		return
	}

	order, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondOrderLookupError(c, err)
		return
	}
	if order.CustomerID != customerID {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "this order does not belong to you")
		return
	}

	cancelled, err := h.service.Cancel(c.Request.Context(), order.ID)
	if err != nil {
		respondTransitionError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "order cancelled", cancelled)
}

// Lookup backs guest order tracking (no account needed): knowing both the
// order number and the phone number on it is treated as sufficient proof
// it's yours — the same bar a lot of real delivery trackers use.
func (h *Handler) Lookup(c *gin.Context) {
	var req LookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	order, err := h.service.LookupGuestOrder(c.Request.Context(), req.OrderNumber, req.Phone)
	if err != nil {
		respondOrderLookupError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "order", order)
}

func respondOrderLookupError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "order not found")
		return
	}
	response.InternalError(c, err, "could not load order")
}

// ── Staff-facing, branch-scoped (mounted under branches.RequireBranchAccess
// in main.go — the same pattern inventory's admin routes use) ──

func (h *Handler) ListForBranch(c *gin.Context) {
	list, err := h.service.ListForBranch(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err, "could not load orders")
		return
	}
	response.OK(c, http.StatusOK, "branch orders", list)
}

func (h *Handler) GetForBranch(c *gin.Context) {
	order, err := h.service.GetByID(c.Request.Context(), c.Param("orderId"))
	if err != nil {
		respondOrderLookupError(c, err)
		return
	}
	if order.BranchID != c.Param("id") {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "order not found at this branch")
		return
	}
	response.OK(c, http.StatusOK, "order", order)
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	order, err := h.service.GetByID(c.Request.Context(), c.Param("orderId"))
	if err != nil {
		respondOrderLookupError(c, err)
		return
	}
	if order.BranchID != c.Param("id") {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "order not found at this branch")
		return
	}

	updated, err := h.service.UpdateStatus(c.Request.Context(), order, req.Status, req.Note)
	if err != nil {
		respondTransitionError(c, err)
		return
	}
	if req.PaymentMethod != "" {
		// Best-effort and after the fact, deliberately — the status
		// change itself (which is what actually deducts stock) already
		// succeeded and committed by this point. A failure recording
		// which method was used is a reporting nuisance, never a reason
		// to undo an already-committed payment confirmation.
		_ = h.service.SetPaymentMethod(c.Request.Context(), updated.ID, req.PaymentMethod)
		updated.PaymentMethod = req.PaymentMethod
	}
	response.OK(c, http.StatusOK, "status updated", updated)
}

func respondTransitionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidStatus):
		response.Error(c, http.StatusBadRequest, "INVALID_STATUS", err.Error())
	case errors.Is(err, ErrInvalidTransition):
		response.Error(c, http.StatusConflict, "INVALID_TRANSITION", err.Error())
	case errors.Is(err, ErrOrderTypeMismatch):
		response.Error(c, http.StatusConflict, "ORDER_TYPE_MISMATCH", err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusConflict, "ALREADY_CHANGED", "order status changed since it was last read — reload and try again")
	default:
		response.InternalError(c, err, "status update failed")
	}
}

// ── Admin, cross-branch (SUPER_ADMIN/ADMIN only — mounted directly
// under adminGroup in main.go, not through branches.RequireBranchAccess,
// since these deliberately aren't scoped to one branch) ──

// AdminList backs GET /api/v1/admin/orders — the dashboard's orders
// table. Optional ?status=, ?branch_id=, ?search= narrow it; ?page=/
// ?limit= page it. Branch names aren't joined in here (see
// AdminListFilter's comment) — the frontend already has the branch list
// from GET /branches and maps branch_id to a name client-side.
func (h *Handler) AdminList(c *gin.Context) {
	filter := AdminListFilter{
		Status:   c.Query("status"),
		BranchID: c.Query("branch_id"),
		Search:   c.Query("search"),
		Page:     queryInt(c, "page", 1),
		Limit:    queryInt(c, "limit", 20),
	}

	result, err := h.service.ListAdmin(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err, "could not load orders")
		return
	}
	response.OK(c, http.StatusOK, "orders", result)
}

// AdminGet backs GET /api/v1/admin/orders/:id — like GetForBranch, but
// without the branch-ownership check, since an admin can look at any
// order on the platform. Routes through the same Service.GetByID as
// every other single-order read, so the lazy-expiry check still applies.
func (h *Handler) AdminGet(c *gin.Context) {
	order, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondOrderLookupError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "order", order)
}

// AdminUpdateStatus backs PUT /api/v1/admin/orders/:id/status — like
// UpdateStatus, but without the branch-ownership check. Same validated
// transition graph either way (Service.UpdateStatus doesn't know or
// care which route called it).
func (h *Handler) AdminUpdateStatus(c *gin.Context) {
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	order, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondOrderLookupError(c, err)
		return
	}

	updated, err := h.service.UpdateStatus(c.Request.Context(), order, req.Status, req.Note)
	if err != nil {
		respondTransitionError(c, err)
		return
	}
	if req.PaymentMethod != "" {
		_ = h.service.SetPaymentMethod(c.Request.Context(), updated.ID, req.PaymentMethod)
		updated.PaymentMethod = req.PaymentMethod
	}
	response.OK(c, http.StatusOK, "status updated", updated)
}

func queryInt(c *gin.Context, key string, fallback int) int {
	v := c.Query(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// SweepExpired backs an operator/cron-triggered maintenance endpoint —
// see the root README for how to schedule it. Safe to call as often as
// you like: an order with nothing to expire just costs a cheap SELECT.
func (h *Handler) SweepExpired(c *gin.Context) {
	count, err := h.service.SweepExpired(c.Request.Context(), 100)
	if err != nil {
		response.InternalError(c, err, "sweep failed")
		return
	}
	response.OK(c, http.StatusOK, "sweep complete", gin.H{"expired": count})
}

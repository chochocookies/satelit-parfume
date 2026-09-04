package cart

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/internal/inventory"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// identity reads whichever side of the carts table's "customer OR
// session" pair applies: a customer subject from auth.OptionalAuth's
// context if the caller sent a valid Bearer token, otherwise the
// X-Cart-Token header a guest's browser is expected to have saved from an
// earlier response's session_token.
//
// Both fields are read whenever both are present — not one or the
// other — so a customer who added items before logging in (their cart
// tracked under the guest X-Cart-Token the whole time) doesn't have
// that cart go missing the moment they authenticate: resolve() below
// merges it into their own cart precisely because it still has both
// pieces of identity to work with here. See the same duplicated
// function in internal/orders/handler.go, which mattered most in
// practice — Checkout is where this cart is actually needed for real.
func identity(c *gin.Context) Identity {
	id := Identity{SessionToken: c.GetHeader("X-Cart-Token")}
	if subjectType, ok := c.Get(auth.ContextSubjectType); ok && subjectType == "customer" {
		subjectID, _ := c.Get(auth.ContextSubjectID)
		id.CustomerID = subjectID.(string)
	}
	return id
}

func (h *Handler) Get(c *gin.Context) {
	cartResult, err := h.service.Get(c.Request.Context(), identity(c))
	if err != nil {
		response.InternalError(c, err, "could not load cart")
		return
	}
	response.OK(c, http.StatusOK, "cart", cartResult)
}

func (h *Handler) AddItem(c *gin.Context) {
	var req AddItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	cartResult, err := h.service.AddItem(c.Request.Context(), identity(c), req)
	if err != nil {
		respondCartError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "item added", cartResult)
}

func (h *Handler) UpdateItem(c *gin.Context) {
	var req UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	cartResult, err := h.service.UpdateItemQuantity(c.Request.Context(), identity(c), c.Param("itemId"), req.Quantity)
	if err != nil {
		respondCartError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "item updated", cartResult)
}

func (h *Handler) RemoveItem(c *gin.Context) {
	cartResult, err := h.service.RemoveItem(c.Request.Context(), identity(c), c.Param("itemId"))
	if err != nil {
		respondCartError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "item removed", cartResult)
}

func (h *Handler) Clear(c *gin.Context) {
	if err := h.service.Clear(c.Request.Context(), identity(c)); err != nil {
		response.InternalError(c, err, "could not clear cart")
		return
	}
	response.OK(c, http.StatusOK, "cart cleared", nil)
}

func respondCartError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrDifferentBranch):
		response.Error(c, http.StatusConflict, "DIFFERENT_BRANCH", err.Error())
	case errors.Is(err, ErrInsufficientStock):
		response.Error(c, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error())
	case errors.Is(err, ErrItemNotFound):
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "cart item not found")
	case errors.Is(err, inventory.ErrNotFound):
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "product variant not found or inactive")
	default:
		response.InternalError(c, err, "something went wrong")
	}
}

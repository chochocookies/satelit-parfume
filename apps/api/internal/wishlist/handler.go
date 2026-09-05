package wishlist

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/internal/products"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// customerID mirrors the same subject-type check payments.Handler.Pay
// uses (duplicated for the same reason orders/cart's own identity()
// helpers each are their own few lines rather than a shared import) —
// RequireAuth alone accepts a staff token just as readily as a
// customer's (see main.go's own comment on it), and a wishlist is
// customer-only the same way "my orders" is: staff don't have one to
// view.
func customerID(c *gin.Context) (string, bool) {
	subjectType, _ := c.Get(auth.ContextSubjectType)
	if subjectType != "customer" {
		return "", false
	}
	subjectID, _ := c.Get(auth.ContextSubjectID)
	id, _ := subjectID.(string)
	return id, id != ""
}

// List backs GET /api/v1/wishlist.
func (h *Handler) List(c *gin.Context) {
	id, ok := customerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to view your wishlist")
		return
	}
	entries, err := h.service.List(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, err, "could not load wishlist")
		return
	}
	response.OK(c, http.StatusOK, "wishlist", entries)
}

// Add backs POST /api/v1/wishlist/:productId.
func (h *Handler) Add(c *gin.Context) {
	id, ok := customerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to save items to your wishlist")
		return
	}
	if err := h.service.Add(c.Request.Context(), id, c.Param("productId")); err != nil {
		if errors.Is(err, products.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "product not found")
			return
		}
		response.InternalError(c, err, "could not save to wishlist")
		return
	}
	response.OK(c, http.StatusCreated, "saved to wishlist", nil)
}

// Remove backs DELETE /api/v1/wishlist/:productId.
func (h *Handler) Remove(c *gin.Context) {
	id, ok := customerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to manage your wishlist")
		return
	}
	if err := h.service.Remove(c.Request.Context(), id, c.Param("productId")); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "product is not in your wishlist")
			return
		}
		response.InternalError(c, err, "could not remove from wishlist")
		return
	}
	response.OK(c, http.StatusOK, "removed from wishlist", nil)
}

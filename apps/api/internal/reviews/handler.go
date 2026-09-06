package reviews

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/internal/products"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service  *Service
	products *products.Repository
}

func NewHandler(service *Service, productsRepo *products.Repository) *Handler {
	return &Handler{service: service, products: productsRepo}
}

// customerID mirrors wishlist.Handler's own copy of this exact check —
// see that package's doc comment on why it's duplicated rather than
// shared.
func customerID(c *gin.Context) (string, bool) {
	subjectType, _ := c.Get(auth.ContextSubjectType)
	if subjectType != "customer" {
		return "", false
	}
	subjectID, _ := c.Get(auth.ContextSubjectID)
	id, _ := subjectID.(string)
	return id, id != ""
}

type submitRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

// GetForProduct backs GET /api/v1/products/:slug/reviews — public, no
// auth, same as viewing the product page itself. Takes a slug (not a
// product id) to match how the product detail route it sits next to
// already works; Submit/Remove below take a product id instead, since
// their only real caller already has one loaded from that same page.
func (h *Handler) GetForProduct(c *gin.Context) {
	ctx := c.Request.Context()
	product, err := h.products.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		if errors.Is(err, products.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "product not found")
			return
		}
		response.InternalError(c, err, "could not load product")
		return
	}

	summary, err := h.service.SummaryForProduct(ctx, product.ID)
	if err != nil {
		response.InternalError(c, err, "could not load review summary")
		return
	}
	list, err := h.service.ListForProduct(ctx, product.ID)
	if err != nil {
		response.InternalError(c, err, "could not load reviews")
		return
	}

	response.OK(c, http.StatusOK, "reviews", ProductReviews{Summary: summary, Reviews: list})
}

// Submit backs POST /api/v1/products/:slug/reviews.
func (h *Handler) Submit(c *gin.Context) {
	id, ok := customerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to leave a review")
		return
	}

	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "rating must be between 1 and 5")
		return
	}

	ctx := c.Request.Context()
	product, err := h.products.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		if errors.Is(err, products.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "product not found")
			return
		}
		response.InternalError(c, err, "could not load product")
		return
	}

	if err := h.service.Submit(ctx, id, product.ID, req.Rating, req.Comment); err != nil {
		switch {
		case errors.Is(err, ErrNotEligible):
			response.Error(c, http.StatusForbidden, "NOT_ELIGIBLE", err.Error())
		case errors.Is(err, ErrInvalidRating):
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		default:
			response.InternalError(c, err, "could not save review")
		}
		return
	}
	response.OK(c, http.StatusOK, "review saved", nil)
}

// Remove backs DELETE /api/v1/products/:slug/reviews.
func (h *Handler) Remove(c *gin.Context) {
	id, ok := customerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to manage your review")
		return
	}

	ctx := c.Request.Context()
	product, err := h.products.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		if errors.Is(err, products.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "product not found")
			return
		}
		response.InternalError(c, err, "could not load product")
		return
	}

	if err := h.service.Delete(ctx, id, product.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "review not found")
			return
		}
		response.InternalError(c, err, "could not remove review")
		return
	}
	response.OK(c, http.StatusOK, "review removed", nil)
}

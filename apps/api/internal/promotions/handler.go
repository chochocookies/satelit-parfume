package promotions

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type validateRequest struct {
	Code     string `json:"code" binding:"required"`
	Subtotal int64  `json:"subtotal" binding:"required,min=1"`
}

type validateResponse struct {
	Code     string `json:"code"`
	Discount int64  `json:"discount"`
}

// Validate backs POST /api/v1/promo/validate — public, no auth: see
// this package's own doc comment on why a code isn't gated to logged-in
// customers. Checkout re-validates and actually applies the code for
// real (internal/orders' Service.Checkout, via ApplyInTx) — this is
// only the checkout form's on-demand "apply" preview.
func (h *Handler) Validate(c *gin.Context) {
	var req validateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "code and subtotal are required")
		return
	}

	_, discount, err := h.service.Validate(c.Request.Context(), req.Code, req.Subtotal)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PROMO", promoErrorMessage(err))
		return
	}
	response.OK(c, http.StatusOK, "promo code applied", validateResponse{Code: req.Code, Discount: discount})
}

// promoErrorMessage turns any of this package's own sentinel errors (or
// ErrNotFound) into one of two messages — deliberately generic for
// everything except "below minimum purchase": telling a caller exactly
// which codes exist, which have expired, or how many times a code has
// already been used is more information than a validation error needs
// to leak. "Add more to your cart" isn't a comparable leak, so that one
// gets to be specific.
func promoErrorMessage(err error) string {
	if errors.Is(err, ErrBelowMinimum) {
		return "belum memenuhi minimum pembelian untuk kode ini"
	}
	return "kode promo tidak valid atau sudah tidak berlaku"
}

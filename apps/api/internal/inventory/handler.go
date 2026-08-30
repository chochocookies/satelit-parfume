package inventory

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// ListForBranch backs GET /api/v1/admin/branches/:id/inventory — gated by
// RequireRole + branches.RequireBranchAccess in main.go, so by the time
// this runs, the caller is already confirmed allowed to see this branch.
func (h *Handler) ListForBranch(c *gin.Context) {
	items, err := h.repo.ListForBranch(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err, "could not load inventory")
		return
	}
	response.OK(c, http.StatusOK, "branch inventory", items)
}

// SetStock backs PUT /api/v1/admin/branches/:id/inventory/:variantId.
func (h *Handler) SetStock(c *gin.Context) {
	var req SetStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	item, err := h.repo.SetStock(c.Request.Context(), c.Param("id"), c.Param("variantId"), req)
	if err != nil {
		response.InternalError(c, err, "could not set stock — check the variant id exists")
		return
	}
	response.OK(c, http.StatusOK, "stock updated", item)
}

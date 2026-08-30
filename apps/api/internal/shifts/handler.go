package shifts

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func callerID(c *gin.Context) string {
	v, _ := c.Get(auth.ContextSubjectID)
	id, _ := v.(string)
	return id
}

// Open backs POST /admin/branches/:id/shifts. Mounted behind
// RequireBranchAccess in main.go, same as inventory/staff/orders — a
// BRANCH_MANAGER/CASHIER/INVENTORY_STAFF can only open a shift at their
// own assigned branch; SUPER_ADMIN/ADMIN can open one anywhere.
func (h *Handler) Open(c *gin.Context) {
	var req OpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	shift, err := h.repo.Open(c.Request.Context(), c.Param("id"), callerID(c), req.OpeningBalance, req.Notes)
	if err != nil {
		if errors.Is(err, ErrAlreadyOpen) {
			response.Error(c, http.StatusConflict, "SHIFT_ALREADY_OPEN", err.Error())
			return
		}
		response.InternalError(c, err, "could not open shift")
		return
	}
	response.OK(c, http.StatusCreated, "shift opened", shift)
}

// Current backs GET /admin/branches/:id/shifts/current — the POS
// screen's own "do I already have a till open" check on load. Returns a
// null payload rather than a 404 when there's no open shift — "you have
// no open shift" is a normal, expected state here, not an error.
func (h *Handler) Current(c *gin.Context) {
	shift, err := h.repo.GetOpenForUser(c.Request.Context(), callerID(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.OK(c, http.StatusOK, "no open shift", nil)
			return
		}
		response.InternalError(c, err, "could not load current shift")
		return
	}
	response.OK(c, http.StatusOK, "current shift", shift)
}

// Close backs PUT /admin/branches/:id/shifts/:shiftId/close.
func (h *Handler) Close(c *gin.Context) {
	var req CloseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	shift, err := h.repo.Close(c.Request.Context(), c.Param("shiftId"), req.ClosingBalance, req.Notes)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "shift not found")
		case errors.Is(err, ErrAlreadyClosed):
			response.Error(c, http.StatusConflict, "ALREADY_CLOSED", err.Error())
		default:
			response.InternalError(c, err, "could not close shift")
		}
		return
	}
	response.OK(c, http.StatusOK, "shift closed", shift)
}

// List backs GET /admin/branches/:id/shifts — shift history for a branch.
func (h *Handler) List(c *gin.Context) {
	list, err := h.repo.ListForBranch(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err, "could not load shift history")
		return
	}
	response.OK(c, http.StatusOK, "shifts", list)
}

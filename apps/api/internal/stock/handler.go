package stock

import (
	"errors"
	"net/http"
	"strconv"

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

// ── Movements ────────────────────────────────────────────────────────

// Receive backs POST /admin/branches/:id/inventory/:variantId/receive.
func (h *Handler) Receive(c *gin.Context) {
	var req ReceiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	movement, err := h.repo.Receive(c.Request.Context(), c.Param("id"), c.Param("variantId"), req.Quantity, req.Note, callerID(c))
	if err != nil {
		response.InternalError(c, err, "could not record stock received")
		return
	}
	response.OK(c, http.StatusCreated, "stock received", movement)
}

// Adjust backs POST /admin/branches/:id/inventory/:variantId/adjust.
func (h *Handler) Adjust(c *gin.Context) {
	var req AdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	movement, err := h.repo.Adjust(c.Request.Context(), c.Param("id"), c.Param("variantId"), req.QuantityChange, req.Note, callerID(c))
	if err != nil {
		if errors.Is(err, ErrNegativeResult) {
			response.Error(c, http.StatusConflict, "NEGATIVE_RESULT", err.Error())
			return
		}
		response.InternalError(c, err, "could not adjust stock")
		return
	}
	response.OK(c, http.StatusOK, "stock adjusted", movement)
}

// Movements backs GET /admin/branches/:id/inventory/:variantId/movements.
func (h *Handler) Movements(c *gin.Context) {
	list, err := h.repo.MovementHistory(c.Request.Context(), c.Param("id"), c.Param("variantId"), queryInt(c, "limit", 50))
	if err != nil {
		response.InternalError(c, err, "could not load movement history")
		return
	}
	response.OK(c, http.StatusOK, "movements", list)
}

// ── Transfers ────────────────────────────────────────────────────────

// CreateTransfer backs POST /admin/branches/:id/transfers.
func (h *Handler) CreateTransfer(c *gin.Context) {
	var req CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	transfer, err := h.repo.CreateTransfer(c.Request.Context(), c.Param("id"), callerID(c), req)
	if err != nil {
		if errors.Is(err, ErrInsufficientStock) {
			response.Error(c, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error())
			return
		}
		response.InternalError(c, err, "could not create transfer")
		return
	}
	response.OK(c, http.StatusCreated, "transfer created", transfer)
}

// ListTransfers backs GET /admin/branches/:id/transfers.
func (h *Handler) ListTransfers(c *gin.Context) {
	list, err := h.repo.ListForBranch(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err, "could not load transfers")
		return
	}
	response.OK(c, http.StatusOK, "transfers", list)
}

// GetTransfer backs GET /admin/transfers/:transferId — global (SUPER_ADMIN/
// ADMIN only, via adminGroup), since a transfer spans two branches and
// doesn't belong under either one's URL alone.
func (h *Handler) GetTransfer(c *gin.Context) {
	transfer, err := h.repo.GetTransfer(c.Request.Context(), c.Param("transferId"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "transfer not found")
			return
		}
		response.InternalError(c, err, "could not load transfer")
		return
	}
	response.OK(c, http.StatusOK, "transfer", transfer)
}

// CompleteTransfer backs PUT /admin/branches/:id/transfers/:transferId/complete
// — only the destination branch can complete (they're the ones physically
// receiving and confirming the goods arrived).
func (h *Handler) CompleteTransfer(c *gin.Context) {
	transfer, err := h.repo.CompleteTransfer(c.Request.Context(), c.Param("transferId"), c.Param("id"), callerID(c))
	if err != nil {
		respondTransferError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "transfer completed", transfer)
}

// CancelTransfer backs PUT /admin/branches/:id/transfers/:transferId/cancel
// — only the source branch can cancel (they're the ones who requested it).
func (h *Handler) CancelTransfer(c *gin.Context) {
	transfer, err := h.repo.CancelTransfer(c.Request.Context(), c.Param("transferId"), c.Param("id"), callerID(c))
	if err != nil {
		respondTransferError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "transfer cancelled", transfer)
}

func respondTransferError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "transfer not found")
	case errors.Is(err, ErrTransferWrongBranch):
		response.Error(c, http.StatusConflict, "WRONG_BRANCH", err.Error())
	case errors.Is(err, ErrTransferNotPending):
		response.Error(c, http.StatusConflict, "NOT_PENDING", err.Error())
	default:
		response.InternalError(c, err, "could not update transfer")
	}
}

// ── Opname ───────────────────────────────────────────────────────────

// StartOpname backs POST /admin/branches/:id/opnames.
func (h *Handler) StartOpname(c *gin.Context) {
	opname, err := h.repo.StartOpname(c.Request.Context(), c.Param("id"), callerID(c))
	if err != nil {
		if errors.Is(err, ErrOpnameAlreadyOpen) {
			response.Error(c, http.StatusConflict, "OPNAME_ALREADY_OPEN", err.Error())
			return
		}
		response.InternalError(c, err, "could not start stock count")
		return
	}
	response.OK(c, http.StatusCreated, "stock count started", opname)
}

// CurrentOpname backs GET /admin/branches/:id/opnames/current — returns a
// null payload rather than a 404 when there's none open, same reasoning
// as shifts.Handler.Current.
func (h *Handler) CurrentOpname(c *gin.Context) {
	opname, err := h.repo.GetOpenForBranch(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.OK(c, http.StatusOK, "no open stock count", nil)
			return
		}
		response.InternalError(c, err, "could not load current stock count")
		return
	}
	response.OK(c, http.StatusOK, "current stock count", opname)
}

// ListOpnames backs GET /admin/branches/:id/opnames.
func (h *Handler) ListOpnames(c *gin.Context) {
	list, err := h.repo.ListForBranchOpnames(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err, "could not load stock count history")
		return
	}
	response.OK(c, http.StatusOK, "stock counts", list)
}

// GetOpname backs GET /admin/branches/:id/opnames/:opnameId.
func (h *Handler) GetOpname(c *gin.Context) {
	opname, err := h.repo.GetOpname(c.Request.Context(), c.Param("opnameId"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "stock count not found")
			return
		}
		response.InternalError(c, err, "could not load stock count")
		return
	}
	response.OK(c, http.StatusOK, "stock count", opname)
}

// CountItem backs PUT /admin/branches/:id/opnames/:opnameId/items/:itemId.
func (h *Handler) CountItem(c *gin.Context) {
	var req CountItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	item, err := h.repo.CountItem(c.Request.Context(), c.Param("opnameId"), c.Param("itemId"), req.CountedQuantity)
	if err != nil {
		switch {
		case errors.Is(err, ErrOpnameItemNotFound):
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "item not found in this stock count")
		case errors.Is(err, ErrOpnameNotOpen):
			response.Error(c, http.StatusConflict, "NOT_OPEN", err.Error())
		default:
			response.InternalError(c, err, "could not record count")
		}
		return
	}
	response.OK(c, http.StatusOK, "count recorded", item)
}

// CompleteOpname backs PUT /admin/branches/:id/opnames/:opnameId/complete.
func (h *Handler) CompleteOpname(c *gin.Context) {
	var req CompleteOpnameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	opname, err := h.repo.CompleteOpname(c.Request.Context(), c.Param("opnameId"), c.Param("id"), callerID(c), req.Notes)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "stock count not found")
		case errors.Is(err, ErrOpnameNotOpen):
			response.Error(c, http.StatusConflict, "NOT_OPEN", err.Error())
		default:
			response.InternalError(c, err, "could not complete stock count")
		}
		return
	}
	response.OK(c, http.StatusOK, "stock count completed", opname)
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

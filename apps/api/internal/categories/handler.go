package categories

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

// List backs the category filter dropdown on the (future) shop page.
func (h *Handler) List(c *gin.Context) {
	list, err := h.repo.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load categories")
		return
	}
	response.OK(c, http.StatusOK, "categories", list)
}

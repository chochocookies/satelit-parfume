package dashboard

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

// Stats backs GET /api/v1/admin/dashboard/stats — the admin dashboard's
// overview page. SUPER_ADMIN/ADMIN only (see main.go's route
// registration under adminGroup).
func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.repo.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load dashboard stats")
		return
	}
	response.OK(c, http.StatusOK, "dashboard stats", stats)
}

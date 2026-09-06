package membership

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// customerID mirrors wishlist/reviews' own copy of this check — see
// wishlist.Handler's doc comment on why it's duplicated per package
// rather than shared.
func customerID(c *gin.Context) (string, bool) {
	subjectType, _ := c.Get(auth.ContextSubjectType)
	if subjectType != "customer" {
		return "", false
	}
	subjectID, _ := c.Get(auth.ContextSubjectID)
	id, _ := subjectID.(string)
	return id, id != ""
}

// Me backs GET /api/v1/membership/me.
func (h *Handler) Me(c *gin.Context) {
	id, ok := customerID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "sign in to view your membership status")
		return
	}
	status, err := h.service.StatusFor(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, err, "could not load membership status")
		return
	}
	response.OK(c, http.StatusOK, "membership status", status)
}

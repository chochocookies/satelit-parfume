package branches

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// List backs GET /api/v1/branches (section 10's branch selector). Passing
// ?lat=&lng= sorts nearest-first (section 11); omitting them sorts by name.
func (h *Handler) List(c *gin.Context) {
	lat, latErr := queryFloat(c, "lat")
	lng, lngErr := queryFloat(c, "lng")
	if latErr != nil || lngErr != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "lat and lng must both be numbers")
		return
	}

	list, err := h.repo.List(c.Request.Context(), lat, lng)
	if err != nil {
		response.InternalError(c, err, "could not load branches")
		return
	}
	response.OK(c, http.StatusOK, "branches", list)
}

func queryFloat(c *gin.Context, key string) (*float64, error) {
	v := c.Query(key)
	if v == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (h *Handler) GetBySlug(c *gin.Context) {
	branch, err := h.repo.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		respondNotFoundOrError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "branch", branch)
}

func (h *Handler) Create(c *gin.Context) {
	var req UpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	branch, err := h.repo.Create(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, err, "could not create branch — check that the code is unique")
		return
	}
	response.OK(c, http.StatusCreated, "branch created", branch)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	branch, err := h.repo.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondNotFoundOrError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "branch updated", branch)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.repo.SoftDelete(c.Request.Context(), c.Param("id")); err != nil {
		respondNotFoundOrError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "branch deleted", nil)
}

func (h *Handler) ListStaff(c *gin.Context) {
	staff, err := h.repo.ListStaff(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err, "could not load branch staff")
		return
	}
	response.OK(c, http.StatusOK, "branch staff", staff)
}

func (h *Handler) AssignStaff(c *gin.Context) {
	var req AssignStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.repo.AssignStaff(c.Request.Context(), c.Param("id"), req.UserID); err != nil {
		response.InternalError(c, err, "could not assign staff — check the user id exists")
		return
	}
	response.OK(c, http.StatusOK, "staff assigned", nil)
}

func (h *Handler) UnassignStaff(c *gin.Context) {
	if err := h.repo.UnassignStaff(c.Request.Context(), c.Param("id"), c.Param("userId")); err != nil {
		response.InternalError(c, err, "could not unassign staff")
		return
	}
	response.OK(c, http.StatusOK, "staff unassigned", nil)
}

func respondNotFoundOrError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "branch not found")
		return
	}
	response.InternalError(c, err, "something went wrong")
}

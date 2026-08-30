package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/customers"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	result, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, customers.ErrEmailTaken) {
			response.Error(c, http.StatusConflict, "EMAIL_TAKEN", "an account with this email already exists")
			return
		}
		response.InternalError(c, err, "could not create account")
		return
	}

	response.OK(c, http.StatusCreated, "account created", result)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	result, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		respondLoginError(c, err)
		return
	}

	response.OK(c, http.StatusOK, "logged in", result)
}

func (h *Handler) StaffLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	result, err := h.service.StaffLogin(c.Request.Context(), req)
	if err != nil {
		respondLoginError(c, err)
		return
	}

	response.OK(c, http.StatusOK, "logged in", result)
}

func respondLoginError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTooManyAttempts):
		response.Error(c, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "incorrect email or password")
	default:
		response.InternalError(c, err, "could not log in")
	}
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "refresh token is invalid, expired, or already used")
		return
	}

	response.OK(c, http.StatusOK, "token refreshed", result)
}

func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	_ = h.service.Logout(c.Request.Context(), req.RefreshToken)
	response.OK(c, http.StatusOK, "logged out", nil)
}

func (h *Handler) Me(c *gin.Context) {
	subjectID, _ := c.Get(ContextSubjectID)
	subjectType, _ := c.Get(ContextSubjectType)

	subject, err := h.service.GetSubject(c.Request.Context(), subjectType.(string), subjectID.(string))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "account not found")
		return
	}

	response.OK(c, http.StatusOK, "current session", subject)
}

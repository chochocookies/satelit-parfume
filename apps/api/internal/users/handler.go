package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/pkg/password"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// List backs GET /api/v1/admin/users — the staff directory. Every result
// goes through ToStaffAccount() so PasswordHash never reaches a
// response body (see the User doc comment in model.go).
func (h *Handler) List(c *gin.Context) {
	list, err := h.repo.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, err, "could not load staff accounts")
		return
	}

	accounts := make([]StaffAccount, 0, len(list))
	for i := range list {
		accounts = append(accounts, list[i].ToStaffAccount())
	}
	response.OK(c, http.StatusOK, "staff accounts", accounts)
}

// Get backs GET /api/v1/admin/users/:id — used by the admin edit form to
// load one staff account before showing it.
func (h *Handler) Get(c *gin.Context) {
	u, err := h.repo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondLookupError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "staff account", u.ToStaffAccount())
}

// Create backs POST /api/v1/admin/users — the admin-facing "create staff
// user" endpoint this package's repository doc comment (pre-Phase-9)
// flagged as missing. Granting the SUPER_ADMIN role itself is restricted
// to callers who already hold it: an ADMIN can freely create
// BRANCH_MANAGER/CASHIER/INVENTORY_STAFF/ADMIN accounts, but can't mint
// another SUPER_ADMIN. Without this check, adminGroup's existing
// SUPER_ADMIN-or-ADMIN gate in main.go would let any ADMIN hand
// themselves or anyone else the platform's top role.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if hasRole(req.Roles, "SUPER_ADMIN") && !callerHasRole(c, "SUPER_ADMIN") {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "only a super admin can grant the SUPER_ADMIN role")
		return
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		response.InternalError(c, err, "could not create staff account")
		return
	}

	u, err := h.repo.Create(c.Request.Context(), req.Name, req.Email, hash, req.Roles)
	if err != nil {
		respondUpsertError(c, err)
		return
	}
	response.OK(c, http.StatusCreated, "staff account created", u.ToStaffAccount())
}

// Update backs PUT /api/v1/admin/users/:id — full replace of name,
// status, and roles (no email or password field; see UpdateRequest's doc
// comment on why). Same SUPER_ADMIN safeguard as Create.
//
// Deliberately not handled here — see the root README's "Deliberately
// not in Phase 9" note: nothing stops a SUPER_ADMIN from deactivating or
// demoting the platform's last other SUPER_ADMIN, or from editing their
// own account through this same endpoint. A real safeguard for either
// would need a "how many active SUPER_ADMINs remain" check this phase
// doesn't add.
func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if hasRole(req.Roles, "SUPER_ADMIN") && !callerHasRole(c, "SUPER_ADMIN") {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "only a super admin can grant the SUPER_ADMIN role")
		return
	}

	u, err := h.repo.Update(c.Request.Context(), c.Param("id"), req.Name, req.Status, req.Roles)
	if err != nil {
		respondUpsertError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "staff account updated", u.ToStaffAccount())
}

func respondLookupError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "staff account not found")
		return
	}
	response.InternalError(c, err, "could not load staff account")
}

func respondUpsertError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "staff account not found")
	case errors.Is(err, ErrEmailTaken):
		response.Error(c, http.StatusConflict, "EMAIL_TAKEN", err.Error())
	case errors.Is(err, ErrUnknownRole):
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		response.InternalError(c, err, "could not save staff account")
	}
}

// contextRolesKey mirrors auth.ContextRoles's value exactly (see
// internal/auth/middleware.go) — duplicated here as a literal instead of
// imported, because internal/auth already imports this package (its
// Service holds a *users.Repository to look up staff by email/id), so
// importing auth back from here would be a dependency cycle. If that
// constant's value ever changes, this needs to change with it.
const contextRolesKey = "auth_roles"

func callerHasRole(c *gin.Context, target string) bool {
	roles, _ := c.Get(contextRolesKey)
	rs, _ := roles.([]string)
	return hasRole(rs, target)
}

func hasRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

package branches

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/auth"
	"satelit-parfume-api/pkg/response"
)

// RequireBranchAccess restricts a :id-scoped route to staff who can
// legitimately act on that specific branch: SUPER_ADMIN and ADMIN pass
// through unconditionally (section 41: cross-branch access), everyone
// else must be assigned to this exact branch via branch_staff (section 9's
// business rule: "branch users cannot access unauthorized branches").
//
// Always chain this after auth.RequireAuth and auth.RequireRole — this
// middleware only narrows an already-qualifying role down to "and only
// for branches you're actually assigned to".
func RequireBranchAccess(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, _ := c.Get(auth.ContextRoles)
		for _, r := range toStringSlice(roles) {
			if r == "SUPER_ADMIN" || r == "ADMIN" {
				c.Next()
				return
			}
		}

		userIDVal, _ := c.Get(auth.ContextSubjectID)
		userID, _ := userIDVal.(string)
		branchID := c.Param("id")

		assigned, err := repo.IsStaffAssignedToBranch(c.Request.Context(), userID, branchID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not verify branch access")
			c.Abort()
			return
		}
		if !assigned {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "you are not assigned to this branch")
			c.Abort()
			return
		}
		c.Next()
	}
}

func toStringSlice(v any) []string {
	if s, ok := v.([]string); ok {
		return s
	}
	return nil
}

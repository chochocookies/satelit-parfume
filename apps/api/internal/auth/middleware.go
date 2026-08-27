package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/pkg/jwt"
	"satelit-parfume-api/pkg/response"
)

const (
	ContextSubjectID   = "auth_subject_id"
	ContextSubjectType = "auth_subject_type"
	ContextRoles       = "auth_roles"
)

// RequireAuth verifies the Authorization header's access token and
// attaches the caller's identity to the request context. It does not
// check roles on its own — chain RequireRole after it for staff-only
// routes.
func RequireAuth(accessTokens *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or malformed Authorization header")
			c.Abort()
			return
		}

		claims, err := accessTokens.Verify(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token")
			c.Abort()
			return
		}

		c.Set(ContextSubjectID, claims.Subject)
		c.Set(ContextSubjectType, claims.SubjectType)
		c.Set(ContextRoles, claims.Roles)
		c.Next()
	}
}

// OptionalAuth verifies the Authorization header if one is present,
// attaching identity to the request context on success — but unlike
// RequireAuth, it never aborts the request when the header is missing or
// invalid; it just proceeds without those context values set. For
// endpoints that behave differently for a logged-in customer vs. a guest
// without requiring login either way — the cart today.
func OptionalAuth(accessTokens *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			c.Next()
			return
		}

		claims, err := accessTokens.Verify(token)
		if err != nil {
			c.Next()
			return
		}

		c.Set(ContextSubjectID, claims.Subject)
		c.Set(ContextSubjectType, claims.SubjectType)
		c.Set(ContextRoles, claims.Roles)
		c.Next()
	}
}

// RequireRole restricts a route to staff accounts holding at least one of
// the given roles. Always chain it after RequireAuth. A customer subject
// (which carries no roles) is rejected by any non-empty role list here.
func RequireRole(allowed ...string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		roles, _ := c.Get(ContextRoles)
		for _, r := range toStringSlice(roles) {
			if _, ok := allowedSet[r]; ok {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, "FORBIDDEN", "you do not have permission to perform this action")
		c.Abort()
	}
}

func toStringSlice(v any) []string {
	if s, ok := v.([]string); ok {
		return s
	}
	return nil
}

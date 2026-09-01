// Package response gives every handler the same JSON envelope, matching
// the API spec: {"success", "message", "code", "data"}. Introduced now
// (Phase 2) because auth adds enough handlers that hand-rolling the
// envelope per-handler, as Phase 1's single health check did, would mean
// five slightly-different copies of the same three lines.
package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"satelit-parfume-api/pkg/logger"
)

type Envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// OK writes a success envelope.
func OK(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Envelope{Success: true, Message: message, Data: data})
}

// Error writes a failure envelope with a machine-readable code, e.g.
// "INVALID_CREDENTIALS" — never a raw internal error string; callers pass
// a message safe to show the client, and log details separately.
func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, Envelope{Success: false, Message: message, Code: code})
}

// InternalError is Error's 500-specific sibling — added after a real bug
// report turned out to be undiagnosable because every handler's generic
// "something went wrong" branch matched Error's own doc comment's second
// half ("log details separately") in word only: nothing anywhere in this
// codebase actually called the logger for one of these before this
// existed. This is the fix: the real err always gets logged server-side
// with the request's method and path, so `docker logs` (or any other
// stdout collector) shows exactly what happened, while the client still
// only ever sees the same safe, generic clientMessage Error would have
// sent — nothing about what the client receives changes, only what the
// operator can now see.
//
// When err is a Postgres error, its own Error() string is deliberately
// thin — pgconn.PgError.Error() is just "SEVERITY: message (SQLSTATE
// code)" and never includes Detail, which is where Postgres actually
// names the offending value (e.g. "Key (product_variant_id)=(...) is not
// present in table..." for a foreign-key violation). Logging Detail and
// ConstraintName alongside the plain error, when they're present, is the
// difference between a log line that says *that* something violated a
// constraint and one that says *which value* did.
func InternalError(c *gin.Context, err error, clientMessage string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Detail != "" {
		logger.Errorf("%s %s -> %v | detail: %s | constraint: %s",
			c.Request.Method, c.Request.URL.Path, err, pgErr.Detail, pgErr.ConstraintName)
	} else {
		logger.Errorf("%s %s -> %v", c.Request.Method, c.Request.URL.Path, err)
	}
	c.JSON(http.StatusInternalServerError, Envelope{Success: false, Message: clientMessage, Code: "INTERNAL_ERROR"})
}

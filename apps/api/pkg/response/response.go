// Package response gives every handler the same JSON envelope, matching
// the API spec: {"success", "message", "code", "data"}. Introduced now
// (Phase 2) because auth adds enough handlers that hand-rolling the
// envelope per-handler, as Phase 1's single health check did, would mean
// five slightly-different copies of the same three lines.
package response

import "github.com/gin-gonic/gin"

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

// Package logger provides minimal structured-ish logging for Phase 1.
// It is deliberately small: swap in zerolog/zap later if request volume
// or log-aggregation needs outgrow plain stdout lines.
package logger

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var std = log.New(os.Stdout, "", 0)

func Init() {
	std.SetFlags(0)
}

func Infof(format string, args ...interface{}) {
	std.Printf("[%s] INFO  "+format, prepend(time.Now().Format(time.RFC3339), args)...)
}

func Errorf(format string, args ...interface{}) {
	std.Printf("[%s] ERROR "+format, prepend(time.Now().Format(time.RFC3339), args)...)
}

func prepend(first string, rest []interface{}) []interface{} {
	out := make([]interface{}, 0, len(rest)+1)
	out = append(out, first)
	out = append(out, rest...)
	return out
}

// GinMiddleware logs one line per request: method, path, status, latency.
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		Infof("%s %s %d %s", c.Request.Method, path, c.Writer.Status(), time.Since(start))
	}
}

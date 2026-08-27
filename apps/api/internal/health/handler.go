// Package health exposes the API's liveness/readiness endpoint.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"satelit-parfume-api/pkg/response"
)

// Handler checks the API process itself plus its two hard dependencies
// (PostgreSQL, Redis) and reports each individually. Docker healthchecks,
// uptime monitors, and the landing page's status widget all hit this.
func Handler(pg *pgxpool.Pool, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		checks := map[string]any{"api": "ok"}
		allOK := true

		if err := pg.Ping(ctx); err != nil {
			checks["database"] = "unreachable"
			allOK = false
		} else {
			checks["database"] = "ok"
		}

		if err := rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "unreachable"
			allOK = false
		} else {
			checks["redis"] = "ok"
		}

		if allOK {
			response.OK(c, http.StatusOK, "Satelit Parfume API is healthy", checks)
			return
		}
		c.JSON(http.StatusServiceUnavailable, response.Envelope{
			Success: false,
			Message: "Satelit Parfume API is degraded",
			Data:    checks,
		})
	}
}

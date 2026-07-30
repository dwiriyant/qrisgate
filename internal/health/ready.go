package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// ReadyHandler returns 200 when Postgres is reachable.
func ReadyHandler(pool *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "not ready",
				"db":     err.Error(),
			})
		}
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ready",
			"db":     "ok",
		})
	}
}

package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// ReadyHandler returns 200 when Postgres (and Redis if configured) are reachable.
func ReadyHandler(pool *pgxpool.Pool, rdb *redis.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()

		out := map[string]string{"status": "ready"}
		if err := pool.Ping(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "not ready",
				"db":     err.Error(),
			})
		}
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"status": "not ready",
					"redis":  err.Error(),
				})
			}
			out["redis"] = "ok"
		}
		out["db"] = "ok"
		return c.JSON(http.StatusOK, out)
	}
}

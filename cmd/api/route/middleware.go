package route

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/qrisgate/qrisgate/internal/observability"
)

func Middleware(e *echo.Echo) {
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORS())
	e.Use(requestLogger())
	e.Use(metricsMiddleware())
}

func requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:  true,
		LogURI:     true,
		LogMethod:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			c.Logger().Infof("%s %s %d %s", v.Method, v.URI, v.Status, v.Latency)
			return nil
		},
	})
}

func metricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			method := c.Request().Method
			path := c.Path()

			observability.HTTPInFlight.WithLabelValues(method, path).Inc()
			defer observability.HTTPInFlight.WithLabelValues(method, path).Dec()

			err := next(c)
			status := c.Response().Status
			if status == 0 {
				status = 200
			}
			statusStr := strconv.Itoa(status)
			observability.HTTPRequests.WithLabelValues(method, path, statusStr).Inc()
			observability.HTTPDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
			return err
		}
	}
}

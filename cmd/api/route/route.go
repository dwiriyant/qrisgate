package route

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"golang.org/x/time/rate"

	"github.com/qrisgate/qrisgate/internal/admin"
	"github.com/qrisgate/qrisgate/internal/config"
	"github.com/qrisgate/qrisgate/internal/health"
	"github.com/qrisgate/qrisgate/internal/openapi"
	"github.com/qrisgate/qrisgate/internal/payment"
)

type Deps struct {
	Config  config.Config
	Pool    *pgxpool.Pool
	Admin   *admin.Handler
	Payment *payment.Handler
}

func Register(e *echo.Echo, d Deps) {
	e.GET("/healthz", func(c echo.Context) error { return c.NoContent(200) })
	e.GET("/readyz", health.ReadyHandler(d.Pool))

	if d.Config.SwaggerEnabled() {
		registerSwagger(e, openapi.Spec)
	}

	v1 := e.Group("/v1")
	adminGroup := v1.Group("/apps", admin.AdminAuth(d.Config.AdminToken))
	adminGroup.POST("", d.Admin.CreateApp)
	adminGroup.POST("/:id/webhooks", d.Admin.CreateWebhook)

	pay := v1.Group("/payments", rateLimit(d.Config.RateLimitPerMin))
	pay.POST("", d.Payment.Create)
	pay.GET("/:id", d.Payment.Get)
}

func NewEcho(cfg config.Config, serviceName string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.BodyLimit(cfg.BodyLimit))
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 30 * time.Second}))
	Middleware(e)
	if !cfg.OTELDisabled() {
		e.Use(otelecho.Middleware(serviceName))
	}
	return e
}

func rateLimit(perMin int) echo.MiddlewareFunc {
	if perMin <= 0 {
		perMin = 100
	}
	store := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      rate.Limit(float64(perMin) / 60),
		Burst:     perMin,
		ExpiresIn: 3 * time.Minute,
	})
	return middleware.RateLimiter(store)
}

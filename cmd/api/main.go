package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/qrisgate/qrisgate/cmd/api/route"
	"github.com/qrisgate/qrisgate/internal/admin"
	"github.com/qrisgate/qrisgate/internal/config"
	"github.com/qrisgate/qrisgate/internal/observability"
	"github.com/qrisgate/qrisgate/internal/payment"
	"github.com/qrisgate/qrisgate/internal/repository/postgres"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(slog.LevelInfo)

	ctx := context.Background()
	shutdownTrace, err := observability.InitTracing(ctx, cfg.OTELService, cfg.OTELEndpoint, log)
	if err != nil {
		log.Error("otel init failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTrace(context.Background()) }()

	if cfg.MetricsEnabled() {
		observability.StartMetricsServer(cfg.MetricsAddr, log)
	}
	if cfg.SwaggerEnabled() {
		log.Info("swagger UI enabled", "path", "/swagger/")
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	appRepo := postgres.NewAppRepository(pool)
	payRepo := postgres.NewPaymentRepository(pool)
	webhookRepo := postgres.NewWebhookRepository(pool)

	adminSvc := admin.NewService(appRepo, webhookRepo)
	paySvc := payment.NewService(appRepo, payRepo, cfg.DefaultExpiresIn)

	e := route.NewEcho(cfg, cfg.OTELService)
	route.Register(e, route.Deps{
		Config:  cfg,
		Pool:    pool,
		Admin:   admin.NewHandler(adminSvc),
		Payment: payment.NewHandler(paySvc),
	})

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      e,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
	go func() {
		log.Info("api listening", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func runMigrations(databaseURL string) error {
	db, err := goose.OpenDBWithDriver("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return goose.Up(db, "db/migrations")
}

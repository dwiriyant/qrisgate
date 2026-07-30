package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_defaults(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("RATE_LIMIT_PER_MIN", "")
	t.Setenv("DEFAULT_EXPIRES_IN_SEC", "")

	cfg := Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("addr=%q", cfg.Addr)
	}
	if cfg.RateLimitPerMin != 100 {
		t.Fatalf("rate=%d", cfg.RateLimitPerMin)
	}
	if cfg.DefaultExpiresIn != 15*time.Minute {
		t.Fatalf("expires=%v", cfg.DefaultExpiresIn)
	}
	if cfg.BodyLimit != "64K" {
		t.Fatalf("body=%q", cfg.BodyLimit)
	}
}

func TestLoad_fromEnv(t *testing.T) {
	t.Setenv("ADDR", ":9999")
	t.Setenv("ADMIN_TOKEN", "secret")
	t.Setenv("RATE_LIMIT_PER_MIN", "42")
	t.Setenv("DEFAULT_EXPIRES_IN_SEC", "120")
	t.Setenv("HTTP_READ_TIMEOUT_SEC", "5")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("METRICS_ENABLED", "false")

	cfg := Load()
	if cfg.Addr != ":9999" || cfg.AdminToken != "secret" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.RateLimitPerMin != 42 || cfg.DefaultExpiresIn != 2*time.Minute {
		t.Fatalf("rate/expires mismatch")
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Fatalf("read=%v", cfg.ReadTimeout)
	}
	if cfg.OTELDisabled() {
		t.Fatal("otel should be enabled")
	}
	if cfg.MetricsEnabled() {
		t.Fatal("metrics should be disabled")
	}
}

func TestOTELDisabled_empty(t *testing.T) {
	var cfg Config
	if !cfg.OTELDisabled() {
		t.Fatal("expected disabled")
	}
}

func TestSwaggerEnabled(t *testing.T) {
	if (Config{SwaggerActive: true, Env: "development"}).SwaggerEnabled() != true {
		t.Fatal("expected enabled in development")
	}
	if (Config{SwaggerActive: true, Env: "production"}).SwaggerEnabled() {
		t.Fatal("expected disabled in production")
	}
	if (Config{SwaggerActive: false, Env: "development"}).SwaggerEnabled() {
		t.Fatal("expected disabled when flag off")
	}
}

func TestGetenvInt_invalidFallsBack(t *testing.T) {
	t.Setenv("RATE_LIMIT_PER_MIN", "not-a-number")
	cfg := Load()
	if cfg.RateLimitPerMin != 100 {
		t.Fatalf("got %d", cfg.RateLimitPerMin)
	}
	_ = os.Getenv("RATE_LIMIT_PER_MIN")
}

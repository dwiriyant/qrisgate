package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr              string
	DatabaseURL       string
	RedisAddr         string
	AdminToken        string
	OTELEndpoint      string
	OTELService       string
	MetricsAddr       string
	Env               string
	RateLimitPerMin   int
	DefaultExpiresIn  time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	BodyLimit         string
	SwaggerActive     bool
}

func Load() Config {
	return Config{
		Addr:             getenv("ADDR", ":8080"),
		DatabaseURL:      getenv("DATABASE_URL", "postgres://qrisgate:qrisgate@localhost:5432/qrisgate?sslmode=disable"),
		RedisAddr:        getenv("REDIS_ADDR", "localhost:6379"),
		AdminToken:       os.Getenv("ADMIN_TOKEN"),
		OTELEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTELService:      getenv("OTEL_SERVICE_NAME", "qrisgate-api"),
		MetricsAddr:      getenv("METRICS_ADDR", ":9090"),
		Env:              getenv("ENV", "development"),
		RateLimitPerMin:  getenvInt("RATE_LIMIT_PER_MIN", 100),
		DefaultExpiresIn: time.Duration(getenvInt("DEFAULT_EXPIRES_IN_SEC", 900)) * time.Second,
		ReadTimeout:      time.Duration(getenvInt("HTTP_READ_TIMEOUT_SEC", 15)) * time.Second,
		WriteTimeout:     time.Duration(getenvInt("HTTP_WRITE_TIMEOUT_SEC", 30)) * time.Second,
		IdleTimeout:      time.Duration(getenvInt("HTTP_IDLE_TIMEOUT_SEC", 60)) * time.Second,
		BodyLimit:        getenv("HTTP_BODY_LIMIT", "64K"),
		SwaggerActive:    getenvBool("SWAGGER_ACTIVE", false),
	}
}

// SwaggerEnabled exposes /swagger only when explicitly enabled and not in production.
func (c Config) SwaggerEnabled() bool {
	return c.SwaggerActive && c.Env != "production"
}

func (c Config) OTELDisabled() bool {
	return c.OTELEndpoint == ""
}

func (c Config) MetricsEnabled() bool {
	v := getenv("METRICS_ENABLED", "true")
	b, _ := strconv.ParseBool(v)
	return b
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

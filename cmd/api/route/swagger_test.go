package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qrisgate/qrisgate/internal/config"
	"github.com/qrisgate/qrisgate/internal/openapi"
)

func TestRegister_swagger_enabled(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	deps.Config = config.Config{
		AdminToken:    "admin",
		RateLimitPerMin: 100,
		BodyLimit:     "64K",
		SwaggerActive: true,
		Env:           "development",
	}

	e := NewEcho(deps.Config, "test")
	Register(e, deps)

	req := httptest.NewRequest(http.MethodGet, "/swagger/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("spec status=%d", rec.Code)
	}
	if len(rec.Body.Bytes()) == 0 {
		t.Fatal("empty spec")
	}

	req = httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ui status=%d", rec.Code)
	}
}

func TestRegister_swagger_disabled(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	deps.Config.SwaggerActive = false

	e := NewEcho(config.Config{BodyLimit: "64K"}, "test")
	Register(e, deps)

	req := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	_ = openapi.Spec
}

func TestRegister_swagger_blockedInProduction(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()
	deps.Config = config.Config{
		BodyLimit:     "64K",
		SwaggerActive: true,
		Env:           "production",
	}

	e := NewEcho(deps.Config, "test")
	Register(e, deps)

	req := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

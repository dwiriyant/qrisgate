package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/qrisgate/qrisgate/internal/admin"
	"github.com/qrisgate/qrisgate/internal/config"
	"github.com/qrisgate/qrisgate/internal/domain"
	"github.com/qrisgate/qrisgate/internal/payment"
	"github.com/qrisgate/qrisgate/internal/qris"
)

func testDeps(t *testing.T) (Deps, func()) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://127.0.0.1:1/none?connect_timeout=1")
	if err != nil {
		t.Fatalf("pool: %v", err)
	}

	apps := &stubAppStore{}
	pays := &stubPayStore{}
	adminSvc := admin.NewService(apps, &stubWebhookStore{})
	paySvc := payment.NewService(apps, pays, time.Minute)

	return Deps{
		Config:  config.Config{AdminToken: "admin", RateLimitPerMin: 100},
		Pool:    pool,
		Admin:   admin.NewHandler(adminSvc),
		Payment: payment.NewHandler(paySvc),
	}, pool.Close
}

type stubAppStore struct{}

func (stubAppStore) Create(ctx context.Context, name, apiKeyHash, merchantQRIS string) (*domain.App, error) {
	return &domain.App{ID: "app-1", Name: name, MerchantQRIS: merchantQRIS}, nil
}

func (stubAppStore) GetByID(ctx context.Context, id string) (*domain.App, error) {
	if id == "app-1" {
		return &domain.App{ID: "app-1", MerchantQRIS: qris.SampleStaticQRIS()}, nil
	}
	return nil, domain.ErrNotFound
}

func (stubAppStore) UpdateMerchantQRIS(ctx context.Context, id, merchantQRIS string) (*domain.App, error) {
	if id != "app-1" {
		return nil, domain.ErrNotFound
	}
	return &domain.App{ID: "app-1", MerchantQRIS: merchantQRIS}, nil
}

func (stubAppStore) GetByAPIKeyHash(ctx context.Context, hash string) (*domain.App, error) {
	return &domain.App{ID: "app-1", MerchantQRIS: qris.SampleStaticQRIS()}, nil
}

type stubPayStore struct {
	byID map[string]*domain.Payment
}

func (s *stubPayStore) GetByAppAndOrder(ctx context.Context, appID, orderID string) (*domain.Payment, error) {
	return nil, domain.ErrNotFound
}

func (s *stubPayStore) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	if s.byID != nil {
		if p, ok := s.byID[id]; ok {
			return p, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *stubPayStore) Create(ctx context.Context, p *domain.Payment) error {
	p.ID = "pay-route-1"
	if s.byID == nil {
		s.byID = map[string]*domain.Payment{}
	}
	s.byID[p.ID] = p
	return nil
}

func (s *stubPayStore) MarkPaid(ctx context.Context, id string) (*domain.Payment, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Status = domain.PaymentPaid
	return p, nil
}

func (s *stubPayStore) InsertEvent(ctx context.Context, paymentID, eventType string, payload []byte) error {
	return nil
}

type stubWebhookStore struct{}

func (stubWebhookStore) Create(ctx context.Context, appID, url, secret string) (*domain.WebhookEndpoint, error) {
	return &domain.WebhookEndpoint{ID: "wh-1", AppID: appID, URL: url}, nil
}

func (stubWebhookStore) ListByAppID(ctx context.Context, appID string) ([]*domain.WebhookEndpoint, error) {
	return nil, nil
}

func TestRegister_healthz(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	cfg := config.Config{AdminToken: "admin", RateLimitPerMin: 100, BodyLimit: "64K"}
	e := NewEcho(cfg, "test")
	Register(e, deps)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRegister_readyz_degraded(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	e := NewEcho(config.Config{BodyLimit: "64K"}, "test")
	Register(e, deps)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNewEcho_middleware(t *testing.T) {
	e := NewEcho(config.Config{BodyLimit: "64K"}, "test")
	e.GET("/ping", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get(echo.HeaderXRequestID) == "" {
		t.Fatal("expected request id header")
	}
}

func TestRateLimit_zeroUsesDefault(t *testing.T) {
	mw := rateLimit(0)
	if mw == nil {
		t.Fatal("nil middleware")
	}
}

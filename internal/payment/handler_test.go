package payment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/qrisgate/qrisgate/internal/domain"
	"github.com/qrisgate/qrisgate/internal/qris"
)

func testPaymentHandler(t *testing.T) (*Handler, *echo.Echo, *mockPayments) {
	t.Helper()
	static := qris.SampleStaticQRIS()
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	app := &domain.App{ID: "app-1", MerchantQRIS: static}
	svc := NewService(&mockApp{app: app}, store, time.Minute)
	return NewHandler(svc), echo.New(), store
}

func TestHandler_Create(t *testing.T) {
	h, e, store := testPaymentHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/payments", strings.NewReader(`{"order_id":"ORD-1","amount":5000}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", "qg_test")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Create(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.created) != 1 {
		t.Fatalf("created=%d", len(store.created))
	}
	var out PaymentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.QRISString == "" {
		t.Fatal("empty qris")
	}
}

func TestHandler_Create_unauthorized(t *testing.T) {
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	svc := NewService(&mockApp{err: domain.ErrNotFound}, store, time.Minute)
	h := NewHandler(svc)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/v1/payments", strings.NewReader(`{"order_id":"x","amount":100}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", "qg_invalid")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Create(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandler_Create_badInput(t *testing.T) {
	h, e, _ := testPaymentHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/payments", strings.NewReader(`{"order_id":"","amount":0}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", "qg_test")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Create(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandler_Get(t *testing.T) {
	h, e, store := testPaymentHandler(t)
	static := qris.SampleStaticQRIS()
	store.byOrder["app-1:ORD-1"] = &domain.Payment{
		ID: "pay-99", AppID: "app-1", OrderID: "ORD-1", Amount: 1000,
		QRISString: static, Status: domain.PaymentPending,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/payments/pay-99", nil)
	req.Header.Set("X-API-Key", "qg_test")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("pay-99")

	if err := h.Get(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Get_notFound(t *testing.T) {
	h, e, _ := testPaymentHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/payments/missing", nil)
	req.Header.Set("X-API-Key", "qg_test")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("missing")

	if err := h.Get(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandler_Get_wrongTenant(t *testing.T) {
	h, e, store := testPaymentHandler(t)
	static := qris.SampleStaticQRIS()
	store.byOrder["other:ORD-1"] = &domain.Payment{
		ID: "pay-other", AppID: "other-app", OrderID: "ORD-1", Amount: 1000,
		QRISString: static, Status: domain.PaymentPending,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/payments/pay-other", nil)
	req.Header.Set("X-API-Key", "qg_test")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("pay-other")

	if err := h.Get(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

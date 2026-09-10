package admin

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

func testHandler(t *testing.T, token string) (*Handler, *echo.Echo) {
	t.Helper()
	svc := NewService(&mockAppStore{apps: map[string]*domain.App{
		"app-1": {ID: "app-1", Name: "Demo", CreatedAt: time.Now().UTC()},
	}}, &mockWebhookStore{})
	return NewHandler(svc), echo.New()
}

func TestHandler_CreateApp(t *testing.T) {
	h, e := testHandler(t, "admin")
	body := `{"name":"Shop","merchant_qris":"` + qris.SampleStaticQRIS() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/apps", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Authorization", "Bearer admin")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.CreateApp(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out CreateAppOutput
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.APIKey == "" || out.App.Name != "Shop" {
		t.Fatalf("%+v", out)
	}
}

func TestHandler_CreateApp_invalidJSON(t *testing.T) {
	h, e := testHandler(t, "admin")
	req := httptest.NewRequest(http.MethodPost, "/v1/apps", strings.NewReader("{"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.CreateApp(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandler_CreateApp_invalidQRIS(t *testing.T) {
	h, e := testHandler(t, "admin")
	req := httptest.NewRequest(http.MethodPost, "/v1/apps", strings.NewReader(`{"name":"x","merchant_qris":"bad"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.CreateApp(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandler_CreateWebhook(t *testing.T) {
	h, e := testHandler(t, "admin")
	req := httptest.NewRequest(http.MethodPost, "/v1/apps/app-1/webhooks", strings.NewReader(`{"url":"https://example.com/hook"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("app-1")

	if err := h.CreateWebhook(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_CreateWebhook_notFound(t *testing.T) {
	h, e := testHandler(t, "admin")
	req := httptest.NewRequest(http.MethodPost, "/v1/apps/missing/webhooks", strings.NewReader(`{"url":"https://example.com/hook"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("missing")

	if err := h.CreateWebhook(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandler_UpdateApp(t *testing.T) {
	h, e := testHandler(t, "admin")
	body := `{"merchant_qris":"` + qris.SampleStaticQRIS() + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/apps/app-1", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("app-1")

	if err := h.UpdateApp(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminAuth(t *testing.T) {
	e := echo.New()
	e.GET("/admin", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, AdminAuth("secret"))

	t.Run("ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing token config", func(t *testing.T) {
		e2 := echo.New()
		e2.GET("/admin", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, AdminAuth(""))
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		rec := httptest.NewRecorder()
		e2.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("wrong bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

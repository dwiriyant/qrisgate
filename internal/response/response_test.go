package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func newCtx(method, path string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) ErrorBody {
	t.Helper()
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%q", err, rec.Body.String())
	}
	return body
}

func TestOK(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/")
	if err := OK(c, map[string]string{"ok": "true"}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestCreated(t *testing.T) {
	c, rec := newCtx(http.MethodPost, "/")
	if err := Created(c, map[string]int{"id": 1}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestBadRequest(t *testing.T) {
	c, rec := newCtx(http.MethodPost, "/")
	_ = BadRequest(c, "nope")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := decodeError(t, rec); body.Error != "nope" {
		t.Fatalf("body=%q", body.Error)
	}
}

func TestUnauthorized(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/")
	_ = Unauthorized(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/")
	_ = NotFound(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestConflict(t *testing.T) {
	c, rec := newCtx(http.MethodPost, "/")
	_ = Conflict(c, "duplicate")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestInternal(t *testing.T) {
	c, rec := newCtx(http.MethodGet, "/")
	_ = Internal(c)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

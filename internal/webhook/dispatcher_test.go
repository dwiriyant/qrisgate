package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qrisgate/qrisgate/internal/domain"
)

func TestSign(t *testing.T) {
	got := Sign("secret", []byte(`{"id":"1"}`))
	if got[:7] != "sha256=" || len(got) != 7+64 {
		t.Fatalf("got=%s", got)
	}
}

func TestDispatchPaid(t *testing.T) {
	var gotSig, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotSig = r.Header.Get(SignatureHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher()
	p := &domain.Payment{ID: "pay-1", OrderID: "ORD-1", Amount: 5000, Status: domain.PaymentPaid}
	err := d.DispatchPaid(context.Background(), p, []*domain.WebhookEndpoint{
		{URL: srv.URL, Secret: "whsec_test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody == "" || gotSig != Sign("whsec_test", []byte(gotBody)) {
		t.Fatalf("body=%s sig=%s", gotBody, gotSig)
	}
}

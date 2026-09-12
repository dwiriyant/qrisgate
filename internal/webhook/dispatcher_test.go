package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/qrisgate/qrisgate/internal/domain"
)

func TestSign(t *testing.T) {
	got := Sign("secret", []byte(`{"id":"1"}`))
	if got[:7] != "sha256=" || len(got) != 7+64 {
		t.Fatalf("got=%s", got)
	}
}

type memStore struct {
	mu   sync.Mutex
	byKey map[string]*domain.WebhookDelivery
}

func (m *memStore) key(eventID, dest string) string { return eventID + "|" + dest }

func (m *memStore) EnsureDelivery(ctx context.Context, paymentID, destination, eventID, secret string, payload []byte) (*domain.WebhookDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byKey == nil {
		m.byKey = map[string]*domain.WebhookDelivery{}
	}
	k := m.key(eventID, destination)
	if d, ok := m.byKey[k]; ok {
		cp := *d
		return &cp, nil
	}
	now := time.Now().UTC()
	d := &domain.WebhookDelivery{
		ID: "d1", PaymentID: paymentID, Destination: destination, EventID: eventID,
		Status: domain.WebhookPending, Payload: payload, Secret: secret, CreatedAt: now, UpdatedAt: now,
	}
	m.byKey[k] = d
	cp := *d
	return &cp, nil
}

func (m *memStore) MarkDelivered(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.byKey {
		if d.ID == id {
			d.Status = domain.WebhookDelivered
			now := time.Now().UTC()
			d.DeliveredAt = &now
		}
	}
	return nil
}

func (m *memStore) MarkAttempt(ctx context.Context, id string, attempts int, lastErr string, nextAttemptAt *time.Time, dead bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.byKey {
		if d.ID == id {
			d.Attempts = attempts
			d.LastError = lastErr
			d.NextAttemptAt = nextAttemptAt
			if dead {
				d.Status = domain.WebhookDead
			} else {
				d.Status = domain.WebhookPending
			}
		}
	}
	return nil
}

func (m *memStore) ListDue(ctx context.Context, limit int) ([]*domain.WebhookDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.WebhookDelivery
	for _, d := range m.byKey {
		if d.Status == domain.WebhookPending {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func TestDispatchPaid_idempotentSkip(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		b, _ := io.ReadAll(r.Body)
		var payload PaidPayload
		_ = json.Unmarshal(b, &payload)
		if payload.EventID == "" || r.Header.Get(IdempotencyHeader) == "" {
			t.Errorf("missing idempotency")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &memStore{}
	d := NewDispatcher(store)
	d.sleep = func(time.Duration) {}
	p := &domain.Payment{ID: "pay-1", OrderID: "ORD-1", Amount: 5000, Status: domain.PaymentPaid}
	hooks := []*domain.WebhookEndpoint{{URL: srv.URL, Secret: "whsec_test"}}

	if err := d.DispatchPaid(context.Background(), p, hooks); err != nil {
		t.Fatal(err)
	}
	if err := d.DispatchPaid(context.Background(), p, hooks); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits=%d want 1 (second dispatch must skip)", hits)
	}
}

func TestDispatchPaid_retriesThenSucceeds(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &memStore{}
	d := NewDispatcher(store)
	d.sleep = func(time.Duration) {}
	p := &domain.Payment{ID: "pay-2", OrderID: "ORD-2", Amount: 1000, Status: domain.PaymentPaid}
	if err := d.DispatchPaid(context.Background(), p, []*domain.WebhookEndpoint{{URL: srv.URL}}); err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Fatalf("hits=%d", hits)
	}
	k := store.key(PaidEventID("pay-2"), srv.URL)
	if store.byKey[k].Status != domain.WebhookDelivered {
		t.Fatalf("status=%s", store.byKey[k].Status)
	}
}

func TestDispatchPaid_clientErrorDead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	store := &memStore{}
	d := NewDispatcher(store)
	d.sleep = func(time.Duration) {}
	p := &domain.Payment{ID: "pay-3", OrderID: "ORD-3", Amount: 1000, Status: domain.PaymentPaid}
	_ = d.DispatchPaid(context.Background(), p, []*domain.WebhookEndpoint{{URL: srv.URL}})
	k := store.key(PaidEventID("pay-3"), srv.URL)
	if store.byKey[k].Status != domain.WebhookDead {
		t.Fatalf("status=%s", store.byKey[k].Status)
	}
}

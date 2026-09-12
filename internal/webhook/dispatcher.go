package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/qrisgate/qrisgate/internal/domain"
)

const (
	SignatureHeader    = "X-Qrisgate-Signature"
	IdempotencyHeader  = "Idempotency-Key"
	maxSyncAttempts    = 3
	maxTotalAttempts   = 8
	defaultRetryEvery  = 30 * time.Second
)

type DeliveryStore interface {
	EnsureDelivery(ctx context.Context, paymentID, destination, eventID, secret string, payload []byte) (*domain.WebhookDelivery, error)
	MarkDelivered(ctx context.Context, id string) error
	MarkAttempt(ctx context.Context, id string, attempts int, lastErr string, nextAttemptAt *time.Time, dead bool) error
	ListDue(ctx context.Context, limit int) ([]*domain.WebhookDelivery, error)
}

type Dispatcher struct {
	client *http.Client
	store  DeliveryStore
	log    *slog.Logger
	now    func() time.Time
	sleep  func(time.Duration)
}

func NewDispatcher(store DeliveryStore) *Dispatcher {
	return &Dispatcher{
		client: &http.Client{Timeout: 8 * time.Second},
		store:  store,
		log:    slog.Default(),
		now:    time.Now,
		sleep:  time.Sleep,
	}
}

func (d *Dispatcher) WithLogger(log *slog.Logger) *Dispatcher {
	if log != nil {
		d.log = log
	}
	return d
}

func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func PaidEventID(paymentID string) string {
	return "payment.paid:" + paymentID
}

type PaidPayload struct {
	ID             string `json:"id"`
	OrderID        string `json:"order_id"`
	Amount         int64  `json:"amount"`
	Status         string `json:"status"`
	EventID        string `json:"event_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (d *Dispatcher) DispatchPaid(ctx context.Context, p *domain.Payment, hooks []*domain.WebhookEndpoint) error {
	eventID := PaidEventID(p.ID)
	body, err := json.Marshal(PaidPayload{
		ID:             p.ID,
		OrderID:        p.OrderID,
		Amount:         p.Amount,
		Status:         string(domain.PaymentPaid),
		EventID:        eventID,
		IdempotencyKey: eventID,
	})
	if err != nil {
		return err
	}

	type dest struct {
		url    string
		secret string
	}
	seen := map[string]struct{}{}
	var dests []dest
	for _, ep := range hooks {
		if ep == nil || ep.URL == "" {
			continue
		}
		if _, ok := seen[ep.URL]; ok {
			continue
		}
		seen[ep.URL] = struct{}{}
		dests = append(dests, dest{url: ep.URL, secret: ep.Secret})
	}
	if p.CallbackURL != "" {
		if _, ok := seen[p.CallbackURL]; !ok {
			secret := ""
			if len(hooks) > 0 {
				secret = hooks[0].Secret
			}
			dests = append(dests, dest{url: p.CallbackURL, secret: secret})
		}
	}

	for _, dst := range dests {
		if err := d.enqueueAndSend(ctx, p.ID, dst.url, eventID, dst.secret, body); err != nil {
			d.log.Warn("webhook dispatch failed", "destination", dst.url, "event_id", eventID, "err", err)
		}
	}
	return nil
}

func (d *Dispatcher) enqueueAndSend(ctx context.Context, paymentID, url, eventID, secret string, body []byte) error {
	if d.store == nil {
		return d.postWithRetries(ctx, url, body, secret, maxSyncAttempts)
	}
	rec, err := d.store.EnsureDelivery(ctx, paymentID, url, eventID, secret, body)
	if err != nil {
		return err
	}
	if rec.Status == domain.WebhookDelivered {
		return nil // idempotent: already delivered
	}
	return d.attempt(ctx, rec, body, secret, true)
}

func (d *Dispatcher) attempt(ctx context.Context, rec *domain.WebhookDelivery, body []byte, secret string, syncBurst bool) error {
	if body == nil {
		body = rec.Payload
	}
	if secret == "" {
		secret = rec.Secret
	}

	attemptsBudget := 1
	if syncBurst {
		attemptsBudget = maxSyncAttempts
	}

	var lastErr error
	for i := 0; i < attemptsBudget; i++ {
		if i > 0 {
			d.sleep(syncBackoff(i))
		}
		lastErr = d.post(ctx, rec.Destination, body, secret, rec.EventID)
		rec.Attempts++
		if lastErr == nil {
			return d.store.MarkDelivered(ctx, rec.ID)
		}
		if errors.Is(lastErr, errWebhookClient) {
			break // permanent client error — stop sync/async retries
		}
	}

	dead := rec.Attempts >= maxTotalAttempts || errors.Is(lastErr, errWebhookClient)
	var next *time.Time
	if !dead {
		t := d.now().UTC().Add(asyncBackoff(rec.Attempts))
		next = &t
	}
	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	_ = d.store.MarkAttempt(ctx, rec.ID, rec.Attempts, errMsg, next, dead)
	return lastErr
}

func syncBackoff(i int) time.Duration {
	// i=1 → 200ms, i=2 → 500ms
	switch i {
	case 1:
		return 200 * time.Millisecond
	default:
		return 500 * time.Millisecond
	}
}

func asyncBackoff(attempts int) time.Duration {
	// 30s, 1m, 2m, 5m, 10m, 30m, 1h…
	switch {
	case attempts <= 1:
		return 30 * time.Second
	case attempts == 2:
		return time.Minute
	case attempts == 3:
		return 2 * time.Minute
	case attempts == 4:
		return 5 * time.Minute
	case attempts == 5:
		return 10 * time.Minute
	case attempts == 6:
		return 30 * time.Minute
	default:
		return time.Hour
	}
}

func (d *Dispatcher) postWithRetries(ctx context.Context, url string, body []byte, secret string, n int) error {
	var last error
	for i := 0; i < n; i++ {
		if i > 0 {
			d.sleep(syncBackoff(i))
		}
		last = d.post(ctx, url, body, secret, "")
		if last == nil {
			return nil
		}
	}
	return last
}

var errWebhookClient = fmt.Errorf("webhook client error")

func (d *Dispatcher) post(ctx context.Context, url string, body []byte, secret, eventID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if eventID != "" {
		req.Header.Set(IdempotencyHeader, eventID)
	}
	if secret != "" {
		req.Header.Set(SignatureHeader, Sign(secret, body))
	}
	res, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("webhook status %d", res.StatusCode)
	}
	return fmt.Errorf("%w: status %d", errWebhookClient, res.StatusCode)
}

// RunRetryLoop processes pending deliveries until ctx is cancelled.
func (d *Dispatcher) RunRetryLoop(ctx context.Context, every time.Duration) {
	if d.store == nil {
		return
	}
	if every <= 0 {
		every = defaultRetryEvery
	}
	t := time.NewTicker(every)
	defer t.Stop()
	d.retryOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.retryOnce(ctx)
		}
	}
}

func (d *Dispatcher) retryOnce(ctx context.Context) {
	list, err := d.store.ListDue(ctx, 50)
	if err != nil {
		d.log.Warn("webhook list due failed", "err", err)
		return
	}
	for _, rec := range list {
		if err := d.attempt(ctx, rec, rec.Payload, rec.Secret, false); err != nil {
			d.log.Warn("webhook retry failed", "destination", rec.Destination, "event_id", rec.EventID, "attempts", rec.Attempts+1, "err", err)
		}
	}
}

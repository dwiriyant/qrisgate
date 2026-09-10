package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/qrisgate/qrisgate/internal/domain"
)

const SignatureHeader = "X-Qrisgate-Signature"

type Dispatcher struct {
	client *http.Client
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{client: &http.Client{Timeout: 8 * time.Second}}
}

func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type PaidPayload struct {
	ID      string `json:"id"`
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
	Status  string `json:"status"`
}

func (d *Dispatcher) DispatchPaid(ctx context.Context, p *domain.Payment, hooks []*domain.WebhookEndpoint) error {
	body, err := json.Marshal(PaidPayload{
		ID:      p.ID,
		OrderID: p.OrderID,
		Amount:  p.Amount,
		Status:  string(domain.PaymentPaid),
	})
	if err != nil {
		return err
	}

	seen := map[string]struct{}{}
	for _, ep := range hooks {
		if ep == nil || ep.URL == "" {
			continue
		}
		seen[ep.URL] = struct{}{}
		_ = d.post(ctx, ep.URL, body, ep.Secret)
	}
	if p.CallbackURL != "" {
		if _, ok := seen[p.CallbackURL]; !ok {
			secret := ""
			if len(hooks) > 0 {
				secret = hooks[0].Secret
			}
			_ = d.post(ctx, p.CallbackURL, body, secret)
		}
	}
	return nil
}

func (d *Dispatcher) post(ctx context.Context, url string, body []byte, secret string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set(SignatureHeader, Sign(secret, body))
	}
	res, err := d.client.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

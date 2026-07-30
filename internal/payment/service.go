package payment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/qrisgate/qrisgate/internal/auth"
	"github.com/qrisgate/qrisgate/internal/domain"
	"github.com/qrisgate/qrisgate/internal/observability"
	"github.com/qrisgate/qrisgate/internal/qris"
)

type AppLookup interface {
	GetByAPIKeyHash(ctx context.Context, hash string) (*domain.App, error)
}

type PaymentStore interface {
	GetByAppAndOrder(ctx context.Context, appID, orderID string) (*domain.Payment, error)
	GetByID(ctx context.Context, id string) (*domain.Payment, error)
	Create(ctx context.Context, p *domain.Payment) error
	InsertEvent(ctx context.Context, paymentID, eventType string, payload []byte) error
}

type Service struct {
	apps        AppLookup
	payments    PaymentStore
	defaultTTL  time.Duration
}

func NewService(apps AppLookup, payments PaymentStore, defaultTTL time.Duration) *Service {
	if defaultTTL <= 0 {
		defaultTTL = 15 * time.Minute
	}
	return &Service{apps: apps, payments: payments, defaultTTL: defaultTTL}
}

type FeeInput struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

type CreateInput struct {
	OrderID     string    `json:"order_id"`
	Amount      int64     `json:"amount"`
	Fee         *FeeInput `json:"fee"`
	CallbackURL string    `json:"callback_url"`
	ExpiresIn   int       `json:"expires_in"`
}

type PaymentResponse struct {
	ID            string     `json:"id"`
	AppID         string     `json:"app_id"`
	OrderID       string     `json:"order_id"`
	Amount        int64      `json:"amount"`
	Status        string     `json:"status"`
	QRISString    string     `json:"qris_string"`
	QRImageBase64 string     `json:"qr_image_base64"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CallbackURL   string     `json:"callback_url,omitempty"`
}

func (s *Service) Create(ctx context.Context, apiKey string, in CreateInput) (*PaymentResponse, error) {
	if apiKey == "" || in.OrderID == "" || in.Amount <= 0 {
		return nil, domain.ErrInvalidInput
	}
	app, err := s.apps.GetByAPIKeyHash(ctx, auth.HashAPIKey(apiKey))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}

	existing, err := s.payments.GetByAppAndOrder(ctx, app.ID, in.OrderID)
	if err == nil {
		return s.toResponse(existing)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	v := qris.Validate(app.MerchantQRIS)
	if !v.Valid {
		return nil, domain.ErrInvalidQRIS
	}

	opts := qris.ConvertOptions{Amount: int(in.Amount)}
	var feeJSON []byte
	if in.Fee != nil && in.Fee.Value > 0 {
		if in.Fee.Type != "fixed" && in.Fee.Type != "percentage" {
			return nil, domain.ErrInvalidInput
		}
		opts.Fee = &qris.Fee{Type: in.Fee.Type, Value: in.Fee.Value}
		feeJSON, _ = json.Marshal(in.Fee)
	}

	ttl := s.defaultTTL
	if in.ExpiresIn > 0 {
		ttl = time.Duration(in.ExpiresIn) * time.Second
	}
	expiresAt := time.Now().UTC().Add(ttl)

	dynamic := qris.Convert(app.MerchantQRIS, opts)
	p := &domain.Payment{
		AppID:       app.ID,
		OrderID:     in.OrderID,
		Amount:      in.Amount,
		QRISString:  dynamic,
		Status:      domain.PaymentPending,
		ExpiresAt:   &expiresAt,
		CallbackURL: in.CallbackURL,
		FeeJSON:     feeJSON,
	}
	if err := s.payments.Create(ctx, p); err != nil {
		return nil, err
	}
	observability.PaymentsCreated.Inc()
	payload, _ := json.Marshal(map[string]any{"amount": in.Amount, "order_id": in.OrderID})
	_ = s.payments.InsertEvent(ctx, p.ID, "payment.created", payload)
	return s.toResponse(p)
}

func (s *Service) Get(ctx context.Context, apiKey, id string) (*PaymentResponse, error) {
	if apiKey == "" {
		return nil, domain.ErrUnauthorized
	}
	app, err := s.apps.GetByAPIKeyHash(ctx, auth.HashAPIKey(apiKey))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	p, err := s.payments.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.AppID != app.ID {
		return nil, domain.ErrNotFound
	}
	return s.toResponse(p)
}

func (s *Service) toResponse(p *domain.Payment) (*PaymentResponse, error) {
	img, err := qris.PNGBase64(p.QRISString, 256)
	if err != nil {
		return nil, err
	}
	return &PaymentResponse{
		ID:            p.ID,
		AppID:         p.AppID,
		OrderID:       p.OrderID,
		Amount:        p.Amount,
		Status:        string(p.Status),
		QRISString:    p.QRISString,
		QRImageBase64: img,
		ExpiresAt:     p.ExpiresAt,
		CallbackURL:   p.CallbackURL,
	}, nil
}

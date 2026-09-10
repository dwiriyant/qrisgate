package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"

	"github.com/qrisgate/qrisgate/internal/auth"
	"github.com/qrisgate/qrisgate/internal/domain"
	"github.com/qrisgate/qrisgate/internal/qris"
)

type AppStore interface {
	Create(ctx context.Context, name, apiKeyHash, merchantQRIS string) (*domain.App, error)
	GetByID(ctx context.Context, id string) (*domain.App, error)
	UpdateMerchantQRIS(ctx context.Context, id, merchantQRIS string) (*domain.App, error)
}

type WebhookStore interface {
	Create(ctx context.Context, appID, webhookURL, secret string) (*domain.WebhookEndpoint, error)
}

type Service struct {
	apps     AppStore
	webhooks WebhookStore
}

func NewService(apps AppStore, webhooks WebhookStore) *Service {
	return &Service{apps: apps, webhooks: webhooks}
}

type CreateAppInput struct {
	Name         string `json:"name"`
	MerchantQRIS string `json:"merchant_qris"`
}

type CreateAppOutput struct {
	App    *domain.App `json:"app"`
	APIKey string      `json:"api_key"`
}

type CreateWebhookInput struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

type CreateWebhookOutput struct {
	Webhook *domain.WebhookEndpoint `json:"webhook"`
	Secret  string                  `json:"secret,omitempty"`
}

func (s *Service) CreateApp(ctx context.Context, in CreateAppInput) (*CreateAppOutput, error) {
	if in.Name == "" || in.MerchantQRIS == "" {
		return nil, domain.ErrInvalidInput
	}
	v := qris.Validate(in.MerchantQRIS)
	if !v.Valid {
		return nil, domain.ErrInvalidQRIS
	}
	raw, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, err
	}
	hash := auth.HashAPIKey(raw)
	app, err := s.apps.Create(ctx, in.Name, hash, in.MerchantQRIS)
	if err != nil {
		return nil, err
	}
	app.APIKeyHash = ""
	return &CreateAppOutput{App: app, APIKey: raw}, nil
}

func (s *Service) CreateWebhook(ctx context.Context, appID string, in CreateWebhookInput) (*CreateWebhookOutput, error) {
	if appID == "" || in.URL == "" {
		return nil, domain.ErrInvalidInput
	}
	u, err := url.ParseRequestURI(in.URL)
	if err != nil || u.Scheme != "https" {
		return nil, domain.ErrInvalidInput
	}
	if _, err := s.apps.GetByID(ctx, appID); err != nil {
		return nil, err
	}
	secret := in.Secret
	var reveal string
	if secret == "" {
		secret, err = generateWebhookSecret()
		if err != nil {
			return nil, err
		}
		reveal = secret
	}
	ep, err := s.webhooks.Create(ctx, appID, in.URL, secret)
	if err != nil {
		return nil, err
	}
	ep.Secret = ""
	return &CreateWebhookOutput{Webhook: ep, Secret: reveal}, nil
}

type UpdateAppInput struct {
	MerchantQRIS string `json:"merchant_qris"`
}

func (s *Service) UpdateApp(ctx context.Context, id string, in UpdateAppInput) (*domain.App, error) {
	if id == "" || in.MerchantQRIS == "" {
		return nil, domain.ErrInvalidInput
	}
	v := qris.Validate(in.MerchantQRIS)
	if !v.Valid {
		return nil, domain.ErrInvalidQRIS
	}
	app, err := s.apps.UpdateMerchantQRIS(ctx, id, in.MerchantQRIS)
	if err != nil {
		return nil, err
	}
	app.APIKeyHash = ""
	return app, nil
}

func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(b), nil
}

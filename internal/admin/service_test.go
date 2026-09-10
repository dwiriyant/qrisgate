package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qrisgate/qrisgate/internal/domain"
	"github.com/qrisgate/qrisgate/internal/qris"
)

type mockAppStore struct {
	apps map[string]*domain.App
}

func (m *mockAppStore) Create(ctx context.Context, name, apiKeyHash, merchantQRIS string) (*domain.App, error) {
	a := &domain.App{ID: "app-new", Name: name, MerchantQRIS: merchantQRIS, CreatedAt: time.Now().UTC()}
	m.apps[a.ID] = a
	return a, nil
}

func (m *mockAppStore) GetByID(ctx context.Context, id string) (*domain.App, error) {
	if a, ok := m.apps[id]; ok {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockAppStore) UpdateMerchantQRIS(ctx context.Context, id, merchantQRIS string) (*domain.App, error) {
	a, err := m.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	a.MerchantQRIS = merchantQRIS
	return a, nil
}

type mockWebhookStore struct {
	created []*domain.WebhookEndpoint
}

func (m *mockWebhookStore) Create(ctx context.Context, appID, url, secret string) (*domain.WebhookEndpoint, error) {
	ep := &domain.WebhookEndpoint{ID: "wh-1", AppID: appID, URL: url, Secret: secret}
	m.created = append(m.created, ep)
	return ep, nil
}

func TestCreateApp(t *testing.T) {
	svc := NewService(&mockAppStore{apps: map[string]*domain.App{}}, &mockWebhookStore{})
	out, err := svc.CreateApp(context.Background(), CreateAppInput{
		Name:         "Demo",
		MerchantQRIS: qris.SampleStaticQRIS(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.APIKey == "" || out.App.Name != "Demo" {
		t.Fatalf("%+v", out)
	}
}

func TestCreateWebhook(t *testing.T) {
	apps := &mockAppStore{apps: map[string]*domain.App{"app-1": {ID: "app-1"}}}
	wh := &mockWebhookStore{}
	svc := NewService(apps, wh)

	out, err := svc.CreateWebhook(context.Background(), "app-1", CreateWebhookInput{
		URL: "https://example.com/webhooks/qris",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Webhook.URL == "" || out.Secret == "" {
		t.Fatalf("expected generated secret %+v", out)
	}
}

func TestCreateWebhook_appNotFound(t *testing.T) {
	svc := NewService(&mockAppStore{apps: map[string]*domain.App{}}, &mockWebhookStore{})
	_, err := svc.CreateWebhook(context.Background(), "missing", CreateWebhookInput{URL: "https://example.com/hook"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreateWebhook_requiresHTTPS(t *testing.T) {
	apps := &mockAppStore{apps: map[string]*domain.App{"app-1": {ID: "app-1"}}}
	svc := NewService(apps, &mockWebhookStore{})
	_, err := svc.CreateWebhook(context.Background(), "app-1", CreateWebhookInput{URL: "http://insecure.example/hook"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreateApp_invalidInput(t *testing.T) {
	svc := NewService(&mockAppStore{apps: map[string]*domain.App{}}, &mockWebhookStore{})
	_, err := svc.CreateApp(context.Background(), CreateAppInput{Name: "", MerchantQRIS: ""})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreateApp_invalidQRIS(t *testing.T) {
	svc := NewService(&mockAppStore{apps: map[string]*domain.App{}}, &mockWebhookStore{})
	_, err := svc.CreateApp(context.Background(), CreateAppInput{Name: "x", MerchantQRIS: "not-qris"})
	if !errors.Is(err, domain.ErrInvalidQRIS) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreateWebhook_customSecret(t *testing.T) {
	apps := &mockAppStore{apps: map[string]*domain.App{"app-1": {ID: "app-1"}}}
	svc := NewService(apps, &mockWebhookStore{})
	out, err := svc.CreateWebhook(context.Background(), "app-1", CreateWebhookInput{
		URL:    "https://example.com/hook",
		Secret: "whsec_custom",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Secret != "" {
		t.Fatal("should not reveal provided secret")
	}
}

func TestUpdateApp(t *testing.T) {
	static := qris.SampleStaticQRIS()
	apps := &mockAppStore{apps: map[string]*domain.App{"app-1": {ID: "app-1", MerchantQRIS: "old"}}}
	svc := NewService(apps, &mockWebhookStore{})
	out, err := svc.UpdateApp(context.Background(), "app-1", UpdateAppInput{MerchantQRIS: static})
	if err != nil {
		t.Fatal(err)
	}
	if out.MerchantQRIS != static {
		t.Fatalf("qris=%s", out.MerchantQRIS)
	}
}

func TestUpdateApp_notFound(t *testing.T) {
	svc := NewService(&mockAppStore{apps: map[string]*domain.App{}}, &mockWebhookStore{})
	_, err := svc.UpdateApp(context.Background(), "missing", UpdateAppInput{MerchantQRIS: qris.SampleStaticQRIS()})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateApp_invalidQRIS(t *testing.T) {
	apps := &mockAppStore{apps: map[string]*domain.App{"app-1": {ID: "app-1"}}}
	svc := NewService(apps, &mockWebhookStore{})
	_, err := svc.UpdateApp(context.Background(), "app-1", UpdateAppInput{MerchantQRIS: "nope"})
	if !errors.Is(err, domain.ErrInvalidQRIS) {
		t.Fatalf("err=%v", err)
	}
}

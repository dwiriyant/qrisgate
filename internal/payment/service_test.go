package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qrisgate/qrisgate/internal/domain"
	"github.com/qrisgate/qrisgate/internal/qris"
)

type mockApp struct {
	app *domain.App
	err error
}

func (m *mockApp) GetByAPIKeyHash(ctx context.Context, hash string) (*domain.App, error) {
	return m.app, m.err
}

type mockPayments struct {
	byOrder map[string]*domain.Payment
	created []*domain.Payment
	claims  map[string]string // provider:external_id -> payment id
}

func (m *mockPayments) GetByAppAndOrder(ctx context.Context, appID, orderID string) (*domain.Payment, error) {
	if p, ok := m.byOrder[appID+":"+orderID]; ok {
		return p, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockPayments) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	for _, p := range m.byOrder {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockPayments) Create(ctx context.Context, p *domain.Payment) error {
	p.ID = "pay-1"
	m.created = append(m.created, p)
	m.byOrder[p.AppID+":"+p.OrderID] = p
	return nil
}

func (m *mockPayments) MarkPaid(ctx context.Context, id string) (*domain.Payment, error) {
	p, err := m.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status != domain.PaymentPending {
		return nil, domain.ErrConflict
	}
	p.Status = domain.PaymentPaid
	return p, nil
}

func (m *mockPayments) InsertEvent(ctx context.Context, paymentID, eventType string, payload []byte) error {
	return nil
}

func (m *mockPayments) ClaimByAmount(ctx context.Context, appID, provider, externalID string, amount int64, paidAt time.Time, lookback time.Duration) (*domain.Payment, bool, error) {
	key := provider + ":" + externalID
	if m.claims == nil {
		m.claims = map[string]string{}
	}
	if id, ok := m.claims[key]; ok {
		p, err := m.GetByID(ctx, id)
		if err != nil {
			return nil, false, err
		}
		if p.AppID != appID {
			return nil, false, domain.ErrConflict
		}
		return p, true, nil
	}
	var best *domain.Payment
	for _, p := range m.byOrder {
		if p.AppID != appID || p.Status != domain.PaymentPending || p.Amount != amount {
			continue
		}
		if best == nil || p.CreatedAt.Before(best.CreatedAt) {
			best = p
		}
	}
	if best == nil {
		return nil, false, domain.ErrNotFound
	}
	best.Status = domain.PaymentPaid
	m.claims[key] = best.ID
	return best, false, nil
}

func (m *mockPayments) ExpirePending(ctx context.Context, now time.Time, limit int) (int64, error) {
	var n int64
	for _, p := range m.byOrder {
		if p.Status != domain.PaymentPending || p.ExpiresAt == nil {
			continue
		}
		if !p.ExpiresAt.After(now) {
			p.Status = domain.PaymentExpired
			n++
			if limit > 0 && n >= int64(limit) {
				break
			}
		}
	}
	return n, nil
}

func TestCreate_happyPath(t *testing.T) {
	static := qris.SampleStaticQRIS()
	app := &domain.App{ID: "app-1", MerchantQRIS: static}
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	svc := NewService(&mockApp{app: app}, store, time.Minute)

	out, err := svc.Create(context.Background(), "qg_test", CreateInput{OrderID: "ORD-1", Amount: 25000})
	if err != nil {
		t.Fatal(err)
	}
	if out.QRISString == "" || out.QRImageBase64 == "" {
		t.Fatal("missing qris output")
	}
	if out.ExpiresAt == nil {
		t.Fatal("expected expires_at")
	}
	if len(store.created) != 1 {
		t.Fatalf("created=%d", len(store.created))
	}
}

func TestCreate_idempotent(t *testing.T) {
	static := qris.SampleStaticQRIS()
	existing := &domain.Payment{
		ID: "existing", AppID: "app-1", OrderID: "ORD-1", Amount: 1000,
		QRISString: static, Status: domain.PaymentPending,
	}
	store := &mockPayments{byOrder: map[string]*domain.Payment{"app-1:ORD-1": existing}}
	svc := NewService(&mockApp{app: &domain.App{ID: "app-1"}}, store, time.Minute)

	out, err := svc.Create(context.Background(), "qg_test", CreateInput{OrderID: "ORD-1", Amount: 99999})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "existing" {
		t.Fatalf("id=%s", out.ID)
	}
	if len(store.created) != 0 {
		t.Fatal("should not create duplicate")
	}
}

func TestCreate_unauthorized(t *testing.T) {
	svc := NewService(
		&mockApp{err: domain.ErrNotFound},
		&mockPayments{byOrder: map[string]*domain.Payment{}},
		time.Minute,
	)
	_, err := svc.Create(context.Background(), "bad", CreateInput{OrderID: "x", Amount: 1})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreate_withFee(t *testing.T) {
	static := qris.SampleStaticQRIS()
	app := &domain.App{ID: "app-1", MerchantQRIS: static}
	svc := NewService(&mockApp{app: app}, &mockPayments{byOrder: map[string]*domain.Payment{}}, time.Minute)

	out, err := svc.Create(context.Background(), "qg_test", CreateInput{
		OrderID: "ORD-2",
		Amount:  50000,
		Fee:     &FeeInput{Type: "fixed", Value: 1000},
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed := qris.Parse(out.QRISString)
	if parsed.TipFixed != "1000" {
		t.Fatalf("tip fixed=%q", parsed.TipFixed)
	}
}

func TestCreate_invalidFeeType(t *testing.T) {
	static := qris.SampleStaticQRIS()
	app := &domain.App{ID: "app-1", MerchantQRIS: static}
	svc := NewService(&mockApp{app: app}, &mockPayments{byOrder: map[string]*domain.Payment{}}, time.Minute)

	_, err := svc.Create(context.Background(), "qg_test", CreateInput{
		OrderID: "ORD-3",
		Amount:  1000,
		Fee:     &FeeInput{Type: "bogus", Value: 1},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreate_invalidQRIS(t *testing.T) {
	app := &domain.App{ID: "app-1", MerchantQRIS: "bad"}
	svc := NewService(&mockApp{app: app}, &mockPayments{byOrder: map[string]*domain.Payment{}}, time.Minute)
	_, err := svc.Create(context.Background(), "qg_test", CreateInput{OrderID: "x", Amount: 100})
	if !errors.Is(err, domain.ErrInvalidQRIS) {
		t.Fatalf("err=%v", err)
	}
}

func TestGet_happyPath(t *testing.T) {
	static := qris.SampleStaticQRIS()
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	store.byOrder["app-1:ORD-1"] = &domain.Payment{
		ID: "pay-1", AppID: "app-1", OrderID: "ORD-1", Amount: 1000,
		QRISString: static, Status: domain.PaymentPending,
	}
	svc := NewService(&mockApp{app: &domain.App{ID: "app-1"}}, store, time.Minute)

	out, err := svc.Get(context.Background(), "qg_test", "pay-1")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "pay-1" {
		t.Fatalf("id=%s", out.ID)
	}
}

func TestGet_unauthorized(t *testing.T) {
	svc := NewService(&mockApp{err: domain.ErrNotFound}, &mockPayments{byOrder: map[string]*domain.Payment{}}, time.Minute)
	_, err := svc.Get(context.Background(), "", "pay-1")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err=%v", err)
	}
}

func TestNewService_defaultTTL(t *testing.T) {
	svc := NewService(&mockApp{}, &mockPayments{byOrder: map[string]*domain.Payment{}}, 0)
	if svc.defaultTTL != 15*time.Minute {
		t.Fatalf("ttl=%v", svc.defaultTTL)
	}
}

type mockHooks struct {
	hooks []*domain.WebhookEndpoint
}

func (m *mockHooks) ListByAppID(ctx context.Context, appID string) ([]*domain.WebhookEndpoint, error) {
	return m.hooks, nil
}

type mockDispatch struct {
	called int
	last   *domain.Payment
}

func (m *mockDispatch) DispatchPaid(ctx context.Context, p *domain.Payment, hooks []*domain.WebhookEndpoint) error {
	m.called++
	m.last = p
	return nil
}

func TestMarkPaid(t *testing.T) {
	static := qris.SampleStaticQRIS()
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	store.byOrder["app-1:ORD-1"] = &domain.Payment{
		ID: "pay-1", AppID: "app-1", OrderID: "ORD-1", Amount: 1000,
		QRISString: static, Status: domain.PaymentPending,
	}
	disp := &mockDispatch{}
	svc := NewService(&mockApp{app: &domain.App{ID: "app-1"}}, store, time.Minute).
		WithPaidNotify(&mockHooks{hooks: []*domain.WebhookEndpoint{{URL: "https://example.com/hook", Secret: "s"}}}, disp)

	out, err := svc.MarkPaid(context.Background(), "pay-1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != string(domain.PaymentPaid) {
		t.Fatalf("status=%s", out.Status)
	}
	if disp.called != 1 {
		t.Fatalf("dispatch=%d", disp.called)
	}
}

func TestMarkPaid_idempotent(t *testing.T) {
	static := qris.SampleStaticQRIS()
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	store.byOrder["app-1:ORD-1"] = &domain.Payment{
		ID: "pay-1", AppID: "app-1", OrderID: "ORD-1", Amount: 1000,
		QRISString: static, Status: domain.PaymentPaid,
	}
	disp := &mockDispatch{}
	svc := NewService(&mockApp{app: &domain.App{ID: "app-1"}}, store, time.Minute).WithPaidNotify(&mockHooks{}, disp)

	out, err := svc.MarkPaid(context.Background(), "pay-1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != string(domain.PaymentPaid) || disp.called != 0 {
		t.Fatalf("status=%s dispatch=%d", out.Status, disp.called)
	}
}

func TestClaim_matchesPendingAndDispatches(t *testing.T) {
	static := qris.SampleStaticQRIS()
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	store.byOrder["app-1:ORD-1"] = &domain.Payment{
		ID: "pay-1", AppID: "app-1", OrderID: "ORD-1", Amount: 15000,
		QRISString: static, Status: domain.PaymentPending, CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	disp := &mockDispatch{}
	svc := NewService(&mockApp{app: &domain.App{ID: "app-1"}}, store, time.Minute).
		WithPaidNotify(&mockHooks{hooks: []*domain.WebhookEndpoint{{URL: "https://example.com/hook"}}}, disp)

	out, err := svc.Claim(context.Background(), ClaimInput{
		AppID: "app-1", Amount: 15000, Provider: "gobiz", ExternalID: "tx-1", PaidAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "pay-1" || out.Status != string(domain.PaymentPaid) {
		t.Fatalf("%+v", out)
	}
	if disp.called != 1 {
		t.Fatalf("dispatch=%d", disp.called)
	}

	out2, err := svc.Claim(context.Background(), ClaimInput{
		AppID: "app-1", Amount: 15000, Provider: "gobiz", ExternalID: "tx-1", PaidAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out2.ID != "pay-1" || disp.called != 1 {
		t.Fatalf("idempotent claim failed: %+v dispatch=%d", out2, disp.called)
	}
}

func TestClaim_notFound(t *testing.T) {
	svc := NewService(&mockApp{app: &domain.App{ID: "app-1"}}, &mockPayments{byOrder: map[string]*domain.Payment{}}, time.Minute)
	_, err := svc.Claim(context.Background(), ClaimInput{AppID: "app-1", Amount: 1000, Provider: "gobiz", ExternalID: "x"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestClaim_invalidInput(t *testing.T) {
	svc := NewService(&mockApp{}, &mockPayments{byOrder: map[string]*domain.Payment{}}, time.Minute)
	_, err := svc.Claim(context.Background(), ClaimInput{Amount: 1000, Provider: "gobiz", ExternalID: "x"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestClaim_scopesToApp(t *testing.T) {
	static := qris.SampleStaticQRIS()
	store := &mockPayments{byOrder: map[string]*domain.Payment{}}
	store.byOrder["app-2:ORD-1"] = &domain.Payment{
		ID: "pay-other", AppID: "app-2", OrderID: "ORD-1", Amount: 15000,
		QRISString: static, Status: domain.PaymentPending, CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	svc := NewService(&mockApp{app: &domain.App{ID: "app-1"}}, store, time.Minute)
	_, err := svc.Claim(context.Background(), ClaimInput{
		AppID: "app-1", Amount: 15000, Provider: "gobiz", ExternalID: "tx-x",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestExpireOnce(t *testing.T) {
	past := time.Now().UTC().Add(-time.Minute)
	future := time.Now().UTC().Add(time.Hour)
	store := &mockPayments{byOrder: map[string]*domain.Payment{
		"app-1:old": {
			ID: "pay-old", AppID: "app-1", OrderID: "old", Amount: 1000,
			Status: domain.PaymentPending, ExpiresAt: &past,
		},
		"app-1:fresh": {
			ID: "pay-fresh", AppID: "app-1", OrderID: "fresh", Amount: 1000,
			Status: domain.PaymentPending, ExpiresAt: &future,
		},
	}}
	svc := NewService(&mockApp{}, store, time.Minute)
	svc.expireOnce(context.Background())
	if store.byOrder["app-1:old"].Status != domain.PaymentExpired {
		t.Fatalf("old status=%s", store.byOrder["app-1:old"].Status)
	}
	if store.byOrder["app-1:fresh"].Status != domain.PaymentPending {
		t.Fatalf("fresh status=%s", store.byOrder["app-1:fresh"].Status)
	}
}

func TestRunExpireLoop_cancels(t *testing.T) {
	svc := NewService(&mockApp{}, &mockPayments{byOrder: map[string]*domain.Payment{}}, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunExpireLoop(ctx, time.Hour)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expire loop did not exit")
	}
}

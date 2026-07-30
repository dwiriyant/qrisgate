package domain

import "time"

type App struct {
	ID            string
	Name          string
	APIKeyHash    string
	MerchantQRIS  string
	CreatedAt     time.Time
}

type WebhookEndpoint struct {
	ID        string
	AppID     string
	URL       string
	Secret    string
	CreatedAt time.Time
}

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentPaid      PaymentStatus = "paid"
	PaymentExpired   PaymentStatus = "expired"
	PaymentFailed    PaymentStatus = "failed"
)

type Payment struct {
	ID           string
	AppID        string
	OrderID      string
	Amount       int64
	QRISString   string
	Status       PaymentStatus
	ExpiresAt    *time.Time
	CallbackURL  string
	FeeJSON      []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PaymentEvent struct {
	ID        string
	PaymentID string
	Type      string
	Payload   []byte
	CreatedAt time.Time
}

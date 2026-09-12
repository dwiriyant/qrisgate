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

type WebhookDeliveryStatus string

const (
	WebhookPending   WebhookDeliveryStatus = "pending"
	WebhookDelivered WebhookDeliveryStatus = "delivered"
	WebhookDead      WebhookDeliveryStatus = "dead"
)

// WebhookDelivery is one outbound webhook attempt track (idempotent per event_id+destination).
type WebhookDelivery struct {
	ID            string
	PaymentID     string
	Destination   string
	EventID       string
	Status        WebhookDeliveryStatus
	Attempts      int
	LastError     string
	NextAttemptAt *time.Time
	DeliveredAt   *time.Time
	Payload       []byte
	Secret        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

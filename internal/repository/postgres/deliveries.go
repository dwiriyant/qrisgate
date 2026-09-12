package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/qrisgate/qrisgate/internal/domain"
)

// EnsureDelivery inserts a pending delivery row, or returns the existing one (idempotent).
func (r *WebhookRepository) EnsureDelivery(ctx context.Context, paymentID, destination, eventID, secret string, payload []byte) (*domain.WebhookDelivery, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (
			id, payment_id, destination, event_id, status, attempts, payload, secret, next_attempt_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,0,$6,$7,$8,$9,$9)
		ON CONFLICT (event_id, destination) DO NOTHING`,
		id, paymentID, destination, eventID, domain.WebhookPending, payload, secret, now, now,
	)
	if err != nil {
		return nil, err
	}
	return r.getDelivery(ctx, eventID, destination)
}

func (r *WebhookRepository) getDelivery(ctx context.Context, eventID, destination string) (*domain.WebhookDelivery, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, payment_id, destination, event_id, status, attempts,
		       COALESCE(last_error, ''), next_attempt_at, delivered_at, payload, secret, created_at, updated_at
		FROM webhook_deliveries
		WHERE event_id = $1 AND destination = $2`, eventID, destination)
	return scanDelivery(row)
}

func (r *WebhookRepository) MarkDelivered(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = $1, delivered_at = $2, updated_at = $2, last_error = NULL, next_attempt_at = NULL
		WHERE id = $3`,
		domain.WebhookDelivered, now, id,
	)
	return err
}

func (r *WebhookRepository) MarkAttempt(ctx context.Context, id string, attempts int, lastErr string, nextAttemptAt *time.Time, dead bool) error {
	status := domain.WebhookPending
	if dead {
		status = domain.WebhookDead
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = $1, attempts = $2, last_error = $3, next_attempt_at = $4, updated_at = $5
		WHERE id = $6`,
		status, attempts, nullIfEmpty(lastErr), nextAttemptAt, now, id,
	)
	return err
}

func (r *WebhookRepository) ListDue(ctx context.Context, limit int) ([]*domain.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().UTC()
	rows, err := r.pool.Query(ctx, `
		SELECT id, payment_id, destination, event_id, status, attempts,
		       COALESCE(last_error, ''), next_attempt_at, delivered_at, payload, secret, created_at, updated_at
		FROM webhook_deliveries
		WHERE status = $1 AND (next_attempt_at IS NULL OR next_attempt_at <= $2)
		ORDER BY next_attempt_at NULLS FIRST
		LIMIT $3`,
		domain.WebhookPending, now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.WebhookDelivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type deliveryScanner interface {
	Scan(dest ...any) error
}

func scanDelivery(row deliveryScanner) (*domain.WebhookDelivery, error) {
	var d domain.WebhookDelivery
	var status string
	err := row.Scan(
		&d.ID, &d.PaymentID, &d.Destination, &d.EventID, &status, &d.Attempts,
		&d.LastError, &d.NextAttemptAt, &d.DeliveredAt, &d.Payload, &d.Secret, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	d.Status = domain.WebhookDeliveryStatus(status)
	return &d, nil
}

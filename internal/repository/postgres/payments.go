package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qrisgate/qrisgate/internal/domain"
)

type PaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

const paymentCols = `id, app_id, order_id, amount, qris_string, status, expires_at, callback_url, fee_json, created_at, updated_at`

func (r *PaymentRepository) GetByAppAndOrder(ctx context.Context, appID, orderID string) (*domain.Payment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+paymentCols+`
		FROM payments WHERE app_id = $1 AND order_id = $2`, appID, orderID)
	return scanPayment(row)
}

func (r *PaymentRepository) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+paymentCols+`
		FROM payments WHERE id = $1`, id)
	return scanPayment(row)
}

func (r *PaymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payments (id, app_id, order_id, amount, qris_string, status, expires_at, callback_url, fee_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.ID, p.AppID, p.OrderID, p.Amount, p.QRISString, p.Status, p.ExpiresAt, nullIfEmpty(p.CallbackURL), p.FeeJSON, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *PaymentRepository) InsertEvent(ctx context.Context, paymentID, eventType string, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payment_events (id, payment_id, type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.NewString(), paymentID, eventType, payload, time.Now().UTC(),
	)
	return err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func scanPayment(row pgx.Row) (*domain.Payment, error) {
	var p domain.Payment
	var callbackURL *string
	if err := row.Scan(&p.ID, &p.AppID, &p.OrderID, &p.Amount, &p.QRISString, &p.Status, &p.ExpiresAt, &callbackURL, &p.FeeJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if callbackURL != nil {
		p.CallbackURL = *callbackURL
	}
	return &p, nil
}

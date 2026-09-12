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

func (r *PaymentRepository) MarkPaid(ctx context.Context, id string) (*domain.Payment, error) {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
		UPDATE payments SET status = $1, updated_at = $2
		WHERE id = $3 AND status = $4`,
		domain.PaymentPaid, now, id, domain.PaymentPending,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		p, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		if p.Status == domain.PaymentPaid {
			return p, nil
		}
		return nil, domain.ErrConflict
	}
	return r.GetByID(ctx, id)
}

func (r *PaymentRepository) InsertEvent(ctx context.Context, paymentID, eventType string, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payment_events (id, payment_id, type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.NewString(), paymentID, eventType, payload, time.Now().UTC(),
	)
	return err
}

// ExpirePending flips pending payments whose expires_at has passed.
// ponytail: batch LIMIT + SKIP LOCKED so multi-replica API is safe enough; no expire webhooks yet.
func (r *PaymentRepository) ExpirePending(ctx context.Context, now time.Time, limit int) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if limit <= 0 {
		limit = 500
	}
	tag, err := r.pool.Exec(ctx, `
		WITH due AS (
			SELECT id FROM payments
			WHERE status = $1
			  AND expires_at IS NOT NULL
			  AND expires_at <= $2
			ORDER BY expires_at ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		UPDATE payments p
		SET status = $4, updated_at = $2
		FROM due
		WHERE p.id = due.id`,
		domain.PaymentPending, now, limit, domain.PaymentExpired,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ClaimByAmount matches the oldest eligible pending payment for appID to a provider pay-in.
// Returns (payment, alreadyClaimed, err). alreadyClaimed means this provider+external_id was settled before.
func (r *PaymentRepository) ClaimByAmount(ctx context.Context, appID, provider, externalID string, amount int64, paidAt time.Time, lookback time.Duration) (*domain.Payment, bool, error) {
	if appID == "" {
		return nil, false, domain.ErrInvalidInput
	}
	if lookback <= 0 {
		lookback = 24 * time.Hour
	}
	if paidAt.IsZero() {
		paidAt = time.Now().UTC()
	} else {
		paidAt = paidAt.UTC()
	}
	createdFrom := paidAt.Add(-lookback)
	createdTo := paidAt.Add(2 * time.Minute) // small clock skew

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var existingPaymentID string
	err = tx.QueryRow(ctx, `
		SELECT payment_id FROM payment_claims
		WHERE provider = $1 AND external_id = $2`, provider, externalID).Scan(&existingPaymentID)
	if err == nil {
		p, getErr := scanPayment(tx.QueryRow(ctx, `SELECT `+paymentCols+` FROM payments WHERE id = $1`, existingPaymentID))
		if getErr != nil {
			return nil, false, getErr
		}
		if p.AppID != appID {
			return nil, false, domain.ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return p, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	now := time.Now().UTC()
	row := tx.QueryRow(ctx, `
		SELECT `+paymentCols+`
		FROM payments
		WHERE app_id = $1
		  AND status = $2
		  AND amount = $3
		  AND (expires_at IS NULL OR expires_at > $4)
		  AND created_at >= $5
		  AND created_at <= $6
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		appID, domain.PaymentPending, amount, now, createdFrom, createdTo,
	)
	p, err := scanPayment(row)
	if err != nil {
		return nil, false, err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE payments SET status = $1, updated_at = $2
		WHERE id = $3 AND status = $4`,
		domain.PaymentPaid, now, p.ID, domain.PaymentPending,
	)
	if err != nil {
		return nil, false, err
	}
	if tag.RowsAffected() == 0 {
		return nil, false, domain.ErrConflict
	}
	p.Status = domain.PaymentPaid
	p.UpdatedAt = now

	_, err = tx.Exec(ctx, `
		INSERT INTO payment_claims (id, payment_id, provider, external_id, amount, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.NewString(), p.ID, provider, externalID, amount, now,
	)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return p, false, nil
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

package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qrisgate/qrisgate/internal/domain"
)

type WebhookRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookRepository(pool *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{pool: pool}
}

func (r *WebhookRepository) Create(ctx context.Context, appID, url, secret string) (*domain.WebhookEndpoint, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_endpoints (id, app_id, url, secret, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		id, appID, url, secret, now,
	)
	if err != nil {
		return nil, err
	}
	return &domain.WebhookEndpoint{
		ID:        id,
		AppID:     appID,
		URL:       url,
		Secret:    secret,
		CreatedAt: now,
	}, nil
}

func (r *WebhookRepository) ListByAppID(ctx context.Context, appID string) ([]*domain.WebhookEndpoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, app_id, url, secret, created_at
		FROM webhook_endpoints WHERE app_id = $1 ORDER BY created_at ASC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.WebhookEndpoint
	for rows.Next() {
		var ep domain.WebhookEndpoint
		if err := rows.Scan(&ep.ID, &ep.AppID, &ep.URL, &ep.Secret, &ep.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &ep)
	}
	return out, rows.Err()
}

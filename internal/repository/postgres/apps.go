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

type AppRepository struct {
	pool *pgxpool.Pool
}

func NewAppRepository(pool *pgxpool.Pool) *AppRepository {
	return &AppRepository{pool: pool}
}

func (r *AppRepository) Create(ctx context.Context, name, apiKeyHash, merchantQRIS string) (*domain.App, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO apps (id, name, api_key_hash, merchant_qris, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		id, name, apiKeyHash, merchantQRIS, now,
	)
	if err != nil {
		return nil, err
	}
	return &domain.App{
		ID:           id,
		Name:         name,
		APIKeyHash:   apiKeyHash,
		MerchantQRIS: merchantQRIS,
		CreatedAt:    now,
	}, nil
}

func (r *AppRepository) GetByAPIKeyHash(ctx context.Context, hash string) (*domain.App, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, api_key_hash, merchant_qris, created_at
		FROM apps WHERE api_key_hash = $1`, hash)
	var a domain.App
	if err := row.Scan(&a.ID, &a.Name, &a.APIKeyHash, &a.MerchantQRIS, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *AppRepository) GetByID(ctx context.Context, id string) (*domain.App, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, api_key_hash, merchant_qris, created_at
		FROM apps WHERE id = $1`, id)
	var a domain.App
	if err := row.Scan(&a.ID, &a.Name, &a.APIKeyHash, &a.MerchantQRIS, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *AppRepository) UpdateMerchantQRIS(ctx context.Context, id, merchantQRIS string) (*domain.App, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE apps SET merchant_qris = $1 WHERE id = $2`, merchantQRIS, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrNotFound
	}
	return r.GetByID(ctx, id)
}

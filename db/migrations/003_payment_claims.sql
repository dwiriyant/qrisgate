-- +goose Up
CREATE TABLE payment_claims (
    id TEXT PRIMARY KEY,
    payment_id TEXT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, external_id)
);

CREATE INDEX idx_payment_claims_payment ON payment_claims (payment_id);

CREATE INDEX idx_payments_pending_amount ON payments (amount, created_at)
    WHERE status = 'pending';

-- +goose Down
DROP INDEX IF EXISTS idx_payments_pending_amount;
DROP INDEX IF EXISTS idx_payment_claims_payment;
DROP TABLE IF EXISTS payment_claims;

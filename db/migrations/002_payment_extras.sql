-- +goose Up
ALTER TABLE payments
    ADD COLUMN expires_at TIMESTAMPTZ,
    ADD COLUMN callback_url TEXT,
    ADD COLUMN fee_json JSONB;

CREATE INDEX idx_payments_status_expires ON payments (status, expires_at)
    WHERE status = 'pending';

-- +goose Down
DROP INDEX IF EXISTS idx_payments_status_expires;
ALTER TABLE payments
    DROP COLUMN IF EXISTS fee_json,
    DROP COLUMN IF EXISTS callback_url,
    DROP COLUMN IF EXISTS expires_at;

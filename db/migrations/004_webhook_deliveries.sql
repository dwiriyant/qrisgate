-- +goose Up
CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    payment_id TEXT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    destination TEXT NOT NULL,
    event_id TEXT NOT NULL,
    status TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    next_attempt_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    payload JSONB NOT NULL DEFAULT '{}',
    secret TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_id, destination)
);

CREATE INDEX idx_webhook_deliveries_retry
    ON webhook_deliveries (status, next_attempt_at)
    WHERE status = 'pending';

-- +goose Down
DROP INDEX IF EXISTS idx_webhook_deliveries_retry;
DROP TABLE IF EXISTS webhook_deliveries;

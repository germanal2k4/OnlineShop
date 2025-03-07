-- +goose Up
CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    recipient_id TEXT NOT NULL,
    storage_deadline TIMESTAMPTZ NOT NULL,

    accepted_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    returned_at TIMESTAMPTZ,
    client_return_at TIMESTAMPTZ,
    last_state_change TIMESTAMPTZ NOT NULL,

    weight DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost DOUBLE PRECISION NOT NULL DEFAULT 0,

    packaging TEXT[] NOT NULL DEFAULT '{}'
);

-- +goose Down
DROP TABLE IF EXISTS orders;

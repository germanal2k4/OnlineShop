-- +goose Up
CREATE TABLE audit_logs
(
    id         SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    data       BYTEA NOT NULL
);

-- +goose Down
DROP TABLE audit_logs;
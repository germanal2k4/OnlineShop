-- +goose Up
CREATE TABLE audit_logs
(
    id        SERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    order_id  TEXT,
    old_state TEXT,
    new_state TEXT,
    endpoint  TEXT,
    request   TEXT,
    response  TEXT,
    message   TEXT
);

-- +goose Down
DROP TABLE audit_logs;
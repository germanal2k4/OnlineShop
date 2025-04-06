-- +goose Up
CREATE TABLE tasks
(
    id              SERIAL PRIMARY KEY,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    audit_data      JSONB       NOT NULL,
    status          TEXT        NOT NULL,
    attempt_count   INTEGER     NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ
);

INSERT INTO tasks (created_at, updated_at, audit_data, status)
SELECT created_at, created_at, to_jsonb(audit_logs), 'CREATED'
FROM audit_logs;

-- +goose Down
DROP TABLE tasks;

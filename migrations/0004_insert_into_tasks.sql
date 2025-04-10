INSERT INTO tasks (created_at, updated_at, audit_data, status)
SELECT created_at, created_at, to_jsonb(audit_logs), 'CREATED'
FROM audit_logs;
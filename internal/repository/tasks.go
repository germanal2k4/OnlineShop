package repository

import (
	"context"
	"database/sql"
	"time"
)

type TaskStatus string

const (
	TaskStatusCreated        TaskStatus = "CREATED"
	TaskStatusProcessing     TaskStatus = "PROCESSING"
	TaskStatusFailed         TaskStatus = "FAILED"
	TaskStatusNoAttemptsLeft TaskStatus = "NO_ATTEMPTS_LEFT"
)

type Task struct {
	ID            int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FinishedAt    sql.NullTime
	AuditData     []byte
	Status        TaskStatus
	AttemptCount  int
	NextAttemptAt sql.NullTime
}

type TaskRepository interface {
	CreateTask(ctx context.Context, auditData []byte) error
	GetPendingTasks(ctx context.Context, limit int) ([]*Task, error)
	MarkTaskProcessing(ctx context.Context, taskID int) error
	DeleteTask(ctx context.Context, taskID int) error
	UpdateTaskFailure(ctx context.Context, taskID int, attemptCount int, newStatus TaskStatus, nextAttemptAt time.Time) error
}

type PostgresTaskRepository struct {
	db *sql.DB
}

func NewPostgresTaskRepository(db *sql.DB) *PostgresTaskRepository {
	return &PostgresTaskRepository{db: db}
}

func (r *PostgresTaskRepository) CreateTask(ctx context.Context, auditData []byte) error {
	query := `
		INSERT INTO tasks (created_at, updated_at, audit_data, status, attempt_count)
		VALUES (NOW(), NOW(), $1, $2, 0)
	`
	_, err := r.db.ExecContext(ctx, query, auditData, TaskStatusCreated)
	return err
}

func (r *PostgresTaskRepository) GetPendingTasks(ctx context.Context, limit int) ([]*Task, error) {
	query := `
		SELECT id, created_at, updated_at, finished_at, audit_data, status, attempt_count, next_attempt_at
		FROM tasks
		WHERE status IN ($1, $2)
		  AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
		  AND attempt_count < 3
		ORDER BY created_at
		LIMIT $3
	`
	rows, err := r.db.QueryContext(ctx, query, TaskStatusCreated, TaskStatusFailed, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(&t.ID, &t.CreatedAt,
			&t.UpdatedAt, &t.FinishedAt,
			&t.AuditData, &t.Status,
			&t.AttemptCount, &t.NextAttemptAt); err != nil {
			return nil, err
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *PostgresTaskRepository) MarkTaskProcessing(ctx context.Context, taskID int) error {
	query := `
		UPDATE tasks SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, TaskStatusProcessing, taskID)
	return err
}

func (r *PostgresTaskRepository) DeleteTask(ctx context.Context, taskID int) error {
	query := `DELETE FROM tasks WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, taskID)
	return err
}

func (r *PostgresTaskRepository) UpdateTaskFailure(ctx context.Context, taskID int, attemptCount int, newStatus TaskStatus, nextAttemptAt time.Time) error {
	query := `
		UPDATE tasks 
		SET status = $1, attempt_count = $2, updated_at = NOW(), next_attempt_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, newStatus, attemptCount, nextAttemptAt, taskID)
	return err
}

package taskprocessor

import (
	"context"
	"log"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/kafka"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
)

type TaskProcessor struct {
	repo         repository.TaskRepository
	producer     kafka.SaramaProducer
	topic        string
	pollInterval time.Duration
	limit        int
	maxAttempts  int
	retryDelay   time.Duration
}

func NewTaskProcessor(repo repository.TaskRepository, producer kafka.SaramaProducer, topic string, pollInterval time.Duration, limit int) *TaskProcessor {
	return &TaskProcessor{
		repo:         repo,
		producer:     producer,
		topic:        topic,
		pollInterval: pollInterval,
		limit:        limit,
		maxAttempts:  3,
		retryDelay:   2 * time.Second,
	}
}

func (p *TaskProcessor) Start(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.processPendingTasks(ctx)
			ticker.Reset(p.pollInterval)
		}
	}
}

func (p *TaskProcessor) processPendingTasks(ctx context.Context) {
	tasks, err := p.repo.GetPendingTasks(ctx, p.limit, p.maxAttempts, p.retryDelay)
	if err != nil {
		log.Printf("Error fetching pending tasks: %v", err)
		return
	}
	for _, task := range tasks {
		err = p.repo.MarkTaskProcessing(ctx, task.ID)
		if err != nil {
			log.Printf("Error marking task %d as PROCESSING: %v", task.ID, err)
			continue
		}

		err = p.producer.Publish(p.topic, task.AuditData)
		if err != nil {
			p.update(ctx, task, err)
			continue
		}
		log.Printf("Task %d processed and published to Kafka", task.ID)
		err = p.repo.DeleteTask(ctx, task.ID)
		if err != nil {
			log.Printf("Error deleting task %d after successful publish: %v", task.ID, err)
		}
	}
}

func (p *TaskProcessor) update(ctx context.Context, task *repository.Task, err error) {
	newAttempt := task.AttemptCount + 1
	var newStatus repository.TaskStatus
	if newAttempt >= p.maxAttempts {
		newStatus = repository.TaskStatusNoAttemptsLeft
	} else {
		newStatus = repository.TaskStatusFailed
	}
	nextAttempt := time.Now().Add(p.retryDelay)
	errUpd := p.repo.UpdateTaskFailure(ctx, task.ID, newAttempt, newStatus, nextAttempt)
	if errUpd != nil {
		log.Printf("Error updating task %d on failure: %v", task.ID, errUpd)
	}
	log.Printf("Failed to publish task %d: %v", task.ID, err)
}

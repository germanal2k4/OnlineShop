package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type AuditLog struct {
	Timestamp time.Time
	OrderID   string
	OldState  string
	NewState  string
	Endpoint  string
	Request   string
	Response  string
	Message   string
}

type AuditLogProcessor interface {
	Process(batch []AuditLog) error
}

type DBProcessor struct {
	Db *sql.DB
}

func (p *DBProcessor) Process(batch []AuditLog) error {
	var sb strings.Builder
	sb.WriteString("INSERT INTO tasks (created_at, updated_at, audit_data, status, attempt_count) VALUES ")

	params := []interface{}{}
	paramIndex := 1
	for i, rec := range batch {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", paramIndex, paramIndex+1, paramIndex+2, paramIndex+3, paramIndex+4))
		paramIndex += 5

		jsonData, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("DBProcessor: error marshalling audit log: %w", err)
		}
		params = append(params, rec.Timestamp, rec.Timestamp, jsonData, "CREATED", 0)
	}
	_, err := p.Db.Exec(sb.String(), params...)
	if err != nil {
		return fmt.Errorf("DBProcessor error: %w", err)
	}
	return nil
}

type StdoutProcessor struct {
	Filter string
}

func (p *StdoutProcessor) Process(batch []AuditLog) error {
	fmt.Println("StdoutProcessor: Writing batch to stdout:")
	for _, rec := range batch {
		if p.Filter != "" && !strings.Contains(strings.ToLower(rec.Message), strings.ToLower(p.Filter)) {
			continue
		}
		fmt.Printf("STDOUT: %s | Order: %s | %s -> %s | Msg: %s\n",
			rec.Timestamp.Format(time.RFC3339), rec.OrderID, rec.OldState, rec.NewState, rec.Message)
	}
	return nil
}

type ProcessorConfig struct {
	Processor   AuditLogProcessor
	BatchSize   int
	Timeout     time.Duration
	ChannelSize int
}

type auditWorker struct {
	ch        chan AuditLog
	processor AuditLogProcessor
	batchSize int
	timeout   time.Duration
	wg        sync.WaitGroup
}

func (w *auditWorker) start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		batch := make([]AuditLog, 0, w.batchSize)
		timer := time.NewTimer(w.timeout)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				if len(batch) > 0 {
					if err := w.processor.Process(batch); err != nil {
						log.Printf("Error processing batch: %v", err)
					}
				}
				return
			case rec := <-w.ch:
				batch = append(batch, rec)
				if len(batch) >= w.batchSize {
					if !timer.Stop() {
						<-timer.C
					}
					if err := w.processor.Process(batch); err != nil {
						log.Printf("Error processing batch: %v", err)
					}
					batch = nil
					timer.Reset(w.timeout)
				}
			case <-timer.C:
				if len(batch) > 0 {
					if err := w.processor.Process(batch); err != nil {
						log.Printf("Error processing batch: %v", err)
					}
					batch = batch[:]
				}
				timer.Reset(w.timeout)
			}
		}
	}()
}

type AuditWorkerPool struct {
	workers []*auditWorker
}

func NewAuditWorkerPool(configs ...ProcessorConfig) *AuditWorkerPool {
	var workers []*auditWorker
	for _, cfg := range configs {
		worker := &auditWorker{
			ch:        make(chan AuditLog, cfg.ChannelSize),
			processor: cfg.Processor,
			batchSize: cfg.BatchSize,
			timeout:   cfg.Timeout,
		}
		workers = append(workers, worker)
	}
	return &AuditWorkerPool{
		workers: workers,
	}
}

func (p *AuditWorkerPool) Start(ctx context.Context) {
	for _, worker := range p.workers {
		worker.start(ctx)
	}
}

func (p *AuditWorkerPool) Log(record AuditLog) {
	go func(workers []*auditWorker) {
		for _, worker := range p.workers {
			worker.ch <- record
		}
	}(p.workers)

}

func (p *AuditWorkerPool) Shutdown() {
	for _, worker := range p.workers {
		worker.wg.Wait()
	}
}

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

type AuditPoolConfig struct {
	BatchSize   int
	Timeout     time.Duration
	ChannelSize int
	Worker      int
}

type AuditLogProcessor interface {
	Process(batch []AuditLog) error
}

type DBProcessor struct {
	Db *sql.DB
}

func (p *DBProcessor) Process(batch []AuditLog) error {
	var sb strings.Builder
	sb.WriteString("INSERT INTO audit_logs (created_at, data) VALUES ")

	params := []interface{}{}
	paramIndex := 1
	for i, rec := range batch {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("($%d, $%d)", paramIndex, paramIndex+1))
		paramIndex += 2

		jsonData, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("DBProcessor: error marshalling audit log: %w", err)
		}
		params = append(params, rec.Timestamp, jsonData)
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
		if p.Filter != "" &&
			!strings.Contains(strings.ToLower(rec.Message), strings.ToLower(p.Filter)) {
			continue
		}
		fmt.Printf("STDOUT: %s | Order: %s | %s -> %s | Msg: %s\n",
			rec.Timestamp.Format(time.RFC3339), rec.OrderID, rec.OldState, rec.NewState, rec.Message)
	}
	return nil
}

type AuditWorkerPool struct {
	inputCh    chan AuditLog
	processors []AuditLogProcessor
	batchSize  int
	timeout    time.Duration
	workers    int

	wg sync.WaitGroup
}

func NewAuditWorkerPool(cfg AuditPoolConfig, processors ...AuditLogProcessor) *AuditWorkerPool {
	return &AuditWorkerPool{
		inputCh:    make(chan AuditLog, cfg.ChannelSize),
		processors: processors,
		batchSize:  cfg.BatchSize,
		timeout:    cfg.Timeout,
		workers:    cfg.Worker,
	}
}

func (p *AuditWorkerPool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func(ctx context.Context) {
			defer p.wg.Done()
			p.worker(ctx)
		}(ctx)
	}
}

func (p *AuditWorkerPool) worker(ctx context.Context) {
	var batch []AuditLog
	timer := time.NewTimer(p.timeout)
	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				p.processBatch(batch)
			}
			return
		case rec := <-p.inputCh:
			batch = append(batch, rec)
			if len(batch) >= p.batchSize {
				if !timer.Stop() {
					<-timer.C
				}
				p.processBatch(batch)
				batch = nil
				timer.Reset(p.timeout)
			}
		case <-timer.C:
			if len(batch) > 0 {
				p.processBatch(batch)
				batch = nil
			}
			timer.Reset(p.timeout)
		}
	}
}

func (p *AuditWorkerPool) processBatch(batch []AuditLog) {
	var wg sync.WaitGroup
	for _, proc := range p.processors {
		wg.Add(1)
		go func(pr AuditLogProcessor) {
			defer wg.Done()
			if err := pr.Process(batch); err != nil {
				log.Printf("Error processing batch: %v", err)
			}
		}(proc)
	}
	wg.Wait()
}

func (p *AuditWorkerPool) Log(record AuditLog) {
	go func(rec AuditLog) {
		p.inputCh <- record
	}(record)
}

func (p *AuditWorkerPool) Shutdown() {
	p.wg.Wait()
}

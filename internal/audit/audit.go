package audit

import (
	"context"
	"database/sql"
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
	db *sql.DB
}

func (p *DBProcessor) Process(batch []AuditLog) error {
	var sb strings.Builder
	sb.WriteString(`INSERT INTO audit_logs (timestamp, order_id, old_state, new_state, endpoint, request, response, message) VALUES `)

	params := []interface{}{}
	paramIndex := 1
	for i, rec := range batch {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", paramIndex, paramIndex+1, paramIndex+2, paramIndex+3, paramIndex+4, paramIndex+5, paramIndex+6, paramIndex+7))
		paramIndex += 8
		params = append(params, rec.Timestamp, rec.OrderID, rec.OldState, rec.NewState, rec.Endpoint, rec.Request, rec.Response, rec.Message)
	}
	_, err := p.db.Exec(sb.String(), params...)
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
		if p.Filter != "" {
			if !strings.Contains(strings.ToLower(rec.Message), strings.ToLower(p.Filter)) {
				continue
			}
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

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func NewAuditWorkerPool(batchSize int, timeout time.Duration, processors ...AuditLogProcessor) *AuditWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &AuditWorkerPool{
		inputCh:    make(chan AuditLog, 100),
		processors: processors,
		batchSize:  batchSize,
		timeout:    timeout,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (p *AuditWorkerPool) Start(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *AuditWorkerPool) worker() {
	defer p.wg.Done()
	var batch []AuditLog
	timer := time.NewTimer(p.timeout)
	for {
		select {
		case <-p.ctx.Done():
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
	for _, proc := range p.processors {
		if err := proc.Process(batch); err != nil {
			log.Printf("Error processing batch: %v", err)
		}
	}
}

func (p *AuditWorkerPool) Log(record AuditLog) {
	select {
	case p.inputCh <- record:
	default:
		log.Println("Audit log channel full, dropping log")
	}
}

func (p *AuditWorkerPool) Shutdown() {
	p.cancel()
	p.wg.Wait()
}

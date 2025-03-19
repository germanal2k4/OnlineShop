package audit

import (
	"context"
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

type DBProcessor struct{}

func (p *DBProcessor) Process(batch []AuditLog) error {
	fmt.Println("DBProcessor: Saving batch to DB:")
	for _, rec := range batch {
		fmt.Printf("DB: %s | Order: %s | %s -> %s\n",
			rec.Timestamp.Format(time.RFC3339), rec.OrderID, rec.OldState, rec.NewState)
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

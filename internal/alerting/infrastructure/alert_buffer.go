package infrastructure

import (
	"context"
	"log"
	"sync"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
)

type AlertRecordBufferInterface interface {
	Start()
	Stop()
	Add(record *models.AlertRecord)
	AddFromAlert(fingerprint string, labels, annotations map[string]string, status models.AlertStatus, startsAt time.Time, endsAt *time.Time, source string)
	Flush()
	GetBufferSize() int
}

type AlertRecordBufferConfig struct {
	BufferSize    int
	FlushInterval time.Duration
}

func DefaultAlertRecordBufferConfig() AlertRecordBufferConfig {
	return AlertRecordBufferConfig{
		BufferSize:    1000,
		FlushInterval: 5 * time.Second,
	}
}

type AlertRecordBuffer struct {
	buffer  []*models.AlertRecord
	mu      sync.Mutex
	config  AlertRecordBufferConfig
	repo    storage.AlertRecordRepositoryInterface
	queue   infra.QueueInterface
	stopCh  chan struct{}
	flushCh chan struct{}
	running bool
}

func NewAlertRecordBuffer(
	repo storage.AlertRecordRepositoryInterface,
	queue infra.QueueInterface,
	config AlertRecordBufferConfig,
) *AlertRecordBuffer {
	return &AlertRecordBuffer{
		buffer:  make([]*models.AlertRecord, 0, config.BufferSize),
		config:  config,
		repo:    repo,
		queue:   queue,
		stopCh:  make(chan struct{}),
		flushCh: make(chan struct{}, 1),
	}
}

func (b *AlertRecordBuffer) Start() {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.mu.Unlock()

	go b.runFlushLoop()
}

func (b *AlertRecordBuffer) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	b.mu.Unlock()

	close(b.stopCh)
	b.Flush()
}

func (b *AlertRecordBuffer) Add(record *models.AlertRecord) {
	b.mu.Lock()
	b.buffer = append(b.buffer, record)
	shouldFlush := len(b.buffer) >= b.config.BufferSize
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushCh <- struct{}{}:
		default:
		}
	}
}

func (b *AlertRecordBuffer) AddFromAlert(
	fingerprint string,
	labels, annotations map[string]string,
	status models.AlertStatus,
	startsAt time.Time,
	endsAt *time.Time,
	source string,
) {
	record := &models.AlertRecord{
		ID:          uuid.New(),
		Fingerprint: fingerprint,
		Labels:      labels,
		Annotations: annotations,
		Status:      status,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		Source:      source,
	}
	record.ExtractSeverity()
	record.CalculateDuration()

	b.Add(record)
}

func (b *AlertRecordBuffer) Flush() {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.buffer
	b.buffer = make([]*models.AlertRecord, 0, b.config.BufferSize)
	b.mu.Unlock()

	if err := b.repo.CreateBatch(context.Background(), batch); err != nil {
		log.Printf("[AlertRecordBuffer] Failed to flush %d records: %v", len(batch), err)
		b.requeueFailed(batch)
	} else {
		log.Printf("[AlertRecordBuffer] Flushed %d records", len(batch))
	}
}

func (b *AlertRecordBuffer) requeueFailed(records []*models.AlertRecord) {
	if b.queue == nil {
		return
	}

	for _, record := range records {
		_, _ = b.queue.Enqueue(context.Background(), "alert:persist:retry", map[string]any{
			"record": record,
		})
	}
}

func (b *AlertRecordBuffer) runFlushLoop() {
	ticker := time.NewTicker(b.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.Flush()
		case <-b.flushCh:
			b.Flush()
		}
	}
}

func (b *AlertRecordBuffer) GetBufferSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buffer)
}

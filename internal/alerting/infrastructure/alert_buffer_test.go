package infrastructure

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDefaultAlertRecordBufferConfig(t *testing.T) {
	config := DefaultAlertRecordBufferConfig()

	assert.Equal(t, 1000, config.BufferSize)
	assert.Equal(t, 5*time.Second, config.FlushInterval)
}

func TestAlertRecordBuffer_Add(t *testing.T) {
	recordRepo := new(storage.MockAlertRecordRepository)
	config := AlertRecordBufferConfig{BufferSize: 10, FlushInterval: 1 * time.Second}
	buffer := NewAlertRecordBuffer(recordRepo, nil, config)

	record := &models.AlertRecord{
		ID:          uuid.New(),
		Fingerprint: "fp123",
		Status:      models.AlertStatusFiring,
	}

	buffer.Add(record)

	assert.Equal(t, 1, buffer.GetBufferSize())
}

func TestAlertRecordBuffer_AddFromAlert(t *testing.T) {
	recordRepo := new(storage.MockAlertRecordRepository)
	config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 1 * time.Second}
	buffer := NewAlertRecordBuffer(recordRepo, nil, config)

	now := time.Now()
	buffer.AddFromAlert("fp123", map[string]string{"alertname": "Test"}, map[string]string{"desc": "test"}, models.AlertStatusFiring, now, nil, "api")

	assert.Equal(t, 1, buffer.GetBufferSize())
}

func TestAlertRecordBuffer_Flush(t *testing.T) {
	t.Run("flush with records", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 1 * time.Second}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		recordRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil)

		buffer.Add(&models.AlertRecord{ID: uuid.New(), Fingerprint: "fp1"})
		buffer.Add(&models.AlertRecord{ID: uuid.New(), Fingerprint: "fp2"})

		buffer.Flush()

		assert.Equal(t, 0, buffer.GetBufferSize())
		recordRepo.AssertExpectations(t)
	})

	t.Run("flush empty buffer", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 1 * time.Second}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		buffer.Flush()

		assert.Equal(t, 0, buffer.GetBufferSize())
	})

	t.Run("flush with error", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 1 * time.Second}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		recordRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(errors.New("db error"))

		buffer.Add(&models.AlertRecord{ID: uuid.New(), Fingerprint: "fp1"})

		buffer.Flush()

		recordRepo.AssertExpectations(t)
	})
}

func TestAlertRecordBuffer_StartStop(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 100 * time.Millisecond}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		buffer.Start()
		time.Sleep(50 * time.Millisecond)
		buffer.Stop()
	})

	t.Run("double start", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 100 * time.Millisecond}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		buffer.Start()
		buffer.Start()
		buffer.Stop()
	})

	t.Run("double stop", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 100 * time.Millisecond}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		buffer.Start()
		buffer.Stop()
		buffer.Stop()
	})

	t.Run("stop without start", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 100 * time.Millisecond}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		buffer.Stop()
	})
}

func TestAlertRecordBuffer_ConcurrentAdd(t *testing.T) {
	recordRepo := new(storage.MockAlertRecordRepository)
	config := AlertRecordBufferConfig{BufferSize: 1000, FlushInterval: 1 * time.Second}
	buffer := NewAlertRecordBuffer(recordRepo, nil, config)

	recordRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			buffer.Add(&models.AlertRecord{
				ID:          uuid.New(),
				Fingerprint: "fp" + string(rune('0'+idx%10)),
			})
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 100, buffer.GetBufferSize())

	buffer.Flush()
	recordRepo.AssertExpectations(t)
}

func TestAlertRecordBuffer_AutoFlush(t *testing.T) {
	recordRepo := new(storage.MockAlertRecordRepository)
	config := AlertRecordBufferConfig{BufferSize: 5, FlushInterval: 50 * time.Millisecond}
	buffer := NewAlertRecordBuffer(recordRepo, nil, config)

	recordRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil)

	buffer.Start()

	for i := 0; i < 5; i++ {
		buffer.Add(&models.AlertRecord{
			ID:          uuid.New(),
			Fingerprint: "fp",
		})
	}

	time.Sleep(100 * time.Millisecond)

	buffer.Stop()

	recordRepo.AssertCalled(t, "CreateBatch", mock.Anything, mock.Anything)
	recordRepo.AssertExpectations(t)
}

func TestAlertRecordBuffer_TimeBasedFlush(t *testing.T) {
	recordRepo := new(storage.MockAlertRecordRepository)
	config := AlertRecordBufferConfig{BufferSize: 1000, FlushInterval: 50 * time.Millisecond}
	buffer := NewAlertRecordBuffer(recordRepo, nil, config)

	recordRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil)

	buffer.Start()

	buffer.Add(&models.AlertRecord{
		ID:          uuid.New(),
		Fingerprint: "fp",
	})

	time.Sleep(100 * time.Millisecond)

	buffer.Stop()

	recordRepo.AssertCalled(t, "CreateBatch", mock.Anything, mock.Anything)
	recordRepo.AssertExpectations(t)
}

func TestAlertRecordBuffer_RequeueFailed(t *testing.T) {
	t.Run("with queue", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		queue := new(MockQueueForBuffer)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 1 * time.Second}
		buffer := NewAlertRecordBuffer(recordRepo, queue, config)

		recordRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(errors.New("db error"))
		queue.On("Enqueue", mock.Anything, "alert:persist:retry", mock.Anything).Return(nil, nil)

		buffer.Add(&models.AlertRecord{ID: uuid.New(), Fingerprint: "fp1"})
		buffer.Flush()

		recordRepo.AssertExpectations(t)
		queue.AssertExpectations(t)
	})

	t.Run("without queue", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 1 * time.Second}
		buffer := NewAlertRecordBuffer(recordRepo, nil, config)

		recordRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(errors.New("db error"))

		buffer.Add(&models.AlertRecord{ID: uuid.New(), Fingerprint: "fp1"})
		buffer.Flush()

		recordRepo.AssertExpectations(t)
	})
}

type MockQueueForBuffer struct {
	mock.Mock
}

func (m *MockQueueForBuffer) Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	args := m.Called(ctx, taskType, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*asynq.TaskInfo), args.Error(1)
}

func (m *MockQueueForBuffer) EnqueueWithDelay(ctx context.Context, taskType string, payload any, delay time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	args := m.Called(ctx, taskType, payload, delay)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*asynq.TaskInfo), args.Error(1)
}

func (m *MockQueueForBuffer) RegisterHandler(taskType string, handler infra.TaskHandler) {
	m.Called(taskType, handler)
}

func (m *MockQueueForBuffer) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockQueueForBuffer) Stop() {
	m.Called()
}

func (m *MockQueueForBuffer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockQueueForBuffer) GetInspector() *asynq.Inspector {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*asynq.Inspector)
}

func TestAlertRecordBuffer_GetBufferSize(t *testing.T) {
	recordRepo := new(storage.MockAlertRecordRepository)
	config := AlertRecordBufferConfig{BufferSize: 100, FlushInterval: 1 * time.Second}
	buffer := NewAlertRecordBuffer(recordRepo, nil, config)

	assert.Equal(t, 0, buffer.GetBufferSize())

	buffer.Add(&models.AlertRecord{ID: uuid.New()})
	assert.Equal(t, 1, buffer.GetBufferSize())

	buffer.Add(&models.AlertRecord{ID: uuid.New()})
	assert.Equal(t, 2, buffer.GetBufferSize())
}

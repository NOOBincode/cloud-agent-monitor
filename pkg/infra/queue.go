package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

type TaskHandler func(ctx context.Context, payload []byte) error

type QueueInterface interface {
	Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) (*asynq.TaskInfo, error)
	EnqueueWithDelay(ctx context.Context, taskType string, payload any, delay time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error)
	RegisterHandler(taskType string, handler TaskHandler)
	Start() error
	Stop()
	Close() error
	GetInspector() *asynq.Inspector
}

type Queue struct {
	client    *asynq.Client
	server    *asynq.Server
	mux       *asynq.ServeMux
	handlers  map[string]TaskHandler
	inspector *asynq.Inspector
}

type QueueConfig struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Concurrency   int
	MaxRetry      int
}

func NewQueue(config QueueConfig) *Queue {
	redisOpt := asynq.RedisClientOpt{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	}

	client := asynq.NewClient(redisOpt)

	maxRetry := config.MaxRetry
	if maxRetry == 0 {
		maxRetry = 3
	}

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: config.Concurrency,
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				if n >= maxRetry {
					return 0
				}
				return time.Duration(n+1) * time.Second
			},
		},
	)

	inspector := asynq.NewInspector(redisOpt)

	return &Queue{
		client:    client,
		server:    server,
		mux:       asynq.NewServeMux(),
		handlers:  make(map[string]TaskHandler),
		inspector: inspector,
	}
}

func (q *Queue) Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(taskType, data, opts...)
	return q.client.EnqueueContext(ctx, task)
}

func (q *Queue) EnqueueWithDelay(ctx context.Context, taskType string, payload any, delay time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	opts = append(opts, asynq.ProcessIn(delay))
	return q.Enqueue(ctx, taskType, payload, opts...)
}

func (q *Queue) RegisterHandler(taskType string, handler TaskHandler) {
	q.mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
		return handler(ctx, t.Payload())
	})
}

func (q *Queue) Start() error {
	return q.server.Start(q.mux)
}

func (q *Queue) Stop() {
	q.server.Shutdown()
	_ = q.client.Close()
}

func (q *Queue) Close() error {
	q.Stop()
	return nil
}

func (q *Queue) GetInspector() *asynq.Inspector {
	return q.inspector
}

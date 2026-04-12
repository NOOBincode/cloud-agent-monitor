package promclient

import (
	"context"
	"fmt"
	"time"

	"cloud-agent-monitor/pkg/config"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Client struct {
	v1api v1.API
}

func NewClient(cfg config.PrometheusConfig) *Client {
	client, err := api.NewClient(api.Config{
		Address: cfg.URL,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create prometheus client: %v", err))
	}

	return &Client{
		v1api: v1.NewAPI(client),
	}
}

type InstantVectorResult struct {
	Metric model.Metric
	Value  float64
	Time   time.Time
}

func (c *Client) Query(ctx context.Context, query string) (model.Value, error) {
	result, _, err := c.v1api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}
	return result, nil
}

func (c *Client) QueryInstantVector(ctx context.Context, query string) ([]InstantVectorResult, error) {
	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("expected vector result, got %T", result)
	}

	var results []InstantVectorResult
	for _, sample := range vector {
		results = append(results, InstantVectorResult{
			Metric: sample.Metric,
			Value:  float64(sample.Value),
			Time:   sample.Timestamp.Time(),
		})
	}

	return results, nil
}

func (c *Client) QueryScalar(ctx context.Context, query string) (float64, error) {
	results, err := c.QueryInstantVector(ctx, query)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("no data returned for query: %s", query)
	}

	return results[0].Value, nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.v1api.Config(ctx)
	if err != nil {
		return fmt.Errorf("prometheus health check failed: %w", err)
	}
	return nil
}

func (c *Client) API() v1.API {
	return c.v1api
}

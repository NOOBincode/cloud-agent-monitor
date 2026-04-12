package infrastructure

import (
	"context"
	"fmt"
	"time"

	"cloud-agent-monitor/internal/promclient"
	"cloud-agent-monitor/internal/slo/domain"

	"github.com/prometheus/common/model"
)

type PrometheusSLICollector struct {
	client *promclient.Client
}

func NewPrometheusSLICollector(client *promclient.Client) *PrometheusSLICollector {
	return &PrometheusSLICollector{client: client}
}

func (c *PrometheusSLICollector) CollectAvailability(ctx context.Context, query string, window string) (float64, error) {
	fullQuery := fmt.Sprintf("avg_over_time(%s[%s]) * 100", query, window)
	return c.queryScalar(ctx, fullQuery)
}

func (c *PrometheusSLICollector) CollectLatency(ctx context.Context, query string, threshold float64, window string) (float64, error) {
	fullQuery := fmt.Sprintf(
		"sum(rate(%s[%s])) - sum(rate(%s{le=\"%.0f\"}[%s])) / sum(rate(%s[%s])) * 100",
		query, window, query, threshold, window, query, window,
	)
	return c.queryScalar(ctx, fullQuery)
}

func (c *PrometheusSLICollector) CollectThroughput(ctx context.Context, query string, threshold float64, window string) (float64, error) {
	fullQuery := fmt.Sprintf("avg_over_time(%s[%s])", query, window)
	value, err := c.queryScalar(ctx, fullQuery)
	if err != nil {
		return 0, err
	}
	if value >= threshold {
		return 100, nil
	}
	return (value / threshold) * 100, nil
}

func (c *PrometheusSLICollector) CollectErrorRate(ctx context.Context, query string, window string) (float64, error) {
	fullQuery := fmt.Sprintf("avg_over_time(%s[%s]) * 100", query, window)
	errorRate, err := c.queryScalar(ctx, fullQuery)
	if err != nil {
		return 0, err
	}
	return 100 - errorRate, nil
}

func (c *PrometheusSLICollector) CollectCustom(ctx context.Context, query string, window string) (float64, error) {
	fullQuery := fmt.Sprintf("avg_over_time(%s[%s])", query, window)
	return c.queryScalar(ctx, fullQuery)
}

func (c *PrometheusSLICollector) CollectBurnRate(ctx context.Context, slo *domain.SLO) (float64, error) {
	shortWindow := c.getShortWindow(slo.Window)
	longWindow := slo.Window

	shortQuery := fmt.Sprintf("(1 - avg_over_time((%s)[%s]))", slo.SLI.Query, shortWindow)
	longQuery := fmt.Sprintf("(1 - avg_over_time((%s)[%s]))", slo.SLI.Query, longWindow)

	shortValue, err := c.queryScalar(ctx, shortQuery)
	if err != nil {
		return 0, fmt.Errorf("short window query failed: %w", err)
	}

	longValue, err := c.queryScalar(ctx, longQuery)
	if err != nil {
		return 0, fmt.Errorf("long window query failed: %w", err)
	}

	if longValue == 0 {
		return 0, nil
	}

	longDuration := c.parseDuration(longWindow)
	shortDuration := c.parseDuration(shortWindow)
	burnRate := (shortValue / longValue) * (float64(longDuration) / float64(shortDuration))

	return burnRate, nil
}

func (c *PrometheusSLICollector) CollectSLI(ctx context.Context, sli *domain.SLI, window string) (float64, error) {
	switch sli.Type {
	case domain.SLITypeAvailability:
		return c.CollectAvailability(ctx, sli.Query, window)
	case domain.SLITypeLatency:
		return c.CollectLatency(ctx, sli.Query, sli.Threshold, window)
	case domain.SLITypeThroughput:
		return c.CollectThroughput(ctx, sli.Query, sli.Threshold, window)
	case domain.SLITypeErrorRate:
		return c.CollectErrorRate(ctx, sli.Query, window)
	case domain.SLITypeCustom:
		return c.CollectCustom(ctx, sli.Query, window)
	default:
		return 0, fmt.Errorf("unknown SLI type: %s", sli.Type)
	}
}

func (c *PrometheusSLICollector) queryScalar(ctx context.Context, query string) (float64, error) {
	result, err := c.client.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("prometheus query failed: %w", err)
	}

	switch r := result.(type) {
	case model.Vector:
		if len(r) == 0 {
			return 0, fmt.Errorf("no data returned for query: %s", query)
		}
		return float64(r[0].Value), nil
	case *model.Scalar:
		return float64(r.Value), nil
	default:
		return 0, fmt.Errorf("unexpected result type: %T", result)
	}
}

func (c *PrometheusSLICollector) getShortWindow(window string) string {
	duration := c.parseDuration(window)
	shortDuration := duration / 6
	if shortDuration < time.Minute {
		shortDuration = time.Minute
	}
	return c.formatDuration(shortDuration)
}

func (c *PrometheusSLICollector) parseDuration(window string) time.Duration {
	d, err := time.ParseDuration(window)
	if err != nil {
		return 30 * 24 * time.Hour
	}
	return d
}

func (c *PrometheusSLICollector) formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 {
		days := hours / 24
		return fmt.Sprintf("%dd", days)
	}
	if hours >= 1 {
		return fmt.Sprintf("%dh", hours)
	}
	minutes := int(d.Minutes())
	return fmt.Sprintf("%dm", minutes)
}

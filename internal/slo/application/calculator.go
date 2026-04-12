package application

import (
	"context"
	"time"

	"cloud-agent-monitor/internal/slo/domain"
)

type SLOCalculator struct {
	collector domain.SLICollectorInterface
}

func NewSLOCalculator(collector domain.SLICollectorInterface) *SLOCalculator {
	return &SLOCalculator{collector: collector}
}

func (c *SLOCalculator) CalculateAvailability(ctx context.Context, successQuery, totalQuery, window string) (float64, error) {
	success, err := c.collector.CollectCustom(ctx, successQuery, window)
	if err != nil {
		return 0, err
	}

	total, err := c.collector.CollectCustom(ctx, totalQuery, window)
	if err != nil {
		return 0, err
	}

	if total == 0 {
		return 100, nil
	}

	return (success / total) * 100, nil
}

func (c *SLOCalculator) CalculateLatencySLO(ctx context.Context, query string, threshold float64, window string) (float64, error) {
	return c.collector.CollectLatency(ctx, query, threshold, window)
}

func (c *SLOCalculator) CalculateErrorBudgetConsumption(slo *domain.SLO, currentValue float64) float64 {
	if slo.Target <= 0 {
		return 0
	}

	budget := 100.0 - slo.Target
	if budget <= 0 {
		return 0
	}

	currentError := 100.0 - currentValue
	targetError := 100.0 - slo.Target

	if targetError == 0 {
		return 0
	}

	return (currentError / targetError) * 100
}

func (c *SLOCalculator) CalculateBurnRate(ctx context.Context, slo *domain.SLO) (float64, error) {
	return c.collector.CollectBurnRate(ctx, slo)
}

func (c *SLOCalculator) EstimateBudgetExhaustion(slo *domain.SLO, burnRate float64) *time.Time {
	if burnRate <= 0 {
		return nil
	}

	budget := slo.ErrorBudget.Remaining
	if budget <= 0 {
		now := time.Now()
		return &now
	}

	windowDuration, err := time.ParseDuration(slo.Window)
	if err != nil {
		windowDuration = 30 * 24 * time.Hour
	}

	hoursRemaining := (budget / 100) * windowDuration.Hours() / burnRate
	exhaustionTime := time.Now().Add(time.Duration(hoursRemaining) * time.Hour)

	return &exhaustionTime
}

func (c *SLOCalculator) GetSLOHealthScore(slos []*domain.SLO) float64 {
	if len(slos) == 0 {
		return 100
	}

	var totalScore float64
	for _, slo := range slos {
		switch slo.Status {
		case domain.SLOStatusHealthy:
			totalScore += 100
		case domain.SLOStatusWarning:
			totalScore += 70
		case domain.SLOStatusCritical:
			totalScore += 30
		default:
			totalScore += 50
		}
	}

	return totalScore / float64(len(slos))
}

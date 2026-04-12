package application

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"cloud-agent-monitor/internal/slo/domain"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
)

var (
	ErrSLONotFound      = errors.New("slo not found")
	ErrSLIInvalid       = errors.New("invalid SLI configuration")
	ErrSLIQueryFailed   = errors.New("SLI query failed")
)

type SLOService struct {
	sloRepo       domain.SLORepositoryInterface
	sliRepo       domain.SLIRepositoryInterface
	collector     domain.SLICollectorInterface
	cache         *infra.Cache
	statusCache   map[uuid.UUID]*sloStatusCache
	cacheMutex    sync.RWMutex
	refreshTicker *time.Ticker
	stopCh        chan struct{}
}

type sloStatusCache struct {
	Status    domain.SLOStatus
	Current   float64
	BurnRate  float64
	UpdatedAt time.Time
}

func NewSLOService(
	sloRepo domain.SLORepositoryInterface,
	sliRepo domain.SLIRepositoryInterface,
	collector domain.SLICollectorInterface,
	cache *infra.Cache,
) *SLOService {
	return &SLOService{
		sloRepo:     sloRepo,
		sliRepo:     sliRepo,
		collector:   collector,
		cache:       cache,
		statusCache: make(map[uuid.UUID]*sloStatusCache),
		stopCh:      make(chan struct{}),
	}
}

func (s *SLOService) CreateSLO(ctx context.Context, slo *domain.SLO, sli *domain.SLI) (*domain.SLO, error) {
	if err := s.sloRepo.Create(ctx, slo); err != nil {
		return nil, err
	}

	sli.SLOID = slo.ID
	if err := s.sliRepo.Create(ctx, sli); err != nil {
		_ = s.sloRepo.Delete(ctx, slo.ID)
		return nil, err
	}

	slo.SLI = *sli
	return slo, nil
}

func (s *SLOService) UpdateSLO(ctx context.Context, slo *domain.SLO) (*domain.SLO, error) {
	existing, err := s.sloRepo.GetByID(ctx, slo.ID)
	if err != nil {
		return nil, err
	}

	existing.Name = slo.Name
	existing.Description = slo.Description
	existing.Target = slo.Target
	existing.Window = slo.Window
	existing.WarningBurn = slo.WarningBurn
	existing.CriticalBurn = slo.CriticalBurn
	existing.UpdatedAt = time.Now()

	if err := s.sloRepo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *SLOService) DeleteSLO(ctx context.Context, id uuid.UUID) error {
	slis, err := s.sliRepo.GetBySLOID(ctx, id)
	if err == nil {
		for _, sli := range slis {
			_ = s.sliRepo.Delete(ctx, sli.ID)
		}
	}

	return s.sloRepo.Delete(ctx, id)
}

func (s *SLOService) GetSLO(ctx context.Context, id uuid.UUID) (*domain.SLO, error) {
	slo, err := s.sloRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.enrichWithCachedStatus(slo)
	return slo, nil
}

func (s *SLOService) GetSLOByService(ctx context.Context, serviceID uuid.UUID) ([]*domain.SLO, error) {
	slos, err := s.sloRepo.GetByServiceID(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	for _, slo := range slos {
		s.enrichWithCachedStatus(slo)
	}

	return slos, nil
}

func (s *SLOService) ListSLOs(ctx context.Context, filter domain.SLOFilter) ([]*domain.SLO, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	slos, total, err := s.sloRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	for _, slo := range slos {
		s.enrichWithCachedStatus(slo)
	}

	return slos, total, nil
}

func (s *SLOService) GetSLOSummary(ctx context.Context) (*domain.SLOSummary, error) {
	return s.sloRepo.GetSummary(ctx)
}

func (s *SLOService) RefreshSLOStatus(ctx context.Context, id uuid.UUID) (*domain.SLO, error) {
	slo, err := s.sloRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	slis, err := s.sliRepo.GetBySLOID(ctx, id)
	if err != nil || len(slis) == 0 {
		slo.Status = domain.SLOStatusUnknown
		return slo, nil
	}

	slo.SLI = *slis[0]

	current, err := s.collector.CollectSLI(ctx, &slo.SLI, slo.Window)
	if err != nil {
		log.Printf("[SLOService] Failed to collect SLI for %s: %v", slo.Name, err)
		slo.Status = domain.SLOStatusUnknown
		return slo, nil
	}
	slo.Current = current

	burnRate, err := s.collector.CollectBurnRate(ctx, slo)
	if err != nil {
		log.Printf("[SLOService] Failed to calculate burn rate for %s: %v", slo.Name, err)
		burnRate = 0
	}
	slo.BurnRate = burnRate

	slo.Status = slo.CalculateStatus()
	slo.ErrorBudget = slo.CalculateErrorBudget()

	s.updateStatusCache(slo.ID, slo.Status, slo.Current, slo.BurnRate)

	return slo, nil
}

func (s *SLOService) RefreshAllSLOStatus(ctx context.Context) error {
	slos, _, err := s.sloRepo.List(ctx, domain.SLOFilter{Page: 1, PageSize: 1000})
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, slo := range slos {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, err := s.RefreshSLOStatus(ctx, id)
			if err != nil {
				log.Printf("[SLOService] Failed to refresh SLO %s: %v", id, err)
			}
		}(slo.ID)
	}

	wg.Wait()
	return nil
}

func (s *SLOService) GetErrorBudget(ctx context.Context, id uuid.UUID) (*domain.ErrorBudget, error) {
	slo, err := s.RefreshSLOStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	budget := slo.CalculateErrorBudget()
	return &budget, nil
}

func (s *SLOService) GetBurnRateAlerts(ctx context.Context) ([]*domain.BurnRateAlert, error) {
	slos, _, err := s.sloRepo.List(ctx, domain.SLOFilter{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}

	var alerts []*domain.BurnRateAlert
	now := time.Now()

	for _, slo := range slos {
		s.enrichWithCachedStatus(slo)

		if slo.BurnRate >= slo.WarningBurn {
			severity := "warning"
			if slo.BurnRate >= slo.CriticalBurn {
				severity = "critical"
			}

			alerts = append(alerts, &domain.BurnRateAlert{
				SLOID:       slo.ID,
				SLOName:     slo.Name,
				ServiceName: slo.ServiceName,
				CurrentRate: slo.BurnRate,
				Threshold:   slo.WarningBurn,
				Severity:    severity,
				Window:      slo.Window,
				FiredAt:     now,
			})
		}
	}

	return alerts, nil
}

func (s *SLOService) StartBackgroundRefresh(interval time.Duration) {
	s.refreshTicker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-s.stopCh:
				return
			case <-s.refreshTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				if err := s.RefreshAllSLOStatus(ctx); err != nil {
					log.Printf("[SLOService] Background refresh failed: %v", err)
				}
				cancel()
			}
		}
	}()
}

func (s *SLOService) Stop() {
	close(s.stopCh)
	if s.refreshTicker != nil {
		s.refreshTicker.Stop()
	}
}

func (s *SLOService) enrichWithCachedStatus(slo *domain.SLO) {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	if cached, ok := s.statusCache[slo.ID]; ok {
		if time.Since(cached.UpdatedAt) < 5*time.Minute {
			slo.Status = cached.Status
			slo.Current = cached.Current
			slo.BurnRate = cached.BurnRate
			slo.ErrorBudget = slo.CalculateErrorBudget()
		}
	}
}

func (s *SLOService) updateStatusCache(id uuid.UUID, status domain.SLOStatus, current, burnRate float64) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.statusCache[id] = &sloStatusCache{
		Status:    status,
		Current:   current,
		BurnRate:  burnRate,
		UpdatedAt: time.Now(),
	}
}

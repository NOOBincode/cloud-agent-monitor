package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
)

var (
	ErrDuplicateOperation = errors.New("duplicate operation")
	ErrOperationConflict  = errors.New("operation conflict")
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

type IdempotencyKey struct {
	UserID      uuid.UUID
	Operation   models.AlertOperationType
	Fingerprint string
}

func (k *IdempotencyKey) String() string {
	h := sha256.New()
	h.Write(k.UserID[:])
	h.Write([]byte(k.Operation))
	h.Write([]byte(k.Fingerprint))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

type OperationManager struct {
	opRepo    storage.AlertOperationRepositoryInterface
	noiseRepo storage.AlertNoiseRepositoryInterface
}

func NewOperationManager(
	opRepo storage.AlertOperationRepositoryInterface,
	noiseRepo storage.AlertNoiseRepositoryInterface,
) *OperationManager {
	return &OperationManager{
		opRepo:    opRepo,
		noiseRepo: noiseRepo,
	}
}

func (m *OperationManager) BeginOperation(
	ctx context.Context,
	key *IdempotencyKey,
	labels models.Labels,
	requestData string,
	ipAddress, userAgent string,
) (*models.AlertOperation, error) {
	recentOps, err := m.opRepo.GetByFingerprint(ctx, key.Fingerprint, 5)
	if err == nil {
		for _, op := range recentOps {
			if op.UserID == key.UserID &&
				op.OperationType == key.Operation &&
				op.Status != models.AlertOpStatusFailed &&
				time.Since(op.CreatedAt) < 5*time.Minute {
				return nil, ErrDuplicateOperation
			}
		}
	}

	tenantID := uuid.Nil
	op := &models.AlertOperation{
		ID:               uuid.New(),
		AlertFingerprint: key.Fingerprint,
		OperationType:    key.Operation,
		Status:           models.AlertOpStatusPending,
		UserID:           key.UserID,
		TenantID:         &tenantID,
		AlertLabels:      labels,
		RequestData:      requestData,
		RetryCount:       0,
		MaxRetries:       3,
		IPAddress:        ipAddress,
		UserAgent:        userAgent,
	}

	if err := m.opRepo.Create(ctx, op); err != nil {
		return nil, err
	}

	return op, nil
}

func (m *OperationManager) CompleteOperation(ctx context.Context, opID uuid.UUID, responseData string) error {
	return m.opRepo.UpdateStatus(ctx, opID, models.AlertOpStatusSuccess, responseData)
}

func (m *OperationManager) FailOperation(ctx context.Context, opID uuid.UUID, errMsg string) error {
	op, err := m.opRepo.GetByID(ctx, opID)
	if err != nil {
		return err
	}

	if op.RetryCount >= op.MaxRetries {
		return m.opRepo.UpdateStatus(ctx, opID, models.AlertOpStatusFailed, errMsg)
	}

	if err := m.opRepo.IncrementRetry(ctx, opID); err != nil {
		return err
	}

	return m.opRepo.UpdateStatus(ctx, opID, models.AlertOpStatusRetrying, errMsg)
}

func (m *OperationManager) GetPendingOperations(ctx context.Context, limit int) ([]*models.AlertOperation, error) {
	return m.opRepo.ListPending(ctx, limit)
}

type RetryPolicy struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:  3,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}

func (p *RetryPolicy) WaitDuration(retryCount int) time.Duration {
	wait := p.InitialWait
	for i := 0; i < retryCount; i++ {
		wait = time.Duration(float64(wait) * p.Multiplier)
		if wait > p.MaxWait {
			wait = p.MaxWait
		}
	}
	return wait
}

type Compensator struct {
	opRepo storage.AlertOperationRepositoryInterface
}

func NewCompensator(opRepo storage.AlertOperationRepositoryInterface) *Compensator {
	return &Compensator{opRepo: opRepo}
}

func (c *Compensator) Compensate(ctx context.Context, op *models.AlertOperation) error {
	switch op.OperationType {
	case models.AlertOpSilence:
		log.Printf("[Compensator] Compensating silence operation %s", op.ID)
		return c.compensateSilence(ctx, op)
	case models.AlertOpUnsilence:
		log.Printf("[Compensator] Compensating unsilence operation %s", op.ID)
		return c.compensateUnsilence(ctx, op)
	default:
		return nil
	}
}

func (c *Compensator) compensateSilence(ctx context.Context, op *models.AlertOperation) error {
	log.Printf("[Compensator] Silence compensation: would delete silence for alert %s", op.AlertFingerprint)
	return nil
}

func (c *Compensator) compensateUnsilence(ctx context.Context, op *models.AlertOperation) error {
	log.Printf("[Compensator] Unsilence compensation: would recreate silence for alert %s", op.AlertFingerprint)
	return nil
}

type ErrorHandler struct {
	opManager   *OperationManager
	compensator *Compensator
	retryPolicy *RetryPolicy
}

func NewErrorHandler(
	opManager *OperationManager,
	compensator *Compensator,
) *ErrorHandler {
	return &ErrorHandler{
		opManager:   opManager,
		compensator: compensator,
		retryPolicy: DefaultRetryPolicy(),
	}
}

func (h *ErrorHandler) HandleError(ctx context.Context, op *models.AlertOperation, err error) error {
	log.Printf("[ErrorHandler] Operation %s failed: %v", op.ID, err)

	if op.RetryCount >= op.MaxRetries {
		if compErr := h.compensator.Compensate(ctx, op); compErr != nil {
			log.Printf("[ErrorHandler] Compensation failed: %v", compErr)
		}
		return h.opManager.FailOperation(ctx, op.ID, err.Error())
	}

	return h.opManager.FailOperation(ctx, op.ID, err.Error())
}

func (h *ErrorHandler) ShouldRetry(op *models.AlertOperation) bool {
	return op.RetryCount < op.MaxRetries && op.Status == models.AlertOpStatusRetrying
}

func (h *ErrorHandler) GetRetryDelay(op *models.AlertOperation) time.Duration {
	return h.retryPolicy.WaitDuration(op.RetryCount)
}

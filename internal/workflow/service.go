package workflow

import (
	"context"
	"strings"
	"sync"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
)

type Clock func() time.Time

type Service struct {
	store *storage.Store
	now   Clock
	id    IDGenerator
	// projectionMu guards projectionCache against concurrent map reads and
	// writes triggered by parallel GET /api/batches/{batchID} requests for the
	// same batch and role.
	projectionMu    sync.RWMutex
	projectionCache map[string]*BatchProjection
}

type Options struct {
	Clock       Clock
	IDGenerator IDGenerator
}

func New(store *storage.Store, options Options) *Service {
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	ids := options.IDGenerator
	if ids == nil {
		ids = RandomID
	}
	return &Service{store: store, now: clock, id: ids, projectionCache: make(map[string]*BatchProjection)}
}

func (s *Service) CreateBatch(ctx context.Context, command CreateBatchCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	batch, err := domain.NewBatch(s.id("batch"), command.Input, now)
	if err != nil {
		return nil, false, err
	}
	batch.AddEvent(domain.TimelineEvent{
		ID: s.id("event"), Type: "batch_created", Actor: batch.OwnerName,
		Message: "创建交接批次", At: now,
		Details: map[string]any{"sourceLab": batch.SourceLab, "plannedHandoverAt": batch.PlannedHandoverAt},
	})
	return s.store.CreateBatch(ctx, strings.TrimSpace(command.RequestID), batch)
}

func (s *Service) GetBatch(ctx context.Context, id string) (*domain.HandoverBatch, error) {
	return s.store.GetBatch(ctx, strings.TrimSpace(id))
}

func (s *Service) ListBatches(ctx context.Context, limit, offset int, due ...domain.DueStatus) ([]domain.HandoverBatch, error) {
	var filter domain.DueStatus
	if len(due) > 0 {
		filter = due[0]
	}
	now := s.now().UTC()
	batches, err := s.store.ListBatches(ctx, limit, offset, filter, now)
	if err != nil {
		return nil, err
	}
	for index := range batches {
		batches[index].DueStatus = domain.CalculateDueStatus(batches[index].PlannedHandoverAt, now)
	}
	return batches, nil
}

func (s *Service) VerifyReceipt(ctx context.Context, batchID string) (*domain.ReceiptVerification, error) {
	batch, receipt, err := s.store.GetBatchAndReceipt(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	if batch.Status != domain.StatusArchived {
		return nil, domain.NewRuleError("not_archived", "只有已归档批次可以核验凭据", "status")
	}
	if receipt == nil {
		return nil, domain.NewRuleError("receipt_missing", "该批次不存在归档凭据", "receipt")
	}
	report, err := batch.VerifyReceipt(*receipt)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

type ReceiptBundle struct {
	Receipt      *domain.ArchiveReceipt      `json:"receipt"`
	Verification *domain.ReceiptVerification `json:"verification"`
}

func (s *Service) ReceiptDownload(ctx context.Context, batchID string) (*ReceiptBundle, error) {
	batch, receipt, err := s.store.GetBatchAndReceipt(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	if batch.Status != domain.StatusArchived {
		return nil, domain.NewRuleError("not_archived", "只有已归档批次可以下载凭据", "status")
	}
	if receipt == nil {
		return nil, domain.NewRuleError("receipt_missing", "该批次不存在归档凭据", "receipt")
	}
	report, err := batch.VerifyReceipt(*receipt)
	if err != nil {
		return nil, err
	}
	return &ReceiptBundle{Receipt: receipt, Verification: &report}, nil
}

func (s *Service) GetReceipt(ctx context.Context, batchID string) (*domain.ArchiveReceipt, error) {
	return s.store.GetReceipt(ctx, strings.TrimSpace(batchID))
}

func normalizeMeta(meta CommandMeta) CommandMeta {
	meta.RequestID = strings.TrimSpace(meta.RequestID)
	meta.Actor = strings.TrimSpace(meta.Actor)
	return meta
}

func validateMeta(meta CommandMeta) error {
	if meta.RequestID == "" {
		return domain.Required("requestId", "requestId")
	}
	if meta.ExpectedVersion < 1 {
		return domain.Invalid("expectedVersion", "expectedVersion 必须大于 0")
	}
	if meta.Role != RoleSafetyOfficer && meta.Role != RoleReviewer {
		return domain.NewRuleError("invalid_role", "角色必须是 safety_officer 或 reviewer", "role")
	}
	if meta.Actor == "" {
		return domain.Required("actor", "操作人")
	}
	if len(meta.Actor) > 80 {
		return domain.Invalid("actor", "操作人不能超过 80 个字符")
	}
	return nil
}

func (s *Service) update(ctx context.Context, batchID, operation string, meta CommandMeta, mutation storage.Mutation) (*domain.HandoverBatch, bool, error) {
	meta = normalizeMeta(meta)
	if err := validateMeta(meta); err != nil {
		return nil, false, err
	}
	return s.store.UpdateBatch(ctx, strings.TrimSpace(batchID), meta.ExpectedVersion, meta.RequestID, operation, mutation)
}

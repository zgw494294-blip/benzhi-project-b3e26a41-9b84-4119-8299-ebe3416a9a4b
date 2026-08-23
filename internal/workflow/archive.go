package workflow

import (
	"context"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

func (s *Service) ConfirmHandover(ctx context.Context, command ConfirmCommand) (*domain.HandoverBatch, bool, error) {
	if command.Meta.Role != RoleSafetyOfficer && command.Meta.Role != RoleReviewer {
		return nil, false, domain.NewRuleError("invalid_role", "角色必须是 safety_officer 或 reviewer", "role")
	}
	now := s.now().UTC()
	operation := "confirm_handover:" + string(command.Meta.Role)
	return s.update(ctx, command.BatchID, operation, command.Meta, func(batch *domain.HandoverBatch) error {
		if err := batch.Confirm(string(command.Meta.Role), command.Name, now); err != nil {
			return err
		}
		if batch.Sender != nil && batch.Receiver != nil {
			_, err := batch.IssueReceipt(s.id("receipt"), now)
			return err
		}
		return nil
	})
}

func (s *Service) Timeline(ctx context.Context, batchID string) ([]domain.TimelineEvent, error) {
	batch, err := s.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return batch.SortedTimeline(), nil
}

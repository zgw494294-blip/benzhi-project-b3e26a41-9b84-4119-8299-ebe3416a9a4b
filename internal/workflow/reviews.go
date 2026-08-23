package workflow

import (
	"context"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

func (s *Service) SubmitReview(ctx context.Context, command SubmitCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.update(ctx, command.BatchID, "submit_review", command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.SubmitReview(command.Meta.Actor, now)
	})
}

func (s *Service) DecideReview(ctx context.Context, command ReviewCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleReviewer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	decisionID := s.id("review")
	return s.update(ctx, command.BatchID, itemOperation("review_item", command.ItemID), command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.Review(command.ItemID, decisionID, command.Input, now)
	})
}

func (s *Service) CompleteReview(ctx context.Context, command SubmitCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleReviewer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.update(ctx, command.BatchID, "complete_review", command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.CompleteReview(command.Meta.Actor, now)
	})
}

func (s *Service) FreezeManifest(ctx context.Context, command SubmitCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleReviewer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.update(ctx, command.BatchID, "freeze_manifest", command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.Freeze(command.Meta.Actor, now)
	})
}

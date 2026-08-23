package workflow

import (
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"context"
)

func (s *Service) Reschedule(ctx context.Context, command RescheduleCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.update(ctx, command.BatchID, "reschedule_batch", command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.Reschedule(command.PlannedHandoverAt, command.Reason, command.Meta.Actor, now)
	})
}

func (s *Service) SetPackagesBulk(ctx context.Context, command PackageBulkCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.update(ctx, command.BatchID, "set_packages_bulk", command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.SetPackages(command.Changes, command.Meta.Actor, s.id("packages"), now)
	})
}

func (s *Service) DecideReviewsBulk(ctx context.Context, command ReviewBulkCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleReviewer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	ids := make([]string, len(command.Changes))
	for index := range ids {
		ids[index] = s.id("review")
	}
	return s.update(ctx, command.BatchID, "review_bulk", command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.ReviewBulk(command.Changes, ids, command.Meta.Actor, s.id("reviews"), now)
	})
}

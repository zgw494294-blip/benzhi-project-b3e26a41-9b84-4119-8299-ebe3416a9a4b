package workflow

import (
	"context"
	"fmt"
	"strings"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

func (s *Service) AddItem(ctx context.Context, command AddItemCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	itemID := s.id("item")
	now := s.now().UTC()
	return s.update(ctx, command.BatchID, "add_item", command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.AddItem(itemID, command.Input, command.Meta.Actor, now)
	})
}

func (s *Service) UpdateItem(ctx context.Context, command UpdateItemCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	operation := "update_item:" + command.ItemID
	return s.update(ctx, command.BatchID, operation, command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.UpdateItem(command.ItemID, command.Input, command.Meta.Actor, now)
	})
}

func (s *Service) SetPackage(ctx context.Context, command PackageCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	operation := "set_package:" + command.ItemID
	return s.update(ctx, command.BatchID, operation, command.Meta, func(batch *domain.HandoverBatch) error {
		return batch.SetPackage(command.ItemID, command.Input, command.Meta.Actor, now)
	})
}

func (s *Service) CorrectItem(ctx context.Context, command CorrectionCommand) (*domain.HandoverBatch, bool, error) {
	if err := requireRole(command.Meta.Role, RoleSafetyOfficer); err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(command.Note) == "" {
		return nil, false, domain.Required("note", "整改说明")
	}
	if len(strings.TrimSpace(command.Note)) > 500 {
		return nil, false, domain.Invalid("note", "整改说明不能超过 500 个字符")
	}
	now := s.now().UTC()
	operation := "correct_item:" + command.ItemID
	return s.update(ctx, command.BatchID, operation, command.Meta, func(batch *domain.HandoverBatch) error {
		if batch.Status != domain.StatusCorrection {
			return domain.NewRuleError("invalid_status", "只有需整改批次可以提交整改", "status")
		}
		if err := batch.UpdateItem(command.ItemID, command.Item, command.Meta.Actor, now); err != nil {
			return err
		}
		if err := batch.SetPackage(command.ItemID, command.Package, command.Meta.Actor, now); err != nil {
			return err
		}
		batch.AddEvent(domain.TimelineEvent{
			ID: s.id("event"), Type: "item_corrected", Actor: command.Meta.Actor,
			Message: "完成退回条目整改", At: now,
			Details: map[string]any{"itemId": command.ItemID, "note": strings.TrimSpace(command.Note)},
		})
		return nil
	})
}

func itemOperation(prefix, itemID string) string {
	return fmt.Sprintf("%s:%s", prefix, itemID)
}

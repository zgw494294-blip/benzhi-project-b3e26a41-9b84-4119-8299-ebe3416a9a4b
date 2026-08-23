package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type PackageMissingItem struct {
	ItemID string   `json:"itemId"`
	Fields []string `json:"fields"`
}

type PackageChecklist struct {
	CompleteCount int                  `json:"completeCount"`
	TotalCount    int                  `json:"totalCount"`
	Missing       []PackageMissingItem `json:"missing"`
}

func (b HandoverBatch) PackageChecklist() PackageChecklist {
	result := PackageChecklist{TotalCount: len(b.Items), Missing: []PackageMissingItem{}}
	for _, item := range b.SortedItems() {
		fields := make([]string, 0, 3)
		if strings.TrimSpace(item.ContainerType) == "" {
			fields = append(fields, "containerType")
		}
		if !item.SealChecked {
			fields = append(fields, "sealChecked")
		}
		if !item.LabelChecked {
			fields = append(fields, "labelChecked")
		}
		if len(fields) == 0 {
			result.CompleteCount++
		} else {
			result.Missing = append(result.Missing, PackageMissingItem{ItemID: item.ID, Fields: fields})
		}
	}
	return result
}

type PackageChange struct {
	ItemID string       `json:"itemId"`
	Input  PackageInput `json:"package"`
}

func (b *HandoverBatch) SetPackages(changes []PackageChange, actor, eventID string, now time.Time) error {
	if err := requireStatus(b.Status, StatusDraft, StatusCorrection); err != nil {
		return err
	}
	if len(changes) == 0 || len(changes) > 100 {
		return Invalid("items", "批量封装核验必须包含一至一百个条目")
	}
	seen := make(map[string]bool, len(changes))
	for index, change := range changes {
		field := fmt.Sprintf("items[%d].itemId", index)
		if strings.TrimSpace(change.ItemID) == "" {
			return Required(field, "条目 ID")
		}
		if seen[change.ItemID] {
			return NewDetailedRuleError("duplicate_item", "批量封装核验包含重复条目", field, map[string]any{"itemId": change.ItemID})
		}
		seen[change.ItemID] = true
		item, err := b.Item(change.ItemID)
		if err != nil {
			return NewDetailedRuleError("item_not_found", err.Error(), field, map[string]any{"itemId": change.ItemID})
		}
		if b.Status == StatusCorrection && item.ReviewStatus != ReviewRejected {
			return NewDetailedRuleError("correction_scope", "整改阶段只能核验本轮被退回条目", field, map[string]any{"itemId": change.ItemID})
		}
		if err := change.Input.Validate(); err != nil {
			if rule, ok := err.(*RuleError); ok {
				return NewDetailedRuleError(rule.Code, rule.Message, fmt.Sprintf("items[%d].%s", index, rule.Field), map[string]any{"itemId": change.ItemID})
			}
			return err
		}
	}
	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		item, _ := b.Item(change.ItemID)
		item.ContainerType = strings.TrimSpace(change.Input.ContainerType)
		item.SealChecked = change.Input.SealChecked
		item.LabelChecked = change.Input.LabelChecked
		item.UpdatedAt = now.UTC()
		ids = append(ids, item.ID)
	}
	sort.Strings(ids)
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: eventID, Type: "packages_checked_bulk", Actor: actor, Message: "批量记录封装核验", At: now.UTC(), Details: map[string]any{"processedCount": len(ids), "itemIds": ids}})
	return nil
}

type ReviewChange struct {
	ItemID string      `json:"itemId"`
	Input  ReviewInput `json:"review"`
}

func (b *HandoverBatch) ReviewBulk(changes []ReviewChange, decisionIDs []string, actor, eventID string, now time.Time) error {
	if err := requireStatus(b.Status, StatusUnderReview); err != nil {
		return err
	}
	if len(changes) == 0 || len(changes) > 100 {
		return Invalid("items", "批量裁决必须包含一至一百个条目")
	}
	if len(decisionIDs) != len(changes) {
		return Invalid("items", "审查决定标识数量不匹配")
	}
	seen := make(map[string]bool, len(changes))
	for index, change := range changes {
		field := fmt.Sprintf("items[%d].itemId", index)
		if seen[change.ItemID] {
			return NewDetailedRuleError("duplicate_item", "批量裁决包含重复条目", field, map[string]any{"itemId": change.ItemID})
		}
		seen[change.ItemID] = true
		item, err := b.Item(change.ItemID)
		if err != nil {
			return NewDetailedRuleError("item_not_found", err.Error(), field, map[string]any{"itemId": change.ItemID})
		}
		if item.ReviewStatus != ReviewPending {
			return NewDetailedRuleError("review_not_pending", "只能裁决当前待审条目", field, map[string]any{"itemId": change.ItemID})
		}
		if err := change.Input.Validate(); err != nil {
			if rule, ok := err.(*RuleError); ok {
				return NewDetailedRuleError(rule.Code, rule.Message, fmt.Sprintf("items[%d].%s", index, rule.Field), map[string]any{"itemId": change.ItemID})
			}
			return err
		}
	}
	itemIDs := make([]string, 0, len(changes))
	for index, change := range changes {
		if err := b.applyReview(change.ItemID, decisionIDs[index], change.Input, now); err != nil {
			return err
		}
		itemIDs = append(itemIDs, change.ItemID)
	}
	sort.Strings(itemIDs)
	b.AddEvent(TimelineEvent{ID: eventID, Type: "reviews_decided_bulk", Actor: actor, Message: "批量记录条目审查结论", At: now.UTC(), Details: map[string]any{"processedCount": len(itemIDs), "itemIds": itemIDs}})
	return nil
}

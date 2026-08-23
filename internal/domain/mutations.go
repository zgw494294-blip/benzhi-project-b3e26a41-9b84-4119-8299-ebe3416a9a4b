package domain

import (
	"fmt"
	"strings"
	"time"
)

func (b *HandoverBatch) AddItem(id string, input ItemInput, actor string, now time.Time) error {
	if err := requireStatus(b.Status, StatusDraft, StatusCorrection); err != nil {
		return err
	}
	if b.Status == StatusCorrection {
		return NewRuleError("correction_scope", "整改阶段不能新增废弃物条目", "status")
	}
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	if len(b.Items) >= 200 {
		return NewRuleError("item_limit", "单个批次最多登记 200 个条目", "items")
	}
	if _, err := b.Item(id); err == nil {
		return NewRuleError("duplicate_item", "废弃物条目 ID 已存在", "id")
	}
	b.Items = append(b.Items, WasteItem{
		ID: id, BatchID: b.ID, MaterialName: input.MaterialName,
		HazardClasses: input.HazardClasses, Quantity: input.Quantity, Unit: input.Unit,
		DisposalCategory: input.DisposalCategory, ReviewStatus: ReviewPending, UpdatedAt: now.UTC(),
	})
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: id + "-created", Type: "item_registered", Actor: actor, Message: "登记废弃物条目", At: now.UTC(), Details: map[string]any{"itemId": id, "materialName": input.MaterialName}})
	return nil
}

func (b *HandoverBatch) UpdateItem(itemID string, input ItemInput, actor string, now time.Time) error {
	if err := requireStatus(b.Status, StatusDraft, StatusCorrection); err != nil {
		return err
	}
	item, err := b.Item(itemID)
	if err != nil {
		return err
	}
	if b.Status == StatusCorrection && item.ReviewStatus != ReviewRejected {
		return NewRuleError("correction_scope", "整改阶段只能修改被退回条目", "itemId")
	}
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return err
	}
	item.MaterialName = input.MaterialName
	item.HazardClasses = input.HazardClasses
	item.Quantity = input.Quantity
	item.Unit = input.Unit
	item.DisposalCategory = input.DisposalCategory
	item.UpdatedAt = now.UTC()
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: fmt.Sprintf("%s-update-%d", itemID, now.UnixNano()), Type: "item_updated", Actor: actor, Message: "更新废弃物条目", At: now.UTC(), Details: map[string]any{"itemId": itemID}})
	return nil
}

func (b *HandoverBatch) SetPackage(itemID string, input PackageInput, actor string, now time.Time) error {
	if err := requireStatus(b.Status, StatusDraft, StatusCorrection); err != nil {
		return err
	}
	if err := input.Validate(); err != nil {
		return err
	}
	item, err := b.Item(itemID)
	if err != nil {
		return err
	}
	if b.Status == StatusCorrection && item.ReviewStatus != ReviewRejected {
		return NewRuleError("correction_scope", "整改阶段只能修改被退回条目的封装信息", "itemId")
	}
	item.ContainerType = strings.TrimSpace(input.ContainerType)
	item.SealChecked = input.SealChecked
	item.LabelChecked = input.LabelChecked
	item.UpdatedAt = now.UTC()
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: fmt.Sprintf("%s-package-%d", itemID, now.UnixNano()), Type: "package_checked", Actor: actor, Message: "记录容器、封口和标签检查", At: now.UTC(), Details: map[string]any{"itemId": itemID, "complete": item.PackageComplete()}})
	return nil
}

func (b *HandoverBatch) SubmitReview(actor string, now time.Time) error {
	if err := requireStatus(b.Status, StatusDraft, StatusCorrection); err != nil {
		return err
	}
	if len(b.Items) == 0 {
		return NewRuleError("empty_batch", "至少登记一个废弃物条目后才能提交审查", "items")
	}
	if !b.AllPackagesComplete() {
		return NewDetailedRuleError("package_incomplete", "所有条目必须完成容器、封口和标签核验", "items", b.PackageChecklist())
	}
	preflight := b.CompatibilityPreflight()
	if preflight.BlockingCount > 0 {
		return NewDetailedRuleError("compatibility_blocked", "相容性预检存在阻断项，不能提交审查", "items", preflight)
	}
	for index := range b.Items {
		if b.Status == StatusDraft || b.Items[index].ReviewStatus == ReviewRejected {
			b.Items[index].ReviewStatus = ReviewPending
			b.Items[index].CorrectionNote = ""
		}
	}
	b.Status = StatusUnderReview
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: fmt.Sprintf("review-submit-%d", now.UnixNano()), Type: "review_submitted", Actor: actor, Message: "提交相容性与接收条件审查", At: now.UTC(), Details: map[string]any{"ruleVersion": preflight.RuleVersion, "warningCount": preflight.WarningCount, "summary": preflight.Summary}})
	return nil
}

func (b *HandoverBatch) Review(itemID, decisionID string, input ReviewInput, now time.Time) error {
	if err := requireStatus(b.Status, StatusUnderReview); err != nil {
		return err
	}
	if err := input.Validate(); err != nil {
		return err
	}
	return b.applyReview(itemID, decisionID, input, now)
}

func (b *HandoverBatch) applyReview(itemID, decisionID string, input ReviewInput, now time.Time) error {
	item, err := b.Item(itemID)
	if err != nil {
		return err
	}
	previous := latestDecision(b.Reviews, itemID)
	supersedes := ""
	if previous != nil {
		supersedes = previous.ID
	}
	review := ReviewDecision{
		ID: decisionID, BatchID: b.ID, ItemID: itemID, Decision: input.Decision,
		ReasonCode: strings.TrimSpace(input.ReasonCode), Comment: strings.TrimSpace(input.Comment),
		ReviewerName: strings.TrimSpace(input.ReviewerName), DecidedAt: now.UTC(), SupersedesID: supersedes,
	}
	b.Reviews = append(b.Reviews, review)
	if input.Decision == DecisionApprove {
		item.ReviewStatus = ReviewApproved
		item.CorrectionNote = ""
	} else {
		item.ReviewStatus = ReviewRejected
		item.CorrectionNote = review.Comment
	}
	item.UpdatedAt = now.UTC()
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: decisionID, Type: "review_decided", Actor: review.ReviewerName, Message: "记录条目审查结论", At: now.UTC(), Details: map[string]any{"itemId": itemID, "decision": input.Decision, "reasonCode": review.ReasonCode}})
	return nil
}

func (b *HandoverBatch) CompleteReview(actor string, now time.Time) error {
	if err := requireStatus(b.Status, StatusUnderReview); err != nil {
		return err
	}
	for _, item := range b.Items {
		if item.ReviewStatus == ReviewPending {
			return NewRuleError("review_incomplete", "仍有条目尚未给出审查结论", "items")
		}
	}
	if b.RejectedCount() > 0 {
		b.Status = StatusCorrection
		b.AddEvent(TimelineEvent{ID: fmt.Sprintf("review-return-%d", now.UnixNano()), Type: "correction_required", Actor: actor, Message: "审查完成，批次退回整改", At: now.UTC(), Details: map[string]any{"rejectedCount": b.RejectedCount()}})
	} else {
		b.AddEvent(TimelineEvent{ID: fmt.Sprintf("review-pass-%d", now.UnixNano()), Type: "review_approved", Actor: actor, Message: "全部条目审查通过", At: now.UTC()})
	}
	b.UpdatedAt = now.UTC()
	return nil
}

package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type HandoverBatch struct {
	ID                string           `json:"id"`
	SourceLab         string           `json:"sourceLab"`
	OwnerName         string           `json:"ownerName"`
	PlannedHandoverAt time.Time        `json:"plannedHandoverAt"`
	Status            BatchStatus      `json:"status"`
	Version           int64            `json:"version"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
	Items             []WasteItem      `json:"items"`
	Reviews           []ReviewDecision `json:"reviews"`
	Timeline          []TimelineEvent  `json:"timeline"`
	ManifestDigest    string           `json:"manifestDigest,omitempty"`
	Sender            *Confirmation    `json:"senderConfirmation,omitempty"`
	Receiver          *Confirmation    `json:"receiverConfirmation,omitempty"`
	Receipt           *ArchiveReceipt  `json:"receipt,omitempty"`
	DueStatus         DueStatus        `json:"dueStatus,omitempty"`
}

type BatchInput struct {
	SourceLab         string    `json:"sourceLab"`
	OwnerName         string    `json:"ownerName"`
	PlannedHandoverAt time.Time `json:"plannedHandoverAt"`
}

func (in BatchInput) Normalize() BatchInput {
	in.SourceLab = strings.TrimSpace(in.SourceLab)
	in.OwnerName = strings.TrimSpace(in.OwnerName)
	return in
}

func (in BatchInput) Validate(now time.Time) error {
	in = in.Normalize()
	if in.SourceLab == "" {
		return Required("sourceLab", "来源实验室")
	}
	if len(in.SourceLab) > 120 {
		return Invalid("sourceLab", "来源实验室不能超过 120 个字符")
	}
	if in.OwnerName == "" {
		return Required("ownerName", "责任人")
	}
	if len(in.OwnerName) > 80 {
		return Invalid("ownerName", "责任人不能超过 80 个字符")
	}
	if in.PlannedHandoverAt.IsZero() {
		return Required("plannedHandoverAt", "计划交接日期")
	}
	if in.PlannedHandoverAt.Before(now.Add(-24 * time.Hour)) {
		return Invalid("plannedHandoverAt", "计划交接日期不能早于当前日期")
	}
	return nil
}

func NewBatch(id string, in BatchInput, now time.Time) (*HandoverBatch, error) {
	in = in.Normalize()
	if err := in.Validate(now); err != nil {
		return nil, err
	}
	batch := &HandoverBatch{
		ID: id, SourceLab: in.SourceLab, OwnerName: in.OwnerName,
		PlannedHandoverAt: in.PlannedHandoverAt, Status: StatusDraft,
		Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		Items: []WasteItem{}, Reviews: []ReviewDecision{}, Timeline: []TimelineEvent{},
	}
	return batch, nil
}

func (b *HandoverBatch) Validate() error {
	if b.ID == "" {
		return Required("id", "批次 ID")
	}
	if !b.Status.Valid() {
		return Invalid("status", "批次状态无效")
	}
	if b.Version < 1 {
		return Invalid("version", "批次版本必须大于 0")
	}
	seen := make(map[string]bool)
	for index := range b.Items {
		item := b.Items[index]
		if item.ID == "" || item.BatchID != b.ID {
			return Invalid("items", "条目身份或所属批次无效")
		}
		if seen[item.ID] {
			return Invalid("items", "条目 ID 重复")
		}
		seen[item.ID] = true
		if err := (ItemInput{MaterialName: item.MaterialName, HazardClasses: item.HazardClasses, Quantity: item.Quantity, Unit: item.Unit, DisposalCategory: item.DisposalCategory}).Validate(); err != nil {
			return err
		}
	}
	if b.Status == StatusReady || b.Status == StatusArchived {
		if b.ManifestDigest == "" {
			return Invalid("manifestDigest", "待交接或归档批次必须包含冻结清单摘要")
		}
	}
	if b.Status == StatusArchived && (b.Receipt == nil || !b.Receipt.Complete()) {
		return Invalid("receipt", "已归档批次必须包含完整凭据")
	}
	return nil
}

func (b *HandoverBatch) Item(id string) (*WasteItem, error) {
	for index := range b.Items {
		if b.Items[index].ID == id {
			return &b.Items[index], nil
		}
	}
	return nil, &NotFoundError{Resource: "废弃物条目", ID: id}
}

func (b HandoverBatch) SortedItems() []WasteItem {
	items := append([]WasteItem(nil), b.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (b HandoverBatch) AllPackagesComplete() bool {
	if len(b.Items) == 0 {
		return false
	}
	for _, item := range b.Items {
		if !item.PackageComplete() {
			return false
		}
	}
	return true
}

func (b HandoverBatch) AllReviewsApproved() bool {
	if len(b.Items) == 0 {
		return false
	}
	for _, item := range b.Items {
		if item.ReviewStatus != ReviewApproved {
			return false
		}
	}
	return true
}

func (b HandoverBatch) RejectedCount() int {
	count := 0
	for _, item := range b.Items {
		if item.ReviewStatus == ReviewRejected {
			count++
		}
	}
	return count
}

func (b HandoverBatch) Summary() string {
	return fmt.Sprintf("%s / %s / %d 项 / %s", b.SourceLab, b.OwnerName, len(b.Items), b.Status)
}

package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type manifestRecord struct {
	BatchID           string      `json:"batchId"`
	SourceLab         string      `json:"sourceLab"`
	OwnerName         string      `json:"ownerName"`
	PlannedHandoverAt string      `json:"plannedHandoverAt"`
	Items             []WasteItem `json:"items"`
}

func hashJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("序列化摘要数据: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalHazards(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	write := 0
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values[write] = value
		write++
	}
	values = values[:write]
	sort.Strings(values)
	return values
}

func (b HandoverBatch) CalculateManifestDigest() (string, error) {
	items := b.SortedItems()
	for index := range items {
		// 清单摘要按危险属性的规范顺序计算，避免同一语义产生不同摘要。
		items[index].HazardClasses = canonicalHazards(items[index].HazardClasses)
		items[index].UpdatedAt = time.Time{}
		items[index].CorrectionNote = ""
	}
	return hashJSON(manifestRecord{
		BatchID: b.ID, SourceLab: b.SourceLab, OwnerName: b.OwnerName,
		PlannedHandoverAt: b.PlannedHandoverAt.UTC().Format(time.RFC3339Nano), Items: items,
	})
}

func (b HandoverBatch) CalculateTimelineDigest() (string, error) {
	return hashJSON(b.SortedTimeline())
}

func (b *HandoverBatch) Freeze(actor string, now time.Time) error {
	if err := requireStatus(b.Status, StatusUnderReview); err != nil {
		return err
	}
	if !b.AllReviewsApproved() {
		return NewRuleError("review_not_approved", "全部条目通过审查后才能冻结交接清单", "items")
	}
	digest, err := b.CalculateManifestDigest()
	if err != nil {
		return err
	}
	b.ManifestDigest = digest
	b.Status = StatusReady
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: fmt.Sprintf("manifest-freeze-%d", now.UnixNano()), Type: "manifest_frozen", Actor: actor, Message: "冻结交接清单", At: now.UTC(), Details: map[string]any{"manifestDigest": digest}})
	return nil
}

func (b *HandoverBatch) Confirm(role, name string, now time.Time) error {
	if err := requireStatus(b.Status, StatusReady); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Required("name", "确认人姓名")
	}
	if len(name) > 80 {
		return Invalid("name", "确认人姓名不能超过 80 个字符")
	}
	confirmation := &Confirmation{Name: name, ConfirmedAt: now.UTC()}
	switch role {
	case "safety_officer":
		if b.Sender != nil {
			return NewRuleError("already_confirmed", "安全员已完成现场确认", "role")
		}
		b.Sender = confirmation
	case "reviewer":
		if b.Receiver != nil {
			return NewRuleError("already_confirmed", "接收复核员已完成现场确认", "role")
		}
		b.Receiver = confirmation
	default:
		return NewRuleError("invalid_role", "现场确认角色必须是 safety_officer 或 reviewer", "role")
	}
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: fmt.Sprintf("confirm-%s-%d", role, now.UnixNano()), Type: "handover_confirmed", Actor: name, Message: "确认现场交接", At: now.UTC(), Details: map[string]any{"role": role}})
	return nil
}

func (b *HandoverBatch) IssueReceipt(receiptID string, now time.Time) (*ArchiveReceipt, error) {
	if err := requireStatus(b.Status, StatusReady); err != nil {
		return nil, err
	}
	if b.Sender == nil || b.Receiver == nil {
		return nil, NewRuleError("confirmation_incomplete", "双方完成现场确认后才能签发归档凭据", "confirmations")
	}
	if b.ManifestDigest == "" {
		return nil, NewRuleError("manifest_missing", "冻结清单摘要不存在", "manifestDigest")
	}
	b.Status = StatusArchived
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{ID: receiptID, Type: "receipt_issued", Actor: "system", Message: "签发不可变归档凭据", At: now.UTC()})
	timelineDigest, err := b.CalculateTimelineDigest()
	if err != nil {
		return nil, err
	}
	receipt := &ArchiveReceipt{
		ID: receiptID, BatchID: b.ID, ManifestDigest: b.ManifestDigest,
		TimelineDigest: timelineDigest, SenderConfirmation: *b.Sender,
		ReceiverConfirmation: *b.Receiver, IssuedAt: now.UTC(), Timeline: b.SortedTimeline(),
	}
	b.Receipt = receipt
	return receipt, nil
}

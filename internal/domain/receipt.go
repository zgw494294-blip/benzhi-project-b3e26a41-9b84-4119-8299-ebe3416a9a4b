package domain

import (
	"crypto/subtle"
	"time"
)

type Confirmation struct {
	Name        string    `json:"name"`
	ConfirmedAt time.Time `json:"confirmedAt"`
}

type ArchiveReceipt struct {
	ID                   string          `json:"id"`
	BatchID              string          `json:"batchId"`
	ManifestDigest       string          `json:"manifestDigest"`
	TimelineDigest       string          `json:"timelineDigest"`
	SenderConfirmation   Confirmation    `json:"senderConfirmation"`
	ReceiverConfirmation Confirmation    `json:"receiverConfirmation"`
	IssuedAt             time.Time       `json:"issuedAt"`
	Timeline             []TimelineEvent `json:"timeline"`
}

func (r ArchiveReceipt) Complete() bool {
	return r.ID != "" && r.BatchID != "" && r.ManifestDigest != "" && r.TimelineDigest != "" &&
		r.SenderConfirmation.Name != "" && r.ReceiverConfirmation.Name != "" && !r.IssuedAt.IsZero()
}

type ReceiptCheck struct {
	Code    string `json:"code"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type ReceiptVerification struct {
	BatchID   string         `json:"batchId"`
	ReceiptID string         `json:"receiptId"`
	Passed    bool           `json:"passed"`
	Checks    []ReceiptCheck `json:"checks"`
}

func (b HandoverBatch) VerifyReceipt(receipt ArchiveReceipt) (ReceiptVerification, error) {
	if b.Status != StatusArchived {
		return ReceiptVerification{}, NewRuleError("not_archived", "只有已归档批次可以核验凭据", "status")
	}
	if !receipt.Complete() {
		return ReceiptVerification{}, NewRuleError("receipt_incomplete", "归档凭据缺失或结构不完整", "receipt")
	}
	if receipt.BatchID != b.ID {
		return ReceiptVerification{}, NewRuleError("receipt_batch_mismatch", "归档凭据所属批次不一致", "batchId")
	}
	manifest, err := b.CalculateManifestDigest()
	if err != nil {
		return ReceiptVerification{}, err
	}
	timeline, err := b.CalculateTimelineDigest()
	if err != nil {
		return ReceiptVerification{}, err
	}
	receiptTimeline, err := hashJSON(receipt.Timeline)
	if err != nil {
		return ReceiptVerification{}, err
	}
	checks := []ReceiptCheck{
		{Code: "manifest_digest_match", Passed: subtle.ConstantTimeCompare([]byte(manifest), []byte(receipt.ManifestDigest)) == 1, Message: "冻结清单摘要一致"},
		{Code: "timeline_digest_match", Passed: subtle.ConstantTimeCompare([]byte(timeline), []byte(receipt.TimelineDigest)) == 1, Message: "时间线摘要一致"},
		{Code: "receipt_timeline_match", Passed: subtle.ConstantTimeCompare([]byte(receiptTimeline), []byte(receipt.TimelineDigest)) == 1, Message: "凭据内时间线摘要一致"},
		{Code: "receipt_identity_match", Passed: receipt.BatchID == b.ID && receipt.ID != "", Message: "凭据身份与批次一致"},
		{Code: "issued_event_match", Passed: receipt.IssuedAt.Equal(lastIssuedAt(b.Timeline)) && lastIssuedID(b.Timeline) == receipt.ID, Message: "签发时间与最后签发事件一致"},
		{Code: "confirmation_match", Passed: receipt.SenderConfirmation == valueOrZero(b.Sender) && receipt.ReceiverConfirmation == valueOrZero(b.Receiver), Message: "双方确认信息一致"},
	}
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
		}
	}
	return ReceiptVerification{BatchID: b.ID, ReceiptID: receipt.ID, Passed: passed, Checks: checks}, nil
}

func valueOrZero(value *Confirmation) Confirmation {
	if value == nil {
		return Confirmation{}
	}
	return *value
}

func lastIssuedAt(events []TimelineEvent) time.Time {
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Type == "receipt_issued" {
			return events[index].At
		}
	}
	return time.Time{}
}

func lastIssuedID(events []TimelineEvent) string {
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Type == "receipt_issued" {
			return events[index].ID
		}
	}
	return ""
}

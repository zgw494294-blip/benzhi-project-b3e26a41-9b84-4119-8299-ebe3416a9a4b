package domain

import "fmt"

type BatchStatus string

const (
	StatusDraft       BatchStatus = "draft"
	StatusUnderReview BatchStatus = "under_review"
	StatusCorrection  BatchStatus = "correction_required"
	StatusReady       BatchStatus = "ready_for_handover"
	StatusArchived    BatchStatus = "archived"
)

func (s BatchStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusUnderReview, StatusCorrection, StatusReady, StatusArchived:
		return true
	default:
		return false
	}
}

func (s BatchStatus) Editable() bool {
	return s == StatusDraft || s == StatusCorrection
}

func (s BatchStatus) String() string { return string(s) }

func requireStatus(actual BatchStatus, allowed ...BatchStatus) error {
	for _, candidate := range allowed {
		if actual == candidate {
			return nil
		}
	}
	return NewRuleError("invalid_status", fmt.Sprintf("当前状态 %s 不允许执行该操作", actual), "status")
}

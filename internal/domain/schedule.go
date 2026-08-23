package domain

import (
	"fmt"
	"strings"
	"time"
)

type DueStatus string

const (
	DueNormal  DueStatus = "normal"
	DueSoon    DueStatus = "due_soon"
	DueToday   DueStatus = "due_today"
	DueOverdue DueStatus = "overdue"
)

func (s DueStatus) Valid() bool {
	switch s {
	case DueNormal, DueSoon, DueToday, DueOverdue:
		return true
	default:
		return false
	}
}

func dayStart(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func CalculateDueStatus(planned, now time.Time) DueStatus {
	today := dayStart(now)
	due := dayStart(planned.In(now.Location()))
	days := int(due.Sub(today) / (24 * time.Hour))
	switch {
	case days < 0:
		return DueOverdue
	case days == 0:
		return DueToday
	case days <= 3:
		return DueSoon
	default:
		return DueNormal
	}
}

func (b *HandoverBatch) Reschedule(planned time.Time, reason, actor string, now time.Time) error {
	if err := requireStatus(b.Status, StatusDraft, StatusCorrection); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < 1 || len([]rune(reason)) > 300 {
		return Invalid("reason", "改期原因必须为一至三百字")
	}
	if planned.IsZero() {
		return Required("plannedHandoverAt", "新计划交接日期")
	}
	if dayStart(planned).Before(dayStart(now)) {
		return Invalid("plannedHandoverAt", "新计划交接日期不能早于当前日期")
	}
	planned = planned.UTC()
	if dayStart(planned).Equal(dayStart(b.PlannedHandoverAt.In(planned.Location()))) {
		return NewRuleError("schedule_unchanged", "新计划交接日期不能与原计划相同", "plannedHandoverAt")
	}
	original := b.PlannedHandoverAt
	b.PlannedHandoverAt = planned
	b.UpdatedAt = now.UTC()
	b.AddEvent(TimelineEvent{
		ID: fmt.Sprintf("batch-rescheduled-%d", now.UnixNano()), Type: "batch_rescheduled", Actor: actor,
		Message: "调整计划交接日期", At: now.UTC(), Details: map[string]any{
			"originalPlannedHandoverAt": original, "newPlannedHandoverAt": planned, "reason": reason,
		},
	})
	return nil
}

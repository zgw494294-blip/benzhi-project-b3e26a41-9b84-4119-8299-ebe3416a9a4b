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

// calendarDay returns the midnight UTC instant of the calendar day that value
// falls on in its own timezone. Comparing two such values yields the whole-day
// difference between calendar days regardless of each value's offset.
func calendarDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// CalculateDueStatus keeps the natural-day semantics of the submitted planned
// handover time: the planned calendar day is taken in the time's own offset,
// while the current calendar day is taken in now's timezone. This avoids
// misclassifying a batch whose submitted local date is a future day but whose
// UTC instant still falls on the current UTC day.
func CalculateDueStatus(planned, now time.Time) DueStatus {
	today := calendarDay(now)
	due := calendarDay(planned)
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
	if calendarDay(planned).Before(calendarDay(now)) {
		return Invalid("plannedHandoverAt", "新计划交接日期不能早于当前日期")
	}
	if calendarDay(planned).Equal(calendarDay(b.PlannedHandoverAt)) {
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

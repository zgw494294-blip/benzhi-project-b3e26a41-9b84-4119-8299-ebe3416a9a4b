package domain

import (
	"sort"
	"time"
)

type TimelineEvent struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Actor    string         `json:"actor"`
	Message  string         `json:"message"`
	At       time.Time      `json:"at"`
	Details  map[string]any `json:"details,omitempty"`
	Sequence int64          `json:"sequence"`
}

func (b *HandoverBatch) AddEvent(event TimelineEvent) {
	var max int64
	for _, current := range b.Timeline {
		if current.Sequence > max {
			max = current.Sequence
		}
	}
	event.Sequence = max + 1
	b.Timeline = append(b.Timeline, event)
}

func (b HandoverBatch) SortedTimeline() []TimelineEvent {
	result := append([]TimelineEvent(nil), b.Timeline...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Sequence != result[j].Sequence {
			return result[i].Sequence < result[j].Sequence
		}
		return result[i].ID < result[j].ID
	})
	return result
}

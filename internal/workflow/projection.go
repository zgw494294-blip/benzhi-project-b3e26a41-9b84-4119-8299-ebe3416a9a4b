package workflow

import (
	"context"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

type BatchProjection struct {
	Batch             *domain.HandoverBatch      `json:"batch"`
	AllowedActions    []string                   `json:"allowedActions"`
	PackageReady      bool                       `json:"packageReady"`
	ReviewReady       bool                       `json:"reviewReady"`
	RejectedCount     int                        `json:"rejectedCount"`
	ReceiptReady      bool                       `json:"receiptReady"`
	DueStatus         domain.DueStatus           `json:"dueStatus"`
	Compatibility     domain.CompatibilityReport `json:"compatibility"`
	PackageChecklist  domain.PackageChecklist    `json:"packageChecklist"`
	ReviewReasonCodes []string                   `json:"reviewReasonCodes"`
}

func (s *Service) Projection(ctx context.Context, batchID string, role Role) (*BatchProjection, error) {
	cacheKey := batchID + ":" + string(role)
	s.projectionMu.Lock()
	// Mark the entry while the current version is being loaded.
	if _, ok := s.projectionCache[cacheKey]; !ok {
		s.projectionCache[cacheKey] = nil
	}
	s.projectionMu.Unlock()
	batch, err := s.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	s.projectionMu.RLock()
	cached, ok := s.projectionCache[cacheKey]
	s.projectionMu.RUnlock()
	if ok && cached != nil && cached.Batch.Version == batch.Version {
		return cached, nil
	}
	projection := &BatchProjection{
		Batch: batch, PackageReady: batch.AllPackagesComplete(),
		ReviewReady: batch.AllReviewsApproved(), RejectedCount: batch.RejectedCount(),
		ReceiptReady:      batch.Receipt != nil,
		DueStatus:         domain.CalculateDueStatus(batch.PlannedHandoverAt, s.now().UTC()),
		Compatibility:     batch.CompatibilityPreflight(),
		PackageChecklist:  batch.PackageChecklist(),
		ReviewReasonCodes: append([]string(nil), domain.ReviewReasonCodes...),
	}
	projection.AllowedActions = allowedActions(batch, role)
	s.projectionMu.Lock()
	s.projectionCache[cacheKey] = projection
	s.projectionMu.Unlock()
	return projection, nil
}

func allowedActions(batch *domain.HandoverBatch, role Role) []string {
	actions := make([]string, 0)
	switch batch.Status {
	case domain.StatusDraft:
		if role == RoleSafetyOfficer {
			actions = append(actions, "add_item", "update_item", "set_package")
			if batch.AllPackagesComplete() && batch.CompatibilityPreflight().BlockingCount == 0 {
				actions = append(actions, "submit_review")
			}
		}
	case domain.StatusUnderReview:
		if role == RoleReviewer {
			actions = append(actions, "review_item")
			allDecided := true
			for _, item := range batch.Items {
				if item.ReviewStatus == domain.ReviewPending {
					allDecided = false
				}
			}
			if allDecided {
				actions = append(actions, "complete_review")
			}
			if batch.AllReviewsApproved() {
				actions = append(actions, "freeze_manifest")
			}
		}
	case domain.StatusCorrection:
		if role == RoleSafetyOfficer {
			actions = append(actions, "correct_item")
			if batch.AllPackagesComplete() && batch.CompatibilityPreflight().BlockingCount == 0 {
				actions = append(actions, "submit_review")
			}
		}
	case domain.StatusReady:
		if role == RoleSafetyOfficer && batch.Sender == nil {
			actions = append(actions, "confirm_handover")
		}
		if role == RoleReviewer && batch.Receiver == nil {
			actions = append(actions, "confirm_handover")
		}
	case domain.StatusArchived:
		actions = append(actions, "view_receipt")
	}
	return actions
}

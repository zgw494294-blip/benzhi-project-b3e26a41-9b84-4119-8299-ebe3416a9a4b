package workflow

import (
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"time"
)

type CommandMeta struct {
	RequestID       string
	ExpectedVersion int64
	Role            Role
	Actor           string
}

type CreateBatchCommand struct {
	RequestID string
	Role      Role
	Input     domain.BatchInput
}

type AddItemCommand struct {
	BatchID string
	Meta    CommandMeta
	Input   domain.ItemInput
}

type UpdateItemCommand struct {
	BatchID string
	ItemID  string
	Meta    CommandMeta
	Input   domain.ItemInput
}

type PackageCommand struct {
	BatchID string
	ItemID  string
	Meta    CommandMeta
	Input   domain.PackageInput
}

type SubmitCommand struct {
	BatchID string
	Meta    CommandMeta
}

type ReviewCommand struct {
	BatchID string
	ItemID  string
	Meta    CommandMeta
	Input   domain.ReviewInput
}

type CorrectionCommand struct {
	BatchID string
	ItemID  string
	Meta    CommandMeta
	Item    domain.ItemInput
	Package domain.PackageInput
	Note    string
}

type ConfirmCommand struct {
	BatchID string
	Meta    CommandMeta
	Name    string
}

type RescheduleCommand struct {
	BatchID           string
	Meta              CommandMeta
	PlannedHandoverAt time.Time
	Reason            string
}

type PackageBulkCommand struct {
	BatchID string
	Meta    CommandMeta
	Changes []domain.PackageChange
}

type ReviewBulkCommand struct {
	BatchID string
	Meta    CommandMeta
	Changes []domain.ReviewChange
}

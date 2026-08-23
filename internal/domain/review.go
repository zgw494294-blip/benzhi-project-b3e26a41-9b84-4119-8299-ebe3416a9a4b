package domain

import (
	"strings"
	"time"
)

const (
	ReasonCompatibility = "COMPATIBILITY_CONFLICT"
	ReasonPackage       = "PACKAGE_DEFECT"
	ReasonLabel         = "LABEL_MISMATCH"
	ReasonQuantity      = "QUANTITY_QUESTION"
	ReasonAcceptance    = "ACCEPTANCE_CONDITION"
)

var ReviewReasonCodes = []string{ReasonCompatibility, ReasonPackage, ReasonLabel, ReasonQuantity, ReasonAcceptance, "LABEL_DETAIL", "LABEL", "INCOMPATIBLE"}

func validReasonCode(value string) bool {
	for _, code := range ReviewReasonCodes {
		if value == code {
			return true
		}
	}
	return false
}

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReject  Decision = "reject"
)

type ReviewDecision struct {
	ID           string    `json:"id"`
	BatchID      string    `json:"batchId"`
	ItemID       string    `json:"itemId"`
	Decision     Decision  `json:"decision"`
	ReasonCode   string    `json:"reasonCode,omitempty"`
	Comment      string    `json:"comment,omitempty"`
	ReviewerName string    `json:"reviewerName"`
	DecidedAt    time.Time `json:"decidedAt"`
	SupersedesID string    `json:"supersedesId,omitempty"`
}

type ReviewInput struct {
	Decision     Decision `json:"decision"`
	ReasonCode   string   `json:"reasonCode"`
	Comment      string   `json:"comment"`
	ReviewerName string   `json:"reviewerName"`
}

func (in ReviewInput) Validate() error {
	in.ReasonCode = strings.TrimSpace(in.ReasonCode)
	in.Comment = strings.TrimSpace(in.Comment)
	if in.Decision != DecisionApprove && in.Decision != DecisionReject {
		return Invalid("decision", "审查结论必须是 approve 或 reject")
	}
	if strings.TrimSpace(in.ReviewerName) == "" {
		return Required("reviewerName", "复核员姓名")
	}
	if len(strings.TrimSpace(in.ReviewerName)) > 80 {
		return Invalid("reviewerName", "复核员姓名不能超过 80 个字符")
	}
	if in.Decision == DecisionReject {
		if in.ReasonCode == "" {
			return Required("reasonCode", "退回原因代码")
		}
		if !validReasonCode(in.ReasonCode) {
			return Invalid("reasonCode", "退回原因代码不在内置原因列表中")
		}
	} else if in.ReasonCode != "" {
		return NewRuleError("contradictory_review", "通过结论不能携带退回原因代码", "decision")
	}
	if len(strings.TrimSpace(in.Comment)) > 500 {
		return Invalid("comment", "审查说明不能超过 500 个字符")
	}
	return nil
}

func latestDecision(history []ReviewDecision, itemID string) *ReviewDecision {
	for index := len(history) - 1; index >= 0; index-- {
		if history[index].ItemID == itemID {
			copy := history[index]
			return &copy
		}
	}
	return nil
}

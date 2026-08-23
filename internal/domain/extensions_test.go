package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCompatibilityBlocksSubmitAndPackageBulkIsAtomic(t *testing.T) {
	now := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	batch, err := NewBatch("batch-bulk", BatchInput{SourceLab: "实验室", OwnerName: "安全员", PlannedHandoverAt: now.Add(48 * time.Hour)}, now)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []ItemInput{
		{MaterialName: "易燃废液", HazardClasses: []string{"flammable"}, Quantity: 1, Unit: "L", DisposalCategory: "liquid"},
		{MaterialName: "氧化性废液", HazardClasses: []string{"oxidizing"}, Quantity: 1, Unit: "L", DisposalCategory: "liquid"},
	}
	for index, input := range inputs {
		if err := batch.AddItem("item-"+itoa(index+1), input, "安全员", now); err != nil {
			t.Fatal(err)
		}
	}
	beforeTimeline := len(batch.Timeline)
	err = batch.SetPackages([]PackageChange{
		{ItemID: "item-1", Input: PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true}},
		{ItemID: "missing", Input: PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true}},
	}, "安全员", "bulk-invalid", now)
	var rule *RuleError
	if !errors.As(err, &rule) || rule.Code != "item_not_found" {
		t.Fatalf("expected item error, got %v", err)
	}
	if batch.Items[0].ContainerType != "" || len(batch.Timeline) != beforeTimeline {
		t.Fatal("invalid bulk package mutated aggregate")
	}
	if err := batch.SetPackages([]PackageChange{
		{ItemID: "item-1", Input: PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true}},
		{ItemID: "item-2", Input: PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true}},
	}, "安全员", "bulk-valid", now); err != nil {
		t.Fatal(err)
	}
	report := batch.CompatibilityPreflight()
	if report.BlockingCount != 1 || report.Findings[0].RuleCode != "HAZARD_OXIDIZER_FLAMMABLE" {
		t.Fatalf("unexpected report: %#v", report)
	}
	err = batch.SubmitReview("安全员", now)
	if !errors.As(err, &rule) || rule.Code != "compatibility_blocked" || batch.Status != StatusDraft {
		t.Fatalf("expected compatibility block, got %v status=%s", err, batch.Status)
	}
}

func TestRescheduleAndReceiptVerification(t *testing.T) {
	now := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	batch := testBatch(t)
	original := batch.PlannedHandoverAt
	if err := batch.Reschedule(original.Add(12*time.Hour), "同一天", "安全员", now); err == nil {
		t.Fatal("expected same-day reschedule rejection")
	}
	if !batch.PlannedHandoverAt.Equal(original) {
		t.Fatal("failed reschedule changed date")
	}

	if err := batch.SubmitReview("安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Review("item-test", "review-ok", ReviewInput{Decision: DecisionApprove, ReviewerName: "复核员"}, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.CompleteReview("复核员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Freeze("复核员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Confirm("safety_officer", "安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Confirm("reviewer", "复核员", now); err != nil {
		t.Fatal(err)
	}
	receipt, err := batch.IssueReceipt("receipt-verify", now)
	if err != nil {
		t.Fatal(err)
	}
	version := batch.Version
	timelineCount := len(batch.Timeline)
	verification, err := batch.VerifyReceipt(*receipt)
	if err != nil || !verification.Passed {
		t.Fatalf("verification failed: %#v %v", verification, err)
	}
	if batch.Version != version || len(batch.Timeline) != timelineCount {
		t.Fatal("verification mutated batch")
	}
	tampered := *receipt
	tampered.ManifestDigest = "bad"
	verification, err = batch.VerifyReceipt(tampered)
	if err != nil || verification.Passed || verification.Checks[0].Passed {
		t.Fatalf("tampering not detected: %#v %v", verification, err)
	}
}

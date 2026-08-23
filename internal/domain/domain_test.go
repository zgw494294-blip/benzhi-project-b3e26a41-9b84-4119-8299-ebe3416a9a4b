package domain

import (
	"testing"
	"time"
)

func testBatch(t *testing.T) *HandoverBatch {
	t.Helper()
	now := time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC)
	batch, err := NewBatch("batch-test", BatchInput{SourceLab: "实验室 A", OwnerName: "安全员", PlannedHandoverAt: now.Add(24 * time.Hour)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.AddItem("item-test", ItemInput{MaterialName: "乙腈废液", HazardClasses: []string{"flammable"}, Quantity: 2, Unit: "L", DisposalCategory: "liquid"}, "安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.SetPackage("item-test", PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true}, "安全员", now); err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestBatchLifecycleWithCorrection(t *testing.T) {
	now := time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC)
	batch := testBatch(t)
	if err := batch.SubmitReview("安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Review("item-test", "review-1", ReviewInput{Decision: DecisionReject, ReasonCode: "LABEL", Comment: "标签缺少危害说明", ReviewerName: "复核员"}, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.CompleteReview("复核员", now); err != nil {
		t.Fatal(err)
	}
	if batch.Status != StatusCorrection || batch.RejectedCount() != 1 {
		t.Fatalf("expected correction, got %s rejected=%d", batch.Status, batch.RejectedCount())
	}
	if err := batch.UpdateItem("item-test", ItemInput{MaterialName: "乙腈废液", HazardClasses: []string{"flammable", "toxic"}, Quantity: 2, Unit: "L", DisposalCategory: "liquid"}, "安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.SetPackage("item-test", PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true}, "安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.SubmitReview("安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Review("item-test", "review-2", ReviewInput{Decision: DecisionApprove, ReviewerName: "复核员"}, now); err != nil {
		t.Fatal(err)
	}
	if err := batch.CompleteReview("复核员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Freeze("复核员", now); err != nil {
		t.Fatal(err)
	}
	if batch.ManifestDigest == "" || batch.Status != StatusReady {
		t.Fatal("manifest was not frozen")
	}
	if err := batch.UpdateItem("item-test", ItemInput{MaterialName: "禁止修改", HazardClasses: []string{"toxic"}, Quantity: 1, Unit: "L", DisposalCategory: "liquid"}, "安全员", now); err == nil {
		t.Fatal("expected frozen batch to reject item edits")
	}
	if err := batch.Confirm("safety_officer", "安全员", now); err != nil {
		t.Fatal(err)
	}
	if err := batch.Confirm("reviewer", "复核员", now); err != nil {
		t.Fatal(err)
	}
	receipt, err := batch.IssueReceipt("receipt-test", now)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != StatusArchived || !receipt.Complete() || len(receipt.Timeline) < 1 {
		t.Fatalf("invalid receipt: %#v", receipt)
	}
}

func TestInputValidationAndDigestStability(t *testing.T) {
	invalid := ItemInput{MaterialName: "", HazardClasses: []string{"unknown"}, Quantity: -1, Unit: "kg", DisposalCategory: "liquid"}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected invalid item error")
	}
	first := testBatch(t)
	digestOne, err := first.CalculateManifestDigest()
	if err != nil {
		t.Fatal(err)
	}
	second := testBatch(t)
	digestTwo, err := second.CalculateManifestDigest()
	if err != nil {
		t.Fatal(err)
	}
	if digestOne != digestTwo {
		t.Fatalf("digest changed for equivalent manifests: %s != %s", digestOne, digestTwo)
	}
}

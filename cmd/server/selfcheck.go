package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func runSelfCheck(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	directory, err := os.MkdirTemp("", "benzhi-handover-selfcheck-")
	if err != nil {
		return fmt.Errorf("创建自检临时目录: %w", err)
	}
	defer os.RemoveAll(directory)
	store, err := storage.Open(ctx, storage.Options{Path: filepath.Join(directory, "selfcheck.db")})
	if err != nil {
		return err
	}
	defer store.Close()
	service := workflow.New(store, workflow.Options{})
	now := time.Now().UTC()
	batch, _, err := service.CreateBatch(ctx, workflow.CreateBatchCommand{
		RequestID: "selfcheck-create", Role: workflow.RoleSafetyOfficer,
		Input: domain.BatchInput{SourceLab: "自检实验室", OwnerName: "自检安全员", PlannedHandoverAt: now.Add(24 * time.Hour)},
	})
	if err != nil {
		return fmt.Errorf("自检创建批次: %w", err)
	}
	safetyMeta := func(requestID string) workflow.CommandMeta {
		return workflow.CommandMeta{RequestID: requestID, ExpectedVersion: batch.Version, Role: workflow.RoleSafetyOfficer, Actor: "自检安全员"}
	}
	reviewerMeta := func(requestID string) workflow.CommandMeta {
		return workflow.CommandMeta{RequestID: requestID, ExpectedVersion: batch.Version, Role: workflow.RoleReviewer, Actor: "自检复核员"}
	}
	batch, _, err = service.AddItem(ctx, workflow.AddItemCommand{
		BatchID: batch.ID, Meta: safetyMeta("selfcheck-item"),
		Input: domain.ItemInput{MaterialName: "含丙酮废液", HazardClasses: []string{"flammable", "toxic"}, Quantity: 2.5, Unit: "L", DisposalCategory: "liquid"},
	})
	if err != nil {
		return fmt.Errorf("自检登记条目: %w", err)
	}
	itemID := batch.Items[0].ID
	batch, _, err = service.SetPackage(ctx, workflow.PackageCommand{
		BatchID: batch.ID, ItemID: itemID, Meta: safetyMeta("selfcheck-package"),
		Input: domain.PackageInput{ContainerType: "5L HDPE 防漏桶", SealChecked: true, LabelChecked: true},
	})
	if err != nil {
		return fmt.Errorf("自检封装核验: %w", err)
	}
	batch, _, err = service.SubmitReview(ctx, workflow.SubmitCommand{BatchID: batch.ID, Meta: safetyMeta("selfcheck-submit-1")})
	if err != nil {
		return fmt.Errorf("自检首次提交审查: %w", err)
	}
	batch, _, err = service.DecideReview(ctx, workflow.ReviewCommand{
		BatchID: batch.ID, ItemID: itemID, Meta: reviewerMeta("selfcheck-reject"),
		Input: domain.ReviewInput{Decision: domain.DecisionReject, ReasonCode: "LABEL_DETAIL", Comment: "标签需补充危险属性", ReviewerName: "自检复核员"},
	})
	if err != nil {
		return fmt.Errorf("自检退回条目: %w", err)
	}
	batch, _, err = service.CompleteReview(ctx, workflow.SubmitCommand{BatchID: batch.ID, Meta: reviewerMeta("selfcheck-complete-1")})
	if err != nil {
		return fmt.Errorf("自检完成退回审查: %w", err)
	}
	batch, _, err = service.CorrectItem(ctx, workflow.CorrectionCommand{
		BatchID: batch.ID, ItemID: itemID, Meta: safetyMeta("selfcheck-correct"), Note: "已补充完整危险属性标签",
		Item:    domain.ItemInput{MaterialName: "含丙酮废液", HazardClasses: []string{"flammable", "toxic"}, Quantity: 2.5, Unit: "L", DisposalCategory: "liquid"},
		Package: domain.PackageInput{ContainerType: "5L HDPE 防漏桶", SealChecked: true, LabelChecked: true},
	})
	if err != nil {
		return fmt.Errorf("自检整改条目: %w", err)
	}
	batch, _, err = service.SubmitReview(ctx, workflow.SubmitCommand{BatchID: batch.ID, Meta: safetyMeta("selfcheck-submit-2")})
	if err != nil {
		return fmt.Errorf("自检整改重提: %w", err)
	}
	batch, _, err = service.DecideReview(ctx, workflow.ReviewCommand{
		BatchID: batch.ID, ItemID: itemID, Meta: reviewerMeta("selfcheck-approve"),
		Input: domain.ReviewInput{Decision: domain.DecisionApprove, Comment: "相容性与接收条件符合", ReviewerName: "自检复核员"},
	})
	if err != nil {
		return fmt.Errorf("自检通过条目: %w", err)
	}
	batch, _, err = service.CompleteReview(ctx, workflow.SubmitCommand{BatchID: batch.ID, Meta: reviewerMeta("selfcheck-complete-2")})
	if err != nil {
		return fmt.Errorf("自检完成通过审查: %w", err)
	}
	batch, _, err = service.FreezeManifest(ctx, workflow.SubmitCommand{BatchID: batch.ID, Meta: reviewerMeta("selfcheck-freeze")})
	if err != nil {
		return fmt.Errorf("自检冻结清单: %w", err)
	}
	batch, _, err = service.ConfirmHandover(ctx, workflow.ConfirmCommand{BatchID: batch.ID, Meta: safetyMeta("selfcheck-sender-confirm"), Name: "自检安全员"})
	if err != nil {
		return fmt.Errorf("自检移交确认: %w", err)
	}
	batch, _, err = service.ConfirmHandover(ctx, workflow.ConfirmCommand{BatchID: batch.ID, Meta: reviewerMeta("selfcheck-receiver-confirm"), Name: "自检复核员"})
	if err != nil {
		return fmt.Errorf("自检接收确认: %w", err)
	}
	if batch.Status != domain.StatusArchived || batch.Receipt == nil || !batch.Receipt.Complete() {
		return fmt.Errorf("自检归档结果不完整")
	}
	receipt, err := service.GetReceipt(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("自检读取凭据: %w", err)
	}
	if receipt.ManifestDigest != batch.ManifestDigest || receipt.TimelineDigest == "" {
		return fmt.Errorf("自检凭据摘要不一致")
	}
	fmt.Printf("自检通过：批次 %s 已归档，凭据 %s，时间线 %d 条\n", batch.ID, receipt.ID, len(receipt.Timeline))
	return nil
}

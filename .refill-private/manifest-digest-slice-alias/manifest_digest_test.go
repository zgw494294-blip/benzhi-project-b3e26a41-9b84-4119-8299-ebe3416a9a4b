package manifest_digest_slice_alias

import (
	"reflect"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

func TestManifestDigestPreservesHazardSlice(t *testing.T) {
	now := time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC)
	batch, err := domain.NewBatch("batch-legacy-order", domain.BatchInput{
		SourceLab: "旧数据实验室", OwnerName: "安全员", PlannedHandoverAt: now.Add(48 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.AddItem("item-1", domain.ItemInput{
		MaterialName: "混合废液", HazardClasses: []string{"flammable", "toxic"},
		Quantity: 1, Unit: "L", DisposalCategory: "liquid",
	}, "安全员", now); err != nil {
		t.Fatal(err)
	}

	// 模拟旧版本恢复的聚合：校验允许该数据，但危险属性顺序尚未规范化。
	batch.Items[0].HazardClasses = []string{" Toxic ", "flammable", "toxic"}
	before := append([]string(nil), batch.Items[0].HazardClasses...)
	if _, err := batch.CalculateManifestDigest(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batch.Items[0].HazardClasses, before) {
		t.Fatalf("摘要计算污染了聚合中的危险属性顺序: before=%v after=%v", before, batch.Items[0].HazardClasses)
	}
}

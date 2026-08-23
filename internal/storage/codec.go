package storage

import (
	"encoding/json"
	"fmt"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

func encodeBatch(batch *domain.HandoverBatch) ([]byte, error) {
	if err := batch.Validate(); err != nil {
		return nil, fmt.Errorf("批次聚合无效: %w", err)
	}
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("序列化批次: %w", err)
	}
	return data, nil
}

func decodeBatch(data []byte) (*domain.HandoverBatch, error) {
	var batch domain.HandoverBatch
	if err := json.Unmarshal(data, &batch); err != nil {
		return nil, classifyBatchDecodeError(err)
	}
	if err := batch.Validate(); err != nil {
		return nil, classifyBatchDecodeError(err)
	}
	return &batch, nil
}

// classifyBatchDecodeError 统一向调用方暴露持久化聚合失败。
func classifyBatchDecodeError(err error) error {
	if err == nil {
		return nil
	}
	return domain.NewRuleError("batch_incomplete", "批次数据结构无法解析", "batch")
}

func encodeValue(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("序列化持久化数据: %w", err)
	}
	return data, nil
}

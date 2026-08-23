package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

type Mutation func(batch *domain.HandoverBatch) error

func validateRequest(requestID, operation string) error {
	if requestID == "" {
		return domain.Required("requestId", "requestId")
	}
	if len(requestID) > 128 {
		return domain.Invalid("requestId", "requestId 不能超过 128 个字符")
	}
	if operation == "" {
		return domain.Required("operation", "操作名称")
	}
	return nil
}

func replay(ctx context.Context, tx *sql.Tx, requestID, operation string) (*domain.HandoverBatch, bool, error) {
	var storedOperation string
	var data []byte
	err := tx.QueryRowContext(ctx, "SELECT operation, result_json FROM idempotency_results WHERE request_id = ?", requestID).Scan(&storedOperation, &data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("读取幂等结果: %w", err)
	}
	if storedOperation != operation {
		return nil, false, domain.NewRuleError("request_id_reused", "requestId 已被其他操作使用", "requestId")
	}
	batch, err := decodeBatch(data)
	if err != nil {
		return nil, false, fmt.Errorf("重放幂等结果: %w", err)
	}
	return batch, true, nil
}

func saveIdempotency(ctx context.Context, tx *sql.Tx, requestID, operation string, batch *domain.HandoverBatch, now time.Time) error {
	data, err := encodeBatch(batch)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency_results(request_id, operation, batch_id, result_json, created_at) VALUES(?, ?, ?, ?, ?)`, requestID, operation, batch.ID, data, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("保存幂等结果: %w", err)
	}
	return nil
}

func (s *Store) CreateBatch(ctx context.Context, requestID string, batch *domain.HandoverBatch) (*domain.HandoverBatch, bool, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, false, err
	}
	if err := validateRequest(requestID, "create_batch"); err != nil {
		return nil, false, err
	}
	if err := batch.Validate(); err != nil {
		return nil, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("开始创建批次事务: %w", err)
	}
	defer tx.Rollback()
	if existing, found, err := replay(ctx, tx, requestID, "create_batch"); err != nil || found {
		return existing, found, err
	}
	data, err := encodeBatch(batch)
	if err != nil {
		return nil, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO batches(id,status,version,source_lab,owner_name,planned_handover_at,data_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		batch.ID, batch.Status, batch.Version, batch.SourceLab, batch.OwnerName, batch.PlannedHandoverAt.UTC().Format(time.RFC3339Nano), data, batch.CreatedAt.UTC().Format(time.RFC3339Nano), batch.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, false, fmt.Errorf("插入交接批次: %w", err)
	}
	if err := syncAssociationsTx(ctx, tx, batch); err != nil {
		return nil, false, err
	}
	if err := saveIdempotency(ctx, tx, requestID, "create_batch", batch, batch.CreatedAt); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("提交创建批次事务: %w", err)
	}
	return batch, false, nil
}

func (s *Store) UpdateBatch(ctx context.Context, batchID string, expectedVersion int64, requestID, operation string, mutate Mutation) (*domain.HandoverBatch, bool, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, false, err
	}
	if err := validateRequest(requestID, operation); err != nil {
		return nil, false, err
	}
	if expectedVersion < 1 {
		return nil, false, domain.Invalid("expectedVersion", "expectedVersion 必须大于 0")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("开始更新批次事务: %w", err)
	}
	defer tx.Rollback()
	if existing, found, err := replay(ctx, tx, requestID, operation); err != nil || found {
		return existing, found, err
	}
	batch, err := scanBatch(tx.QueryRowContext(ctx, "SELECT data_json FROM batches WHERE id = ?", batchID))
	if err == sql.ErrNoRows {
		return nil, false, &domain.NotFoundError{Resource: "交接批次", ID: batchID}
	}
	if err != nil {
		return nil, false, fmt.Errorf("读取待更新批次: %w", err)
	}
	if batch.Version != expectedVersion {
		return nil, false, &domain.ConflictError{Expected: expectedVersion, Actual: batch.Version}
	}
	if err := mutate(batch); err != nil {
		return nil, false, err
	}
	batch.Version++
	if err := batch.Validate(); err != nil {
		return nil, false, err
	}
	data, err := encodeBatch(batch)
	if err != nil {
		return nil, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE batches SET status=?,version=?,source_lab=?,owner_name=?,planned_handover_at=?,data_json=?,updated_at=? WHERE id=? AND version=?`,
		batch.Status, batch.Version, batch.SourceLab, batch.OwnerName, batch.PlannedHandoverAt.UTC().Format(time.RFC3339Nano), data, batch.UpdatedAt.UTC().Format(time.RFC3339Nano), batch.ID, expectedVersion)
	if err != nil {
		return nil, false, fmt.Errorf("更新交接批次: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("确认更新结果: %w", err)
	}
	if affected != 1 {
		return nil, false, &domain.ConflictError{Expected: expectedVersion, Actual: batch.Version}
	}
	if err := saveIdempotency(ctx, tx, requestID, operation, batch, batch.UpdatedAt); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("提交更新批次事务: %w", err)
	}
	if err := s.syncAssociations(ctx, batch); err != nil {
		return nil, false, err
	}
	return batch, false, nil
}

func (s *Store) syncAssociations(ctx context.Context, batch *domain.HandoverBatch) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始关联投影事务: %w", err)
	}
	defer tx.Rollback()
	if err := syncAssociationsTx(ctx, tx, batch); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交关联投影事务: %w", err)
	}
	return nil
}

func syncAssociationsTx(ctx context.Context, tx *sql.Tx, batch *domain.HandoverBatch) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM waste_items WHERE batch_id = ?", batch.ID); err != nil {
		return fmt.Errorf("清理条目投影: %w", err)
	}
	for _, item := range batch.Items {
		data, err := encodeValue(item)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO waste_items(id,batch_id,material_name,disposal_category,quantity,unit,review_status,data_json) VALUES(?,?,?,?,?,?,?,?)`, item.ID, batch.ID, item.MaterialName, item.DisposalCategory, item.Quantity, item.Unit, item.ReviewStatus, data)
		if err != nil {
			return fmt.Errorf("保存条目投影: %w", err)
		}
	}
	for _, review := range batch.Reviews {
		data, err := encodeValue(review)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO review_decisions(id,batch_id,item_id,decision,reviewer_name,decided_at,supersedes_id,data_json) VALUES(?,?,?,?,?,?,?,?)`, review.ID, batch.ID, review.ItemID, review.Decision, review.ReviewerName, review.DecidedAt.UTC().Format(time.RFC3339Nano), nullable(review.SupersedesID), data)
		if err != nil {
			return fmt.Errorf("保存审查历史: %w", err)
		}
	}
	if batch.Receipt != nil {
		data, err := encodeValue(batch.Receipt)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO archive_receipts(id,batch_id,manifest_digest,timeline_digest,receipt_json,issued_at) VALUES(?,?,?,?,?,?)`, batch.Receipt.ID, batch.ID, batch.Receipt.ManifestDigest, batch.Receipt.TimelineDigest, data, batch.Receipt.IssuedAt.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("保存归档凭据: %w", err)
		}
	}
	return nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

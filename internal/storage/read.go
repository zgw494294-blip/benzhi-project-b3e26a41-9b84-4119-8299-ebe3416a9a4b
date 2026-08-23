package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

type batchScanner interface {
	Scan(dest ...any) error
}

func scanBatch(scanner batchScanner) (*domain.HandoverBatch, error) {
	var data []byte
	if err := scanner.Scan(&data); err != nil {
		return nil, err
	}
	return decodeBatch(data)
}

func (s *Store) GetBatch(ctx context.Context, id string) (*domain.HandoverBatch, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	batch, err := scanBatch(s.db.QueryRowContext(ctx, "SELECT data_json FROM batches WHERE id = ?", id))
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "交接批次", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("读取交接批次: %w", err)
	}
	return batch, nil
}

func (s *Store) ListBatches(ctx context.Context, limit, offset int, options ...any) ([]domain.HandoverBatch, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	var due domain.DueStatus
	now := time.Now().UTC()
	for _, option := range options {
		switch value := option.(type) {
		case domain.DueStatus:
			due = value
		case time.Time:
			now = value.UTC()
		}
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	query := "SELECT data_json FROM batches"
	args := make([]any, 0, 4)
	if due != "" {
		if !due.Valid() {
			return nil, domain.Invalid("dueStatus", "到期标记必须是 normal、due_soon、due_today 或 overdue")
		}
		query += " WHERE "
		today := now.UTC().Format("2006-01-02")
		switch due {
		case domain.DueOverdue:
			query += "date(planned_handover_at) < date(?)"
			args = append(args, today)
		case domain.DueToday:
			query += "date(planned_handover_at) = date(?)"
			args = append(args, today)
		case domain.DueSoon:
			query += "date(planned_handover_at) > date(?) AND date(planned_handover_at) <= date(?, '+3 days')"
			args = append(args, today, today)
		case domain.DueNormal:
			query += "date(planned_handover_at) > date(?, '+3 days')"
			args = append(args, today)
		}
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询交接批次列表: %w", err)
	}
	defer rows.Close()
	result := make([]domain.HandoverBatch, 0)
	for rows.Next() {
		batch, err := scanBatch(rows)
		if err != nil {
			return nil, fmt.Errorf("解析交接批次列表: %w", err)
		}
		result = append(result, *batch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历交接批次列表: %w", err)
	}
	return result, nil
}

func (s *Store) GetReceipt(ctx context.Context, batchID string) (*domain.ArchiveReceipt, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	var data []byte
	err := s.db.QueryRowContext(ctx, "SELECT receipt_json FROM archive_receipts WHERE batch_id = ?", batchID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{Resource: "归档凭据", ID: batchID}
	}
	if err != nil {
		return nil, fmt.Errorf("读取归档凭据: %w", err)
	}
	return decodeStoredReceipt(data)
}

// decodeStoredReceipt validates the persisted receipt representation.
func decodeStoredReceipt(data []byte) (*domain.ArchiveReceipt, error) {
	var receipt domain.ArchiveReceipt
	if err := jsonUnmarshal(data, &receipt); err != nil {
		return nil, domain.NewRuleError("receipt_incomplete", "归档凭据结构无法解析", "receipt")
	}
	if !receipt.Complete() {
		return nil, domain.NewRuleError("receipt_incomplete", "归档凭据缺失或结构不完整", "receipt")
	}
	return &receipt, nil
}

func (s *Store) GetBatchAndReceipt(ctx context.Context, batchID string) (*domain.HandoverBatch, *domain.ArchiveReceipt, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, fmt.Errorf("开始凭据读取事务: %w", err)
	}
	defer tx.Rollback()
	batch, err := scanBatch(tx.QueryRowContext(ctx, "SELECT data_json FROM batches WHERE id = ?", batchID))
	if err == sql.ErrNoRows {
		return nil, nil, &domain.NotFoundError{Resource: "交接批次", ID: batchID}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("读取待核验批次: %w", err)
	}
	var data []byte
	err = tx.QueryRowContext(ctx, "SELECT receipt_json FROM archive_receipts WHERE batch_id = ?", batchID).Scan(&data)
	if err == sql.ErrNoRows {
		_ = tx.Commit()
		return batch, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("读取待核验凭据: %w", err)
	}
	receipt, err := decodeStoredReceipt(data)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("提交凭据读取事务: %w", err)
	}
	return batch, receipt, nil
}

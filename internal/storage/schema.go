package storage

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaVersion = 1

const schemaSQL = `
CREATE TABLE IF NOT EXISTS schema_meta (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    version INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS batches (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    source_lab TEXT NOT NULL,
    owner_name TEXT NOT NULL,
    planned_handover_at TEXT NOT NULL,
    data_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_batches_updated ON batches(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_batches_status ON batches(status);
CREATE INDEX IF NOT EXISTS idx_batches_planned ON batches(planned_handover_at);
CREATE TABLE IF NOT EXISTS waste_items (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    material_name TEXT NOT NULL,
    disposal_category TEXT NOT NULL,
    quantity REAL NOT NULL,
    unit TEXT NOT NULL,
    review_status TEXT NOT NULL,
    data_json BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_items_batch ON waste_items(batch_id);
CREATE TABLE IF NOT EXISTS review_decisions (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL REFERENCES batches(id) ON DELETE RESTRICT,
    item_id TEXT NOT NULL,
    decision TEXT NOT NULL,
    reviewer_name TEXT NOT NULL,
    decided_at TEXT NOT NULL,
    supersedes_id TEXT,
    data_json BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reviews_batch ON review_decisions(batch_id, decided_at);
CREATE TABLE IF NOT EXISTS idempotency_results (
    request_id TEXT PRIMARY KEY,
    operation TEXT NOT NULL,
    batch_id TEXT NOT NULL,
    result_json BLOB NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_idempotency_batch ON idempotency_results(batch_id);
CREATE TABLE IF NOT EXISTS archive_receipts (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL UNIQUE REFERENCES batches(id) ON DELETE RESTRICT,
    manifest_digest TEXT NOT NULL,
    timeline_digest TEXT NOT NULL,
    receipt_json BLOB NOT NULL,
    issued_at TEXT NOT NULL
);
CREATE TRIGGER IF NOT EXISTS archive_receipts_immutable_update
BEFORE UPDATE ON archive_receipts BEGIN SELECT RAISE(ABORT, 'archive receipt is immutable'); END;
CREATE TRIGGER IF NOT EXISTS archive_receipts_immutable_delete
BEFORE DELETE ON archive_receipts BEGIN SELECT RAISE(ABORT, 'archive receipt is immutable'); END;
`

func initializeSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始模式事务: %w", err)
	}
	defer tx.Rollback()
	var version int
	err = tx.QueryRowContext(ctx, "SELECT version FROM schema_meta WHERE singleton = 1").Scan(&version)
	if err != nil && err != sql.ErrNoRows && !isMissingTable(err) {
		return fmt.Errorf("读取模式版本: %w", err)
	}
	if err == nil && version != schemaVersion {
		return fmt.Errorf("%w: 当前 %d，支持 %d", ErrIncompatibleSchema, version, schemaVersion)
	}
	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("创建数据库模式: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_meta(singleton, version, created_at) VALUES(1, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, schemaVersion); err != nil {
		return fmt.Errorf("记录模式版本: %w", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT version FROM schema_meta WHERE singleton = 1").Scan(&version); err != nil {
		return fmt.Errorf("复核模式版本: %w", err)
	}
	if version != schemaVersion {
		return fmt.Errorf("%w: 当前 %d，支持 %d", ErrIncompatibleSchema, version, schemaVersion)
	}
	return tx.Commit()
}

func isMissingTable(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return len(message) >= 14 && (contains(message, "no such table") || contains(message, "does not exist"))
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}

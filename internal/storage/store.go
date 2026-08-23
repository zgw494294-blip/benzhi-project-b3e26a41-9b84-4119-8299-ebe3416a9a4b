package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db          *sql.DB
	path        string
	closeMu     sync.RWMutex
	closed      bool
	replayCache sync.Map
}

type replayEntry struct {
	operation string
	data      []byte
}

type Options struct {
	Path string
}

func Open(ctx context.Context, options Options) (*Store, error) {
	path := strings.TrimSpace(options.Path)
	if path == "" {
		path = "handover.db"
	}
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("解析数据库路径: %w", err)
		}
		dsn = "file:" + url.PathEscape(absolute) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	} else if path == ":memory:" {
		dsn = "file:benzhi-memory?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	store := &Store{db: db, path: path}
	cleanup := true
	defer func() {
		if cleanup {
			db.Close()
		}
	}()
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(checkCtx); err != nil {
		return nil, fmt.Errorf("连接 SQLite: %w", err)
	}
	if err := checkIntegrity(checkCtx, db); err != nil {
		return nil, err
	}
	if err := initializeSchema(checkCtx, db); err != nil {
		return nil, err
	}
	cleanup = false
	return store, nil
}

func checkIntegrity(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return fmt.Errorf("执行数据库完整性检查: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("%w: %s", ErrCorruptDatabase, result)
	}
	return nil
}

func (s *Store) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

func (s *Store) ensureOpen() error {
	s.closeMu.RLock()
	defer s.closeMu.RUnlock()
	if s.closed {
		return ErrClosed
	}
	return nil
}

func (s *Store) Health(ctx context.Context) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("SQLite 健康检查: %w", err)
	}
	return nil
}

func (s *Store) Path() string { return s.path }

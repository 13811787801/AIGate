package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(driver, sqlitePath, mysqlDSN string) (*Store, error) {
	var (
		db  *sql.DB
		err error
	)
	switch driver {
	case "sqlite", "":
		if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
			return nil, err
		}
		db, err = sql.Open("sqlite", sqlitePath)
		if err != nil {
			return nil, err
		}
		// SQLite 并发写友好设置
		_, _ = db.Exec(`PRAGMA foreign_keys = ON;`)
		_, _ = db.Exec(`PRAGMA journal_mode = WAL;`)
	case "mysql":
		return nil, fmt.Errorf("mysql driver not wired in phase1; use sqlite for now")
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Store) migrate() error {
	_, err := s.DB.Exec(schemaSQL)
	return err
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS user_group (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  multiplier REAL NOT NULL DEFAULT 1.0,
  description TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL DEFAULT '',
  display_name TEXT,
  group_id INTEGER NOT NULL DEFAULT 1,
  status INTEGER NOT NULL DEFAULT 1,
  balance REAL NOT NULL DEFAULT 0,
  frozen REAL NOT NULL DEFAULT 0,
  total_consumed REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS api_key (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,
  key_prefix TEXT NOT NULL,
  group_id INTEGER,
  quota_total INTEGER,
  quota_used INTEGER NOT NULL DEFAULT 0,
  rpm_limit INTEGER NOT NULL DEFAULT 60,
  tpm_limit INTEGER NOT NULL DEFAULT 100000,
  allowed_models TEXT,
  ip_whitelist TEXT,
  status INTEGER NOT NULL DEFAULT 1,
  expired_at TEXT,
  last_used_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS channel (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  provider TEXT NOT NULL,
  base_url TEXT NOT NULL DEFAULT '',
  api_key_enc TEXT NOT NULL DEFAULT '',
  weight INTEGER NOT NULL DEFAULT 100,
  priority INTEGER NOT NULL DEFAULT 0,
  status INTEGER NOT NULL DEFAULT 1,
  timeout_ms INTEGER NOT NULL DEFAULT 120000,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS model_config (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  model_name TEXT NOT NULL UNIQUE,
  display_name TEXT,
  billing_mode TEXT NOT NULL DEFAULT 'TOKEN',
  input_price_per_1k REAL NOT NULL DEFAULT 0,
  output_price_per_1k REAL NOT NULL DEFAULT 0,
  price_per_request REAL,
  max_tokens INTEGER NOT NULL DEFAULT 8192,
  supports_stream INTEGER NOT NULL DEFAULT 1,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS model_channel (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  model_config_id INTEGER NOT NULL,
  channel_id INTEGER NOT NULL,
  upstream_model TEXT NOT NULL,
  weight INTEGER NOT NULL DEFAULT 100,
  enabled INTEGER NOT NULL DEFAULT 1,
  UNIQUE(model_config_id, channel_id)
);

CREATE TABLE IF NOT EXISTS group_model (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id INTEGER NOT NULL,
  model_config_id INTEGER NOT NULL,
  UNIQUE(group_id, model_config_id)
);

CREATE TABLE IF NOT EXISTS request_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  trace_id TEXT NOT NULL,
  user_id INTEGER NOT NULL,
  api_key_id INTEGER NOT NULL,
  model_name TEXT NOT NULL,
  channel_id INTEGER,
  group_id INTEGER,
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  cost REAL NOT NULL DEFAULT 0,
  multiplier REAL NOT NULL DEFAULT 1,
  status_code INTEGER,
  success INTEGER NOT NULL DEFAULT 0,
  latency_ms INTEGER,
  ip_address TEXT,
  error_message TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS balance_ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  api_key_id INTEGER,
  change_amt REAL NOT NULL,
  balance_after REAL NOT NULL,
  reason TEXT NOT NULL,
  ref_id TEXT,
  created_at TEXT NOT NULL
);
`

func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

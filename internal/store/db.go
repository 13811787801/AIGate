package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB     *sql.DB
	Driver string // sqlite | mysql
}

// tblUser is backtick-quoted because MySQL treats USER as reserved.
// Must be concatenated into SQL (cannot sit inside a raw `...` string).
const tblUser = "`user`"

func Open(driver, sqlitePath, mysqlDSN string) (*Store, error) {
	var (
		db  *sql.DB
		err error
	)
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		driver = "sqlite"
	}
	switch driver {
	case "sqlite":
		dir := filepath.Dir(sqlitePath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}
		db, err = sql.Open("sqlite", sqlitePath)
		if err != nil {
			return nil, err
		}
		_, _ = db.Exec(`PRAGMA foreign_keys = ON;`)
		_, _ = db.Exec(`PRAGMA journal_mode = WAL;`)
	case "mysql":
		if strings.TrimSpace(mysqlDSN) == "" {
			return nil, fmt.Errorf("database.dsn is required when driver=mysql")
		}
		// Ensure parseTime for DATETIME if used later; our app stores RFC3339 TEXT/VARCHAR.
		dsn := mysqlDSN
		if !strings.Contains(dsn, "parseTime=") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if !strings.Contains(dsn, "charset=") {
			if strings.Contains(dsn, "?") {
				dsn += "&charset=utf8mb4"
			} else {
				dsn += "?charset=utf8mb4"
			}
		}
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Hour)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s (use sqlite or mysql)", driver)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s: %w", driver, err)
	}
	s := &Store{DB: db, Driver: driver}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Store) IsMySQL() bool { return s.Driver == "mysql" }

func (s *Store) migrate() error {
	schema := schemaSQLite
	if s.IsMySQL() {
		schema = schemaMySQL
	}
	for _, stmt := range splitSQL(schema) {
		if _, err := s.DB.Exec(stmt); err != nil {
			return fmt.Errorf("schema stmt failed: %w\nSQL: %s", err, truncateSQL(stmt))
		}
	}
	// Additive columns for existing SQLite DBs (ignore errors if exists).
	if !s.IsMySQL() {
		alters := []string{
			`ALTER TABLE user ADD COLUMN plan_id INTEGER`,
			`ALTER TABLE user ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`,
			`ALTER TABLE user ADD COLUMN parent_id INTEGER`,
			`ALTER TABLE user ADD COLUMN org_role TEXT NOT NULL DEFAULT 'member'`,
			`ALTER TABLE user ADD COLUMN perms_json TEXT NOT NULL DEFAULT ''`,
		}
		for _, q := range alters {
			_, _ = s.DB.Exec(q)
		}
	} else {
		// MySQL: add columns if missing (ignore duplicate column errors).
		alters := []string{
			`ALTER TABLE ` + tblUser + ` ADD COLUMN plan_id BIGINT NULL`,
			`ALTER TABLE ` + tblUser + ` ADD COLUMN role VARCHAR(32) NOT NULL DEFAULT 'user'`,
			`ALTER TABLE ` + tblUser + ` ADD COLUMN parent_id BIGINT NULL`,
			`ALTER TABLE ` + tblUser + ` ADD COLUMN org_role VARCHAR(32) NOT NULL DEFAULT 'member'`,
			`ALTER TABLE ` + tblUser + ` ADD COLUMN perms_json TEXT`,
		}
		for _, q := range alters {
			_, _ = s.DB.Exec(q)
		}
	}
	// Additive tables for DBs created before M3 second knife.
	if s.IsMySQL() {
		_, _ = s.DB.Exec(`CREATE TABLE IF NOT EXISTS subscription_order (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  order_no VARCHAR(64) NOT NULL UNIQUE,
  user_id BIGINT NOT NULL,
  plan_code VARCHAR(64) NOT NULL,
  amount DOUBLE NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  payment_method VARCHAR(32) NOT NULL DEFAULT 'mock',
  status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
  paid_at VARCHAR(64) NULL,
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_sub_order_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	} else {
		_, _ = s.DB.Exec(`CREATE TABLE IF NOT EXISTS subscription_order (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_no TEXT NOT NULL UNIQUE,
  user_id INTEGER NOT NULL,
  plan_code TEXT NOT NULL,
  amount REAL NOT NULL,
  currency TEXT NOT NULL DEFAULT 'CNY',
  payment_method TEXT NOT NULL DEFAULT 'mock',
  status TEXT NOT NULL DEFAULT 'PENDING',
  paid_at TEXT,
  created_at TEXT NOT NULL
)`)
	}
	return nil
}

func splitSQL(schema string) []string {
	parts := strings.Split(schema, ";")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "--") {
			continue
		}
		out = append(out, p)
	}
	return out
}

func truncateSQL(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// Dialect helpers for upserts.
func (s *Store) execInsertIgnorePlan(code, name, modsJSON string, maxKeys int, note, now string) error {
	if s.IsMySQL() {
		_, err := s.DB.Exec(`INSERT IGNORE INTO plan(code,name,modules_json,max_keys,status,note,created_at,updated_at)
VALUES(?,?,?,?,1,?,?,?)`, code, name, modsJSON, maxKeys, note, now, now)
		return err
	}
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO plan(code,name,modules_json,max_keys,status,note,created_at,updated_at)
VALUES(?,?,?,?,1,?,?,?)`, code, name, modsJSON, maxKeys, note, now, now)
	return err
}

func (s *Store) execUpsertPlan(code, name, modsJSON string, maxKeys, status int, note, now string) (sql.Result, error) {
	if s.IsMySQL() {
		return s.DB.Exec(`INSERT INTO plan(code,name,modules_json,max_keys,status,note,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE name=VALUES(name), modules_json=VALUES(modules_json),
max_keys=VALUES(max_keys), status=VALUES(status), note=VALUES(note), updated_at=VALUES(updated_at)`,
			code, name, modsJSON, maxKeys, status, note, now, now)
	}
	return s.DB.Exec(`INSERT INTO plan(code,name,modules_json,max_keys,status,note,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(code) DO UPDATE SET name=excluded.name, modules_json=excluded.modules_json,
max_keys=excluded.max_keys, status=excluded.status, note=excluded.note, updated_at=excluded.updated_at`,
		code, name, modsJSON, maxKeys, status, note, now, now)
}

func (s *Store) execInsertIgnoreRedeem(code string, amount float64, maxUses int, note, now string) error {
	if s.IsMySQL() {
		_, err := s.DB.Exec(`INSERT IGNORE INTO redeem_code(code,amount,max_uses,used_count,note,enabled,created_at)
VALUES(?,?,?,0,?,1,?)`, code, amount, maxUses, note, now)
		return err
	}
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO redeem_code(code,amount,max_uses,used_count,note,enabled,created_at)
VALUES(?,?,?,0,?,1,?)`, code, amount, maxUses, note, now)
	return err
}

func (s *Store) execBindModelChannel(modelID, channelID int64, upstream string, weight int) error {
	if s.IsMySQL() {
		_, err := s.DB.Exec(`INSERT INTO model_channel(model_config_id,channel_id,upstream_model,weight,enabled)
VALUES(?,?,?,?,1)
ON DUPLICATE KEY UPDATE upstream_model=VALUES(upstream_model), weight=VALUES(weight), enabled=1`,
			modelID, channelID, upstream, weight)
		return err
	}
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO model_channel(model_config_id,channel_id,upstream_model,weight,enabled)
VALUES(?,?,?,?,1)`, modelID, channelID, upstream, weight)
	return err
}

func (s *Store) execBindGroupModel(groupID, modelID int64) error {
	if s.IsMySQL() {
		_, err := s.DB.Exec(`INSERT IGNORE INTO group_model(group_id, model_config_id) VALUES(?,?)`, groupID, modelID)
		return err
	}
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO group_model(group_id, model_config_id) VALUES(?,?)`, groupID, modelID)
	return err
}

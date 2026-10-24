package store

// schemaSQLite is the canonical schema for local/dev.
const schemaSQLite = `
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
  plan_id INTEGER,
  role TEXT NOT NULL DEFAULT 'user',
  parent_id INTEGER,
  org_role TEXT NOT NULL DEFAULT 'member',
  perms_json TEXT NOT NULL DEFAULT '',
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

CREATE TABLE IF NOT EXISTS redeem_code (
  code TEXT PRIMARY KEY,
  amount REAL NOT NULL,
  max_uses INTEGER NOT NULL DEFAULT 0,
  used_count INTEGER NOT NULL DEFAULT 0,
  note TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS redeem_use (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  user_id INTEGER NOT NULL,
  amount REAL NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(code, user_id)
);

CREATE TABLE IF NOT EXISTS plan (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  modules_json TEXT NOT NULL,
  max_keys INTEGER NOT NULL DEFAULT 0,
  status INTEGER NOT NULL DEFAULT 1,
  note TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor_user_id INTEGER,
  action TEXT NOT NULL,
  target TEXT,
  detail TEXT,
  ip TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS subscription_order (
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
);

CREATE TABLE IF NOT EXISTS chat_session (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  model_name TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id INTEGER NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  model_name TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
`

// schemaMySQL for production. `user` is reserved — always backtick it.
const schemaMySQL = `
CREATE TABLE IF NOT EXISTS user_group (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL UNIQUE,
  multiplier DOUBLE NOT NULL DEFAULT 1.0,
  description TEXT,
  created_at VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ` + "`user`" + ` (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  username VARCHAR(128) NOT NULL UNIQUE,
  password_hash VARCHAR(128) NOT NULL DEFAULT '',
  display_name VARCHAR(256),
  group_id BIGINT NOT NULL DEFAULT 1,
  status INT NOT NULL DEFAULT 1,
  balance DOUBLE NOT NULL DEFAULT 0,
  frozen DOUBLE NOT NULL DEFAULT 0,
  total_consumed DOUBLE NOT NULL DEFAULT 0,
  plan_id BIGINT NULL,
  role VARCHAR(32) NOT NULL DEFAULT 'user',
  parent_id BIGINT NULL,
  org_role VARCHAR(32) NOT NULL DEFAULT 'member',
  perms_json TEXT,
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS api_key (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL,
  key_hash VARCHAR(128) NOT NULL UNIQUE,
  key_prefix VARCHAR(32) NOT NULL,
  group_id BIGINT NULL,
  quota_total BIGINT NULL,
  quota_used BIGINT NOT NULL DEFAULT 0,
  rpm_limit INT NOT NULL DEFAULT 60,
  tpm_limit INT NOT NULL DEFAULT 100000,
  allowed_models TEXT,
  ip_whitelist TEXT,
  status INT NOT NULL DEFAULT 1,
  expired_at VARCHAR(64) NULL,
  last_used_at VARCHAR(64) NULL,
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_api_key_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS channel (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  provider VARCHAR(64) NOT NULL,
  base_url VARCHAR(512) NOT NULL DEFAULT '',
  api_key_enc TEXT NOT NULL,
  weight INT NOT NULL DEFAULT 100,
  priority INT NOT NULL DEFAULT 0,
  status INT NOT NULL DEFAULT 1,
  timeout_ms INT NOT NULL DEFAULT 120000,
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS model_config (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  model_name VARCHAR(128) NOT NULL UNIQUE,
  display_name VARCHAR(256),
  billing_mode VARCHAR(32) NOT NULL DEFAULT 'TOKEN',
  input_price_per_1k DOUBLE NOT NULL DEFAULT 0,
  output_price_per_1k DOUBLE NOT NULL DEFAULT 0,
  price_per_request DOUBLE NULL,
  max_tokens INT NOT NULL DEFAULT 8192,
  supports_stream TINYINT NOT NULL DEFAULT 1,
  enabled TINYINT NOT NULL DEFAULT 1,
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS model_channel (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  model_config_id BIGINT NOT NULL,
  channel_id BIGINT NOT NULL,
  upstream_model VARCHAR(128) NOT NULL,
  weight INT NOT NULL DEFAULT 100,
  enabled TINYINT NOT NULL DEFAULT 1,
  UNIQUE KEY uk_model_channel (model_config_id, channel_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS group_model (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  group_id BIGINT NOT NULL,
  model_config_id BIGINT NOT NULL,
  UNIQUE KEY uk_group_model (group_id, model_config_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS request_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  trace_id VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  api_key_id BIGINT NOT NULL,
  model_name VARCHAR(128) NOT NULL,
  channel_id BIGINT NULL,
  group_id BIGINT NULL,
  input_tokens INT NOT NULL DEFAULT 0,
  output_tokens INT NOT NULL DEFAULT 0,
  cost DOUBLE NOT NULL DEFAULT 0,
  multiplier DOUBLE NOT NULL DEFAULT 1,
  status_code INT NULL,
  success TINYINT NOT NULL DEFAULT 0,
  latency_ms INT NULL,
  ip_address VARCHAR(64) NULL,
  error_message TEXT,
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_req_user (user_id),
  INDEX idx_req_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS balance_ledger (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  api_key_id BIGINT NULL,
  change_amt DOUBLE NOT NULL,
  balance_after DOUBLE NOT NULL,
  reason VARCHAR(64) NOT NULL,
  ref_id VARCHAR(128) NULL,
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_ledger_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS redeem_code (
  code VARCHAR(64) PRIMARY KEY,
  amount DOUBLE NOT NULL,
  max_uses INT NOT NULL DEFAULT 0,
  used_count INT NOT NULL DEFAULT 0,
  note VARCHAR(256) NULL,
  enabled TINYINT NOT NULL DEFAULT 1,
  created_at VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS redeem_use (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  amount DOUBLE NOT NULL,
  created_at VARCHAR(64) NOT NULL,
  UNIQUE KEY uk_redeem_user (code, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS plan (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL UNIQUE,
  name VARCHAR(128) NOT NULL,
  modules_json TEXT NOT NULL,
  max_keys INT NOT NULL DEFAULT 0,
  status INT NOT NULL DEFAULT 1,
  note VARCHAR(256) NULL,
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS audit_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  actor_user_id BIGINT NULL,
  action VARCHAR(64) NOT NULL,
  target VARCHAR(128) NULL,
  detail TEXT,
  ip VARCHAR(64) NULL,
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_audit_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS subscription_order (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS chat_session (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  title VARCHAR(256) NOT NULL,
  model_name VARCHAR(128) NOT NULL DEFAULT '',
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL,
  INDEX idx_chat_session_user (user_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS chat_message (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  session_id BIGINT NOT NULL,
  role VARCHAR(32) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  model_name VARCHAR(128) NOT NULL DEFAULT '',
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_chat_message_session (session_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

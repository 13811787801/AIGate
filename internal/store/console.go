package store

import (
	"database/sql"
	"errors"
)

func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, username, password_hash, COALESCE(display_name,''), group_id, status, balance, frozen, total_consumed FROM user WHERE username=?`, username)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.GroupID, &u.Status, &u.Balance, &u.Frozen, &u.TotalConsumed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) VerifyPassword(u *User, plain string) bool {
	return u.PasswordHash == hashPassword(plain)
}

func (s *Store) ListAPIKeysByUser(userID int64) ([]APIKey, error) {
	rows, err := s.DB.Query(`
SELECT id, user_id, name, key_hash, key_prefix, group_id, quota_total, quota_used, rpm_limit, tpm_limit,
       allowed_models, ip_whitelist, status, expired_at
FROM api_key WHERE user_id=? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		var allowed, ipw sql.NullString
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.GroupID, &k.QuotaTotal, &k.QuotaUsed,
			&k.RPMLimit, &k.TPMLimit, &allowed, &ipw, &k.Status, &k.ExpiredAt); err != nil {
			return nil, err
		}
		k.AllowedModels = parseJSONStringSlice(allowed.String)
		k.IPWhitelist = parseJSONStringSlice(ipw.String)
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) CreateUserAPIKey(userID int64, name string, rpm, tpm int) (plain string, key *APIKey, err error) {
	if name == "" {
		name = "default"
	}
	if rpm <= 0 {
		rpm = 60
	}
	if tpm <= 0 {
		tpm = 100000
	}
	plain, hash, prefix, err := NewAPIKeyPlain()
	if err != nil {
		return "", nil, err
	}
	res, err := s.DB.Exec(`INSERT INTO api_key(user_id,name,key_hash,key_prefix,group_id,quota_total,quota_used,rpm_limit,tpm_limit,status,created_at)
VALUES(?,?,?,?,NULL,NULL,0,?,?,1,?)`, userID, name, hash, prefix, rpm, tpm, Now())
	if err != nil {
		return "", nil, err
	}
	id, _ := res.LastInsertId()
	k, err := s.GetAPIKeyByID(id)
	return plain, k, err
}

func (s *Store) GetAPIKeyByID(id int64) (*APIKey, error) {
	row := s.DB.QueryRow(`
SELECT id, user_id, name, key_hash, key_prefix, group_id, quota_total, quota_used, rpm_limit, tpm_limit,
       allowed_models, ip_whitelist, status, expired_at
FROM api_key WHERE id=?`, id)
	var k APIKey
	var allowed, ipw sql.NullString
	if err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.GroupID, &k.QuotaTotal, &k.QuotaUsed,
		&k.RPMLimit, &k.TPMLimit, &allowed, &ipw, &k.Status, &k.ExpiredAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	k.AllowedModels = parseJSONStringSlice(allowed.String)
	k.IPWhitelist = parseJSONStringSlice(ipw.String)
	return &k, nil
}

func (s *Store) DeleteAPIKey(userID, keyID int64) error {
	res, err := s.DB.Exec(`DELETE FROM api_key WHERE id=? AND user_id=?`, keyID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type LogRow struct {
	ID           int64
	TraceID      string
	ModelName    string
	InputTokens  int
	OutputTokens int
	Cost         float64
	StatusCode   int
	Success      bool
	LatencyMs    int
	IP           string
	ErrorMessage string
	CreatedAt    string
}

func (s *Store) ListLogsByUser(userID int64, limit int) ([]LogRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.DB.Query(`
SELECT id, trace_id, model_name, input_tokens, output_tokens, cost, COALESCE(status_code,0), success, COALESCE(latency_ms,0),
       COALESCE(ip_address,''), COALESCE(error_message,''), created_at
FROM request_log WHERE user_id=? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogRow
	for rows.Next() {
		var l LogRow
		var success int
		if err := rows.Scan(&l.ID, &l.TraceID, &l.ModelName, &l.InputTokens, &l.OutputTokens, &l.Cost, &l.StatusCode, &success, &l.LatencyMs, &l.IP, &l.ErrorMessage, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.Success = success == 1
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) ListChannels() ([]Channel, error) {
	rows, err := s.DB.Query(`SELECT id, name, provider, base_url, api_key_enc, weight, priority, status, timeout_ms FROM channel ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Name, &c.Provider, &c.BaseURL, &c.APIKeyEnc, &c.Weight, &c.Priority, &c.Status, &c.TimeoutMs); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ListAllModels() ([]ModelConfig, error) {
	rows, err := s.DB.Query(`
SELECT id, model_name, COALESCE(display_name,''), billing_mode, input_price_per_1k, output_price_per_1k,
       price_per_request, max_tokens, supports_stream, enabled
FROM model_config ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelConfig
	for rows.Next() {
		var m ModelConfig
		var stream, enabled int
		if err := rows.Scan(&m.ID, &m.ModelName, &m.DisplayName, &m.BillingMode, &m.InputPricePer1K, &m.OutputPricePer1K,
			&m.PricePerRequest, &m.MaxTokens, &stream, &enabled); err != nil {
			return nil, err
		}
		m.SupportsStream = stream == 1
		m.Enabled = enabled == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) StatsOverview() (map[string]any, error) {
	var users, keys, channels, models, logs int
	var consumed float64
	err := s.DB.QueryRow(`SELECT
(SELECT COUNT(1) FROM user),
(SELECT COUNT(1) FROM api_key),
(SELECT COUNT(1) FROM channel),
(SELECT COUNT(1) FROM model_config),
(SELECT COUNT(1) FROM request_log),
(SELECT COALESCE(SUM(total_consumed),0) FROM user)`).Scan(&users, &keys, &channels, &models, &logs, &consumed)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"users": users, "api_keys": keys, "channels": channels, "models": models, "request_logs": logs, "total_consumed": consumed,
	}, nil
}

func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

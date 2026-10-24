package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, username, password_hash, COALESCE(display_name,''), group_id, status, balance, frozen, total_consumed,
COALESCE(plan_id,0), COALESCE(role,'user'), COALESCE(parent_id,0),
COALESCE(org_role,'member'), COALESCE(perms_json,'') FROM `+tblUser+` WHERE username=?`, username)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.GroupID, &u.Status, &u.Balance, &u.Frozen, &u.TotalConsumed, &u.PlanID, &u.Role, &u.ParentID, &u.OrgRole, &u.PermsJSON); err != nil {
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

type APIKeyCreateOpts struct {
	Name          string
	RPM           int
	TPM           int
	ExpiredAt     string   // RFC3339 empty = never
	AllowedModels []string // empty = all models
	IPWhitelist   []string // empty = any IP
}

func encodeJSONSlice(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	b, err := json.Marshal(ss)
	if err != nil {
		return ""
	}
	return string(b)
}

func normalizeExpiredAt(raw string) (sql.NullString, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullString{}, nil
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"}
	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}, nil
		}
	}
	return sql.NullString{}, fmt.Errorf("invalid expired_at: %s", raw)
}

func (s *Store) CreateUserAPIKey(userID int64, name string, rpm, tpm int) (plain string, key *APIKey, err error) {
	return s.CreateUserAPIKeyOpts(userID, APIKeyCreateOpts{Name: name, RPM: rpm, TPM: tpm})
}

func (s *Store) CreateUserAPIKeyOpts(userID int64, opts APIKeyCreateOpts) (plain string, key *APIKey, err error) {
	if opts.Name == "" {
		opts.Name = "default"
	}
	if opts.RPM <= 0 {
		opts.RPM = 60
	}
	if opts.TPM <= 0 {
		opts.TPM = 100000
	}
	exp, err := normalizeExpiredAt(opts.ExpiredAt)
	if err != nil {
		return "", nil, err
	}
	plain, hash, prefix, err := NewAPIKeyPlain()
	if err != nil {
		return "", nil, err
	}
	allowed := encodeJSONSlice(opts.AllowedModels)
	ipw := encodeJSONSlice(opts.IPWhitelist)
	res, err := s.DB.Exec(`INSERT INTO api_key(user_id,name,key_hash,key_prefix,group_id,quota_total,quota_used,rpm_limit,tpm_limit,allowed_models,ip_whitelist,status,expired_at,created_at)
VALUES(?,?,?,?,NULL,NULL,0,?,?,?,?,1,?,?)`,
		userID, opts.Name, hash, prefix, opts.RPM, opts.TPM, nullIfEmpty(allowed), nullIfEmpty(ipw), nullStr(exp), Now())
	if err != nil {
		return "", nil, err
	}
	id, _ := res.LastInsertId()
	k, err := s.GetAPIKeyByID(id)
	return plain, k, err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullStr(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return ns.String
}

func (s *Store) SetAPIKeyStatus(userID, keyID int64, status int) error {
	res, err := s.DB.Exec(`UPDATE api_key SET status=? WHERE id=? AND user_id=?`, status, keyID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetChannelStatus(id int64, status int) error {
	res, err := s.DB.Exec(`UPDATE channel SET status=?, updated_at=? WHERE id=?`, status, Now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetChannel(id int64) (*Channel, error) {
	row := s.DB.QueryRow(`SELECT id, name, provider, base_url, api_key_enc, weight, priority, status, timeout_ms FROM channel WHERE id=?`, id)
	var c Channel
	if err := row.Scan(&c.ID, &c.Name, &c.Provider, &c.BaseURL, &c.APIKeyEnc, &c.Weight, &c.Priority, &c.Status, &c.TimeoutMs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpdateChannel(id int64, name, provider, baseURL, sealedKey string, replaceKey bool) error {
	name = strings.TrimSpace(name)
	provider = strings.TrimSpace(provider)
	baseURL = strings.TrimSpace(baseURL)
	if name == "" || provider == "" {
		return fmt.Errorf("name and provider required")
	}
	var res sql.Result
	var err error
	if replaceKey {
		res, err = s.DB.Exec(`UPDATE channel SET name=?, provider=?, base_url=?, api_key_enc=?, updated_at=? WHERE id=?`,
			name, provider, baseURL, sealedKey, Now(), id)
	} else {
		res, err = s.DB.Exec(`UPDATE channel SET name=?, provider=?, base_url=?, updated_at=? WHERE id=?`,
			name, provider, baseURL, Now(), id)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		if _, err := s.GetChannel(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetModel(id int64) (*ModelConfig, error) {
	row := s.DB.QueryRow(`
SELECT id, model_name, COALESCE(display_name,''), billing_mode, input_price_per_1k, output_price_per_1k,
       price_per_request, max_tokens, supports_stream, enabled
FROM model_config WHERE id=?`, id)
	var m ModelConfig
	var stream, enabled int
	if err := row.Scan(&m.ID, &m.ModelName, &m.DisplayName, &m.BillingMode, &m.InputPricePer1K, &m.OutputPricePer1K,
		&m.PricePerRequest, &m.MaxTokens, &stream, &enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	m.SupportsStream = stream == 1
	m.Enabled = enabled == 1
	return &m, nil
}

func (s *Store) UpdateModel(m ModelConfig) error {
	m.ModelName = strings.TrimSpace(m.ModelName)
	m.DisplayName = strings.TrimSpace(m.DisplayName)
	if m.ModelName == "" {
		return fmt.Errorf("model_name required")
	}
	if m.DisplayName == "" {
		m.DisplayName = m.ModelName
	}
	var dup int
	if err := s.DB.QueryRow(`SELECT COUNT(1) FROM model_config WHERE model_name=? AND id<>?`, m.ModelName, m.ID).Scan(&dup); err != nil {
		return err
	}
	if dup > 0 {
		return ErrDuplicate
	}
	if m.MaxTokens <= 0 {
		m.MaxTokens = 8192
	}
	if m.BillingMode == "" {
		m.BillingMode = "TOKEN"
	}
	res, err := s.DB.Exec(`UPDATE model_config
SET model_name=?, display_name=?, billing_mode=?, input_price_per_1k=?, output_price_per_1k=?, max_tokens=?, updated_at=?
WHERE id=?`, m.ModelName, m.DisplayName, m.BillingMode, m.InputPricePer1K, m.OutputPricePer1K, m.MaxTokens, Now(), m.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		if _, err := s.GetModel(m.ID); err != nil {
			return err
		}
	}
	return nil
}

// SetModelPrimaryChannel keeps one active upstream binding and turns the others off.
func (s *Store) SetModelPrimaryChannel(modelID, channelID int64, upstream string) error {
	if _, err := s.GetChannel(channelID); err != nil {
		return err
	}
	upstream = strings.TrimSpace(upstream)
	if upstream == "" {
		return fmt.Errorf("upstream_model required")
	}
	if _, err := s.DB.Exec(`UPDATE model_channel SET enabled=0 WHERE model_config_id=? AND channel_id<>?`, modelID, channelID); err != nil {
		return err
	}
	return s.BindModelChannel(modelID, channelID, upstream, 100)
}

type ModelBindingSummary struct {
	ChannelID   int64
	Upstream    string
	ChannelName string
}

func (s *Store) ListModelBindingSummary() (map[int64]ModelBindingSummary, error) {
	rows, err := s.DB.Query(`
SELECT mc.model_config_id, mc.channel_id, mc.upstream_model, c.name
FROM model_channel mc
JOIN channel c ON c.id = mc.channel_id
WHERE mc.enabled=1
ORDER BY mc.weight DESC, mc.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]ModelBindingSummary{}
	for rows.Next() {
		var modelID int64
		var b ModelBindingSummary
		if err := rows.Scan(&modelID, &b.ChannelID, &b.Upstream, &b.ChannelName); err != nil {
			return nil, err
		}
		if _, ok := out[modelID]; ok {
			continue
		}
		out[modelID] = b
	}
	return out, rows.Err()
}

func (s *Store) SetModelEnabled(id int64, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	res, err := s.DB.Exec(`UPDATE model_config SET enabled=?, updated_at=? WHERE id=?`, en, Now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreditBalance(userID int64, amount float64, reason, refID string) (float64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var balance, frozen float64
	if err := tx.QueryRow(`SELECT balance, frozen FROM `+tblUser+` WHERE id=?`, userID).Scan(&balance, &frozen); err != nil {
		return 0, err
	}
	balance += amount
	if _, err := tx.Exec(`UPDATE `+tblUser+` SET balance=?, updated_at=? WHERE id=?`, balance, Now(), userID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO balance_ledger(user_id, api_key_id, change_amt, balance_after, reason, ref_id, created_at)
VALUES(?,?,?,?,?,?,?)`, userID, nil, amount, balance, reason, refID, Now()); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *Store) SeedRedeemCodes() error {
	now := Now()
	codes := []struct {
		Code    string
		Amount  float64
		MaxUses int
		Note    string
	}{
		{"AIGATE-WELCOME", 10, 0, "欢迎礼包 ¥10（每用户限兑一次）"},
		{"AIGATE-DEMO-100", 100, 0, "演示充值 ¥100（每用户限兑一次）"},
	}
	for _, c := range codes {
		if err := s.execInsertIgnoreRedeem(c.Code, c.Amount, c.MaxUses, c.Note, now); err != nil {
			return err
		}
	}
	return nil
}

var (
	ErrRedeemInvalid   = errors.New("invalid or disabled redeem code")
	ErrRedeemUsed      = errors.New("you already redeemed this code")
	ErrRedeemExhausted = errors.New("redeem code exhausted")
)

func (s *Store) RedeemCode(userID int64, code string) (amount, balance float64, err error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return 0, 0, ErrRedeemInvalid
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var amt float64
	var maxUses, usedCount, enabled int
	err = tx.QueryRow(`SELECT amount, max_uses, used_count, enabled FROM redeem_code WHERE code=?`, code).
		Scan(&amt, &maxUses, &usedCount, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrRedeemInvalid
		}
		return 0, 0, err
	}
	if enabled != 1 {
		return 0, 0, ErrRedeemInvalid
	}
	if maxUses > 0 && usedCount >= maxUses {
		return 0, 0, ErrRedeemExhausted
	}
	var exists int
	_ = tx.QueryRow(`SELECT COUNT(1) FROM redeem_use WHERE code=? AND user_id=?`, code, userID).Scan(&exists)
	if exists > 0 {
		return 0, 0, ErrRedeemUsed
	}

	var bal float64
	if err := tx.QueryRow(`SELECT balance FROM `+tblUser+` WHERE id=?`, userID).Scan(&bal); err != nil {
		return 0, 0, err
	}
	bal += amt
	if _, err := tx.Exec(`UPDATE `+tblUser+` SET balance=?, updated_at=? WHERE id=?`, bal, Now(), userID); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(`INSERT INTO balance_ledger(user_id, api_key_id, change_amt, balance_after, reason, ref_id, created_at)
VALUES(?,?,?,?,?,?,?)`, userID, nil, amt, bal, "redeem", code, Now()); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(`INSERT INTO redeem_use(code,user_id,amount,created_at) VALUES(?,?,?,?)`, code, userID, amt, Now()); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(`UPDATE redeem_code SET used_count = used_count + 1 WHERE code=?`, code); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return amt, bal, nil
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
(SELECT COUNT(1) FROM `+tblUser+`),
(SELECT COUNT(1) FROM api_key),
(SELECT COUNT(1) FROM channel),
(SELECT COUNT(1) FROM model_config),
(SELECT COUNT(1) FROM request_log),
(SELECT COALESCE(SUM(total_consumed),0) FROM `+tblUser+`)`).Scan(&users, &keys, &channels, &models, &logs, &consumed)
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

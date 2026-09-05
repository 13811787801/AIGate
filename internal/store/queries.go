package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

type UserGroup struct {
	ID          int64
	Name        string
	Multiplier  float64
	Description string
}

type User struct {
	ID            int64
	Username      string
	PasswordHash  string
	DisplayName   string
	GroupID       int64
	Status        int
	Balance       float64
	Frozen        float64
	TotalConsumed float64
}

type APIKey struct {
	ID            int64
	UserID        int64
	Name          string
	KeyHash       string
	KeyPrefix     string
	GroupID       sql.NullInt64
	QuotaTotal    sql.NullInt64
	QuotaUsed     int64
	RPMLimit      int
	TPMLimit      int
	AllowedModels []string
	IPWhitelist   []string
	Status        int
	ExpiredAt     sql.NullString
}

type Channel struct {
	ID        int64
	Name      string
	Provider  string
	BaseURL   string
	APIKeyEnc string // Phase1: plaintext in DB for local/dev; encrypt in later phase
	Weight    int
	Priority  int
	Status    int
	TimeoutMs int
}

type ModelConfig struct {
	ID                int64
	ModelName         string
	DisplayName       string
	BillingMode       string
	InputPricePer1K   float64
	OutputPricePer1K  float64
	PricePerRequest   sql.NullFloat64
	MaxTokens         int
	SupportsStream    bool
	Enabled           bool
}

type ModelChannel struct {
	ID            int64
	ModelConfigID int64
	ChannelID     int64
	UpstreamModel string
	Weight        int
	Enabled       bool
	Channel       Channel
}

func (s *Store) GetGroup(id int64) (*UserGroup, error) {
	row := s.DB.QueryRow(`SELECT id, name, multiplier, COALESCE(description,'') FROM user_group WHERE id=?`, id)
	var g UserGroup
	if err := row.Scan(&g.ID, &g.Name, &g.Multiplier, &g.Description); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

func (s *Store) GetUser(id int64) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, username, password_hash, COALESCE(display_name,''), group_id, status, balance, frozen, total_consumed FROM user WHERE id=?`, id)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.GroupID, &u.Status, &u.Balance, &u.Frozen, &u.TotalConsumed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetAPIKeyByHash(hash string) (*APIKey, error) {
	row := s.DB.QueryRow(`
SELECT id, user_id, name, key_hash, key_prefix, group_id, quota_total, quota_used, rpm_limit, tpm_limit,
       allowed_models, ip_whitelist, status, expired_at
FROM api_key WHERE key_hash=?`, hash)
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

func (s *Store) TouchAPIKey(id int64) {
	_, _ = s.DB.Exec(`UPDATE api_key SET last_used_at=? WHERE id=?`, Now(), id)
}

func (s *Store) GetModelByName(name string) (*ModelConfig, error) {
	row := s.DB.QueryRow(`
SELECT id, model_name, COALESCE(display_name,''), billing_mode, input_price_per_1k, output_price_per_1k,
       price_per_request, max_tokens, supports_stream, enabled
FROM model_config WHERE model_name=?`, name)
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

func (s *Store) ListEnabledModelsForGroup(groupID int64) ([]ModelConfig, error) {
	rows, err := s.DB.Query(`
SELECT m.id, m.model_name, COALESCE(m.display_name,''), m.billing_mode, m.input_price_per_1k, m.output_price_per_1k,
       m.price_per_request, m.max_tokens, m.supports_stream, m.enabled
FROM model_config m
JOIN group_model gm ON gm.model_config_id = m.id
WHERE gm.group_id=? AND m.enabled=1
ORDER BY m.model_name`, groupID)
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

func (s *Store) ListModelChannels(modelID int64) ([]ModelChannel, error) {
	rows, err := s.DB.Query(`
SELECT mc.id, mc.model_config_id, mc.channel_id, mc.upstream_model, mc.weight, mc.enabled,
       c.id, c.name, c.provider, c.base_url, c.api_key_enc, c.weight, c.priority, c.status, c.timeout_ms
FROM model_channel mc
JOIN channel c ON c.id = mc.channel_id
WHERE mc.model_config_id=? AND mc.enabled=1 AND c.status=1
ORDER BY c.priority DESC, mc.weight DESC`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelChannel
	for rows.Next() {
		var mc ModelChannel
		var en int
		if err := rows.Scan(&mc.ID, &mc.ModelConfigID, &mc.ChannelID, &mc.UpstreamModel, &mc.Weight, &en,
			&mc.Channel.ID, &mc.Channel.Name, &mc.Channel.Provider, &mc.Channel.BaseURL, &mc.Channel.APIKeyEnc,
			&mc.Channel.Weight, &mc.Channel.Priority, &mc.Channel.Status, &mc.Channel.TimeoutMs); err != nil {
			return nil, err
		}
		mc.Enabled = en == 1
		out = append(out, mc)
	}
	return out, rows.Err()
}

func (s *Store) GroupAllowsModel(groupID, modelID int64) (bool, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(1) FROM group_model WHERE group_id=? AND model_config_id=?`, groupID, modelID).Scan(&n)
	return n > 0, err
}

func (s *Store) FreezeBalance(userID int64, amount float64, apiKeyID int64, refID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var balance, frozen float64
	if err := tx.QueryRow(`SELECT balance, frozen FROM user WHERE id=?`, userID).Scan(&balance, &frozen); err != nil {
		return err
	}
	if balance < amount {
		return fmtInsufficientBalance(balance, amount)
	}
	balance -= amount
	frozen += amount
	if _, err := tx.Exec(`UPDATE user SET balance=?, frozen=?, updated_at=? WHERE id=?`, balance, frozen, Now(), userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO balance_ledger(user_id, api_key_id, change_amt, balance_after, reason, ref_id, created_at)
VALUES(?,?,?,?,?,?,?)`, userID, apiKeyID, -amount, balance, "FREEZE", refID, Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SettleBalance(userID int64, frozenAmt, actualCost float64, apiKeyID int64, refID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var balance, frozen, total float64
	if err := tx.QueryRow(`SELECT balance, frozen, total_consumed FROM user WHERE id=?`, userID).Scan(&balance, &frozen, &total); err != nil {
		return err
	}
	if frozen < frozenAmt {
		frozenAmt = frozen
	}
	frozen -= frozenAmt
	// 归还冻结，再扣实费
	balance += frozenAmt
	if balance < actualCost {
		actualCost = balance // 不应发生；兜底
	}
	balance -= actualCost
	total += actualCost
	if _, err := tx.Exec(`UPDATE user SET balance=?, frozen=?, total_consumed=?, updated_at=? WHERE id=?`,
		balance, frozen, total, Now(), userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO balance_ledger(user_id, api_key_id, change_amt, balance_after, reason, ref_id, created_at)
VALUES(?,?,?,?,?,?,?)`, userID, apiKeyID, frozenAmt, balance+actualCost, "UNFREEZE", refID, Now()); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO balance_ledger(user_id, api_key_id, change_amt, balance_after, reason, ref_id, created_at)
VALUES(?,?,?,?,?,?,?)`, userID, apiKeyID, -actualCost, balance, "SETTLE", refID, Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UnfreezeBalance(userID int64, amount float64, apiKeyID int64, refID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var balance, frozen float64
	if err := tx.QueryRow(`SELECT balance, frozen FROM user WHERE id=?`, userID).Scan(&balance, &frozen); err != nil {
		return err
	}
	if frozen < amount {
		amount = frozen
	}
	frozen -= amount
	balance += amount
	if _, err := tx.Exec(`UPDATE user SET balance=?, frozen=?, updated_at=? WHERE id=?`, balance, frozen, Now(), userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO balance_ledger(user_id, api_key_id, change_amt, balance_after, reason, ref_id, created_at)
VALUES(?,?,?,?,?,?,?)`, userID, apiKeyID, amount, balance, "UNFREEZE", refID, Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AddQuotaUsed(apiKeyID int64, tokens int64) {
	_, _ = s.DB.Exec(`UPDATE api_key SET quota_used = quota_used + ? WHERE id=?`, tokens, apiKeyID)
}

type RequestLog struct {
	TraceID      string
	UserID       int64
	APIKeyID     int64
	ModelName    string
	ChannelID    sql.NullInt64
	GroupID      sql.NullInt64
	InputTokens  int
	OutputTokens int
	Cost         float64
	Multiplier   float64
	StatusCode   int
	Success      bool
	LatencyMs    int
	IP           string
	ErrorMessage string
}

func (s *Store) InsertRequestLog(l RequestLog) error {
	success := 0
	if l.Success {
		success = 1
	}
	_, err := s.DB.Exec(`INSERT INTO request_log(
trace_id,user_id,api_key_id,model_name,channel_id,group_id,input_tokens,output_tokens,cost,multiplier,
status_code,success,latency_ms,ip_address,error_message,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		l.TraceID, l.UserID, l.APIKeyID, l.ModelName, l.ChannelID, l.GroupID, l.InputTokens, l.OutputTokens, l.Cost, l.Multiplier,
		l.StatusCode, success, l.LatencyMs, l.IP, l.ErrorMessage, Now())
	return err
}

func (s *Store) Count() (groups, users, keys, channels, models int, err error) {
	err = s.DB.QueryRow(`SELECT
(SELECT COUNT(1) FROM user_group),
(SELECT COUNT(1) FROM user),
(SELECT COUNT(1) FROM api_key),
(SELECT COUNT(1) FROM channel),
(SELECT COUNT(1) FROM model_config)`).Scan(&groups, &users, &keys, &channels, &models)
	return
}

func (s *Store) CreateChannel(c Channel) (int64, error) {
	now := Now()
	res, err := s.DB.Exec(`INSERT INTO channel(name,provider,base_url,api_key_enc,weight,priority,status,timeout_ms,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?)`, c.Name, c.Provider, c.BaseURL, c.APIKeyEnc, c.Weight, c.Priority, c.Status, c.TimeoutMs, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CreateModel(m ModelConfig) (int64, error) {
	now := Now()
	stream, en := 0, 0
	if m.SupportsStream {
		stream = 1
	}
	if m.Enabled {
		en = 1
	}
	res, err := s.DB.Exec(`INSERT INTO model_config(model_name,display_name,billing_mode,input_price_per_1k,output_price_per_1k,price_per_request,max_tokens,supports_stream,enabled,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`, m.ModelName, m.DisplayName, m.BillingMode, m.InputPricePer1K, m.OutputPricePer1K, m.PricePerRequest, m.MaxTokens, stream, en, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) BindModelChannel(modelID, channelID int64, upstream string, weight int) error {
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO model_channel(model_config_id,channel_id,upstream_model,weight,enabled)
VALUES(?,?,?,?,1)`, modelID, channelID, upstream, weight)
	return err
}

func (s *Store) BindGroupModel(groupID, modelID int64) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO group_model(group_id, model_config_id) VALUES(?,?)`, groupID, modelID)
	return err
}

type InsufficientBalanceError struct {
	Balance float64
	Need    float64
}

func (e InsufficientBalanceError) Error() string {
	return "insufficient balance"
}

func fmtInsufficientBalance(balance, need float64) error {
	return InsufficientBalanceError{Balance: balance, Need: need}
}

func parseJSONStringSlice(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

func IsExpired(nullStr sql.NullString) bool {
	if !nullStr.Valid || nullStr.String == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, nullStr.String)
	if err != nil {
		t, err = time.Parse(time.RFC3339, nullStr.String)
		if err != nil {
			return false
		}
	}
	return time.Now().After(t)
}

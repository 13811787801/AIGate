package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
)

// SeedResult 首次初始化结果（明文 Key 仅此时可知）
type SeedResult struct {
	DemoAPIKey string
	Created    bool
}

func (s *Store) SeedIfEmpty(adminUser, adminPass string) (*SeedResult, error) {
	_, users, keys, _, _, err := s.Count()
	if err != nil {
		return nil, err
	}
	if users > 0 || keys > 0 {
		return &SeedResult{Created: false}, nil
	}

	now := Now()
	if _, err := s.DB.Exec(`INSERT INTO user_group(id,name,multiplier,description,created_at) VALUES(1,'default',1.0,'默认分组',?)`, now); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`INSERT INTO ` + tblUser + `(username,password_hash,display_name,group_id,status,balance,frozen,total_consumed,role,created_at,updated_at)
VALUES(?,?,?,1,1,100,0,0,'admin',?,?)`, adminUser, hashPassword(adminPass), "Administrator", now, now)
	if err != nil {
		return nil, err
	}
	userID, _ := res.LastInsertId()

	plainKey, keyHash, prefix, err := NewAPIKeyPlain()
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`INSERT INTO api_key(user_id,name,key_hash,key_prefix,group_id,quota_total,quota_used,rpm_limit,tpm_limit,status,created_at)
VALUES(?,?,?,?,1,NULL,0,120,200000,1,?)`, userID, "demo-key", keyHash, prefix, now); err != nil {
		return nil, err
	}

	// mock 渠道：无上游也能验收网关链路
	chID, err := s.CreateChannel(Channel{
		Name: "mock-local", Provider: "mock", BaseURL: "", APIKeyEnc: "",
		Weight: 100, Priority: 10, Status: 1, TimeoutMs: 30000,
	})
	if err != nil {
		return nil, err
	}
	modelID, err := s.CreateModel(ModelConfig{
		ModelName: "mock-chat", DisplayName: "Mock Chat (offline)", BillingMode: "TOKEN",
		InputPricePer1K: 0.001, OutputPricePer1K: 0.002, MaxTokens: 2048, SupportsStream: true, Enabled: true,
	})
	if err != nil {
		return nil, err
	}
	if err := s.BindModelChannel(modelID, chID, "mock-chat", 100); err != nil {
		return nil, err
	}
	if err := s.BindGroupModel(1, modelID); err != nil {
		return nil, err
	}

	log.Printf("seeded demo data: user=%s balance=100 model=mock-chat", adminUser)
	return &SeedResult{DemoAPIKey: plainKey, Created: true}, nil
}

func hashPassword(p string) string {
	sum := sha256.Sum256([]byte("aigate:" + p))
	return hex.EncodeToString(sum[:])
}

func HashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func NewAPIKeyPlain() (plain, hash, prefix string, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return
	}
	plain = "sk-" + hex.EncodeToString(b)
	hash = HashAPIKey(plain)
	prefix = plain[:8]
	return
}

// EnsureDemoModelExists is noop helper for tests.
func (s *Store) EnsureDemoModelExists() error {
	_, err := s.GetModelByName("mock-chat")
	if err == ErrNotFound {
		return fmt.Errorf("mock-chat missing; delete data/aigate.db to re-seed")
	}
	return err
}

var _ = sql.ErrNoRows

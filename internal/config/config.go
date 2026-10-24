package config

import (
	"crypto/sha256"
	"fmt"
	"os"

	"aigate/internal/modules"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	Auth       AuthConfig       `yaml:"auth"`
	Billing    BillingConfig    `yaml:"billing"`
	Plugin     PluginConfig     `yaml:"plugin"`
	LocalModel LocalModelConfig `yaml:"local_model"`
	Log        LogConfig        `yaml:"log"`
	Modules    modules.Flags    `yaml:"modules"`
	Deploy     DeployConfig     `yaml:"deploy"`
	License    LicenseConfig    `yaml:"license"`
	Security   SecurityConfig   `yaml:"security"`
}

type DeployConfig struct {
	// Mode: onprem = 自己的服务器，管理组织内账号；saas = 套餐与订阅。
	Mode string `yaml:"mode"`
	ProductName string `yaml:"product_name"`
}

type LicenseConfig struct {
	File         string `yaml:"file"`           // path to signed license.json
	PublicKeyB64 string `yaml:"public_key_b64"` // ed25519 public key (base64)
	Require      bool   `yaml:"require"`        // if true, refuse start when license invalid
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type DatabaseConfig struct {
	Driver     string `yaml:"driver"`
	SQLitePath string `yaml:"sqlite_path"`
	DSN        string `yaml:"dsn"`
}

type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type AuthConfig struct {
	JWTSecret      string `yaml:"jwt_secret"`
	AdminUsername  string `yaml:"admin_username"`
	AdminPassword  string `yaml:"admin_password"`
	// BootstrapToken: Phase1 X-Admin-Token. Empty = disabled (recommended).
	BootstrapToken string `yaml:"bootstrap_token"`
}

type SecurityConfig struct {
	// DataKey: passphrase for AES-GCM channel secrets (derived via SHA-256).
	DataKey string `yaml:"data_key"`
	// AllowInsecure: if true, skip weak-secret hard fail even in release (not recommended).
	AllowInsecure bool `yaml:"allow_insecure"`
}

type BillingConfig struct {
	FreezeMargin float64 `yaml:"freeze_margin"`
}

type PluginConfig struct {
	Enabled   bool   `yaml:"enabled"`
	BaseURL   string `yaml:"base_url"`
	TimeoutMs int    `yaml:"timeout_ms"`
	FailOpen  bool   `yaml:"fail_open"`
}

type LocalModelConfig struct {
	HealthPath string `yaml:"health_path"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Start with product defaults so omitted modules.* keep SKU defaults;
	// explicit false in YAML still overrides.
	cfg := Config{
		Modules: modules.Default(),
		Deploy:  DeployConfig{Mode: "onprem", ProductName: "AIGate"},
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 3000
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Database.SQLitePath == "" {
		c.Database.SQLitePath = "./data/aigate.db"
	}
	if c.Billing.FreezeMargin <= 0 {
		c.Billing.FreezeMargin = 1.2
	}
	if c.Plugin.TimeoutMs <= 0 {
		c.Plugin.TimeoutMs = 200
	}
	if c.LocalModel.HealthPath == "" {
		c.LocalModel.HealthPath = "/v1/models"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.File == "" {
		c.Log.File = "./logs/aigate.log"
	}
	if c.Deploy.Mode == "" {
		c.Deploy.Mode = "onprem"
	}
	if c.Deploy.ProductName == "" {
		c.Deploy.ProductName = "AIGate"
	}
}

func (c *Config) validate() error {
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required")
	}
	if c.Auth.AdminUsername == "" {
		return fmt.Errorf("auth.admin_username is required")
	}
	return nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// WeakSecrets reports insecure defaults that must not ship to production.
func (c *Config) WeakSecrets() []string {
	var out []string
	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "CHANGE_ME_TO_A_LONG_RANDOM_STRING" || len(c.Auth.JWTSecret) < 16 {
		out = append(out, "auth.jwt_secret is weak or default")
	}
	if c.Auth.AdminPassword == "" || c.Auth.AdminPassword == "admin123" {
		out = append(out, "auth.admin_password is default/empty (admin123)")
	}
	if c.Security.DataKey == "" || c.Security.DataKey == "CHANGE_ME_DATA_KEY" || len(c.Security.DataKey) < 16 {
		out = append(out, "security.data_key is weak or default")
	}
	return out
}

// EnforceSecurity fails startup in release when secrets are weak (unless allow_insecure).
func (c *Config) EnforceSecurity() error {
	weak := c.WeakSecrets()
	if len(weak) == 0 {
		return nil
	}
	if c.Server.Mode == "release" && !c.Security.AllowInsecure {
		return fmt.Errorf("insecure config in release mode: %v (set strong secrets or security.allow_insecure: true for emergency)", weak)
	}
	return nil
}

func (c *Config) DataKeyBytes() []byte {
	key := c.Security.DataKey
	if key == "" {
		// Dev fallback: derive from jwt_secret so local works; release must set data_key.
		key = c.Auth.JWTSecret
	}
	sum := sha256.Sum256([]byte("aigate-data:" + key))
	return sum[:]
}

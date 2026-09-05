package config

import (
	"fmt"
	"os"

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
	JWTSecret     string `yaml:"jwt_secret"`
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
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
	var cfg Config
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
}

func (c *Config) validate() error {
	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "CHANGE_ME_TO_A_LONG_RANDOM_STRING" {
		// 允许开发启动，但打印警告由调用方处理；这里仅硬性要求非空
		if c.Auth.JWTSecret == "" {
			return fmt.Errorf("auth.jwt_secret is required")
		}
	}
	if c.Auth.AdminUsername == "" {
		return fmt.Errorf("auth.admin_username is required")
	}
	return nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

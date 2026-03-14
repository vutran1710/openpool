package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Auth    AuthConfig    `toml:"auth"`
	User    UserConfig    `toml:"user"`
	Server  ServerConfig  `toml:"server"`
}

type AuthConfig struct {
	Token string `toml:"token"`
}

type UserConfig struct {
	PublicID    string `toml:"public_id"`
	DisplayName string `toml:"display_name"`
}

type ServerConfig struct {
	BackendURL   string `toml:"backend_url"`
	SupabaseURL  string `toml:"supabase_url"`
	SupabaseKey  string `toml:"supabase_anon_key"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dating")
}

func Path() string {
	return filepath.Join(Dir(), "config.toml")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(Path(), data, 0600)
}

func (c *Config) IsLoggedIn() bool {
	return c.Auth.Token != ""
}

func (c *Config) Clear() error {
	c.Auth = AuthConfig{}
	c.User = UserConfig{}
	return c.Save()
}

func defaultConfig() *Config {
	cfg := &Config{}
	cfg.applyDefaults()
	return cfg
}

func (c *Config) applyDefaults() {
	if c.Server.BackendURL == "" {
		c.Server.BackendURL = "http://localhost:8080"
	}
}

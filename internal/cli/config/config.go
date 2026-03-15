package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	User           UserConfig   `toml:"user"`
	Pools          []PoolConfig `toml:"pools"`
	Active         string       `toml:"active_pool"`
	Registries     []string     `toml:"registries,omitempty"`
	ActiveRegistry string       `toml:"active_registry,omitempty"`
}

type UserConfig struct {
	PublicID       string `toml:"public_id"`
	DisplayName    string `toml:"display_name"`
	Provider       string `toml:"provider"`
	ProviderUserID string `toml:"provider_user_id"`
}

type PoolConfig struct {
	Name           string `toml:"name"`
	Repo           string `toml:"repo"`
	OperatorPubKey string `toml:"operator_public_key"`
	RelayURL       string `toml:"relay_url,omitempty"`
	Status         string `toml:"status,omitempty"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dating")
}

func KeysDir() string {
	return filepath.Join(Dir(), "keys")
}

func Path() string {
	return filepath.Join(Dir(), "setting.toml")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
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

func (c *Config) IsRegistered() bool {
	return c.User.PublicID != ""
}

func (c *Config) ActivePool() *PoolConfig {
	for i := range c.Pools {
		if c.Pools[i].Name == c.Active {
			return &c.Pools[i]
		}
	}
	return nil
}

func (c *Config) AddPool(pool PoolConfig) {
	for i, p := range c.Pools {
		if p.Name == pool.Name {
			c.Pools[i] = pool
			return
		}
	}
	c.Pools = append(c.Pools, pool)
}

func (c *Config) AddRegistry(repo string) {
	for _, r := range c.Registries {
		if r == repo {
			return
		}
	}
	c.Registries = append(c.Registries, repo)
}

func (c *Config) RemoveRegistry(repo string) bool {
	for i, r := range c.Registries {
		if r == repo {
			c.Registries = append(c.Registries[:i], c.Registries[i+1:]...)
			if c.ActiveRegistry == repo {
				c.ActiveRegistry = ""
			}
			return true
		}
	}
	return false
}

func (c *Config) RemovePool(name string) bool {
	for i, p := range c.Pools {
		if p.Name == name {
			c.Pools = append(c.Pools[:i], c.Pools[i+1:]...)
			if c.Active == name {
				c.Active = ""
			}
			return true
		}
	}
	return false
}

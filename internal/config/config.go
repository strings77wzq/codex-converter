package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Server struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

type Provider struct {
	Name       string `toml:"name"`
	BaseURL    string `toml:"base_url"`
	Model      string `toml:"model"`
	APIKeyEnv  string `toml:"api_key_env"`
	AuthStyle  string `toml:"auth_style"`
}

type Config struct {
	Server          Server     `toml:"server"`
	Providers       []Provider `toml:"providers"`
	DefaultProvider string     `toml:"default_provider"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: Server{
			Port: 8080,
			Host: "127.0.0.1",
		},
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	// Set default auth_style
	for i := range cfg.Providers {
		if cfg.Providers[i].AuthStyle == "" {
			cfg.Providers[i].AuthStyle = "bearer"
		}
	}

	return cfg, nil
}

func (c *Config) GetAPIKey(providerIndex int) (string, error) {
	if providerIndex < 0 || providerIndex >= len(c.Providers) {
		return "", fmt.Errorf("invalid provider index: %d", providerIndex)
	}

	p := c.Providers[providerIndex]
	if p.APIKeyEnv == "" {
		return "", nil
	}

	key := os.Getenv(p.APIKeyEnv)
	if key == "" {
		return "", fmt.Errorf("environment variable %s not set", p.APIKeyEnv)
	}

	return key, nil
}

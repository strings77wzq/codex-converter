package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Server struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

type Provider struct {
	Name          string `toml:"name"`
	BaseURL       string `toml:"base_url"`
	Model         string `toml:"model"`
	APIKey        string `toml:"api_key"`
	APIKeyEnv     string `toml:"api_key_env"`
	AuthStyle     string `toml:"auth_style"`
	ContextWindow int    `toml:"context_window"` // optional, synced to Codex config
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

// SyncCodexConfig intelligently syncs model settings from converter config to ~/.codex/config.toml.
// It only overwrites top-level model/model_provider fields when Codex is currently set to use
// codex-converter (or on first run). When the user has chosen a different provider, only the
// [model_providers.codex-converter] section is kept up to date — user's provider choice is respected.
func SyncCodexConfig(cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	codexDir := filepath.Join(homeDir, ".codex")
	codexPath := filepath.Join(codexDir, "config.toml")

	// Read existing or start fresh
	var lines []string
	if data, err := os.ReadFile(codexPath); err == nil {
		lines = splitLines(string(data))
	} else if !os.IsNotExist(err) {
		return err
	}

	// Find current model_provider value
	currentProvider := findKey(lines, "model_provider")
	shouldSync := currentProvider == "" || currentProvider == "codex-converter"

	// Get the default provider's model/config
	if len(cfg.Providers) == 0 {
		return nil // nothing to sync
	}
	provider := cfg.Providers[0]
	if cfg.DefaultProvider != "" {
		for _, p := range cfg.Providers {
			if p.Name == cfg.DefaultProvider {
				provider = p
				break
			}
		}
	}

	// Update top-level fields if user is using the converter
	if shouldSync {
		lines = setKey(lines, "model", strconv.Quote(provider.Model))
		lines = setKey(lines, "model_provider", strconv.Quote("codex-converter"))
		if provider.ContextWindow > 0 {
			lines = setKey(lines, "model_context_window", strconv.Itoa(provider.ContextWindow))
			compactLimit := int(float64(provider.ContextWindow) * 0.9)
			lines = setKey(lines, "model_auto_compact_token_limit", strconv.Itoa(compactLimit))
		}
	}

	// Ensure provider section exists
	providerSection := `[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"`

	if !hasSection(lines, "[model_providers.codex-converter]") {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, splitLines(providerSection)...)
	}

	// Ensure directory exists
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(codexPath, []byte(joinLines(lines)), 0644)
}

// splitLines splits text into lines, preserving empty lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	if len(lines) > 0 {
		b.WriteByte('\n')
	}
	return b.String()
}

// findKey returns the value of a top-level TOML key (stripped of quotes), or "" if not found.
func findKey(lines []string, key string) string {
	prefix := key + " = "
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if after, ok := strings.CutPrefix(trimmed, prefix); ok {
			return strings.Trim(after, "\"")
		}
	}
	return ""
}

// setKey sets a top-level TOML key to value, updating existing or inserting new.
func setKey(lines []string, key, value string) []string {
	prefix := key + " = "
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = prefix + value
			return lines
		}
	}
	// Not found — insert after any existing top-level keys, before first section header
	insertAt := 0
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			insertAt = i
			break
		}
		insertAt = i + 1
	}
	insertLine := prefix + value
	if insertAt >= len(lines) {
		lines = append(lines, insertLine)
	} else {
		lines = append(lines[:insertAt], append([]string{insertLine}, lines[insertAt:]...)...)
	}
	return lines
}

// hasSection checks if a TOML section header exists.
func hasSection(lines []string, section string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == section {
			return true
		}
	}
	return false
}

func (c *Config) GetAPIKey(providerIndex int) (string, error) {
	if providerIndex < 0 || providerIndex >= len(c.Providers) {
		return "", fmt.Errorf("invalid provider index: %d", providerIndex)
	}

	p := c.Providers[providerIndex]

	// Check direct API key first
	if p.APIKey != "" {
		return p.APIKey, nil
	}

	// Fall back to environment variable
	if p.APIKeyEnv == "" {
		return "", nil
	}

	key := os.Getenv(p.APIKeyEnv)
	if key == "" {
		return "", fmt.Errorf("environment variable %s not set", p.APIKeyEnv)
	}

	return key, nil
}

package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Server struct {
	Port      int    `toml:"port"`
	Host      string `toml:"host"`
	MaxBodyMB int    `toml:"max_body_mb"` // default 10
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

	// Set default max_body_mb
	if cfg.Server.MaxBodyMB <= 0 {
		cfg.Server.MaxBodyMB = 10
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
	// #nosec G304 — codexPath is filepath.Join(homeDir, ".codex", "config.toml"), not user input
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

	// Ensure provider section exists with current host/port
	host := cfg.Server.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Server.Port
	if port <= 0 {
		port = 8080
	}
	baseURL := codexBaseURL(host, port)
	section := "[model_providers.codex-converter]"

	if hasSection(lines, section) {
		lines = setKeyInSection(lines, section, "name", strconv.Quote("codex-converter"))
		lines = setKeyInSection(lines, section, "base_url", strconv.Quote(baseURL))
		lines = setKeyInSection(lines, section, "wire_api", strconv.Quote("responses"))
	} else {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, section)
		lines = append(lines, "name = \"codex-converter\"")
		lines = append(lines, "base_url = "+strconv.Quote(baseURL))
		lines = append(lines, "wire_api = \"responses\"")
	}

	// Ensure directory exists
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		return err
	}

	return os.WriteFile(codexPath, []byte(joinLines(lines)), 0600)
}

// codexBaseURL constructs the base_url that the local Codex CLI should use to
// reach the converter. Listen addresses that are not directly connectable
// (0.0.0.0, ::) are mapped to their loopback equivalents.
func codexBaseURL(host string, port int) string {
	connectHost := host
	switch host {
	case "0.0.0.0", "":
		connectHost = "127.0.0.1"
	case "::", "[::]":
		connectHost = "::1"
	}
	return fmt.Sprintf("http://%s", net.JoinHostPort(connectHost, strconv.Itoa(port)))
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
// It only searches lines before the first [section] header to avoid matching
// keys inside provider-specific or MCP sections.
func findKey(lines []string, key string) string {
	prefix := key + " = "
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			return "" // reached first section — top-level key not found
		}
		if after, ok := strings.CutPrefix(trimmed, prefix); ok {
			return strings.Trim(after, "\"")
		}
	}
	return ""
}

// setKey sets a top-level TOML key to value, updating existing or inserting new.
// It only operates on lines before the first [section] header to avoid
// accidentally mutating provider-internal or MCP keys that share the same name.
func setKey(lines []string, key, value string) []string {
	prefix := key + " = "

	// Find the end of the top-level region (first section header)
	topEnd := len(lines)
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			topEnd = i
			break
		}
	}

	// Try to find existing top-level key
	for i := 0; i < topEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = prefix + value
			return lines
		}
	}

	// Not found — insert after any existing top-level keys, before first section header
	insertAt := 0
	for i := 0; i < topEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
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

// setKeyInSection sets a key-value pair inside a TOML section. If the section
// exists, the key is updated (or inserted at the end of the section). If the
// section does not exist, a new section header and the key are appended.
func setKeyInSection(lines []string, section, key, value string) []string {
	prefix := key + " = "

	// Find section start and end
	secStart := -1
	secEnd := len(lines)
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == section {
			secStart = i
			continue
		}
		if secStart >= 0 && strings.HasPrefix(trimmed, "[") {
			secEnd = i
			break
		}
	}

	if secStart < 0 {
		// Section not found: append section header + key
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, section)
		lines = append(lines, prefix+value)
		return lines
	}

	// Search for key inside the section
	for i := secStart + 1; i < secEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = prefix + value
			return lines
		}
	}

	// Key not found in section — insert at end of section
	insertAt := secEnd
	for insertAt > secStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	if insertAt >= len(lines) {
		lines = append(lines, prefix+value)
	} else {
		lines = append(lines[:insertAt], append([]string{prefix + value}, lines[insertAt:]...)...)
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

// NormalizeBaseURL cleans a user-supplied base URL by stripping common path
// suffixes (/v1/chat/completions, /v1, /chat/completions) so callers can
// safely append "/v1/chat/completions" without duplication.
func NormalizeBaseURL(raw string) string {
	u := strings.TrimRight(raw, "/")
	u = strings.TrimSuffix(u, "/v1/chat/completions")
	u = strings.TrimSuffix(u, "/v1")
	u = strings.TrimSuffix(u, "/chat/completions")
	return u
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

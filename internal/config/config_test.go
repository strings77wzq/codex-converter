package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
default_provider = "deepseek"

[server]
port = 9090
host = "0.0.0.0"

[[providers]]
name = "deepseek"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-pro"
api_key_env = "DEEPSEEK_API_KEY"

[[providers]]
name = "mimo"
base_url = "https://api.xiaomimimo.com"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
auth_style = "api_key_header"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify server config
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}

	// Verify providers
	if len(cfg.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(cfg.Providers))
	}

	ds := cfg.Providers[0]
	if ds.Name != "deepseek" {
		t.Errorf("Providers[0].Name = %q, want %q", ds.Name, "deepseek")
	}
	if ds.BaseURL != "https://api.deepseek.com" {
		t.Errorf("Providers[0].BaseURL = %q", ds.BaseURL)
	}
	if ds.Model != "deepseek-v4-pro" {
		t.Errorf("Providers[0].Model = %q", ds.Model)
	}
	if ds.APIKeyEnv != "DEEPSEEK_API_KEY" {
		t.Errorf("Providers[0].APIKeyEnv = %q", ds.APIKeyEnv)
	}

	mimo := cfg.Providers[1]
	if mimo.AuthStyle != "api_key_header" {
		t.Errorf("Providers[1].AuthStyle = %q, want %q", mimo.AuthStyle, "api_key_header")
	}

	// Verify default provider
	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "deepseek")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Minimal config
	configContent := `
[[providers]]
name = "test"
base_url = "http://localhost:8080"
model = "test-model"
api_key_env = "TEST_KEY"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should use defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want default 8080", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, "127.0.0.1")
	}
}

func TestSyncCodexConfig_CreatesNew(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := &Config{
		DefaultProvider: "mimo",
		Providers: []Provider{
			{Name: "mimo", Model: "mimo-v2.5-pro", ContextWindow: 1000000},
		},
	}

	if err := SyncCodexConfig(cfg); err != nil {
		t.Fatalf("SyncCodexConfig() error = %v", err)
	}

	// Verify file was created with correct content
	codexPath := homeDir + "/.codex/config.toml"
	data, err := os.ReadFile(codexPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", codexPath, err)
	}
	content := string(data)

	if !contains(content, `model = "mimo-v2.5-pro"`) {
		t.Error("missing model field")
	}
	if !contains(content, `model_provider = "codex-converter"`) {
		t.Error("missing model_provider field")
	}
	if !contains(content, `model_context_window = 1000000`) {
		t.Error("missing model_context_window field")
	}
	if !contains(content, `[model_providers.codex-converter]`) {
		t.Error("missing provider section header")
	}
	if !contains(content, `base_url = "http://127.0.0.1:8080"`) {
		t.Error("missing provider base_url")
	}
	if !contains(content, `wire_api = "responses"`) {
		t.Error("missing provider wire_api")
	}
}

func TestSyncCodexConfig_PreservesUserProvider(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Pre-create Codex config where user chose a different provider
	codexDir := homeDir + "/.codex"
	os.MkdirAll(codexDir, 0755)
	existing := `model = "gpt-5.4-mini"
model_provider = "codex"
model_context_window = 200000
# some comment
`
	os.WriteFile(codexDir+"/config.toml", []byte(existing), 0644)

	cfg := &Config{
		DefaultProvider: "mimo",
		Providers: []Provider{
			{Name: "mimo", Model: "mimo-v2.5-pro", ContextWindow: 1000000},
		},
	}

	if err := SyncCodexConfig(cfg); err != nil {
		t.Fatalf("SyncCodexConfig() error = %v", err)
	}

	data, _ := os.ReadFile(codexDir + "/config.toml")
	content := string(data)

	// Top-level fields MUST be preserved (user chose codex, not codex-converter)
	if !contains(content, `model = "gpt-5.4-mini"`) {
		t.Error("model was overwritten despite user choosing different provider")
	}
	if !contains(content, `model_provider = "codex"`) {
		t.Error("model_provider was overwritten")
	}
	if !contains(content, `# some comment`) {
		t.Error("comments were destroyed")
	}
	// Provider section should still be updated
	if !contains(content, `[model_providers.codex-converter]`) {
		t.Error("provider section not added")
	}
}

func TestSyncCodexConfig_UpdatesWhenUsingConverter(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	codexDir := homeDir + "/.codex"
	os.MkdirAll(codexDir, 0755)
	existing := `model = "old-model"
model_provider = "codex-converter"
`
	os.WriteFile(codexDir+"/config.toml", []byte(existing), 0644)

	cfg := &Config{
		DefaultProvider: "mimo",
		Providers: []Provider{
			{Name: "mimo", Model: "mimo-v2.5-flash"}, // no context_window → skip
		},
	}

	if err := SyncCodexConfig(cfg); err != nil {
		t.Fatalf("SyncCodexConfig() error = %v", err)
	}

	data, _ := os.ReadFile(codexDir + "/config.toml")
	content := string(data)

	if !contains(content, `model = "mimo-v2.5-flash"`) {
		t.Error("model was NOT updated to match converter config")
	}
	// context_window should NOT appear (not set in config)
	if contains(content, "model_context_window") {
		t.Error("context_window was written despite not being configured")
	}
}

func TestSyncCodexConfig_NoContextWindow(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := &Config{
		DefaultProvider: "test",
		Providers: []Provider{
			{Name: "test", Model: "test-model"}, // no ContextWindow
		},
	}

	if err := SyncCodexConfig(cfg); err != nil {
		t.Fatalf("SyncCodexConfig() error = %v", err)
	}

	data, _ := os.ReadFile(homeDir + "/.codex/config.toml")
	content := string(data)

	if contains(content, "model_context_window") {
		t.Error("context_window must not appear when not configured")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLoadConfig_APIKeyFromEnv(t *testing.T) {
	// Set test env var
	os.Setenv("TEST_API_KEY", "sk-test-123")
	defer os.Unsetenv("TEST_API_KEY")

	cfg := &Config{
		Providers: []Provider{
			{
				Name:      "test",
				BaseURL:   "http://localhost",
				Model:     "test",
				APIKeyEnv: "TEST_API_KEY",
				AuthStyle: "bearer",
			},
		},
	}

	apiKey, err := cfg.GetAPIKey(0)
	if err != nil {
		t.Fatalf("GetAPIKey() error = %v", err)
	}
	if apiKey != "sk-test-123" {
		t.Errorf("GetAPIKey() = %q, want %q", apiKey, "sk-test-123")
	}
}

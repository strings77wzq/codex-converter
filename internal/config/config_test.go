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

func TestLoadConfig_APIKeyFromEnv(t *testing.T) {
	// Set test env var
	os.Setenv("TEST_API_KEY", "sk-test-123")
	defer os.Unsetenv("TEST_API_KEY")

	cfg := &Config{
		Providers: []Provider{
			{
				Name:       "test",
				BaseURL:    "http://localhost",
				Model:      "test",
				APIKeyEnv:  "TEST_API_KEY",
				AuthStyle:  "bearer",
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

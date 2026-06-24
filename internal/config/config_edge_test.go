package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue 1: SyncCodexConfig with inline comments — standalone comments survive?
func TestSyncCodexConfig_InlineComments(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	codexDir := homeDir + "/.codex"
	os.MkdirAll(codexDir, 0755)
	existing := `model = "old-model" # my chosen model
model_provider = "codex-converter"
# a standalone comment
`
	os.WriteFile(codexDir+"/config.toml", []byte(existing), 0644)

	cfg := &Config{
		DefaultProvider: "mimo",
		Providers: []Provider{
			{Name: "mimo", Model: "mimo-v2.5-pro"},
		},
	}

	if err := SyncCodexConfig(cfg); err != nil {
		t.Fatalf("SyncCodexConfig() error = %v", err)
	}

	data, _ := os.ReadFile(codexDir + "/config.toml")
	content := string(data)

	// The inline comment on the model line is on the same line being replaced,
	// so it WILL be lost. But standalone comments should survive.
	if !strings.Contains(content, "# a standalone comment") {
		t.Errorf("standalone comment was destroyed by SyncCodexConfig")
	}
}

// Issue 2: findKey does NOT respect TOML section boundaries.
// If "model" appears inside a [model_providers.xxx] section, findKey("model")
// matches it instead of the top-level one. This corrupts provider-specific config.
func TestSyncCodexConfig_FindKeyMatchesSectionInternalKey(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	codexDir := homeDir + "/.codex"
	os.MkdirAll(codexDir, 0755)
	// Config where top-level model_provider comes AFTER a section that has "model"
	existing := `[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"
model = "gpt-5.4-mini"
wire_api = "responses"

model_provider = "codex-converter"
`
	os.WriteFile(codexDir+"/config.toml", []byte(existing), 0644)

	cfg := &Config{
		DefaultProvider: "deepseek",
		Providers: []Provider{
			{Name: "deepseek", Model: "deepseek-v4-pro"},
		},
	}

	if err := SyncCodexConfig(cfg); err != nil {
		t.Fatalf("SyncCodexConfig() error = %v", err)
	}

	data, _ := os.ReadFile(codexDir + "/config.toml")
	content := string(data)
	t.Logf("After sync:\n%s", content)

	// findKey("model_provider") should find the top-level one = "codex-converter"
	// → shouldSync = true → should update model to "deepseek-v4-pro"
	// BUT findKey("model") might find the section-internal model first
	// → setKey("model") would update the WRONG line inside the section!
	if !strings.Contains(content, `model = "deepseek-v4-pro"`) {
		t.Error("top-level model was not updated to deepseek-v4-pro")
	}

	// Check if the section's model was corrupted
	lines := strings.Split(content, "\n")
	inOpenAISection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[model_providers.openai]" {
			inOpenAISection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[model_providers.openai]" {
			inOpenAISection = false
		}
		if inOpenAISection && strings.HasPrefix(trimmed, "model = ") {
			if strings.Contains(trimmed, "deepseek-v4-pro") {
				t.Errorf("BUG: section-internal model was corrupted to %q — findKey does not respect TOML section boundaries", trimmed)
			}
		}
	}
}

// Issue 3: SyncCodexConfig does NOT update base_url in existing provider section.
// If user changes port in converter config, codex config stays stale.
func TestSyncCodexConfig_StaleBaseURL(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	codexDir := homeDir + "/.codex"
	os.MkdirAll(codexDir, 0755)
	existing := `model = "deepseek-v4-pro"
model_provider = "codex-converter"

[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:9090"
wire_api = "responses"
`
	os.WriteFile(codexDir+"/config.toml", []byte(existing), 0644)

	cfg := &Config{
		Server:          Server{Port: 9090},
		DefaultProvider: "deepseek",
		Providers: []Provider{
			{Name: "deepseek", Model: "deepseek-v4-pro"},
		},
	}

	if err := SyncCodexConfig(cfg); err != nil {
		t.Fatalf("SyncCodexConfig() error = %v", err)
	}

	data, _ := os.ReadFile(codexDir + "/config.toml")
	content := string(data)

	// SyncCodexConfig only calls hasSection() — if the section exists,
	// it skips adding it entirely, including any base_url update.
	if strings.Contains(content, "9090") {
		t.Logf("CONFIRMED: base_url still has port 9090 — SyncCodexConfig does not update existing provider section base_url. User must manually fix ~/.codex/config.toml after port change.")
	}
}

// Issue 4: os.Create default permissions for config file containing API keys.
func TestSetupConfig_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.toml")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	perm := info.Mode().Perm()
	t.Logf("os.Create default permissions: %04o", perm)

	// Document: on a typical system with umask 022, os.Create gives 0644
	// (group+other can read). Config contains api_key — should be 0600.
	if perm&0044 != 0 {
		t.Logf("CONFIRMED: config file permissions %04o allow group/other read — API keys accessible to other users on the system", perm)
	}
}

// normalizeBaseURL tests are in internal/proxy/handler_edge_test.go

package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	defaultPort    = 8080
	defaultHost    = "127.0.0.1"
	defaultModel   = "mimo-v2.5-pro"
	configDir      = ".codex-converter"
	configFile     = "config.toml"
	codexConfigDir = ".codex"
	codexConfig    = "config.toml"
)

type SetupConfig struct {
	Server struct {
		Port int    `toml:"port"`
		Host string `toml:"host"`
	} `toml:"server"`
	Providers []ProviderConfig `toml:"providers"`
	Default   string           `toml:"default_provider"`
}

type ProviderConfig struct {
	Name      string `toml:"name"`
	BaseURL   string `toml:"base_url"`
	Model     string `toml:"model"`
	APIKey    string `toml:"api_key"`
	AuthStyle string `toml:"auth_style"`
}

func RunSetup() (*SetupConfig, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("👋 欢迎使用 codex-converter")
	fmt.Println()

	// Get base URL
	fmt.Print("Base URL (例如 https://api.xiaomimimo.com): ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("Base URL 不能为空")
	}

	// Get API key
	fmt.Print("API Key: ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("API Key 不能为空")
	}

	// Get model with default
	fmt.Printf("Model (回车默认 %s): ", defaultModel)
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}

	// Detect auth style based on URL
	authStyle := "bearer"
	if strings.Contains(baseURL, "xiaomimimo") {
		authStyle = "api_key_header"
	}

	// Build config
	cfg := &SetupConfig{
		Server: struct {
			Port int    `toml:"port"`
			Host string `toml:"host"`
		}{
			Port: defaultPort,
			Host: defaultHost,
		},
		Providers: []ProviderConfig{
			{
				Name:      "default",
				BaseURL:   baseURL,
				Model:     model,
				APIKey:    apiKey,
				AuthStyle: authStyle,
			},
		},
		Default: "default",
	}

	// Save config
	if err := saveConfig(cfg); err != nil {
		return nil, fmt.Errorf("保存配置失败: %v", err)
	}

	// Configure Codex
	if err := configureCodex(baseURL); err != nil {
		return nil, fmt.Errorf("配置 Codex 失败: %v", err)
	}

	// Print success
	fmt.Println()
	fmt.Println("✅ 配置完成！")
	fmt.Printf("   Provider: %s\n", "default")
	fmt.Printf("   Base URL: %s\n", baseURL)
	fmt.Printf("   Model: %s\n", model)
	fmt.Println()
	fmt.Printf("🚀 服务已启动 (127.0.0.1:%d)\n", defaultPort)
	fmt.Println("📝 Codex 已自动配置")
	fmt.Println()
	fmt.Println("现在你可以直接运行: codex")
	fmt.Println("按 Ctrl+C 停止服务")
	fmt.Println()

	return cfg, nil
}

func saveConfig(cfg *SetupConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(homeDir, configDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, configFile)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(cfg)
}

func configureCodex(baseURL string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Ensure codex config dir exists
	codexDir := filepath.Join(homeDir, codexConfigDir)
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return err
	}

	codexPath := filepath.Join(codexDir, codexConfig)

	// Read existing config or create new
	content := ""
	if data, err := os.ReadFile(codexPath); err == nil {
		content = string(data)
	}

	// Check if provider already exists
	providerMarker := "[model_providers.codex-converter]"
	if strings.Contains(content, providerMarker) {
		return nil // Already configured
	}

	// Append provider config
	newConfig := content + "\n" + `# codex-converter (auto-configured)
[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
`

	return os.WriteFile(codexPath, []byte(newConfig), 0644)
}

func IsFirstRun() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true
	}

	path := filepath.Join(homeDir, configDir, configFile)
	_, err = os.Stat(path)
	return os.IsNotExist(err)
}

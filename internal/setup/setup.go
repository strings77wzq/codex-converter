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

// Colors
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
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

func printBanner() {
	fmt.Println()
	fmt.Printf("%s╔══════════════════════════════════════════════════╗%s\n", colorCyan, colorReset)
	fmt.Printf("%s║%s        %scodex-converter%s v1.0                  %s║%s\n", colorCyan, colorReset, colorBold, colorReset, colorCyan, colorReset)
	fmt.Printf("%s║%s    让 Codex 支持所有 Chat Completions 模型      %s║%s\n", colorCyan, colorReset, colorCyan, colorReset)
	fmt.Printf("%s╚══════════════════════════════════════════════════╝%s\n", colorCyan, colorReset)
	fmt.Println()
}

func printStep(step int, total int, msg string) {
	fmt.Printf("%s[%d/%d]%s %s\n", colorYellow, step, total, colorReset, msg)
}

func printSuccess(msg string) {
	fmt.Printf("%s✓ %s%s\n", colorGreen, msg, colorReset)
}

func printInfo(msg string) {
	fmt.Printf("%s→ %s%s\n", colorBlue, msg, colorReset)
}

func printError(msg string) {
	fmt.Printf("%s✗ %s%s\n", colorRed, msg, colorReset)
}

func RunSetup() (*SetupConfig, error) {
	reader := bufio.NewReader(os.Stdin)

	printBanner()

	fmt.Printf("%s 首次运行，开始配置...%s\n", colorCyan, colorReset)
	fmt.Println()

	// Step 1: Base URL
	printStep(1, 3, "输入你的 LLM Provider 信息")
	fmt.Println()
	fmt.Printf("  %sBase URL%s (例如 https://api.xiaomimimo.com): ", colorBold, colorReset)
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("Base URL 不能为空")
	}
	printSuccess(fmt.Sprintf("Base URL: %s", baseURL))
	fmt.Println()

	// Step 2: API Key
	fmt.Printf("  %sAPI Key%s: ", colorBold, colorReset)
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("API Key 不能为空")
	}
	// Mask key for display
	maskedKey := apiKey
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
	}
	printSuccess(fmt.Sprintf("API Key: %s", maskedKey))
	fmt.Println()

	// Step 3: Model
	fmt.Printf("  %sModel%s (回车默认 %s): ", colorBold, colorReset, defaultModel)
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}
	printSuccess(fmt.Sprintf("Model: %s", model))
	fmt.Println()

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
	printInfo("保存配置...")
	if err := saveConfig(cfg); err != nil {
		printError(fmt.Sprintf("保存配置失败: %v", err))
		return nil, err
	}
	printSuccess(fmt.Sprintf("配置已保存到 ~/.codex-converter/config.toml"))

	// Configure Codex
	printInfo("配置 Codex...")
	if err := configureCodex(baseURL); err != nil {
		printError(fmt.Sprintf("配置 Codex 失败: %v", err))
		return nil, err
	}
	printSuccess("Codex 已自动配置 (~/.codex/config.toml)")

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("  %s🚀 服务已启动 %s(127.0.0.1:%d)%s\n", colorGreen, colorBold, defaultPort, colorReset)
	fmt.Println()
	fmt.Println("  %s现在你可以直接运行:%s", colorBold, colorReset)
	fmt.Println()
	fmt.Printf("    %scodex%s\n", colorCyan, colorReset)
	fmt.Println()
	fmt.Println("  %s按 Ctrl+C 停止服务%s", colorYellow, colorReset)
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

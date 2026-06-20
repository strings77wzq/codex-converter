package setup

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	defaultPort    = 8080
	defaultHost    = "127.0.0.1"
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

// Known providers with their models
type ProviderInfo struct {
	Name      string
	BaseURL   string
	AuthStyle string // "bearer" or "api_key_header"
	Models    []ModelInfo
}

type ModelInfo struct {
	Name          string
	ContextWindow int // in tokens
	Description   string
}

var knownProviders = []ProviderInfo{
	{
		Name:      "deepseek",
		BaseURL:   "https://api.deepseek.com",
		AuthStyle: "bearer",
		Models: []ModelInfo{
			{Name: "deepseek-v4-pro", ContextWindow: 1000000, Description: "Pro版本，推理能力强"},
			{Name: "deepseek-v4-flash", ContextWindow: 1000000, Description: "Flash版本，更快更便宜"},
		},
	},
	{
		Name:      "mimo",
		BaseURL:   "https://api.xiaomimimo.com",
		AuthStyle: "api_key_header",
		Models: []ModelInfo{
			{Name: "mimo-v2.5-pro", ContextWindow: 1000000, Description: "MiMo最新模型，1M上下文"},
		},
	},
	{
		Name:      "qwen",
		BaseURL:   "https://dashscope.aliyuncs.com/compatible-mode",
		AuthStyle: "bearer",
		Models: []ModelInfo{
			{Name: "qwen-max", ContextWindow: 131072, Description: "通义千问最强模型"},
			{Name: "qwen-plus", ContextWindow: 131072, Description: "通义千问均衡模型"},
			{Name: "qwen-turbo", ContextWindow: 131072, Description: "通义千问快速模型"},
		},
	},
	{
		Name:      "glm",
		BaseURL:   "https://open.bigmodel.cn/api/paas",
		AuthStyle: "bearer",
		Models: []ModelInfo{
			{Name: "glm-4-plus", ContextWindow: 128000, Description: "智谱GLM最强模型"},
			{Name: "glm-4", ContextWindow: 128000, Description: "智谱GLM均衡模型"},
			{Name: "glm-4-flash", ContextWindow: 128000, Description: "智谱GLM快速模型"},
		},
	},
	{
		Name:      "moonshot",
		BaseURL:   "https://api.moonshot.cn",
		AuthStyle: "bearer",
		Models: []ModelInfo{
			{Name: "moonshot-v1-128k", ContextWindow: 131072, Description: "月之暗面128K上下文"},
			{Name: "moonshot-v1-32k", ContextWindow: 32768, Description: "月之暗面32K上下文"},
			{Name: "moonshot-v1-8k", ContextWindow: 8192, Description: "月之暗面8K上下文"},
		},
	},
	{
		Name:      "yi",
		BaseURL:   "https://api.lingyiwanwu.com",
		AuthStyle: "bearer",
		Models: []ModelInfo{
			{Name: "yi-large", ContextWindow: 32768, Description: "零一万物最强模型"},
			{Name: "yi-medium", ContextWindow: 32768, Description: "零一万物均衡模型"},
		},
	},
	{
		Name:      "baichuan",
		BaseURL:   "https://api.baichuan-ai.com",
		AuthStyle: "bearer",
		Models: []ModelInfo{
			{Name: "Baichuan4", ContextWindow: 32768, Description: "百川最新模型"},
		},
	},
	{
		Name:      "minimax",
		BaseURL:   "https://api.minimax.chat",
		AuthStyle: "bearer",
		Models: []ModelInfo{
			{Name: "abab6.5s-chat", ContextWindow: 32768, Description: "MiniMax最新模型"},
		},
	},
	{
		Name:      "ollama",
		BaseURL:   "http://localhost:11434/v1",
		AuthStyle: "none",
		Models:    []ModelInfo{}, // 动态获取
	},
	{
		Name:      "custom",
		BaseURL:   "", // 用户自定义
		AuthStyle: "bearer",
		Models:    []ModelInfo{},
	},
}

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

type CodexConfig struct {
	ModelContextWindow         int    `toml:"model_context_window"`
	ModelAutoCompactTokenLimit int    `toml:"model_auto_compact_token_limit"`
	Model                      string `toml:"model"`
	ModelProvider              string `toml:"model_provider"`
}

func printBanner() {
	fmt.Println()
	fmt.Printf("%s╔══════════════════════════════════════════════════════════╗%s\n", colorCyan, colorReset)
	fmt.Printf("%s║%s            %scodex-converter%s v1.0                        %s║%s\n", colorCyan, colorReset, colorBold, colorReset, colorCyan, colorReset)
	fmt.Printf("%s║%s      让 Codex 支持所有 Chat Completions 兼容模型       %s║%s\n", colorCyan, colorReset, colorCyan, colorReset)
	fmt.Printf("%s╚══════════════════════════════════════════════════════════╝%s\n", colorCyan, colorReset)
	fmt.Println()
}

func printStep(step int, total int, msg string) {
	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorYellow, colorReset)
	fmt.Printf("%s  Step %d/%d: %s%s\n", colorYellow, step, total, msg, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", colorYellow, colorReset)
}

func printSuccess(msg string) {
	fmt.Printf("  %s✓ %s%s\n", colorGreen, msg, colorReset)
}

func printInfo(msg string) {
	fmt.Printf("  %s→ %s%s\n", colorBlue, msg, colorReset)
}

func printError(msg string) {
	fmt.Printf("  %s✗ %s%s\n", colorRed, msg, colorReset)
}

func testConnection(baseURL, apiKey, model, authStyle string) error {
	// 清洗base_url，去掉多余的后缀
	cleanURL := strings.TrimRight(baseURL, "/")
	cleanURL = strings.TrimSuffix(cleanURL, "/v1")
	cleanURL = strings.TrimSuffix(cleanURL, "/v1/chat/completions")
	cleanURL = strings.TrimSuffix(cleanURL, "/chat/completions")

	// 构造测试请求
	testURL := cleanURL + "/v1/chat/completions"
	testBody := fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"hi"}],"max_tokens":10}`, model)

	req, err := http.NewRequest("POST", testURL, strings.NewReader(testBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 设置认证头
	if authStyle == "api_key_header" {
		req.Header.Set("api-key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return fmt.Errorf("连接超时: 请检查网络或URL是否正确")
		}
		return fmt.Errorf("连接失败: %v\n  请检查: 1) URL是否正确 2) 网络是否正常", err)
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("认证失败: API Key 无效\n  请检查: 1) Key是否正确复制 2) Key是否过期")
	}

	if resp.StatusCode == 404 {
		return fmt.Errorf("模型不存在: %s\n  请检查模型名称是否正确", model)
	}

	if resp.StatusCode == 429 {
		return fmt.Errorf("请求过于频繁: 请稍后再试")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("请求失败: HTTP %d", resp.StatusCode)
	}

	return nil
}

func RunSetup() (*SetupConfig, error) {
	reader := bufio.NewReader(os.Stdin)

	printBanner()

	fmt.Printf("  %s👋 首次运行，开始配置...%s\n\n", colorCyan, colorReset)

	// Step 1: Select provider
	printStep(1, 4, "选择你的 LLM Provider")

	fmt.Println("  已知的 Provider:")
	fmt.Println()
	for i, p := range knownProviders {
		if p.Name == "custom" {
			fmt.Printf("    %s[%d]%s %s (自定义，输入Base URL)\n", colorCyan, i+1, colorReset, p.Name)
		} else {
			fmt.Printf("    %s[%d]%s %s (%s)\n", colorCyan, i+1, colorReset, p.Name, p.BaseURL)
		}
	}
	fmt.Println()

	var selectedProvider *ProviderInfo
	var baseURL string

	for {
		fmt.Printf("  %s选择 Provider [1-%d]:%s ", colorBold, len(knownProviders), colorReset)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			input = "1" // 默认选择第一个
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(knownProviders) {
			printError(fmt.Sprintf("无效选择，请输入 1-%d", len(knownProviders)))
			continue
		}

		selectedProvider = &knownProviders[choice-1]
		printSuccess(fmt.Sprintf("选择: %s", selectedProvider.Name))
		break
	}

	// Step 2: Base URL (if custom) or API Key
	printStep(2, 4, "配置连接信息")

	if selectedProvider.Name == "custom" {
		fmt.Printf("  %sBase URL:%s ", colorBold, colorReset)
		baseURL, _ = reader.ReadString('\n')
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			return nil, fmt.Errorf("base URL 不能为空")
		}
		printSuccess(fmt.Sprintf("Base URL: %s", baseURL))
	} else {
		baseURL = selectedProvider.BaseURL
		printInfo(fmt.Sprintf("Base URL: %s (自动填充)", baseURL))
	}

	// API Key
	if selectedProvider.AuthStyle != "none" {
		fmt.Printf("  %sAPI Key:%s ", colorBold, colorReset)
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			return nil, fmt.Errorf("API Key 不能为空")
		}
		// Mask key
		maskedKey := apiKey
		if len(apiKey) > 8 {
			maskedKey = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
		}
		printSuccess(fmt.Sprintf("API Key: %s", maskedKey))
	}

	// Step 3: Select model
	printStep(3, 4, "选择模型")

	var models []ModelInfo
	if selectedProvider.Name == "custom" {
		// 自定义provider需要手动输入模型
		fmt.Printf("  %sModel 名称:%s ", colorBold, colorReset)
		modelName, _ := reader.ReadString('\n')
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			return nil, fmt.Errorf("模型名称不能为空")
		}

		fmt.Printf("  %s上下文窗口 (tokens, 默认 128000):%s ", colorBold, colorReset)
		ctxStr, _ := reader.ReadString('\n')
		ctxStr = strings.TrimSpace(ctxStr)
		ctxWindow := 128000
		if ctxStr != "" {
			if v, err := strconv.Atoi(ctxStr); err == nil {
				ctxWindow = v
			}
		}

		models = []ModelInfo{{Name: modelName, ContextWindow: ctxWindow}}
	} else {
		models = selectedProvider.Models
	}

	if len(models) == 0 {
		fmt.Printf("  %sModel 名称:%s ", colorBold, colorReset)
		modelName, _ := reader.ReadString('\n')
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			return nil, fmt.Errorf("模型名称不能为空")
		}
		models = []ModelInfo{{Name: modelName, ContextWindow: 128000}}
	}

	if len(models) == 1 {
		printSuccess(fmt.Sprintf("Model: %s (上下文: %s)", models[0].Name, formatTokens(models[0].ContextWindow)))
	} else {
		fmt.Println("  可用模型:")
		for i, m := range models {
			fmt.Printf("    %s[%d]%s %s - %s (%s)\n", colorCyan, i+1, colorReset, m.Name, m.Description, formatTokens(m.ContextWindow))
		}
		fmt.Println()

		var selectedModel ModelInfo
		for {
			fmt.Printf("  %s选择模型 [1-%d]:%s ", colorBold, len(models), colorReset)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "" {
				input = "1"
			}

			choice, err := strconv.Atoi(input)
			if err != nil || choice < 1 || choice > len(models) {
				printError(fmt.Sprintf("无效选择，请输入 1-%d", len(models)))
				continue
			}

			selectedModel = models[choice-1]
			printSuccess(fmt.Sprintf("Model: %s (上下文: %s)", selectedModel.Name, formatTokens(selectedModel.ContextWindow)))
			models = []ModelInfo{selectedModel}
			break
		}
	}

	// Step 4: Confirm and Test
	printStep(4, 5, "确认配置")

	model := models[0]
	authStyle := selectedProvider.AuthStyle
	if authStyle == "" {
		authStyle = "bearer"
	}

	// Get API key for testing
	var apiKey string
	if selectedProvider.AuthStyle != "none" {
		fmt.Printf("  %sAPI Key:%s ", colorBold, colorReset)
		apiKey, _ = reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			return nil, fmt.Errorf("API Key 不能为空")
		}
		// Mask key
		maskedKey := apiKey
		if len(apiKey) > 8 {
			maskedKey = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
		}
		printSuccess(fmt.Sprintf("API Key: %s", maskedKey))
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
				Name:      selectedProvider.Name,
				BaseURL:   baseURL,
				Model:     model.Name,
				APIKey:    apiKey,
				AuthStyle: authStyle,
			},
		},
		Default: selectedProvider.Name,
	}

	fmt.Println()
	fmt.Printf("  %s配置摘要:%s\n", colorBold, colorReset)
	fmt.Println("  ┌─────────────────────────────────────────────────────┐")
	fmt.Printf("  │ Provider: %-42s │\n", selectedProvider.Name)
	fmt.Printf("  │ Base URL: %-42s │\n", baseURL)
	fmt.Printf("  │ Model:    %-42s │\n", model.Name)
	fmt.Printf("  │ 上下文:   %-42s │\n", formatTokens(model.ContextWindow))
	fmt.Println("  └─────────────────────────────────────────────────────┘")
	fmt.Println()

	// Step 5: Test Connection
	printStep(5, 5, "测试连接")

	for {
		printInfo("正在测试连接...")
		if err := testConnection(baseURL, apiKey, model.Name, authStyle); err != nil {
			printError(err.Error())
			fmt.Println()
			fmt.Printf("  %s是否重新配置? (y/n):%s ", colorBold, colorReset)
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(strings.ToLower(choice))

			if choice != "y" && choice != "yes" {
				return nil, fmt.Errorf("用户取消配置")
			}

			// 重新输入Base URL
			fmt.Printf("  %sBase URL:%s ", colorBold, colorReset)
			baseURL, _ = reader.ReadString('\n')
			baseURL = strings.TrimSpace(baseURL)
			if baseURL == "" {
				return nil, fmt.Errorf("base URL 不能为空")
			}
			printSuccess(fmt.Sprintf("Base URL: %s", baseURL))

			// 重新输入API Key
			if selectedProvider.AuthStyle != "none" {
				fmt.Printf("  %sAPI Key:%s ", colorBold, colorReset)
				apiKey, _ = reader.ReadString('\n')
				apiKey = strings.TrimSpace(apiKey)
				if apiKey == "" {
					return nil, fmt.Errorf("API Key 不能为空")
				}
				maskedKey := apiKey
				if len(apiKey) > 8 {
					maskedKey = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
				}
				printSuccess(fmt.Sprintf("API Key: %s", maskedKey))
			}

			// 重新输入Model
			fmt.Printf("  %sModel:%s ", colorBold, colorReset)
			modelName, _ := reader.ReadString('\n')
			modelName = strings.TrimSpace(modelName)
			if modelName != "" {
				model.Name = modelName
			}
			printSuccess(fmt.Sprintf("Model: %s", model.Name))

			continue
		}
		break
	}

	printSuccess("连接测试通过!")
	fmt.Println()

	// Save config
	printInfo("保存配置...")
	if err := saveConfig(cfg); err != nil {
		printError(fmt.Sprintf("保存配置失败: %v", err))
		return nil, err
	}
	printSuccess("配置已保存到 ~/.codex-converter/config.toml")

	// Configure Codex
	printInfo("配置 Codex...")
	if err := configureCodex(model.Name, model.ContextWindow); err != nil {
		printError(fmt.Sprintf("配置 Codex 失败: %v", err))
		return nil, err
	}
	printSuccess("Codex 已自动配置 (~/.codex/config.toml)")

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("  %s🚀 服务已启动 %s(127.0.0.1:%d)%s\n", colorGreen, colorBold, defaultPort, colorReset)
	fmt.Println()
	fmt.Printf("  %s现在你可以直接运行:%s\n", colorBold, colorReset)
	fmt.Println()
	fmt.Printf("    %scodex%s\n", colorCyan, colorReset)
	fmt.Println()
	fmt.Printf("  %s按 Ctrl+C 停止服务%s\n", colorYellow, colorReset)
	fmt.Println()

	return cfg, nil
}

func formatTokens(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%dM tokens", tokens/1000000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%dK tokens", tokens/1000)
	}
	return fmt.Sprintf("%d tokens", tokens)
}

func saveConfig(cfg *SetupConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(homeDir, configDir)
	if err := os.MkdirAll(dir, 0750); err != nil {
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

func configureCodex(modelName string, contextWindow int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Ensure codex config dir exists
	codexDir := filepath.Join(homeDir, codexConfigDir)
	if err := os.MkdirAll(codexDir, 0750); err != nil {
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

	// Build codex config additions
	codexAdditions := fmt.Sprintf(`
# codex-converter (auto-configured)
model = "%s"
model_provider = "codex-converter"
model_context_window = %d
model_auto_compact_token_limit = %d

[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
`, modelName, contextWindow, int(float64(contextWindow)*0.9))

	return os.WriteFile(codexPath, []byte(content+codexAdditions), 0600)
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

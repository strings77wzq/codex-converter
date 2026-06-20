<h1 align="center">codex-converter</h1>

<p align="center">
  <strong>让 Codex 支持所有 Chat Completions 兼容的模型</strong>
</p>

<p align="center">
  中文 | <a href="README.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/strings77wzq/codex-converter">GitHub</a>
</p>

---

codex-converter 是一个轻量级代理，让 Codex 能够使用任何兼容 Chat Completions 的 LLM 提供商。Codex 只支持 Responses API 格式，但大多数提供商（DeepSeek、MiMo、Qwen、Ollama 等）只支持 Chat Completions。这个转换器完美解决了这个问题。

---

## 快速开始

```bash
# 安装
go install github.com/strings77wzq/codex-converter@latest

# 运行（首次启动会进入交互式配置）
codex-converter
```

首次运行会引导你完成配置：
1. 输入提供商的 Base URL
2. 输入 API Key
3. 输入模型名称（或按回车使用默认值）

就这样！转换器会自动配置 Codex 并启动服务。

---

## 工作原理

```
Codex ──→ codex-converter ──→ 你的 LLM 提供商
         (Responses → Chat)   (DeepSeek/MiMo/Qwen/...)
```

1. Codex 发送 Responses API 格式的请求
2. codex-converter 转换为 Chat Completions 格式
3. 你的提供商处理请求
4. codex-converter 将响应转换回 Responses 格式
5. Codex 无缝接收响应

---

## 功能特性

- ✅ **流式输出** — 逐字实时返回
- ✅ **Tool Calls** — 完整支持函数调用（Codex agent 必需）
- ✅ **任意提供商** — 兼容任何 Chat Completions API
- ✅ **交互式配置** — 首次运行向导，无需手动编辑配置
- ✅ **自动配置 Codex** — 自动配置 `~/.codex/config.toml`
- ✅ **单文件部署** — 无依赖，一个可执行文件搞定

---

## 支持的提供商

任何兼容 Chat Completions API 的提供商都可以使用：

| 提供商 | Base URL | 说明 |
|--------|----------|------|
| **DeepSeek** | `https://api.deepseek.com` | `deepseek-v4-pro`, `deepseek-v4-flash` |
| **MiMo** | `https://api.xiaomimimo.com` | `mimo-v2.5-pro`，使用 `api-key` 头 |
| **Qwen** | `https://dashscope.aliyuncs.com/compatible-mode` | `qwen-max`, `qwen-plus` |
| **Ollama** | `http://localhost:11434/v1` | 本地模型，无需 API Key |
| **vLLM** | `http://localhost:8000/v1` | 自托管模型 |
| **任意 OpenAI 兼容** | 你的提供商 URL | 填入信息即可 |

---

## 配置说明

### 首次运行

```bash
codex-converter
```

交互式向导会询问：
- **Base URL** — 提供商的 API 端点
- **API Key** — 认证密钥
- **Model** — 模型名称（默认：`mimo-v2.5-pro`）

### 后续运行

```bash
codex-converter
# 自动从 ~/.codex-converter/config.toml 加载配置
```

### 手动配置

编辑 `~/.codex-converter/config.toml`：

```toml
default_provider = "default"

[server]
port = 8080
host = "127.0.0.1"

[[providers]]
name = "default"
base_url = "https://api.xiaomimimo.com"
model = "mimo-v2.5-pro"
api_key = "your-api-key"
auth_style = "api_key_header"
```

---

## Codex 集成

运行 `codex-converter` 后，它会自动在 `~/.codex/config.toml` 中添加：

```toml
[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
```

然后直接运行 Codex：

```bash
codex --model mimo-v2.5-pro --model-provider codex-converter
```

---

## 从源码构建

```bash
git clone https://github.com/strings77wzq/codex-converter.git
cd codex-converter
go build -o codex-converter .
```

---

## Docker 部署

```bash
docker build -t codex-converter .
docker run -p 8080:8080 codex-converter
```

---

## 常见问题

**Q: 为什么需要这个工具？**
A: Codex 只支持 Responses API，但大多数 LLM 提供商只支持 Chat Completions。这个转换器完美解决了这个问题。

**Q: Codex 的所有功能都能用吗？**
A: 是的！代码生成、tool calls、流式输出——所有核心功能都能用。只是 OpenAI 专有的内置工具（web search、computer use）无法使用。

**Q: API Key 安全吗？**
A: Key 存储在本地 `~/.codex-converter/config.toml` 中，除了转发请求到你的提供商外，不会离开你的机器。

---

## 许可证

MIT 许可证

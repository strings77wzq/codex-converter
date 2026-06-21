<h1 align="center">codex-converter</h1>

<p align="center">
  <strong>让 Codex 用上任何 Chat Completions 模型 — 一行配置，零改代码</strong>
</p>

<p align="center">
  中文 | <a href="README.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/strings77wzq/codex-converter/releases"><img src="https://img.shields.io/github/v/release/strings77wzq/codex-converter" alt="Release"></a>
  <a href="https://github.com/strings77wzq/codex-converter/blob/master/LICENSE"><img src="https://img.shields.io/github/license/strings77wzq/codex-converter" alt="License"></a>
</p>

---

Codex 只支持 [Responses API](https://platform.openai.com/docs/api-reference/responses)，但国内主流 LLM 提供商（DeepSeek、MiMo、Qwen、GLM、Moonshot、Ollama 等）只支持 [Chat Completions](https://platform.openai.com/docs/api-reference/chat)。**codex-converter** 是一个轻量级本地代理，在两者之间做协议转换，让你无需修改 Codex 或提供商代码，就能直接使用。

---

## 快速开始

```bash
# 1. 安装
go install github.com/strings77wzq/codex-converter@latest

# 2. 运行（首次启动进入交互式配置向导）
codex-converter
```

### 配置向导流程

首次运行时，向导引导你完成 5 步：

| 步骤 | 你做什么 | 示例 |
|------|---------|------|
| 1. 选择提供商 | 从 10+ 内置提供商中选择，或选 "Custom" | `1`（DeepSeek） |
| 2. 输入 API Key | 粘贴提供商的 API Key | `sk-xxx...` |
| 3. 选择模型 | 从提供商的模型列表中选择 | `deepseek-v4-pro` |
| 4. 确认配置 | 检查配置摘要 | — |
| 5. 测试连接 | 向导验证 API 是否可用 | ✓ 连接成功 |

向导会写入两个文件：

| 文件 | 作用 | 谁来编辑 |
|------|------|---------|
| `~/.codex-converter/config.toml` | 转换器配置（提供商、API Key、模型） | 你（或向导） |
| `~/.codex/config.toml` | Codex 配置（转换器自动管理） | 转换器 + 你 |

配置完成后，直接运行 `codex` 即可 — 请求会自动经过转换器转发到你的提供商。

---

## 工作原理

```
                          codex-converter (localhost:8080)
                        ┌─────────────────────────────┐
                        │                             │
Codex ──Responses API──▶│  Responses → Chat Completions│──▶ 你的 LLM 提供商
                        │  Chat Completions → Responses │    (DeepSeek/MiMo/...)
◀──Responses API────────│                             │◀── Chat Completions
                        │                             │
                        └─────────────────────────────┘
```

1. Codex 发送 **Responses API** 请求到 `localhost:8080`
2. 转换器翻译为 **Chat Completions** 格式，转发给提供商
3. 提供商返回响应（流式或非流式）
4. 转换器将响应翻译回 **Responses API** 格式
5. Codex 收到响应，如同在和原生 Responses 端点对话

---

## 功能特性

| 功能 | 说明 |
|------|------|
| **流式输出** | 逐 token 实时返回，完整的 SSE 事件序列 |
| **Tool Calls** | 完整函数调用支持（Codex agent 必需） |
| **多轮历史** | 完整对话历史转换，包括 `function_call` / `function_call_output` |
| **10+ 提供商** | DeepSeek、MiMo、Qwen、GLM、Moonshot、Yi、百川、MiniMax、Ollama、自定义 |
| **模型自动同步** | 启动时自动同步模型配置到 Codex |
| **`codex --model` 切换** | 临时切模型，无需改配置文件 |
| **配置共存** | 使用其他提供商时不会覆盖你的 Codex 设置 |
| **交互式向导** | 首次运行引导配置，自动测连接 |
| **单二进制** | 零运行时依赖，`go install` 一行安装 |

---

## 支持的提供商

| 提供商 | 认证方式 | 模型 | Base URL |
|--------|---------|------|----------|
| **DeepSeek** | `bearer` | `deepseek-v4-pro`、`deepseek-v4-flash` | `https://api.deepseek.com` |
| **MiMo** | `api_key_header` | `mimo-v2.5-pro`、`mimo-v2.5`、`mimo-v2-flash` | `https://api.xiaomimimo.com` |
| **Qwen** | `bearer` | `qwen-max`、`qwen-plus`、`qwen-turbo` | `https://dashscope.aliyuncs.com/compatible-mode` |
| **GLM** | `bearer` | `glm-4-plus`、`glm-4`、`glm-4-flash` | `https://open.bigmodel.cn/api/paas` |
| **Moonshot** | `bearer` | `moonshot-v1-128k`、`moonshot-v1-32k` | `https://api.moonshot.cn` |
| **Yi** | `bearer` | `yi-large`、`yi-medium` | `https://api.lingyiwanwu.com` |
| **百川** | `bearer` | `Baichuan4` | `https://api.baichuan-ai.com` |
| **MiniMax** | `bearer` | `abab6.5s-chat` | `https://api.minimax.chat` |
| **Ollama** | none（本地） | 任意本地模型 | `http://localhost:11434/v1` |
| **自定义** | 可配置 | 你的模型 | 你的 URL |

> **模型名称必须与提供商要求完全一致**。例如 DeepSeek 用 `deepseek-v4-pro`，不能写成 `deepseek-pro` 或 `deepseek-v4`。请查阅提供商文档确认正确的模型名。

### 认证方式（auth_style）

| 方式 | 适用场景 | 工作原理 |
|------|---------|---------|
| `bearer` | 大多数提供商（DeepSeek、Qwen、GLM 等） | 发送 `Authorization: Bearer <key>` 头 |
| `api_key_header` | MiMo、Azure 风格的 API | 发送 `api-key: <key>` 头 |
| `none` | Ollama（本地，无需认证） | 不发送认证头 |

---

## 配置说明

### 两个配置文件

| 文件 | 位置 | 作用 |
|------|------|------|
| **转换器配置** | `~/.codex-converter/config.toml` | 提供商 URL、API Key、模型选择 |
| **Codex 配置** | `~/.codex/config.toml` | Codex 运行时配置（自动管理） |

**你编辑** `~/.codex-converter/config.toml`。转换器**自动管理** `~/.codex/config.toml`。

### 转换器配置（`~/.codex-converter/config.toml`）

```toml
default_provider = "deepseek"

[server]
port = 8080
host = "127.0.0.1"

[[providers]]
name = "deepseek"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-pro"          # 必须与提供商的模型名完全一致
api_key = "your-api-key"           # 或使用下面的 api_key_env
# api_key_env = "DEEPSEEK_API_KEY" # 从环境变量读取 Key
auth_style = "bearer"              # "bearer"（默认）或 "api_key_header"
# context_window = 1000000         # 可选：同步到 Codex 控制 compact 行为

# 添加更多提供商：
# [[providers]]
# name = "mimo"
# base_url = "https://api.xiaomimimo.com"
# model = "mimo-v2.5-pro"
# api_key = "your-mimo-key"
# auth_style = "api_key_header"
```

### Codex 配置（`~/.codex/config.toml`）— 自动配置

转换器会向你的 Codex 配置添加以下内容：

```toml
# codex-converter (auto-configured)
model = "deepseek-v4-pro"
model_provider = "codex-converter"
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
```

> **注意**：首次运行时，转换器会设置 `model_provider = "codex-converter"`。如果你之后切换到其他提供商（如 `codex --model-provider codex`），转换器会尊重你的选择，不会覆盖。

---

## 使用方法

```bash
# 启动转换器（前台运行）
codex-converter

# 在另一个终端正常使用 Codex
codex

# 临时切换模型
codex --model deepseek-v4-flash

# 切回原生 GPT（转换器保持运行）
codex --model gpt-5.4-mini --model-provider codex
```

---

## 切换模型

**永久切换** — 编辑 `~/.codex-converter/config.toml`，修改 `model` 字段，重启转换器：

```toml
[[providers]]
name = "deepseek"
model = "deepseek-v4-flash"   # 从 deepseek-v4-pro 改过来
```

**临时切换** — 用 Codex 的 `--model` 参数（无需重启）：

```bash
codex --model deepseek-v4-flash
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
docker run -p 8080:8080 -v ~/.codex-converter:/root/.codex-converter codex-converter
```

---

## 常见问题

**Q: 模型名怎么填？**
A: 必须使用提供商文档中指定的准确名称。例如 DeepSeek 用 `deepseek-v4-pro`，MiMo 用 `mimo-v2.5-pro`。不要猜测或使用缩写。

**Q: 为什么有两个配置文件？**
A: `~/.codex-converter/config.toml` 是转换器自己的配置（你来管理）。`~/.codex/config.toml` 是 Codex 的配置（转换器自动配置，让它指向转换器）。

**Q: Codex 的所有功能都能用吗？**
A: 代码生成、tool calls、流式输出、多 agent 全部支持。仅 OpenAI 专有的内置工具（web search、computer use）无法通过转换器使用。

**Q: 还能用 GPT 吗？**
A: 能。运行 `codex --model-provider codex` 即可使用原生 GPT。转换器不会干扰。

**Q: API Key 安全吗？**
A: Key 存储在本地 `~/.codex-converter/config.toml` 中，仅转发到你配置的提供商。不会传输到其他地方。

---

## 许可证

MIT 许可证

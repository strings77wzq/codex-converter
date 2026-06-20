<h1 align="center">codex-converter v1.0</h1>

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

codex-converter 是一个轻量级代理，让 Codex 能够使用任何兼容 Chat Completions 的 LLM 提供商。Codex 说 Responses API，但大多数提供商（DeepSeek、MiMo、Qwen、GLM、Moonshot、Ollama 等）只说 Chat Completions。转换器完美桥接这个鸿沟——支持自动模型同步，你只需维护一个配置文件。

---

## 快速开始

```bash
# 安装
go install github.com/strings77wzq/codex-converter@latest

# 运行（首次启动进入交互式配置向导）
codex-converter
```

首次运行引导你完成：
1. 选择提供商（DeepSeek、MiMo、Qwen、GLM …）
2. 输入 API Key
3. 选择模型

**搞定。** 转换器自动配置 Codex 并启动服务。直接运行 `codex` 就能写代码了。

---

## 工作原理

```
Codex ──Responses──▶ codex-converter :8080 ──Chat──▶ 你的 LLM 提供商
                                            Completions   (DeepSeek/MiMo/...)
◀─Responses──        ◀──Chat──
```

1. Codex 说 **Responses API** — 发送请求、期望 SSE 事件流
2. codex-converter 翻译成 **Chat Completions**，转发给后端提供商
3. 提供商返回（流式或非流式）——转换器翻译回 Responses 格式
4. Codex 收到响应，仿佛在和原生 Responses 端点对话

---

## 功能特性

- ✅ **流式输出** — 逐 token 实时返回，完整的 SSE 事件序列
- ✅ **Tool Calls** — 完整函数调用支持（Codex agent 必需）
- ✅ **10+ 内置提供商** — DeepSeek、MiMo、Qwen、GLM、Moonshot、Yi、百川、MiniMax、Ollama，以及自定义
- ✅ **模型自动同步** — 改转换器配置的模型名，重启后 Codex 自动跟上
- ✅ **`codex --model` 随意切** — 临时切模型不用改任何文件
- ✅ **智能配置同步** — 使用其他提供商（如 GPT）时不会覆盖你的 Codex 设置
- ✅ **交互式配置向导** — 首次运行引导配置，自动测连接，失败可重试
- ✅ **单二进制无依赖** — `go install` 一行安装
- ✅ **双认证支持** — `Authorization: Bearer` 和 `api-key` 头

---

## 支持的提供商

| 提供商 | 认证方式 | 示例模型 |
|--------|---------|---------|
| **DeepSeek** | Bearer | `deepseek-v4-pro`, `deepseek-v4-flash` |
| **MiMo** | `api-key` 头 | `mimo-v2.5-pro` |
| **Qwen** | Bearer | `qwen-max`, `qwen-plus`, `qwen-turbo` |
| **GLM** | Bearer | `glm-4-plus`, `glm-4`, `glm-4-flash` |
| **Moonshot** | Bearer | `moonshot-v1-128k`, `moonshot-v1-32k` |
| **Yi（零一万物）** | Bearer | `yi-large`, `yi-medium` |
| **百川** | Bearer | `Baichuan4` |
| **MiniMax** | Bearer | `abab6.5s-chat` |
| **Ollama** | 无需认证（本地） | 任意本地模型 |
| **自定义** | 任选 | 你的模型 |

任何兼容 OpenAI API 的服务都能用——向导里选"Custom"然后填 URL 即可。

---

## 配置说明

### 转换器配置 (`~/.codex-converter/config.toml`)

```toml
default_provider = "mimo"

[server]
port = 8080
host = "127.0.0.1"

[[providers]]
name = "mimo"
base_url = "https://api.xiaomimimo.com"
model = "mimo-v2.5-pro"
api_key = "你的-api-key"
auth_style = "api_key_header"
context_window = 1000000    # 可选：同步到 Codex，控制 compact 行为
```

### 模型自动同步

启动时，codex-converter 智能同步模型配置到 `~/.codex/config.toml`：

| Codex 的 `model_provider` 状态 | 转换器行为 |
|---|---|
| `"codex-converter"` | 更新 `model`、`model_provider`、`context_window`（如配置） |
| 其他值（如 `"codex"`） | 仅更新 provider section——**尊重你当前的提供商选择** |

这样你可以在 GPT 和 MiMo 之间自由切换，转换器不会覆盖你的选择。

### 切模型

**永久切换** —— 编辑 `~/.codex-converter/config.toml` → 重启转换器 → `model` 自动同步：
```toml
model = "mimo-v2.5-flash"   # 从 pro 切过来
```

**临时切换** —— 用 Codex 的 `--model` 参数（无需重启、不改文件）：
```bash
codex --model mimo-v2.5-flash
```

---

## Codex 集成

启动 `codex-converter` 后，`~/.codex/config.toml` 已自动配置：

```toml
model = "mimo-v2.5-pro"
model_provider = "codex-converter"

[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
```

直接运行：

```bash
# 用默认模型（同步过来的）
codex

# 临时切换模型
codex --model mimo-v2.5-flash

# 想用 GPT（转换器保持运行不受影响）
codex --model gpt-5.4-mini --model-provider codex
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

**Q: 为什么需要这个工具？**
A: Codex 只说 Responses API，但大多数 LLM 提供商只说 Chat Completions。转换器桥接这个差异。

**Q: Codex 的所有功能都能用吗？**
A: 能——代码生成、tool calls、流式输出、多 agent 全部支持。仅 OpenAI 专有的内置工具（web search、computer use）无法使用。

**Q: 怎么切模型？**
A: 默认模型改 `~/.codex-converter/config.toml` 的 `model` 字段后重启转换器。临时切换直接用 `codex --model <模型名>`。

**Q: 还能用 GPT 吗？**
A: 能。需要 GPT 时 `codex --model-provider codex`，转换器不会覆盖你的选择。切回转换器只需改 `model_provider = "codex-converter"`，下次启动自动同步模型名。

**Q: API Key 安全吗？**
A: Key 存储在本地 `~/.codex-converter/config.toml` 中，仅转发到你配置的提供商。不会传输到其他地方。

---

## 许可证

MIT 许可证

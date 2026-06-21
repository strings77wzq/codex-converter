<h1 align="center">codex-converter</h1>

<p align="center">
  <strong>Use any Chat Completions model with Codex — one config, zero code changes</strong>
</p>

<p align="center">
  <a href="README.zh.md">中文</a> | English
</p>

<p align="center">
  <a href="https://github.com/strings77wzq/codex-converter/releases"><img src="https://img.shields.io/github/v/release/strings77wzq/codex-converter" alt="Release"></a>
  <a href="https://github.com/strings77wzq/codex-converter/blob/master/LICENSE"><img src="https://img.shields.io/github/license/strings77wzq/codex-converter" alt="License"></a>
</p>

---

Codex only speaks the [Responses API](https://platform.openai.com/docs/api-reference/responses), but most LLM providers (DeepSeek, MiMo, Qwen, GLM, Moonshot, Ollama, etc.) only support [Chat Completions](https://platform.openai.com/docs/api-reference/chat). **codex-converter** is a lightweight local proxy that translates between the two, so you can use any compatible provider with Codex without modifying either side.

---

## Quick Start

```bash
# 1. Install
go install github.com/strings77wzq/codex-converter@latest

# 2. Run (first launch starts interactive setup wizard)
codex-converter
```

### What the setup wizard does

On first run, the wizard guides you through 5 steps:

| Step | What you do | Example |
|------|-------------|---------|
| 1. Select provider | Choose from 10+ built-in providers or "Custom" | `1` (DeepSeek) |
| 2. Enter API Key | Paste your provider's API key | `sk-xxx...` |
| 3. Select model | Pick a model from the provider's list | `deepseek-v4-pro` |
| 4. Confirm | Review your configuration | — |
| 5. Test connection | Wizard verifies the API works | ✓ Connected |

The wizard writes two files:

| File | Purpose | Who edits it |
|------|---------|-------------|
| `~/.codex-converter/config.toml` | Converter config (providers, API keys, models) | You (or the wizard) |
| `~/.codex/config.toml` | Codex config (auto-configured by converter) | Converter + you |

After setup, run `codex` as usual — requests route through the converter to your provider.

---

## How It Works

```
                         codex-converter (localhost:8080)
                        ┌─────────────────────────────┐
                        │                             │
Codex ──Responses API──▶│  Responses → Chat Completions│──▶ Your LLM Provider
                        │  Chat Completions → Responses │    (DeepSeek/MiMo/...)
◀──Responses API────────│                             │◀── Chat Completions
                        │                             │
                        └─────────────────────────────┘
```

1. Codex sends a **Responses API** request to `localhost:8080`
2. Converter translates it to **Chat Completions** format and forwards to your provider
3. Provider responds (streaming or non-streaming)
4. Converter translates the response back to **Responses API** format
5. Codex receives it as if talking to a native Responses endpoint

---

## Features

| Feature | Description |
|---------|-------------|
| **Streaming** | Real-time token-by-token output with correct SSE event sequence |
| **Tool calls** | Full function calling support (essential for Codex agent) |
| **Multi-turn history** | Complete conversation history including `function_call` / `function_call_output` |
| **10+ providers** | DeepSeek, MiMo, Qwen, GLM, Moonshot, Yi, Baichuan, MiniMax, Ollama, Custom |
| **Auto model sync** | Converter syncs model config to Codex on startup |
| **`codex --model`** | Temporary model switch without editing config |
| **Config coexistence** | Won't overwrite your Codex settings when using other providers |
| **Interactive wizard** | First-run guided setup with connection testing |
| **Single binary** | Zero runtime dependencies, `go install` ready |

---

## Supported Providers

| Provider | Auth Style | Models | Base URL |
|----------|-----------|--------|----------|
| **DeepSeek** | `bearer` | `deepseek-v4-pro`, `deepseek-v4-flash` | `https://api.deepseek.com` |
| **MiMo** | `api_key_header` | `mimo-v2.5-pro`, `mimo-v2.5`, `mimo-v2-flash` | `https://api.xiaomimimo.com` |
| **Qwen** | `bearer` | `qwen-max`, `qwen-plus`, `qwen-turbo` | `https://dashscope.aliyuncs.com/compatible-mode` |
| **GLM** | `bearer` | `glm-4-plus`, `glm-4`, `glm-4-flash` | `https://open.bigmodel.cn/api/paas` |
| **Moonshot** | `bearer` | `moonshot-v1-128k`, `moonshot-v1-32k` | `https://api.moonshot.cn` |
| **Yi** | `bearer` | `yi-large`, `yi-medium` | `https://api.lingyiwanwu.com` |
| **Baichuan** | `bearer` | `Baichuan4` | `https://api.baichuan-ai.com` |
| **MiniMax** | `bearer` | `abab6.5s-chat` | `https://api.minimax.chat` |
| **Ollama** | none | Any local model | `http://localhost:11434/v1` |
| **Custom** | configurable | Your model | Your URL |

> **Model names must match exactly** what your provider expects. For example, DeepSeek uses `deepseek-v4-pro`, not `deepseek-pro` or `deepseek-v4`. Check your provider's documentation for the correct model name.

### Auth Style

| Style | When to use | How it works |
|-------|-------------|-------------|
| `bearer` | Most providers (DeepSeek, Qwen, GLM, etc.) | Sends `Authorization: Bearer <key>` header |
| `api_key_header` | MiMo, Azure-style APIs | Sends `api-key: <key>` header |
| `none` | Ollama (local, no auth) | No authentication header |

---

## Configuration

### Two config files

| File | Location | Purpose |
|------|----------|---------|
| **Converter config** | `~/.codex-converter/config.toml` | Provider URLs, API keys, model selection |
| **Codex config** | `~/.codex/config.toml` | Codex runtime config (auto-managed) |

**You edit** `~/.codex-converter/config.toml`. The converter **auto-manages** `~/.codex/config.toml`.

### Converter config (`~/.codex-converter/config.toml`)

```toml
default_provider = "deepseek"

[server]
port = 8080
host = "127.0.0.1"

[[providers]]
name = "deepseek"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-pro"          # Must match provider's model name exactly
api_key = "your-api-key"           # Or use api_key_env below
# api_key_env = "DEEPSEEK_API_KEY" # Read key from environment variable
auth_style = "bearer"              # "bearer" (default) or "api_key_header"
# context_window = 1000000         # Optional: synced to Codex for compact behavior

# Add more providers:
# [[providers]]
# name = "mimo"
# base_url = "https://api.xiaomimimo.com"
# model = "mimo-v2.5-pro"
# api_key = "your-mimo-key"
# auth_style = "api_key_header"
```

### Codex config (`~/.codex/config.toml`) — auto-configured

The converter adds this section to your Codex config:

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

> **Note:** On first run, the converter sets `model_provider = "codex-converter"`. If you later switch to a different provider (e.g., `codex --model-provider codex`), the converter respects your choice and won't overwrite it.

---

## Usage

```bash
# Start converter (runs in foreground)
codex-converter

# In another terminal, use Codex as usual
codex

# Override model temporarily
codex --model deepseek-v4-flash

# Switch back to native GPT (converter stays running)
codex --model gpt-5.4-mini --model-provider codex
```

---

## Switching Models

**Permanent** — edit `~/.codex-converter/config.toml`, change the `model` field, restart converter:

```toml
[[providers]]
name = "deepseek"
model = "deepseek-v4-flash"   # Changed from deepseek-v4-pro
```

**Temporary** — use Codex's `--model` flag (no restart needed):

```bash
codex --model deepseek-v4-flash
```

---

## Build from Source

```bash
git clone https://github.com/strings77wzq/codex-converter.git
cd codex-converter
go build -o codex-converter .
```

---

## Docker

```bash
docker build -t codex-converter .
docker run -p 8080:8080 -v ~/.codex-converter:/root/.codex-converter codex-converter
```

---

## FAQ

**Q: What model name should I use?**
A: Use the exact model name your provider expects. Check your provider's API documentation. For example, DeepSeek uses `deepseek-v4-pro`, MiMo uses `mimo-v2.5-pro`. Do not guess or use abbreviations.

**Q: Why two config files?**
A: `~/.codex-converter/config.toml` is the converter's own config (you manage this). `~/.codex/config.toml` is Codex's config (the converter auto-configures this to point at the converter).

**Q: Does it work with all Codex features?**
A: Code generation, tool calls, streaming, and multi-agent all work. OpenAI-specific built-in tools (web search, computer use) are not available through the converter.

**Q: Can I still use GPT with Codex?**
A: Yes. Run `codex --model-provider codex` to use native GPT. The converter won't interfere.

**Q: Is my API key safe?**
A: The key is stored locally in `~/.codex-converter/config.toml` and only sent to your configured provider. It is never transmitted anywhere else.

---

## License

MIT License

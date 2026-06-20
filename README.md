<h1 align="center">codex-converter</h1>

<p align="center">
  <strong>让 Codex 支持所有 Chat Completions 兼容的模型</strong>
</p>

<p align="center">
  <a href="README.zh.md">中文</a> | English
</p>

<p align="center">
  <a href="https://github.com/strings77wzq/codex-converter">GitHub</a>
</p>

---

codex-converter is a lightweight proxy that enables Codex to work with any Chat Completions compatible LLM provider. Codex only supports the Responses API format, but most providers (DeepSeek, MiMo, Qwen, Ollama, etc.) only support Chat Completions. This converter bridges the gap seamlessly.

---

## Quick Start

```bash
# Install
go install github.com/strings77wzq/codex-converter@latest

# Run (first time starts interactive setup)
codex-converter
```

The first launch guides you through configuration:
1. Enter your provider's Base URL
2. Enter your API Key
3. Enter Model name (or press Enter for default)

That's it! The converter auto-configures Codex and starts the service.

---

## How It Works

```
Codex ──→ codex-converter ──→ Your LLM Provider
         (Responses → Chat)   (DeepSeek/MiMo/Qwen/...)
```

1. Codex sends requests in Responses API format
2. codex-converter converts them to Chat Completions format
3. Your provider processes the request
4. codex-converter converts the response back to Responses format
5. Codex receives the response seamlessly

---

## Features

- ✅ **Text streaming** — Real-time token-by-token output
- ✅ **Tool calls** — Full support for function calling (essential for Codex agent)
- ✅ **Any provider** — Works with any Chat Completions compatible API
- ✅ **Interactive setup** — First-run wizard, no manual config editing
- ✅ **Auto-config Codex** — Automatically configures `~/.codex/config.toml`
- ✅ **Single binary** — No dependencies, just one executable

---

## Supported Providers

Any provider with a Chat Completions compatible API works:

| Provider | Base URL | Notes |
|----------|----------|-------|
| **DeepSeek** | `https://api.deepseek.com` | `deepseek-v4-pro`, `deepseek-v4-flash` |
| **MiMo** | `https://api.xiaomimimo.com` | `mimo-v2.5-pro`, uses `api-key` header |
| **Qwen** | `https://dashscope.aliyuncs.com/compatible-mode` | `qwen-max`, `qwen-plus` |
| **Ollama** | `http://localhost:11434/v1` | Local models, no API key needed |
| **vLLM** | `http://localhost:8000/v1` | Self-hosted models |
| **Any OpenAI-compatible** | Your provider's URL | Just fill in the details |

---

## Configuration

### First Run

```bash
codex-converter
```

Interactive wizard asks for:
- **Base URL** — Your provider's API endpoint
- **API Key** — Your authentication key
- **Model** — Model name (default: `mimo-v2.5-pro`)

### Subsequent Runs

```bash
codex-converter
# Automatically loads config from ~/.codex-converter/config.toml
```

### Manual Config

Edit `~/.codex-converter/config.toml`:

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

## Codex Integration

After running `codex-converter`, it automatically adds this to `~/.codex/config.toml`:

```toml
[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
```

Then just run Codex normally:

```bash
codex --model mimo-v2.5-pro --model-provider codex-converter
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
docker run -p 8080:8080 codex-converter
```

---

## FAQ

**Q: Why do I need this?**
A: Codex only speaks Responses API, but most LLM providers only speak Chat Completions. This converter bridges the gap.

**Q: Does it work with all Codex features?**
A: Yes! Code generation, tool calls, streaming — everything works. Only OpenAI-specific built-in tools (web search, computer use) are not available.

**Q: Is my API key safe?**
A: The key is stored locally in `~/.codex-converter/config.toml` and never leaves your machine except when forwarding requests to your provider.

---

## License

MIT License

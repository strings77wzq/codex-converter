<h1 align="center">codex-converter v1.0</h1>

<p align="center">
  <strong>Let Codex work with any Chat Completions compatible model</strong>
</p>

<p align="center">
  <a href="README.zh.md">中文</a> | English
</p>

<p align="center">
  <a href="https://github.com/strings77wzq/codex-converter">GitHub</a>
</p>

---

codex-converter is a lightweight proxy that enables Codex to work with any Chat Completions compatible LLM provider. Codex speaks the Responses API, but most providers (DeepSeek, MiMo, Qwen, GLM, Moonshot, Ollama, etc.) only speak Chat Completions. This converter bridges the gap — with automatic model sync so you only edit one config file.

---

## Quick Start

```bash
# Install
go install github.com/strings77wzq/codex-converter@latest

# Run (first time starts interactive setup)
codex-converter
```

First launch guides you through:
1. Select your provider (DeepSeek, MiMo, Qwen, GLM, …)
2. Enter your API Key
3. Choose a model

**That's it.** The converter auto-configures Codex and starts. Run `codex` and you're coding.

---

## How It Works

```
Codex ──Responses──▶ codex-converter :8080 ──Chat──▶ Your LLM Provider
                                            Completions   (DeepSeek/MiMo/...)
◀─Responses──        ◀──Chat──
```

1. Codex speaks **Responses API** — sends requests, expects SSE events
2. codex-converter translates to **Chat Completions** and forwards to your provider
3. Provider responds (streaming or not) — converter translates back to Responses format
4. Codex receives the response as if it were talking to a native Responses endpoint

---

## Features

- ✅ **Text streaming** — Real-time token-by-token output with correct SSE event sequence
- ✅ **Tool calls** — Full function calling support (essential for Codex agent)
- ✅ **Multi-turn history** — Complete conversation history conversion including `function_call` / `function_call_output` items
- ✅ **10+ providers built-in** — DeepSeek, MiMo, Qwen, GLM, Moonshot, Yi, Baichuan, MiniMax, Ollama, plus Custom
- ✅ **Auto-sync model to Codex** — Change model in converter config, Codex picks it up on next converter restart
- ✅ **`codex --model` switching** — Temporary model switches without editing any config file
- ✅ **Intelligent config sync** — Won't overwrite your Codex settings when using other providers (e.g., GPT)
- ✅ **Interactive setup wizard** — First-run guided configuration with connection testing and retry
- ✅ **Single binary** — Zero runtime dependencies, `go install` ready
- ✅ **Dual auth support** — `Authorization: Bearer` and `api-key` header

---

## Supported Providers

| Provider | Auth Style | Example Models |
|----------|-----------|----------------|
| **DeepSeek** | Bearer | `deepseek-v4-pro`, `deepseek-v4-flash` |
| **MiMo** | `api-key` header | `mimo-v2.5-pro` |
| **Qwen** | Bearer | `qwen-max`, `qwen-plus`, `qwen-turbo` |
| **GLM** | Bearer | `glm-4-plus`, `glm-4`, `glm-4-flash` |
| **Moonshot** | Bearer | `moonshot-v1-128k`, `moonshot-v1-32k` |
| **Yi** | Bearer | `yi-large`, `yi-medium` |
| **Baichuan** | Bearer | `Baichuan4` |
| **MiniMax** | Bearer | `abab6.5s-chat` |
| **Ollama** | None (local) | Any local model |
| **Custom** | Your choice | Your model |

Any OpenAI-compatible API works — just select "Custom" in the setup wizard.

---

## Configuration

### Converter Config (`~/.codex-converter/config.toml`)

```toml
default_provider = "mimo"

[server]
port = 8080
host = "127.0.0.1"

[[providers]]
name = "mimo"
base_url = "https://api.xiaomimimo.com"
model = "mimo-v2.5-pro"
api_key = "your-api-key"
auth_style = "api_key_header"
context_window = 1000000    # optional: syncs to Codex for correct compact behavior
```

### Model Sync (automatic)

On startup, codex-converter intelligently syncs your model config to `~/.codex/config.toml`:

| Codex `model_provider` state | Behavior |
|---|---|
| `"codex-converter"` | Updates `model`, `model_provider`, `context_window` (if set) |
| Other value (e.g. `"codex"`) | Only updates the provider section — **your current provider choice is respected** |

This means you can switch between GPT and MiMo freely without the converter overwriting your choice.

### Switching Models

**Permanent** — edit `~/.codex-converter/config.toml` → restart converter → `model` auto-syncs:
```toml
model = "mimo-v2.5-flash"   # changed from pro
```

**Temporary** — use Codex's `--model` flag (no restart, no config edit):
```bash
codex --model mimo-v2.5-flash
```

---

## Codex Integration

After running `codex-converter`, your `~/.codex/config.toml` is auto-configured:

```toml
model = "mimo-v2.5-pro"
model_provider = "codex-converter"

[model_providers.codex-converter]
name = "codex-converter"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
```

Run Codex normally:

```bash
# Default (synced model)
codex

# Override model
codex --model mimo-v2.5-flash

# Use GPT instead (converter keeps running untouched)
codex --model gpt-5.4-mini --model-provider codex
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

**Q: Why do I need this?**
A: Codex only speaks Responses API, but most LLM providers only speak Chat Completions. This converter bridges the gap.

**Q: Does it work with all Codex features?**
A: Yes — code generation, tool calls, streaming, multi-agent. OpenAI-specific built-in tools (web search, computer use) are not available.

**Q: How do I switch between models?**
A: For the default model, edit `model` in `~/.codex-converter/config.toml` and restart. For temporary switches, use `codex --model <name>` directly.

**Q: Can I still use GPT with Codex?**
A: Yes. Just run `codex --model-provider codex` when you want GPT. The converter won't overwrite your Codex settings when you've chosen a different provider.

**Q: Is my API key safe?**
A: The key is stored locally in `~/.codex-converter/config.toml` and only sent to your configured provider. It is never transmitted anywhere else.

---

## License

MIT License

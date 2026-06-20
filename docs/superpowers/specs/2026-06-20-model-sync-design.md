# Model Sync — Codex Config Auto-Sync Design Spec

> Date: 2026-06-20 | Feature branch: `feat/model-sync`

## [S1] Problem

When a user changes the model in `~/.codex-converter/config.toml`, they must ALSO manually edit `~/.codex/config.toml` to match. Two config files, one truth. Especially painful for users who frequently switch between models (Pro → Flash → GPT).

## [S2] Solution

On startup, the converter reads its own config and intelligently syncs the model name + provider section into `~/.codex/config.toml`. Single source of truth: converter config. Codex config is auto-maintained.

## [S3] Sync Rules

| Codex `model_provider` state | Behavior |
|---|---|
| `"codex-converter"` | Update `model`, `model_provider`, `context_window` (if set), and `[model_providers.codex-converter]` |
| Other value (e.g., `"codex"`) | Only update `[model_providers.codex-converter]` section; leave top-level fields untouched |
| Field absent | Full initialization (first run) |

This respects user intent: when the user switches to GPT, the converter doesn't overwrite their choice. When they switch back, it picks up the latest model name.

## [S4] Model Switching via CLI

`codex --model mimo-v2.5-flash` works because the converter passes `req.Model` through to the backend. No config change needed for temporary switches. The synced `model` field is only the DEFAULT.

## [S5] Config Changes

**`internal/config/config.go`** — Provider struct:
```go
type Provider struct {
    // ... existing fields ...
    ContextWindow int `toml:"context_window"` // optional, to sync to Codex
}
```

**`~/.codex-converter/config.toml`** — backward compatible:
```toml
[[providers]]
name = "mimo"
model = "mimo-v2.5-pro"
context_window = 1000000   # optional new field
```

## [S6] New Function

`func SyncCodexConfig(cfg *Config) error` in `config.go`:
1. Read `~/.codex/config.toml` (tolerant of missing file)
2. Parse current `model_provider` value
3. If `"codex-converter"` or absent → update top-level fields from provider config
4. If other → skip top-level fields
5. Ensure `[model_providers.codex-converter]` section exists with correct `base_url` + `wire_api`
6. Write back, preserving ALL other TOML content untouched

## [S7] Integration

`main.go`: after config load, before `http.ListenAndServe`:
```go
if err := config.SyncCodexConfig(cfg); err != nil {
    log.Printf("sync codex config: %v", err) // non-fatal
}
```

## [S8] Scope

In scope:
- Auto-sync on startup
- Intelligent model_provider gating
- Optional context_window sync
- TOML-round-trip preserving user's other config

Out of scope:
- Hot reload / file watching
- Auto-complete model names in Codex
- Multi-model listing in provider section

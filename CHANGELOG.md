# Changelog

## v1.0.12 (2026-06-21)

### Fix: P0 hardening — Dockerfile + request body size limit

- Fix Dockerfile build path (`cmd/server` → `.`), align Go version with go.mod (1.25)
- Add `max_body_mb` config (default 10MB) with `http.MaxBytesReader` — prevents unbounded memory consumption
- Return HTTP 413 with actionable error message when body exceeds limit
- Align Dockerfile ldflags with CI (`-s -w`)

### Feat: model-error diagnosis

- Turn cryptic provider 404/400/422 errors into actionable hints
- Detects model-name mismatches (configured vs requested) and suggests fixes
- Injects `[codex-converter]` hint into both log and response body

### Fix: setup wizard double API key prompt (S4b)

- API key is now read once in Step 2, reused in Step 4 and config build
- Previously users had to paste their key twice during setup

### Feat: auth_style auto-detect for custom providers (S4c)

- Custom providers: tries `bearer` first, falls back to `api_key_header` on 401/403
- User never needs to choose auth_style manually

### Fix: DRY — normalize MaxBodyMB in NewHandler

- Remove dual-default antipattern (config.Load + handler fallback)
- Single normalization point in `NewHandler()`

## v1.0.11 (2026-06-21)

### Fix: multi-turn tool call history

Codex sends `function_call` and `function_call_output` items in the input array during multi-turn conversations with tool use. These items have no `role` field, so the converter was silently dropping them — causing tool call context loss.

- `function_call` → `{role: "assistant", tool_calls: [{id, type, function}]}`
- `function_call_output` → `{role: "tool", tool_call_id, content}`
- Added `ToolCallID` field to `ChatMessage` struct
- `convertInput` now routes items by `type` field instead of filtering by `role`
- 3 new tests: FunctionCall, FunctionCallOutput, full ToolCallChain

## v1.0.10 (2026-06-21)

### Fix: multi-turn 400 — convert assistant output_text history to text

Codex echoes the prior assistant reply back as an input item with `type: "output_text"`. The converter only converted `input_text` → `text`, so `output_text` leaked through and the Chat Completions backend rejected it (HTTP 400).

- `convertContent` now flattens all-text content arrays to a single string
- `isTextBlock` handles both `input_text` and `output_text` types
- Mixed content (e.g. images) preserves array form with normalized text types

## v1.0.9 (2026-06-21)

### Feat: friendly port preflight + request logging

- Port-in-use check with human-readable error before startup
- Request logging on startup (model, stream flag)

## v1.0.8 (2026-06-20)

### Fix: streaming response.completed

- Emit codex-compatible SSE sequence so turns actually complete

## v1.0.7 (2026-06-20)

### Fix: convert Responses API input_text blocks to Chat text blocks

- `input_text` → `text` content block conversion for first-turn requests

## v1.0.6 (2026-06-20)

### Fix: use runtime/debug.ReadBuildInfo for version

- Version derived from Go module info, never hardcoded

## v1.0.5 (2026-06-20)

### Fix: normalize base_url

- Strip trailing slashes and common suffixes to prevent double `/v1` in backend URL

## v1.0.4 (2026-06-20)

### Fix: provider API key priority

- Provider config API key takes priority over incoming Authorization header

## v1.0.3 (2026-06-20)

### Fix: SSE event format

- Correct event types for response.created, output_text.delta, etc.

## v1.0.2 (2026-06-20)

### Fix: tool call streaming

- Handle tool_calls deltas in streaming responses

## v1.0.1 (2026-06-20)

### Fix: initial conversion

- Basic request/response conversion between Responses API and Chat Completions

## v1.0.0 (2026-06-20)

### Initial release

- Responses API ↔ Chat Completions proxy
- Streaming support
- TOML config with multi-provider support
- Single binary + Docker
- Health check endpoint
- Interactive setup wizard
- Auto model sync to Codex

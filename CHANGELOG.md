# Changelog

## v1.0.13 (2026-06-24)

### Fix: content formatting — non-streaming responses with array content
- `convertChoice` used `fmt.Sprintf("%v", ...)` which produced Go internal form like `[{text hello}]` when a provider returned content as `[]interface{}` instead of a string
- New `extractText(interface{}) string` mirrors `convertContent` in request.go: handles `string`, `[]interface{}` (text blocks), nil, and unsupported types
- Reuses the existing `isTextBlock` helper; no new abstractions

### Fix: base_url no longer stale after port/host change
- `SyncCodexConfig` and `configureCodex` both hardcoded `http://127.0.0.1:8080` and skipped updates when the `[model_providers.codex-converter]` section already existed — changing port/host in the converter config left Codex pointing at the old address
- New `codexBaseURL(host, port)` maps listen addresses to connectable ones: `0.0.0.0`/`""` → `127.0.0.1`, `::`/`[::]` → `[::1]`, everything else preserved
- New private `setKeyInSection` helper updates `name`/`base_url`/`wire_api` inside the existing section (incremental upsert, preserves user-added keys)
- `findKey`/`setKey` now stop at the first TOML section header so a top-level `model` key is never confused with a `model =` inside a provider section

### Refactor: unify URL normalization (DRY)
- Three copies of the same suffix-stripping logic (`normalizeBaseURL`, `cleanBaseURL`, and an inline copy in `testConnection`) are consolidated into one exported `config.NormalizeBaseURL`
- All three call sites now use the shared function; edge-case tests relocated to `config_test.go`

### Fix: ConvertStream goroutine cancellation
- `ConvertStream` previously had no context plumbing; every `ch <-` blocked on an unbuffered channel, so a client disconnect leaked the goroutine and held the backend connection
- New signature `ConvertStream(ctx context.Context, scanner *bufio.Scanner)`; channel buffered to 64; all sends go through a `sendEvent` closure guarded by `select { case ch <-: case <-ctx.Done(): }`
- Handler passes `r.Context()` so cancel propagates from the client disconnect

### Feat: graceful shutdown on SIGINT/SIGTERM
- `main.go` now installs a signal handler: first signal calls `srv.Shutdown` with a 30s timeout so in-flight streaming requests finish; a second signal forces `os.Exit(1)`
- `ListenAndServe` accepts `http.ErrServerClosed` as the expected clean-exit error

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

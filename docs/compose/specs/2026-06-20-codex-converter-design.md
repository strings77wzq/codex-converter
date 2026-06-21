# codex-converter Design Spec

## [S1] Problem

Codex uses Responses API (`wire_api = "responses"`), but Chinese model providers (DeepSeek, MiMo) only support Chat Completions. Users cannot use Codex with these providers directly.

## [S2] Solution

A Go HTTP proxy that receives Responses API requests, converts them to Chat Completions, forwards to the backend provider, and converts the streaming response back to Responses format.

**User flow:**
1. Download single binary
2. Create config.toml with API key
3. Run `codex-converter`
4. Configure Codex to point at `http://127.0.0.1:8080`

## [S3] Architecture

```
Codex → [codex-converter :8080] → DeepSeek/MiMo API
```

Single binary. TOML config. Provider-as-config (no code changes to add providers).

## [S4] Conversion Rules

### Request (Responses → Chat)
- `input` (string) → `messages: [{role:"user", content:input}]`
- `input` (array) → route by item type:
  - `message` items → `{role, content: flattenContent(...)}` (text-only arrays flatten to string)
  - `function_call` items → `{role:"assistant", tool_calls: [{id, type:"function", function:{name, arguments}}]}`
  - `function_call_output` items → `{role:"tool", tool_call_id, content}`
- `instructions` → insert as `{role:"system"}` at head
- `tools[].type=="function"` → nest into `tools[].function`
- `text.format` → `response_format`

### Response (Chat → Responses)
- `choices[0].message.content` → `output: [{type:"message", content:[{type:"output_text", text:...}]}]`
- `choices[0].tool_calls` → `output: [{type:"function_call", call_id, name, arguments}]`
- `usage.prompt_tokens` → `usage.input_tokens`
- `usage.completion_tokens` → `usage.output_tokens`

### Streaming
- `data: {choices:[{delta:{content:"token"}}]}` → `event: response.output_text.delta\ndata: {delta:"token"}`
- `data: {choices:[{delta:{tool_calls:[...]}}]}` → `event: response.function_call_arguments.delta`
- `data: [DONE]` → `event: response.completed`
- Maintain item order: `response.created` → `output_item.added` → deltas → `output_item.done` → `response.completed`

## [S5] Config Format

```toml
[server]
port = 8080

[[providers]]
name = "deepseek"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-pro"
api_key_env = "DEEPSEEK_API_KEY"

[[providers]]
name = "mimo"
base_url = "https://api.xiaomimimo.com"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
auth_style = "api_key_header"

default_provider = "deepseek"
```

## [S6] Scope

MVP:
- Text streaming conversion
- Tool call streaming conversion
- TOML config with multi-provider support
- Single binary + Docker
- Health check endpoint (`GET /health`)

Not in scope (future):
- WebSocket mode
- Failover/load balancing
- Request logging/stats
- `previous_response_id` state management

## [S7] File Structure

```
main.go
internal/config/config.go
internal/proxy/handler.go        # client/forwarding inlined (no separate client.go)
internal/convert/request.go
internal/convert/response.go
internal/convert/stream.go       # tool-call handling inlined (no separate tools.go)
internal/types/responses.go
internal/types/responses_response.go
internal/types/chat.go
internal/setup/setup.go
config.example.toml
Dockerfile
```

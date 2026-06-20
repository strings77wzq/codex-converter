# Fix Batch Design — P0-1 / P0-2 / P1-1

> Date: 2026-06-20
> Channel: 快速通道 (fast track). All three items are single-file/doc changes, no openspec required.
> Constraints honored: 反幻觉协议, Context Propagation (Mandatory), Error 必须显式处理, 破坏性变更协议 (none triggered), TDD.

## Scope

Three fixes, one spec, three atomic commits:

- **P0-1** — Fix broken build command and spec file-structure drift (docs only).
- **P0-2** — Add staged timeout + context propagation to backend HTTP forwarding.
- **P1-1** — Raise stream buffer limit (64KB → 1MB) and stop swallowing `scanner.Err()`.

Out of scope: P1-2 (setup/main coverage), P2 (stream protocol fidelity, multi-provider routing). Tracked separately.

---

## P0-1 — Fix build command & spec drift

### Problem
`README.md:146` and design spec `[S7]` reference `./cmd/server` and `cmd/server/main.go`, which do not exist — the entry point was moved to root `main.go` (commit `bb11510`). The documented build command fails. Violates 反幻觉协议 (referencing a path that does not exist).

### Change (docs only, no code)
| File | Change |
|------|--------|
| `README.md` | `go build -o codex-converter ./cmd/server` → `go build -o codex-converter .` |
| `README.zh.md` | Same fix in the Chinese build section |
| `docs/compose/specs/2026-06-20-codex-converter-design.md` `[S7]` | `cmd/server/main.go` → `main.go`; remove unimplemented `internal/proxy/client.go` and `internal/convert/tools.go` (note: logic inlined into `handler.go` / `request.go`) |

### Gate
`go build -o codex-converter .` runs successfully — proves the documented command is real (closes 反幻觉协议 loop). No unit test (docs change).

---

## P0-2 — Staged timeout + context on backend forwarding

### Problem
`handler.go:115` uses `http.DefaultClient.Do()` — no timeout, no context. A hung backend stalls the connection indefinitely. Violates `golang/patterns.md` Context Propagation (Mandatory) and `golang/security.md` (timeout control).

### Design constraint
`http.Client.Timeout` covers the **entire request including body read**. The proxy streams LLM responses whose body can take minutes. A blanket `Timeout` would kill legitimate long streams. Therefore we use **staged timeouts** via a custom `Transport`, and leave overall body-read duration unbounded.

### Change — `internal/proxy/handler.go`

Add a reused, non-exported `client` field to `Handler` (also gives connection pooling):

```go
type Handler struct {
    cfg    *config.Config
    client *http.Client
}

func NewHandler(cfg *config.Config) *Handler {
    return &Handler{
        cfg: cfg,
        client: &http.Client{
            Transport: &http.Transport{
                DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
                ResponseHeaderTimeout: 30 * time.Second,
                // No client.Timeout — body read (streaming) is intentionally unbounded.
            },
        },
    }
}
```

- Build the backend request with `http.NewRequestWithContext(r.Context(), "POST", backendURL, ...)` — when the client disconnects, `r.Context()` cancels and the backend request is cancelled.
- Replace `http.DefaultClient.Do(reqBackend)` with `h.client.Do(reqBackend)`.

### Timeout semantics
| Failure mode | Bound | Result |
|--------------|-------|--------|
| Connect hang | 10s (DialContext) | 502 Bad Gateway |
| Backend accepts but never returns response headers | 30s (ResponseHeaderTimeout) | 502 Bad Gateway |
| Normal long stream (headers received, body flowing) | unbounded | streams normally |
| Client disconnects mid-request | immediate (r.Context() cancel) | backend request cancelled |

All error cases reuse the existing `http.StatusBadGateway` branch (`handler.go:117`).

### Breaking-change check
`ServeHTTP` (interface method) and `NewHandler` signatures unchanged. Purely internal. 破坏性变更协议 NOT triggered.

### Testability (white-box)
Tests live in `package proxy` (verified). They set the non-exported `client` field directly — no exported API change, no functional options, no second constructor. Production default stays 30s; tests inject a sub-second `ResponseHeaderTimeout`.

---

## P1-1 — Raise stream buffer limit & report scanner errors

### Problem
1. `handler.go:165` wraps `resp.Body` in a default `bufio.Scanner`, whose per-line limit is `bufio.MaxScanTokenSize = 64KB`. A single SSE line larger than 64KB (large tool-call `arguments` — common in the Codex coding-agent flow) causes `Scan()` to fail.
2. `stream.go` never checks `scanner.Err()` after the loop, so that failure is **silently swallowed** — the stream truncates with no log, no event. Violates `quality-gates.md` 「Error 必须显式处理，不得静默吞掉」.

### Change A — `internal/proxy/handler.go` (`handleStreamingResponse`)
```go
scanner := bufio.NewScanner(resp.Body)
scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // initial 64KB, max 1MB per line
events := convert.ConvertStream(scanner)
```

### Change B — `internal/convert/stream.go` (keep `ConvertStream(scanner)` signature — option A, no breaking change)
After the `for scanner.Scan()` loop, before the goroutine returns:
```go
if err := scanner.Err(); err != nil {
    ch <- StreamEvent{
        Type: "error",
        Data: fmt.Sprintf(`{"type":"error","message":%s}`, escapeJSON(err.Error())),
    }
}
```

### Event-type decision: `error` (not `response.failed`)
`scanner.Err()` is a **transport-level** failure (the proxy failed to read the backend SSE stream), not a model/business failure. The `error` event is the semantically correct one; `response.failed` would misrepresent a transport fault as a model failure. Chosen: route A (`error`).

### Known limitation (待真机验证)
The exact wire shape Codex expects for an `error` event is **not verified against Codex source** — there is no precedent in this repo (current code emits only 7 `response.*` events, zero error events). The fix guarantees the error is **no longer silently swallowed** and is surfaced as an event; whether Codex's client renders/handles it gracefully must be confirmed with a real large-tool-call run. If Codex ignores `error`, escalate to also emitting `response.failed` (route C) in a follow-up.

### Breaking-change check
`ConvertStream(scanner *bufio.Scanner)` signature unchanged (option A). Only 1 call site (`handler.go:166`). 破坏性变更协议 NOT triggered.

---

## Test Plan (TDD, all under `go test -race`)

| Item | Test | File |
|------|------|------|
| P0-2 timeout | httptest backend that sleeps past the injected timeout without sending headers → assert handler returns 502; inject sub-second `ResponseHeaderTimeout` via white-box `h.client` | `internal/proxy/handler_test.go` |
| P1-1 buffer | construct a single SSE `data:` line > 64KB (large tool-call arguments) → assert stream is not truncated, arguments pass through intact | `internal/convert/stream_test.go` |
| P1-1 error report | inject a reader/scanner that triggers `scanner.Err()` → assert an `error` event is emitted | `internal/convert/stream_test.go` |
| P0-1 | not a unit test — gate is `go build -o codex-converter .` | — |

Each fix: write failing test (red) → implement (green) → `go test -race ./...` → `go build ./...`.

## Commits (conventional, atomic)
```
1. docs: fix broken build command and sync spec file structure        (P0-1)
2. test+fix: add staged timeout and context to backend HTTP forwarding (P0-2)
3. test+fix: raise stream buffer limit and report scanner errors       (P1-1)
```

## Completion gate
After all three: `go build ./... && go vet ./... && go test -race ./...` all green. Then push → CI monitoring per project protocol (or full local gate if no CI).

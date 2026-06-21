# P0 Hardening — Dockerfile Fix + Request Body Limit Design Spec

> Date: 2026-06-21 | Topic: p0-hardening | Direction: minimal surgical fixes

## [S1] Problem

Two P0 issues identified in the v1.0.11 production codebase:

**P0-1: Dockerfile build path broken**

`Dockerfile:6` references `./cmd/server` which does not exist. The project's `main.go` is at the repository root. Docker builds fail immediately:

```
RUN CGO_ENABLED=0 go build -o /codex-converter ./cmd/server
# → no required module provides package .../cmd/server
```

Impact: Docker distribution channel completely non-functional. Primary channel (`go install`) unaffected — v1.0.11 works for `go install` users.

**P0-2: No request body size limit**

`internal/proxy/handler.go:77` reads the entire request body without bound:

```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
```

A malformed or malicious client can send an arbitrarily large body, consuming server memory until OOM. As a localhost proxy the attack surface is limited (user can only DoS themselves), but this violates defensive programming principles and would fail any security audit.

## [S2] Root Cause & Direction

**P0-1**: The Dockerfile was likely written for a planned `cmd/server/` layout that was never adopted. The CI workflow (`go-ci.yml:87`) correctly uses `go build ... .` — the Dockerfile was never aligned.

Direction: **Fix Dockerfile to match CI** — change build path from `./cmd/server` to `.`. Do NOT restructure the project layout (that's a separate concern).

**P0-2**: The handler was built for functionality first, hardening deferred. Go's `http.MaxBytesReader` is the standard solution — it wraps `io.Reader` and returns a clear error when the limit is exceeded.

Direction: **Add `http.MaxBytesReader` in `handleResponses`** with a configurable limit. Default 10MB. Configurable via `[server] max_body_mb` in config.toml. No middleware abstraction (YAGNI — one handler, one limit).

## [S3] Change 1 — Dockerfile Fix

**File**: `Dockerfile`

Current (broken):
```dockerfile
RUN CGO_ENABLED=0 go build -o /codex-converter ./cmd/server
```

Fixed:
```dockerfile
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /codex-converter .
```

Also align with CI's `-ldflags="-s -w"` for stripped binaries.

Also fix the Go version: current Dockerfile uses `golang:1.22-alpine` but `go.mod` specifies `go 1.25.11`. Change to `golang:1.25-alpine`.

**Also fix**: `COPY config.example.toml /etc/codex-converter/config.toml` — the CMD uses `-config` flag but the entry point in `main.go` defaults to `~/.codex-converter/config.toml` when no flag is given. The Docker CMD already passes `-config` explicitly, so this is correct. No change needed.

## [S4] Change 2 — Request Body Size Limit

### S4.1 Config (`internal/config/config.go`)

Add `MaxBodyMB` to `Server` struct:

```go
type Server struct {
    Port      int    `toml:"port"`
    Host      string `toml:"host"`
    MaxBodyMB int    `toml:"max_body_mb"` // default 10
}
```

In `Load()`, set default after decode:

```go
if cfg.Server.MaxBodyMB <= 0 {
    cfg.Server.MaxBodyMB = 10
}
```

### S4.2 Handler (`internal/proxy/handler.go`)

In `handleResponses`, wrap `r.Body` before decoding. Add `"errors"` to imports:

```go
func (h *Handler) handleResponses(w http.ResponseWriter, r *http.Request) {
    maxBytes := int64(h.cfg.Server.MaxBodyMB) * 1024 * 1024
    r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

    var req types.ResponsesRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        var maxBytesErr *http.MaxBytesError
        if errors.As(err, &maxBytesErr) {
            h.logf("%s %s -> 413 body too large (limit %dMB)", r.Method, r.URL.Path, h.cfg.Server.MaxBodyMB)
            http.Error(w, fmt.Sprintf("request body too large (limit: %dMB)", h.cfg.Server.MaxBodyMB), http.StatusRequestEntityTooLarge)
            return
        }
        h.logf("%s %s -> 400 invalid request: %v", r.Method, r.URL.Path, err)
        http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
        return
    }
    // ... rest unchanged
}
```

### S4.3 Config example (`config.example.toml`)

Add commented-out option:

```toml
[server]
port = 8080
host = "127.0.0.1"
# max_body_mb = 10  # Request body size limit in MB (default: 10)
```

## [S5] Tests (TDD — write BEFORE implementation)

### S5.1 Dockerfile Test

No unit test for Dockerfile (it's a build artifact). Verification:

```bash
docker build -t codex-converter .
docker run --rm codex-converter --version
```

This is a manual verification step, not an automated test.

### S5.2 Body Size Limit Tests (`internal/proxy/handler_test.go`)

**Test 1: Body within limit succeeds**

```go
func TestHandler_BodyWithinLimit(t *testing.T) {
    // Setup: backend returns valid response
    // Config: MaxBodyMB = 1
    // Request: body < 1MB
    // Assert: status 200
}
```

**Test 2: Body exceeding limit returns 413**

```go
func TestHandler_BodyExceedsLimit(t *testing.T) {
    // Config: MaxBodyMB = 1
    // Request: body = 2MB of JSON
    // Assert: status 413
    // Assert: response body contains "too large"
}
```

**Test 3: Default limit is 10MB**

```go
func TestConfig_DefaultMaxBodyMB(t *testing.T) {
    // Config file without max_body_mb
    // Assert: cfg.Server.MaxBodyMB == 10
}
```

**Test 4: Custom limit from config**

```go
func TestConfig_CustomMaxBodyMB(t *testing.T) {
    // Config file with max_body_mb = 5
    // Assert: cfg.Server.MaxBodyMB == 5
}
```

**Test 5: Zero/negative max_body_mb defaults to 10**

```go
func TestConfig_InvalidMaxBodyMBDefaultsToTen(t *testing.T) {
    // Config file with max_body_mb = 0 or -1
    // Assert: cfg.Server.MaxBodyMB == 10
}
```

### S5.3 Test Implementation Details

For Test 2, generate a body that exceeds the limit:

```go
// 2MB of valid JSON that will fail mid-parse
bigBody := strings.Repeat("x", 2*1024*1024)
reqBody := fmt.Sprintf(`{"model":"m","input":"%s"}`, bigBody)
```

The `http.MaxBytesReader` will return `*http.MaxBytesError` when the reader exceeds the limit during `json.Decode`.

## [S6] Scope

**In scope**:
- Dockerfile build path fix + Go version alignment
- `MaxBodyMB` config field with default
- `http.MaxBytesReader` wrapping in `handleResponses`
- 413 response with clear error message
- 5 test cases (S5.2–S5.3)
- `config.example.toml` documentation

**Out of scope** (YAGNI):
- Middleware abstraction for body limits
- Per-endpoint body size configuration
- Request body compression/decompression
- Project layout restructuring (`cmd/server/`)
- Rate limiting or concurrent request limits

## [S7] Implementation Order

1. Write failing tests (S5.2–S5.3)
2. Implement config changes (S4.1)
3. Implement handler changes (S4.2)
4. Update config.example.toml (S4.3)
5. Fix Dockerfile (S3)
6. Run `go test -race ./...` — all pass
7. Run `go vet ./...` — clean
8. Run `goimports -l .` — clean
9. Manual Docker build verification (S5.1)

# Config UX Clarity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make model-name semantics, the two config files, and `auth_style` unambiguous — via a model-error guard, wizard fixes, and rewritten docs.

**Architecture:** Keep pass-through proxy semantics (Direction A). Add a conservative model-error guard in the proxy that turns provider 404s into actionable hints injected into the Codex error + logged to console. Fix the wizard's double API-key prompt, add model-name guidance, and auto-detect `auth_style` for custom providers. Rewrite docs to collapse the false "two model names" distinction.

**Tech Stack:** Go (stdlib `net/http`, `encoding/json`, `testing` + `httptest`), TOML config, Markdown docs.

**Spec:** `docs/superpowers/specs/2026-06-21-config-ux-clarity-design.md`

---

### Task 1: Model-error guard helpers (`internal/proxy`)

**Files:**
- Create: `internal/proxy/modelerror.go`
- Test: `internal/proxy/modelerror_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/proxy/modelerror_test.go`:

```go
package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLooksLikeModelError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"404 always model error", 404, `{"error":"nope"}`, true},
		{"400 with model not found", 400, `{"error":{"message":"The model deepseek-x does not exist","code":"model_not_found"}}`, true},
		{"422 invalid model", 422, `{"error":{"message":"invalid model name"}}`, true},
		{"400 unrelated", 400, `{"error":{"message":"messages required"}}`, false},
		{"500 server error", 500, `internal error`, false},
		{"401 auth", 401, `{"error":"unauthorized"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeModelError(tt.status, []byte(tt.body)); got != tt.want {
				t.Errorf("looksLikeModelError(%d, %q) = %v, want %v", tt.status, tt.body, got, tt.want)
			}
		})
	}
}

func TestModelErrorHint_NameMatches(t *testing.T) {
	hint := modelErrorHint("glm-x", "glm-x", "glm")
	if !strings.Contains(hint, "EXACT model id") {
		t.Errorf("hint should tell user to fix the configured name; got %q", hint)
	}
	if !strings.Contains(hint, "~/.codex-converter/config.toml") {
		t.Errorf("hint should point at converter config; got %q", hint)
	}
}

func TestModelErrorHint_NameDiffers(t *testing.T) {
	hint := modelErrorHint("typo-model", "glm-4-plus", "glm")
	if !strings.Contains(hint, "typo-model") || !strings.Contains(hint, "glm-4-plus") {
		t.Errorf("hint should mention both requested and configured names; got %q", hint)
	}
	if !strings.Contains(hint, "--model") || !strings.Contains(hint, "stale") {
		t.Errorf("hint should mention --model and stale config; got %q", hint)
	}
}

func TestAugmentErrorMessage_ValidOpenAIError(t *testing.T) {
	out := augmentErrorMessage([]byte(`{"error":{"message":"boom","code":"model_not_found"}}`), "do this")
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v (%s)", err, out)
	}
	if !strings.Contains(parsed.Error.Message, "boom") || !strings.Contains(parsed.Error.Message, "[codex-converter] do this") {
		t.Errorf("augmented message = %q, want original + hint", parsed.Error.Message)
	}
	if parsed.Error.Code != "model_not_found" {
		t.Errorf("code field lost: %q", parsed.Error.Code)
	}
}

func TestAugmentErrorMessage_UnparseableBody(t *testing.T) {
	out := augmentErrorMessage([]byte(`<html>502</html>`), "do this")
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("wrapper not valid JSON: %v (%s)", err, out)
	}
	if !strings.Contains(parsed.Error.Message, "<html>502</html>") || !strings.Contains(parsed.Error.Message, "[codex-converter] do this") {
		t.Errorf("wrapped message = %q, want raw body + hint", parsed.Error.Message)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy/ -run 'TestLooksLikeModelError|TestModelErrorHint|TestAugmentErrorMessage' -v`
Expected: FAIL — `undefined: looksLikeModelError` (and the other three).

- [ ] **Step 3: Write minimal implementation**

Create `internal/proxy/modelerror.go`:

```go
package proxy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// looksLikeModelError reports, conservatively, whether a non-200 backend
// response is most likely caused by an unrecognised model name. A 404 on the
// chat/completions path almost always means the model/resource was not found.
// For 400/422 we require the body to mention "model" alongside a not-found-ish
// phrase, so unrelated client errors are not misattributed.
func looksLikeModelError(status int, body []byte) bool {
	if status == 404 {
		return true
	}
	if status == 400 || status == 422 {
		b := strings.ToLower(string(body))
		if !strings.Contains(b, "model") {
			return false
		}
		for _, marker := range []string{"not found", "does not exist", "unknown", "invalid", "model_not_found"} {
			if strings.Contains(b, marker) {
				return true
			}
		}
	}
	return false
}

// modelErrorHint builds an actionable message. When the requested model equals
// the configured default, the configured name itself is wrong. When they
// differ, the request used `codex --model` or a stale ~/.codex/config.toml.
func modelErrorHint(reqModel, providerModel, providerName string) string {
	if reqModel == providerModel {
		return fmt.Sprintf(
			"Provider %q rejected model %q. This name must match the provider's EXACT model id "+
				"(check the provider's API docs). Fix `model` in ~/.codex-converter/config.toml and restart codex-converter.",
			providerName, reqModel)
	}
	return fmt.Sprintf(
		"Provider %q rejected model %q, but your converter default is %q. "+
			"If you ran `codex --model %s`, that name is wrong. Otherwise ~/.codex/config.toml may be stale — "+
			"restart codex-converter to re-sync.",
		providerName, reqModel, providerModel, reqModel)
}

// augmentErrorMessage appends a hint to an OpenAI-style error body's
// error.message so Codex displays it. If the body is not a parseable
// {"error":{"message":...}} object, it is wrapped in one verbatim.
func augmentErrorMessage(body []byte, hint string) []byte {
	suffix := "\n\n[codex-converter] " + hint

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(body, &parsed); err == nil {
		if rawErr, ok := parsed["error"]; ok {
			var errObj map[string]json.RawMessage
			if err := json.Unmarshal(rawErr, &errObj); err == nil {
				var msg string
				if rawMsg, ok := errObj["message"]; ok {
					_ = json.Unmarshal(rawMsg, &msg)
				}
				newMsg, _ := json.Marshal(msg + suffix)
				errObj["message"] = newMsg
				if reEnc, err := json.Marshal(errObj); err == nil {
					parsed["error"] = reEnc
					if out, err := json.Marshal(parsed); err == nil {
						return out
					}
				}
			}
		}
	}

	// Fallback: wrap the raw body so the client still gets a valid error shape.
	wrapper := map[string]any{
		"error": map[string]any{
			"message": string(body) + suffix,
		},
	}
	out, _ := json.Marshal(wrapper)
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/proxy/ -run 'TestLooksLikeModelError|TestModelErrorHint|TestAugmentErrorMessage' -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/modelerror.go internal/proxy/modelerror_test.go
git commit -m "feat: add model-error diagnosis helpers for proxy guard"
```

---

### Task 2: Wire the guard into the handler (`internal/proxy/handler.go`)

**Files:**
- Modify: `internal/proxy/handler.go:160-167` (the non-200 error block)
- Test: `internal/proxy/handler_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/proxy/handler_test.go`:

```go
func TestHandler_ModelError404_InjectsHint(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"model not found"}}`))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Providers:       []config.Provider{{Name: "glm", BaseURL: backend.URL, Model: "glm-4-plus", AuthStyle: "bearer"}},
		DefaultProvider: "glm",
	}
	handler := NewHandler(cfg)

	// Requested model == configured model → "fix configured name" branch.
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"glm-4-plus","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (original status preserved)", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "[codex-converter]") {
		t.Errorf("response missing injected hint; got %s", body)
	}
	if !strings.Contains(body, "model not found") {
		t.Errorf("response lost original provider message; got %s", body)
	}
	if !strings.Contains(body, "EXACT model id") {
		t.Errorf("expected the name-matches hint branch; got %s", body)
	}
}

func TestHandler_NonModelError_NotInjected(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream exploded"}}`))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Providers:       []config.Provider{{Name: "glm", BaseURL: backend.URL, Model: "glm-4-plus", AuthStyle: "bearer"}},
		DefaultProvider: "glm",
	}
	handler := NewHandler(cfg)

	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"glm-4-plus","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "[codex-converter]") {
		t.Errorf("500 should NOT be injected with a model hint; got %s", w.Body.String())
	}
}

func TestHandler_StreamingModelError404_InjectsHint(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Provider rejects the streaming request with a normal JSON error.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"no such model"}}`))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Providers:       []config.Provider{{Name: "glm", BaseURL: backend.URL, Model: "glm-4-plus", AuthStyle: "bearer"}},
		DefaultProvider: "glm",
	}
	handler := NewHandler(cfg)

	// Requested model differs from configured → "stale / --model" branch.
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"wrong-name","input":"hi","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "[codex-converter]") || !strings.Contains(body, "stale") {
		t.Errorf("streaming 404 should inject the stale/--model hint; got %s", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy/ -run 'TestHandler_ModelError404|TestHandler_NonModelError|TestHandler_StreamingModelError' -v`
Expected: FAIL — current code forwards the body unchanged, so `[codex-converter]` is absent.

- [ ] **Step 3: Write minimal implementation**

In `internal/proxy/handler.go`, replace the block at lines 160-167:

```go
	// Forward backend errors directly
	if resp.StatusCode != http.StatusOK {
		h.logf("✗ backend returned %d; forwarding error body to client", resp.StatusCode)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}
```

with:

```go
	// On a backend error, diagnose likely model-name problems and turn the
	// provider's cryptic error into an actionable hint (logged to the converter
	// console AND injected into the error Codex shows the user). This runs
	// before the streaming branch, so it covers streaming and non-streaming.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if looksLikeModelError(resp.StatusCode, body) {
			hint := modelErrorHint(req.Model, provider.Model, provider.Name)
			h.logf("✗ model error %d: req=%q config=%q — %s", resp.StatusCode, req.Model, provider.Model, hint)
			body = augmentErrorMessage(body, hint)
		} else {
			h.logf("✗ backend returned %d; forwarding error body to client", resp.StatusCode)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
		return
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/proxy/ -v`
Expected: PASS (new tests pass; existing tests unaffected — note `io` is still used elsewhere; if `go vet`/build complains about an unused import, leave `io` since `io.ReadAll` now uses it).

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/handler.go internal/proxy/handler_test.go
git commit -m "feat: turn provider model errors into actionable hints (#issue)"
```

---

### Task 3: auth_style auto-detect for custom providers (`internal/setup`)

**Files:**
- Modify: `internal/setup/setup.go` (add `detectAuthStyle`; use it in the custom-provider path)
- Test: `internal/setup/setup_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/setup/setup_test.go`:

```go
func TestDetectAuthStyle_BearerWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer k" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	style, err := detectAuthStyle(server.URL, "k", "m")
	if err != nil {
		t.Fatalf("detectAuthStyle error = %v", err)
	}
	if style != "bearer" {
		t.Errorf("style = %q, want bearer", style)
	}
}

func TestDetectAuthStyle_FallsBackToApiKeyHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject bearer, accept api-key header.
		if r.Header.Get("api-key") == "k" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	style, err := detectAuthStyle(server.URL, "k", "m")
	if err != nil {
		t.Fatalf("detectAuthStyle error = %v", err)
	}
	if style != "api_key_header" {
		t.Errorf("style = %q, want api_key_header", style)
	}
}

func TestDetectAuthStyle_BothFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	if _, err := detectAuthStyle(server.URL, "k", "m"); err == nil {
		t.Error("detectAuthStyle should return an error when neither auth style works")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/setup/ -run TestDetectAuthStyle -v`
Expected: FAIL — `undefined: detectAuthStyle`.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/setup/setup.go` (after `testConnection`):

```go
// detectAuthStyle probes a custom provider to learn how it expects the API key.
// It tries "bearer" first (the OpenAI-compatible default) and falls back to
// "api_key_header" (Azure-style). It returns the working style, or an error if
// neither authenticates. This frees users from choosing auth_style by hand.
func detectAuthStyle(baseURL, apiKey, model string) (string, error) {
	if err := testConnection(baseURL, apiKey, model, "bearer"); err == nil {
		return "bearer", nil
	}
	if err := testConnection(baseURL, apiKey, model, "api_key_header"); err == nil {
		return "api_key_header", nil
	}
	return "", fmt.Errorf("neither 'bearer' nor 'api_key_header' authenticated; check API key and base URL")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/setup/ -run TestDetectAuthStyle -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/setup/setup.go internal/setup/setup_test.go
git commit -m "feat: auto-detect auth_style (bearer vs api_key_header) for custom providers"
```

---

### Task 4: Wizard wording + double-key-prompt fix + use auto-detect (`internal/setup/setup.go`)

**Files:**
- Modify: `internal/setup/setup.go` — `RunSetup()` (Step 2 / Step 3 / Step 4 interactive flow)

> Note: `RunSetup()` reads `os.Stdin` directly and has no unit test today; these are interactive-flow edits verified by `go build` + manual review. Keep changes minimal and localized.

- [ ] **Step 1: Fix the double API-key prompt**

In `RunSetup()`, the key is read in Step 2 (lines ~307-320) only for masked display, then read AGAIN in Step 4 (lines ~402-416) into the real `apiKey`. Make Step 2 the single source: declare `apiKey` in the outer scope, read it once in Step 2, and DELETE the second prompt in Step 4 (keep the masked-summary print using the already-captured `apiKey`).

Concretely:
- Before Step 1, declare: `var apiKey string`
- In Step 2, change `apiKey, _ := reader.ReadString('\n')` to `apiKey, _ = reader.ReadString('\n')` (assign to the outer var).
- In Step 4, REMOVE the `fmt.Printf("  %sAPI Key:%s ", ...)` re-prompt and its `reader.ReadString`; if you still want a confirmation print, reuse the captured `apiKey` for masking only.

- [ ] **Step 2: Add model-name guidance for custom providers**

In Step 3, at the custom-provider model prompt (line ~328), change the prompt label to guide the user:

```go
		fmt.Printf("  %sModel 名称%s (provider 文档里的精确模型 id, 如 deepseek-v4-pro，不是别名): ", colorBold, colorReset)
```

- [ ] **Step 3: Use auto-detect for custom providers**

In Step 4/5, for `selectedProvider.Name == "custom"`, replace the static `authStyle := selectedProvider.AuthStyle` choice with detection. After `apiKey` and `model.Name` are known and before saving:

```go
	authStyle := selectedProvider.AuthStyle
	if authStyle == "" {
		authStyle = "bearer"
	}
	if selectedProvider.Name == "custom" {
		printInfo("自动探测认证方式 (bearer / api_key_header)...")
		if detected, derr := detectAuthStyle(baseURL, apiKey, model.Name); derr == nil {
			authStyle = detected
			printSuccess(fmt.Sprintf("认证方式: %s (自动探测)", authStyle))
		} else {
			printInfo(fmt.Sprintf("自动探测未通过 (%v)，回退到 bearer", derr))
		}
	}
```

Ensure the `cfg.Providers[0].AuthStyle = authStyle` assignment uses this resolved value (it already does via the `authStyle` variable in the config build).

- [ ] **Step 4: Build and run full setup tests**

Run: `go build ./... && go test ./internal/setup/ -v`
Expected: build OK; all setup tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/setup/setup.go
git commit -m "fix: prompt API key once, guide model id, auto-detect auth in wizard"
```

---

### Task 5: Clarify `config.example.toml`

**Files:**
- Modify: `config.example.toml`

- [ ] **Step 1: Edit comments**

Change the header block (lines 11-15) and the `model` line (line 20) comment to:

```toml
# ============================================================
# 提供商配置
# 你只需要编辑本文件；~/.codex/config.toml 由 converter 自动管理。
# model 必须是 provider 真实模型 id（照抄 provider 文档），
#   它也会被同步进 ~/.codex/config.toml 作为默认模型 —— 不要手改那边的 model。
# auth_style: "bearer"（默认）/ "api_key_header"（MiMo/Azure）/ "none"（Ollama）。
#   custom provider 首次运行时向导会自动探测。
# ============================================================
```

And on the `model` line:

```toml
model = "deepseek-v4-pro"            # provider 真实模型 id（必须照抄 provider 文档）
```

- [ ] **Step 2: Verify TOML still parses**

Run: `go test ./internal/config/ -v`
Expected: PASS (config loader unaffected; comments only).

- [ ] **Step 3: Commit**

```bash
git add config.example.toml
git commit -m "docs: clarify model id and auth_style in example config"
```

---

### Task 6: Rewrite README docs (EN + ZH)

**Files:**
- Modify: `README.md` (Supported Providers note ~L107; Auth Style ~L109-116; Configuration / Two config files ~L119-128; FAQ model/two-files ~L235-239)
- Modify: `README.zh.md` (parallel sections: ~L107 note; ~L109-116 认证方式; ~L121-128 两个配置文件; FAQ ~L235+)

- [ ] **Step 1: Rewrite the "model name — one truth" guidance (EN)**

Replace the `README.md` note at line 107 with:

```markdown
> **There is only one model name: your provider's real model id.** You set it once in
> `~/.codex-converter/config.toml` (`model = "..."`); the converter mirrors it into
> `~/.codex/config.toml` automatically. **Never hand-edit the `model` line in `~/.codex/config.toml`** —
> it is generated. The name must match your provider's docs exactly (e.g. DeepSeek `deepseek-v4-pro`,
> not `deepseek-pro`). A made-up alias is forwarded verbatim and the provider returns 404.
```

- [ ] **Step 2: Rewrite the Auth Style section into a decision guide (EN)**

Replace `README.md` lines 109-116 with:

```markdown
### Auth Style — how to choose

`auth_style` controls which header carries your API key. Decide in order:

| # | Question | Set |
|---|----------|-----|
| 1 | Local model with no key (Ollama)? | `none` |
| 2 | Provider docs show `Authorization: Bearer <key>`? | `bearer` (default; most OpenAI-compatible providers) |
| 3 | Provider docs show `api-key: <key>` (Azure-style)? | `api_key_header` (MiMo, Azure OpenAI) |
| 4 | Not sure? | start with `bearer`; if a correct key still returns 401, switch to `api_key_header` |

> For **Custom** providers the setup wizard auto-detects this for you (tries `bearer`, falls back to `api_key_header`).
```

- [ ] **Step 3: Rewrite "Two config files — who edits what" (EN)**

Replace `README.md` lines 119-128 (the "Two config files" subsection) with:

```markdown
### Two config files — who edits what

| File | You | The converter |
|------|-----|---------------|
| `~/.codex-converter/config.toml` | ✅ The only file you edit (providers, keys, model) | reads it |
| `~/.codex/config.toml` | ⚠️ Only for *other* Codex settings — never touch the converter-written block | auto-writes & syncs `model` + the `[model_providers.codex-converter]` section |

You normally never open `~/.codex/config.toml` by hand. If you do (e.g. to add an MCP server),
leave the converter-managed `model` / `model_provider` / `[model_providers.codex-converter]` lines alone —
they are regenerated on every converter start.
```

- [ ] **Step 4: Update the EN FAQ entries**

In `README.md`, update the two FAQ answers (~L235-239):

```markdown
**Q: What model name should I use?**
A: There is one model name — your provider's exact model id (e.g. DeepSeek `deepseek-v4-pro`). Set it in
`~/.codex-converter/config.toml`; the converter syncs it to Codex. Don't invent aliases — the name is
forwarded as-is and an unknown name returns 404.

**Q: Why two config files?**
A: You edit only `~/.codex-converter/config.toml`. `~/.codex/config.toml` is Codex's own config; the
converter auto-manages the model + provider block inside it so the two never drift.
```

- [ ] **Step 5: Mirror all four edits into `README.zh.md`**

Apply the same four rewrites in Chinese at the parallel locations:

- L107 note → 一个模型名规则：

```markdown
> **模型名只有一个：provider 的真实模型 id。** 你在 `~/.codex-converter/config.toml` 里填一次
> （`model = "..."`），converter 会自动同步进 `~/.codex/config.toml`。**不要手改 `~/.codex/config.toml`
> 里的 `model` 行** —— 它是自动生成的。名字必须和 provider 文档完全一致（如 DeepSeek 用 `deepseek-v4-pro`，
> 不能写 `deepseek-pro`）。乱填别名会被原样转发，provider 直接 404。
```

- L109-116 认证方式 → 决策表：

```markdown
### 认证方式（auth_style）—— 怎么选

`auth_style` 决定 API Key 放在哪个请求头里。按顺序判断：

| # | 问题 | 设为 |
|---|------|------|
| 1 | 本地模型、无需 key（Ollama）？ | `none` |
| 2 | provider 文档写 `Authorization: Bearer <key>`？ | `bearer`（默认，大多数 OpenAI 兼容 provider） |
| 3 | provider 文档写 `api-key: <key>`（Azure 风格）？ | `api_key_header`（MiMo、Azure OpenAI） |
| 4 | 不确定？ | 先用 `bearer`；带正确 key 仍 401，就换 `api_key_header` |

> **自定义** provider 由安装向导自动探测（先试 `bearer`，失败回退 `api_key_header`），你不用手动选。
```

- L121-128 两个配置文件 → 谁编辑谁：

```markdown
### 两个配置文件 —— 谁编辑谁

| 文件 | 你 | 转换器 |
|------|----|--------|
| `~/.codex-converter/config.toml` | ✅ 你唯一需要编辑的文件（提供商、Key、模型） | 读取 |
| `~/.codex/config.toml` | ⚠️ 只在改 Codex *其他* 设置时碰，**别动转换器写入的块** | 自动写入并同步 `model` 和 `[model_providers.codex-converter]` 段 |

正常情况下你根本不用手动打开 `~/.codex/config.toml`。即使要打开（比如加 MCP server），
也别动转换器管理的 `model` / `model_provider` / `[model_providers.codex-converter]` 那几行 —— 它们每次启动都会被重新生成。
```

- FAQ（~L235+）两条改为：

```markdown
**Q: 模型名怎么填？**
A: 模型名只有一个 —— provider 的精确模型 id（如 DeepSeek 的 `deepseek-v4-pro`）。在
`~/.codex-converter/config.toml` 里填，转换器会同步给 Codex。别自己造别名 —— 名字会被原样转发，provider 不认就 404。

**Q: 为什么有两个配置文件？**
A: 你只编辑 `~/.codex-converter/config.toml`。`~/.codex/config.toml` 是 Codex 自己的配置；转换器自动管理
其中的 model 与 provider 段，让两者永不脱节。
```

- [ ] **Step 6: Sanity-check links/format**

Run: `grep -n "codex-converter/config.toml\|api_key_header\|404" README.md README.zh.md | head`
Expected: the new guidance appears in both files.

- [ ] **Step 7: Commit**

```bash
git add README.md README.zh.md
git commit -m "docs: one-truth model name, who-edits-what files, auth_style decision guide"
```

---

### Task 7: Full quality gate + CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Run the full gate**

Run:
```bash
goimports -w . && go vet ./... && go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
```
Expected: vet clean; all tests pass under `-race`; total coverage ≥ 80%.

- [ ] **Step 2: Add a CHANGELOG entry**

Prepend an entry under the next version describing: model-error hint guard; wizard single key prompt + auth auto-detect; docs clarifying model name / two files / auth_style.

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog for config UX clarity + model-error guard"
```

---

## Notes for the implementer

- Do NOT change pass-through proxy semantics — the request `model` still flows to the provider unchanged. The guard only improves the *error path*.
- `looksLikeModelError` is intentionally conservative; if you widen it, add a false-positive test.
- The provider selected for `provider.Model`/`provider.Name` in the hint is the same one the handler already resolved (`default_provider`, else first).

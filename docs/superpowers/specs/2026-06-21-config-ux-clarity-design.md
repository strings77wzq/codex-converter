# Config UX Clarity — Model Name, Two Config Files, auth_style Design Spec

> Date: 2026-06-21 | Topic: config-ux-clarity | Direction: A (merge concept + 404 guard)

## [S1] Problem

Three documentation/UX gaps confuse users during install and configuration:

1. **Model name**: Docs present `~/.codex-converter/config.toml` `model` and `~/.codex/config.toml` `model` as two independently-configurable names. In reality the converter forwards `req.Model` verbatim to the provider (`request.go:17`) and selects the provider by `default_provider` (not by model). So the two names MUST be identical and MUST be the provider's real model id. A user-invented alias in codex.toml causes a cryptic provider 404.
2. **Two config files**: Guidance shows both `nano ~/.codex/config.toml` and `nano ~/.codex-converter/config.toml` without explaining each file's role or who edits it.
3. **auth_style**: No unified rule for choosing `bearer` vs `api_key_header` vs `none`.

## [S2] Root Cause & Direction

The "two model names" is a *false distinction* created by docs, not by code — there is one truth (the provider-real id) synced into two files. Direction A (decided 2026-06-21): **merge the concept** rather than build alias machinery.

- Model name = one value = provider's real id. Converter config is the single source of truth; codex.toml `model` is an auto-generated mirror users never hand-edit.
- Rejected: alias mapping (would create a real two-name divergence; off Codex's native `model = real id` semantics) and force-override (would break `codex --model`).
- Add a guardrail: turn cryptic provider model errors into actionable hints.

## [S3] Change 1 — Model-error guard (`internal/proxy/handler.go`)

Replace the current pass-through error block (handler.go:160-167). On non-200:

1. Buffer the error body with `io.ReadAll`.
2. `looksLikeModelError(status, body)` — conservative, high-confidence only:
   - `status == 404` → true (chat/completions path exists; 404 ≈ model/resource not found).
   - `status == 400 || 422` AND body (lower-cased) contains `"model"` AND one of `not found` / `does not exist` / `unknown` / `invalid` / `model_not_found` → true.
   - else false.
3. If model error: build hint via `modelErrorHint(reqModel, providerModel, providerName)`:
   - `reqModel == providerModel`: configured name itself is wrong → "Provider X rejected model Y. Must match provider's EXACT model id (see provider docs). Fix `model` in ~/.codex-converter/config.toml and restart."
   - `reqModel != providerModel`: stale codex.toml or wrong `--model` → "Provider X rejected Y, converter default is Z. If you ran `codex --model Y`, that name is wrong; otherwise ~/.codex/config.toml may be stale — restart codex-converter to re-sync."
4. Inject hint into Codex response AND log to converter console (decided: both):
   - `augmentErrorMessage(body, hint)`: `json.Unmarshal` to `{"error":{"message":...}}`, append `\n\n[codex-converter] <hint>` to message, re-marshal. On parse failure, wrap: `{"error":{"message":"<original>\n\n[codex-converter] <hint>"}}`.
   - `h.logf("✗ model error %d: req=%q config=%q — %s", ...)`.
5. Write augmented body with original status code.

Placement note: this block is BEFORE `if req.Stream` (handler.go:170), so it covers both streaming and non-streaming (providers reject streaming requests with a normal JSON error + non-200, so buffering is safe).

Non-model errors keep current behavior (forward body unchanged) — no false-positive injection.

## [S4] Change 2 — Setup wizard (`internal/setup/setup.go`)

**(a) Model-name guidance**: at the custom-provider model prompt (~L328), add: "输入 provider 文档里的精确模型 id（如 deepseek-v4-pro），不是别名".

**(b) Fix double API-key prompt [BUG]**: Step 2 (L307-320) reads the key only for masked display; Step 4 (L402-416) reads it AGAIN into the real `apiKey` used for config — user types the key twice. Fix: read once in Step 2, carry `apiKey` through to config build and test. Remove the second prompt.

**(c) auth_style auto-detect (decided: do it)**: For custom providers, `testConnection` tries `bearer` first; on 401/403, automatically retries with `api_key_header`; whichever succeeds is persisted as the provider's `auth_style`. User never chooses auth_style. Known providers keep their built-in `auth_style` (unchanged). Implementation: extend/rename `testConnection` to return the working auth style, or add `detectAuthStyle(baseURL, apiKey, model) (string, error)` that the confirm/test step calls for custom providers.

## [S5] Change 3 — `config.example.toml`

- `model` comment → "provider 真实模型 id（必须照抄 provider 文档），也是同步进 ~/.codex/config.toml 的默认模型".
- Add header note: "你只编辑本文件；~/.codex/config.toml 由 converter 自动管理".

## [S6] Change 4 — Docs (`README.md` + `README.zh.md`, both)

Rewrite three subsections, collapsing the false two-name distinction:

**(1) Model name — one truth**: One rule — the model name is the provider's real id; you set it in `~/.codex-converter/config.toml`; converter mirrors it into `~/.codex/config.toml`; never hand-edit codex.toml's `model`. Add "how to find the exact id" pointers per built-in provider.

**(2) Two config files — who edits what** (table):

| File | You | Converter |
|---|---|---|
| `~/.codex-converter/config.toml` | ✅ the only file you edit | reads |
| `~/.codex/config.toml` | ⚠️ only for other Codex settings; don't touch the converter-written block | auto-writes/syncs model + provider section |

Explicitly resolve the `nano` confusion: you edit codex-converter config; codex config is auto-managed, normally never opened by hand.

**(3) auth_style decision guide**:
```
1. 本地/无 key？                          → none           (Ollama)
2. provider 文档写 Authorization: Bearer？ → bearer         (默认, 大多数 OpenAI 兼容 provider)
3. provider 文档写 api-key:（Azure 风格）？ → api_key_header (MiMo / Azure OpenAI)
4. 不确定？                                → 先 bearer；带正确 key 仍 401 就换 api_key_header
```
Add note: wizard auto-detects for custom providers (per S4c).

## [S7] Tests (TDD)

`internal/proxy/handler_test.go`:
- provider 404 → response `error.message` contains `[codex-converter]` hint.
- `reqModel == config` vs `!= config` → correct hint branch text.
- non-model 500 error → body NOT injected (false-positive guard).
- streaming request hitting provider 404 → same guard applies.

`internal/setup/setup_test.go`:
- API key prompted exactly once (regression for S4b).
- auto-detect: bearer 401 → retries `api_key_header` → persists working style (S4c).

Gates: `go test -race ./...`, coverage ≥ 80%, `go vet`, `goimports`.

## [S8] Scope

In scope: S3–S7 above.

Out of scope (YAGNI): alias mapping, model-based multi-provider routing, hot reload, codex.toml model auto-complete, changing pass-through proxy semantics.

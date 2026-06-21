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

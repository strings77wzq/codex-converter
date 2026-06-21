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

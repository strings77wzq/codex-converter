package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/strings77wzq/codex-converter/internal/config"
)

// Issue: Auth header fallback — when provider has no API key configured,
// the handler falls back to the request's Authorization header.
// This is intended for Codex forwarding its own key, but in a multi-provider
// setup it means request key from provider A leaks to provider B.
func TestHandler_AuthFallbackToRequestHeader(t *testing.T) {
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-fb", "object": "chat.completion", "model": "m",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer backend.Close()

	// Provider with NO api_key and NO api_key_env
	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", BaseURL: backend.URL, Model: "m", AuthStyle: "bearer"},
		},
		DefaultProvider: "test",
	}
	handler := NewHandler(cfg)

	req := httptest.NewRequest("POST", "/v1/responses",
		strings.NewReader(`{"model":"m","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-from-codex")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if gotAuth != "Bearer secret-from-codex" {
		t.Errorf("Authorization = %q, want fallback to request header", gotAuth)
	}
	t.Logf("CONFIRMED: provider with no API key falls back to request Authorization header — request key forwarded as-is")
}

// Issue: When provider has api_key configured AND request also has Authorization,
// provider key wins. Verify this priority.
func TestHandler_ProviderKeyTakesPriority(t *testing.T) {
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-pri", "object": "chat.completion", "model": "m",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer backend.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", BaseURL: backend.URL, Model: "m", APIKey: "provider-key-123", AuthStyle: "bearer"},
		},
		DefaultProvider: "test",
	}
	handler := NewHandler(cfg)

	req := httptest.NewRequest("POST", "/v1/responses",
		strings.NewReader(`{"model":"m","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer request-key-456") // should be ignored
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if gotAuth != "Bearer provider-key-123" {
		t.Errorf("Authorization = %q, want provider key to take priority", gotAuth)
	}
}

// Issue: Handler returns 500 with "no providers configured" when Providers is empty.
// But the error message doesn't tell the user what to do.
func TestHandler_NoProviders_ErrorMessage(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.Provider{},
	}
	handler := NewHandler(cfg)

	req := httptest.NewRequest("POST", "/v1/responses",
		strings.NewReader(`{"model":"m","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "no providers configured") {
		t.Errorf("body = %q, want 'no providers configured'", body)
	}
	t.Logf("CONFIRMED: error message is 'no providers configured' — no guidance on how to fix (run setup wizard, edit config.toml, etc.)")
}

// Issue: normalizeBaseURL edge cases — what happens with unusual URLs.
func TestNormalizeBaseURL_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"double v1", "https://api.example.com/v1/v1", "https://api.example.com/v1"},
		{"chat completions suffix", "https://api.example.com/v1/chat/completions", "https://api.example.com"},
		{"port with trailing slash", "https://api.example.com:8080/", "https://api.example.com:8080"},
		{"empty string", "", ""},
		{"just trailing slash", "https://api.example.com/", "https://api.example.com"},
		// Edge: /v1 as part of a longer path gets stripped too.
		// For custom providers with base_url = "https://example.com/api/v1",
		// the handler will append /v1/chat/completions, giving
		// "https://example.com/api/v1/chat/completions" — which is likely correct
		// but strips user's /v1 and re-adds it. Worth documenting.
		{"api prefix with v1", "https://api.example.com/api/v1", "https://api.example.com/api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.NormalizeBaseURL(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// Issue: Handler doesn't set Access-Control headers.
// If Codex client runs in browser-like environment, CORS will block requests.
func TestHandler_NoCORSHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-cors", "object": "chat.completion", "model": "m",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer backend.Close()

	cfg := &config.Config{
		Providers:       []config.Provider{{Name: "test", BaseURL: backend.URL, Model: "m", AuthStyle: "bearer"}},
		DefaultProvider: "test",
	}
	handler := NewHandler(cfg)

	// Send OPTIONS preflight
	req := httptest.NewRequest("OPTIONS", "/v1/responses", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Currently returns 405 Method Not Allowed — no CORS support
	t.Logf("OPTIONS /v1/responses → status %d (no CORS preflight handling)", w.Code)
}

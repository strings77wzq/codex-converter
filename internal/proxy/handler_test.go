package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/strings77wzq/codex-converter/internal/config"
)

func TestHandler_ResponsesEndpoint(t *testing.T) {
	// Mock backend that returns Chat Completions response
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("backend got path %q, want %q", r.URL.Path, "/v1/chat/completions")
		}

		// Verify content type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("backend got Content-Type %q, want %q", ct, "application/json")
		}

		// Parse request
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode backend request: %v", err)
		}

		if req.Model != "deepseek-v4-pro" {
			t.Errorf("backend got model %q, want %q", req.Model, "deepseek-v4-pro")
		}
		if len(req.Messages) != 1 {
			t.Fatalf("backend got %d messages, want 1", len(req.Messages))
		}
		if req.Messages[0].Role != "user" {
			t.Errorf("backend got role %q, want %q", req.Messages[0].Role, "user")
		}

		// Return Chat Completions response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"model":  "deepseek-v4-pro",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello from backend!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		})
	}))
	defer backend.Close()

	// Create handler with mock provider
	cfg := &config.Config{
		Providers: []config.Provider{
			{
				Name:      "test",
				BaseURL:   backend.URL,
				Model:     "deepseek-v4-pro",
				AuthStyle: "bearer",
			},
		},
		DefaultProvider: "test",
	}

	handler := NewHandler(cfg)

	// Send Responses API request
	reqBody := `{
		"model": "deepseek-v4-pro",
		"input": "Hello",
		"stream": false
	}`

	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Fatalf("handler returned status %d, want %d", w.Code, http.StatusOK)
	}

	// Parse Responses API response
	var resp struct {
		Object string `json:"object"`
		Model  string `json:"model"`
		Output []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Object != "response" {
		t.Errorf("response object = %q, want %q", resp.Object, "response")
	}
	if resp.Model != "deepseek-v4-pro" {
		t.Errorf("response model = %q, want %q", resp.Model, "deepseek-v4-pro")
	}
	if len(resp.Output) != 1 {
		t.Fatalf("output len = %d, want 1", len(resp.Output))
	}
	if resp.Output[0].Type != "message" {
		t.Errorf("output[0].type = %q, want %q", resp.Output[0].Type, "message")
	}
	if resp.Output[0].Content[0].Text != "Hello from backend!" {
		t.Errorf("output[0].content[0].text = %q", resp.Output[0].Content[0].Text)
	}
	if resp.Usage == nil {
		t.Fatal("usage is nil")
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("usage.input_tokens = %d, want 10", resp.Usage.InputTokens)
	}
}

func TestHandler_StreamingLargeLine(t *testing.T) {
	bigContent := strings.Repeat("x", 100*1024) // > 64KB single SSE line
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl-big\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n")
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl-big\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%s\"}}]}\n", bigContent)
		fmt.Fprintf(w, "data: {\"id\":\"chatcmpl-big\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n")
		fmt.Fprintf(w, "data: [DONE]\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer backend.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", BaseURL: backend.URL, Model: "deepseek-v4-pro", AuthStyle: "bearer"},
		},
		DefaultProvider: "test",
	}
	handler := NewHandler(cfg)

	reqBody := `{"model":"deepseek-v4-pro","input":"Hi","stream":true}`
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "event: error") {
		t.Fatalf("stream produced an error event (line buffer too small)")
	}
	if !strings.Contains(body, bigContent) {
		t.Fatal("large content was truncated or missing from the stream")
	}
}

func TestHandler_BackendTimeout(t *testing.T) {
	// Backend that hangs without ever sending response headers.
	release := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // block until test cleanup
	}))
	defer backend.Close()
	defer close(release)

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", BaseURL: backend.URL, Model: "deepseek-v4-pro", AuthStyle: "bearer"},
		},
		DefaultProvider: "test",
	}

	handler := NewHandler(cfg)
	// White-box: shrink the response-header timeout so the test is fast.
	handler.client = &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 200 * time.Millisecond,
		},
	}

	reqBody := `{"model":"deepseek-v4-pro","input":"Hello","stream":false}`
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d (502) on backend header timeout", w.Code, http.StatusBadGateway)
	}

	if body := w.Body.String(); !strings.Contains(body, "timeout") {
		t.Errorf("response body = %q, want it to mention 'timeout'", body)
	}
}

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

func TestHandler_HealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", BaseURL: "http://localhost", Model: "test"},
		},
	}

	handler := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health returned status %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("health status = %q, want %q", resp["status"], "ok")
	}
	// The service identity lets a preflight probe recognise our own instance.
	if resp["service"] != "codex-converter" {
		t.Errorf("health service = %q, want %q", resp["service"], "codex-converter")
	}
}

func TestHandler_LogsRequestAndBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-log",
			"object": "chat.completion",
			"model":  "m",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]any{"role": "assistant", "content": "hi"}, "finish_reason": "stop"},
			},
		})
	}))
	defer backend.Close()

	cfg := &config.Config{
		Providers:       []config.Provider{{Name: "test", BaseURL: backend.URL, Model: "m", AuthStyle: "bearer"}},
		DefaultProvider: "test",
	}
	handler := NewHandler(cfg)

	var buf bytes.Buffer
	handler.logger = log.New(&buf, "", 0)

	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"m","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, "/v1/responses") {
		t.Errorf("log missing request line; got:\n%s", out)
	}
	if !strings.Contains(out, "backend") || !strings.Contains(out, "200") {
		t.Errorf("log missing backend status; got:\n%s", out)
	}
}

func TestHandler_BodyWithinLimit(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-ok",
			"object": "chat.completion",
			"model":  "m",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]any{"role": "assistant", "content": "hi"}, "finish_reason": "stop"},
			},
		})
	}))
	defer backend.Close()

	cfg := &config.Config{
		Server:          config.Server{MaxBodyMB: 1},
		Providers:       []config.Provider{{Name: "test", BaseURL: backend.URL, Model: "m", AuthStyle: "bearer"}},
		DefaultProvider: "test",
	}
	handler := NewHandler(cfg)

	// 500KB body — under 1MB limit
	smallBody := strings.Repeat("x", 500*1024)
	reqBody := fmt.Sprintf(`{"model":"m","input":"%s","stream":false}`, smallBody)

	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body within limit)", w.Code)
	}
}

func TestHandler_BodyExceedsLimit(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("backend should not be called when body exceeds limit")
	}))
	defer backend.Close()

	cfg := &config.Config{
		Server:          config.Server{MaxBodyMB: 1},
		Providers:       []config.Provider{{Name: "test", BaseURL: backend.URL, Model: "m", AuthStyle: "bearer"}},
		DefaultProvider: "test",
	}
	handler := NewHandler(cfg)

	// 2MB body — exceeds 1MB limit
	bigInput := strings.Repeat("x", 2*1024*1024)
	reqBody := fmt.Sprintf(`{"model":"m","input":"%s","stream":false}`, bigInput)

	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body exceeds limit)", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "too large") {
		t.Errorf("response body should mention 'too large'; got %s", body)
	}
}

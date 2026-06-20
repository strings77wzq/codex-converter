package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codex-converter/internal/config"
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
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"model":   "deepseek-v4-pro",
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
}

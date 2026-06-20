package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codex-converter/internal/config"
)

func TestIntegration_MockBackend(t *testing.T) {
	// Mock backend that simulates DeepSeek behavior
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Stream {
			// Streaming response
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)

			chunks := []string{
				`{"id":"chatcmpl-mock","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-mock","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-mock","choices":[{"index":0,"delta":{"content":" from"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-mock","choices":[{"index":0,"delta":{"content":" mock"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-mock","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
				`[DONE]`,
			}

			for _, chunk := range chunks {
				fmt.Fprintf(w, "data: %s\n\n", chunk)
				flusher.Flush()
			}
		} else {
			// Non-streaming response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "chatcmpl-mock",
				"object": "chat.completion",
				"model":  req.Model,
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello from mock backend!",
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
		}
	}))
	defer backend.Close()

	// Create handler
	cfg := &config.Config{
		Providers: []config.Provider{
			{
				Name:      "mock",
				BaseURL:   backend.URL,
				Model:     "mock-model",
				AuthStyle: "bearer",
			},
		},
		DefaultProvider: "mock",
	}

	handler := NewHandler(cfg)

	// Test 1: Non-streaming
	t.Run("NonStreaming", func(t *testing.T) {
		reqBody := `{"model":"mock-model","input":"Hello","stream":false}`
		req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)

		if resp["object"] != "response" {
			t.Errorf("object = %v, want %v", resp["object"], "response")
		}

		output, ok := resp["output"].([]interface{})
		if !ok || len(output) == 0 {
			t.Fatalf("output is empty or not array: %v", resp["output"])
		}

		msg := output[0].(map[string]interface{})
		if msg["type"] != "message" {
			t.Errorf("output[0].type = %v, want %v", msg["type"], "message")
		}
	})

	// Test 2: Streaming
	t.Run("Streaming", func(t *testing.T) {
		reqBody := `{"model":"mock-model","input":"Hello","stream":true}`
		req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		// Parse SSE events
		scanner := bufio.NewScanner(bytes.NewReader(w.Body.Bytes()))
		var events []string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				events = append(events, strings.TrimPrefix(line, "event: "))
			}
		}

		expected := []string{
			"response.created",
			"response.output_item.added",
			"response.output_text.delta",
			"response.output_text.delta",
			"response.output_text.delta",
			"response.output_item.done",
			"response.completed",
		}

		if len(events) != len(expected) {
			t.Fatalf("events count = %d, want %d\nevents: %v", len(events), len(expected), events)
		}

		for i, ev := range events {
			if ev != expected[i] {
				t.Errorf("events[%d] = %q, want %q", i, ev, expected[i])
			}
		}
	})
}

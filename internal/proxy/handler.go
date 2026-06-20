package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/codex-converter/internal/config"
	"github.com/codex-converter/internal/convert"
	"github.com/codex-converter/internal/types"
)

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
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
				// No client.Timeout: body read (streaming) is intentionally unbounded.
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/health" && r.Method == "GET":
		h.handleHealth(w, r)
	case r.URL.Path == "/v1/responses" && r.Method == "POST":
		h.handleResponses(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleResponses(w http.ResponseWriter, r *http.Request) {
	// Parse Responses API request
	var req types.ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Convert to Chat Completions
	chatReq, err := convert.ConvertRequest(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("conversion error: %v", err), http.StatusBadRequest)
		return
	}

	// Get provider config
	if len(h.cfg.Providers) == 0 {
		http.Error(w, "no providers configured", http.StatusInternalServerError)
		return
	}

	provider := h.cfg.Providers[0]
	if h.cfg.DefaultProvider != "" {
		for _, p := range h.cfg.Providers {
			if p.Name == h.cfg.DefaultProvider {
				provider = p
				break
			}
		}
	}

	// Forward to backend
	chatJSON, err := json.Marshal(chatReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal error: %v", err), http.StatusInternalServerError)
		return
	}

	backendURL := strings.TrimRight(provider.BaseURL, "/") + "/v1/chat/completions"
	reqBackend, err := http.NewRequestWithContext(r.Context(), "POST", backendURL, bytes.NewReader(chatJSON))
	if err != nil {
		http.Error(w, fmt.Sprintf("backend request error: %v", err), http.StatusInternalServerError)
		return
	}

	reqBackend.Header.Set("Content-Type", "application/json")

	// Get API key: first from request header, then from config, then from env var
	apiKey := r.Header.Get("Authorization")
	if apiKey == "" {
		// Try to get from config (direct API key)
		if provider.APIKey != "" {
			apiKey = "Bearer " + provider.APIKey
		} else if provider.APIKeyEnv != "" {
			// Fall back to environment variable
			if envKey := os.Getenv(provider.APIKeyEnv); envKey != "" {
				apiKey = "Bearer " + envKey
			}
		}
	}

	if provider.AuthStyle == "bearer" || provider.AuthStyle == "" {
		if apiKey != "" {
			reqBackend.Header.Set("Authorization", apiKey)
		}
	} else if provider.AuthStyle == "api_key_header" {
		if apiKey != "" {
			key := strings.TrimPrefix(apiKey, "Bearer ")
			reqBackend.Header.Set("api-key", key)
		}
	}

	// Execute request
	resp, err := h.client.Do(reqBackend)
	if err != nil {
		http.Error(w, fmt.Sprintf("backend error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Forward backend errors directly
	if resp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	// Handle streaming
	if req.Stream {
		h.handleStreamingResponse(w, resp, req)
		return
	}

	// Handle non-streaming
	var chatResp types.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		http.Error(w, fmt.Sprintf("backend response error: %v", err), http.StatusBadGateway)
		return
	}

	// Convert response
	respAPI, err := convert.ConvertResponse(&chatResp)
	if err != nil {
		http.Error(w, fmt.Sprintf("response conversion error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(respAPI)
}

func (h *Handler) handleStreamingResponse(w http.ResponseWriter, resp *http.Response, req types.ResponsesRequest) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // initial 64KB, max 1MB per SSE line
	events := convert.ConvertStream(scanner)

	for event := range events {
		fmt.Fprintf(w, "event: %s\n", event.Type)
		fmt.Fprintf(w, "data: %s\n\n", event.Data)
		flusher.Flush()
	}
}

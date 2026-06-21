package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/strings77wzq/codex-converter/internal/config"
	"github.com/strings77wzq/codex-converter/internal/convert"
	"github.com/strings77wzq/codex-converter/internal/types"
)

// healthServiceName identifies our /health responses so an instance can
// recognise itself during a port preflight probe (see preflight.go).
const healthServiceName = "codex-converter"

type Handler struct {
	cfg    *config.Config
	client *http.Client
	logger *log.Logger
}

func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		cfg:    cfg,
		logger: log.New(os.Stderr, "[codex-converter] ", log.LstdFlags),
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

// logf writes a diagnostic line if a logger is configured.
func (h *Handler) logf(format string, args ...any) {
	if h.logger != nil {
		h.logger.Printf(format, args...)
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/health" && r.Method == "GET":
		h.handleHealth(w, r)
	case r.URL.Path == "/v1/responses" && r.Method == "POST",
		r.URL.Path == "/responses" && r.Method == "POST":
		h.handleResponses(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": healthServiceName,
	})
}

func (h *Handler) handleResponses(w http.ResponseWriter, r *http.Request) {
	// Parse Responses API request
	var req types.ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logf("%s %s -> 400 invalid request: %v", r.Method, r.URL.Path, err)
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	h.logf("%s %s model=%q stream=%v", r.Method, r.URL.Path, req.Model, req.Stream)

	// Convert to Chat Completions
	chatReq, err := convert.ConvertRequest(&req)
	if err != nil {
		h.logf("conversion error: %v", err)
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

	backendURL := normalizeBaseURL(provider.BaseURL) + "/v1/chat/completions"
	reqBackend, err := http.NewRequestWithContext(r.Context(), "POST", backendURL, bytes.NewReader(chatJSON))
	if err != nil {
		http.Error(w, fmt.Sprintf("backend request error: %v", err), http.StatusInternalServerError)
		return
	}

	reqBackend.Header.Set("Content-Type", "application/json")

	// Get API key: provider config first, then request header as fallback
	apiKey := ""
	if provider.APIKey != "" {
		apiKey = "Bearer " + provider.APIKey
	} else if provider.APIKeyEnv != "" {
		if envKey := os.Getenv(provider.APIKeyEnv); envKey != "" {
			apiKey = "Bearer " + envKey
		}
	}
	if apiKey == "" {
		apiKey = r.Header.Get("Authorization")
	}

	if apiKey != "" {
		switch provider.AuthStyle {
		case "api_key_header":
			key := strings.TrimPrefix(apiKey, "Bearer ")
			reqBackend.Header.Set("api-key", key)
		default: // "bearer" or unset
			reqBackend.Header.Set("Authorization", apiKey)
		}
	}

	// Execute request
	h.logf("→ backend POST %s", backendURL)
	start := time.Now()
	resp, err := h.client.Do(reqBackend)
	if err != nil {
		h.logf("✗ backend error after %s: %v", time.Since(start).Round(time.Millisecond), err)
		http.Error(w, fmt.Sprintf("backend error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	h.logf("← backend %d in %s", resp.StatusCode, time.Since(start).Round(time.Millisecond))

	// Forward backend errors directly
	if resp.StatusCode != http.StatusOK {
		h.logf("✗ backend returned %d; forwarding error body to client", resp.StatusCode)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	// Handle streaming
	if req.Stream {
		h.handleStreamingResponse(w, resp)
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

func (h *Handler) handleStreamingResponse(w http.ResponseWriter, resp *http.Response) {
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

	count := 0
	for event := range events {
		fmt.Fprintf(w, "event: %s\n", event.Type)
		fmt.Fprintf(w, "data: %s\n\n", event.Data)
		flusher.Flush()
		count++
		if event.Type == "error" {
			h.logf("✗ stream error event: %s", event.Data)
		}
	}
	h.logf("✓ stream done (%d events)", count)
}

// normalizeBaseURL cleans a user-supplied base URL so it can be safely
// combined with a path suffix like "/v1/chat/completions". It strips
// trailing slashes and known common suffixes that users may copy from docs.
func normalizeBaseURL(raw string) string {
	u := strings.TrimRight(raw, "/")
	u = strings.TrimSuffix(u, "/v1/chat/completions")
	u = strings.TrimSuffix(u, "/v1")
	u = strings.TrimSuffix(u, "/chat/completions")
	return u
}

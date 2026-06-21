package setup

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTestConnection_Success(t *testing.T) {
	// 创建mock server模拟成功的API响应
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		// 返回成功响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"test","choices":[{"message":{"content":"hi"}}]}`))
	}))
	defer server.Close()

	err := testConnection(server.URL, "test-key", "test-model", "bearer")
	if err != nil {
		t.Errorf("testConnection() error = %v", err)
	}
}

func TestTestConnection_BearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证Bearer认证
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key-123" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-key-123")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	err := testConnection(server.URL, "test-key-123", "test-model", "bearer")
	if err != nil {
		t.Errorf("testConnection() error = %v", err)
	}
}

func TestTestConnection_ApiKeyHeaderAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证api-key头
		apiKey := r.Header.Get("api-key")
		if apiKey != "test-key-456" {
			t.Errorf("api-key = %q, want %q", apiKey, "test-key-456")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	err := testConnection(server.URL, "test-key-456", "test-model", "api_key_header")
	if err != nil {
		t.Errorf("testConnection() error = %v", err)
	}
}

func TestTestConnection_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	err := testConnection(server.URL, "wrong-key", "test-model", "bearer")
	if err == nil {
		t.Error("testConnection() should return error for 401")
	}
}

func TestTestConnection_ModelNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer server.Close()

	err := testConnection(server.URL, "test-key", "nonexistent-model", "bearer")
	if err == nil {
		t.Error("testConnection() should return error for 404")
	}
}

func TestTestConnection_CleansURL(t *testing.T) {
	// 测试URL清洗功能
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证URL被正确清洗
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("URL path = %q, want %q", r.URL.Path, "/v1/chat/completions")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	// 测试带后缀的URL
	err := testConnection(server.URL+"/v1/chat/completions", "test-key", "test-model", "bearer")
	if err != nil {
		t.Errorf("testConnection() error = %v", err)
	}
}

func TestDetectAuthStyle_BearerSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer auth, got Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	style, err := detectAuthStyle(server.URL, "test-key", "test-model")
	if err != nil {
		t.Fatalf("detectAuthStyle() error = %v", err)
	}
	if style != "bearer" {
		t.Errorf("detectAuthStyle() = %q, want %q", style, "bearer")
	}
}

func TestDetectAuthStyle_BearerFailsApiKeyHeaderSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			// Bearer was tried and failed
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		if r.Header.Get("api-key") != "test-key" {
			t.Errorf("expected api-key header, got api-key = %q", r.Header.Get("api-key"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	style, err := detectAuthStyle(server.URL, "test-key", "test-model")
	if err != nil {
		t.Fatalf("detectAuthStyle() error = %v", err)
	}
	if style != "api_key_header" {
		t.Errorf("detectAuthStyle() = %q, want %q", style, "api_key_header")
	}
}

func TestDetectAuthStyle_BothFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	_, err := detectAuthStyle(server.URL, "wrong-key", "test-model")
	if err == nil {
		t.Error("detectAuthStyle() should return error when both auth styles fail")
	}
}

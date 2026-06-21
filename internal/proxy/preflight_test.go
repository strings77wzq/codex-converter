package proxy

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// hostPort extracts host + numeric port from an httptest server URL.
func hostPort(t *testing.T, srvURL string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(srvURL, "http://"))
	if err != nil {
		t.Fatalf("split %q: %v", srvURL, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi %q: %v", portStr, err)
	}
	return host, port
}

func TestProbePort_Free(t *testing.T) {
	// Grab a free port, then release it so nothing is listening.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	if got := ProbePort("127.0.0.1", port, 500*time.Millisecond); got != PortFree {
		t.Errorf("ProbePort on closed port = %v, want PortFree", got)
	}
}

func TestProbePort_Ours(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"status":"ok","service":"codex-converter"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	host, port := hostPort(t, srv.URL)
	if got := ProbePort(host, port, time.Second); got != PortOurs {
		t.Errorf("ProbePort on our server = %v, want PortOurs", got)
	}
}

func TestProbePort_BusyOther(t *testing.T) {
	// A server that responds 200 but is NOT codex-converter.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "I am some other service")
	}))
	defer srv.Close()

	host, port := hostPort(t, srv.URL)
	if got := ProbePort(host, port, time.Second); got != PortBusyOther {
		t.Errorf("ProbePort on foreign server = %v, want PortBusyOther", got)
	}
}

func TestIsAddrInUse(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"syscall", syscall.EADDRINUSE, true},
		{"wrapped string", errors.New("listen tcp 127.0.0.1:8080: bind: address already in use"), true},
		{"unrelated", errors.New("connection refused"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAddrInUse(tc.err); got != tc.want {
				t.Errorf("IsAddrInUse(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestStartupAdvice(t *testing.T) {
	t.Run("ours exits cleanly", func(t *testing.T) {
		msg, code, proceed := StartupAdvice(PortOurs, "127.0.0.1", 8080)
		if proceed {
			t.Error("should not proceed when our instance already runs")
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0 (already running is success)", code)
		}
		if !strings.Contains(msg, "codex") || msg == "" {
			t.Errorf("message %q should explain it is already running", msg)
		}
	})

	t.Run("busy by other fails with guidance", func(t *testing.T) {
		msg, code, proceed := StartupAdvice(PortBusyOther, "127.0.0.1", 8080)
		if proceed {
			t.Error("should not proceed when port is taken by another program")
		}
		if code == 0 {
			t.Error("exit code should be non-zero when blocked")
		}
		if !strings.Contains(msg, "8080") || !strings.Contains(msg, "fuser") {
			t.Errorf("message %q should give actionable commands (port + how to free it)", msg)
		}
	})

	t.Run("free proceeds", func(t *testing.T) {
		_, _, proceed := StartupAdvice(PortFree, "127.0.0.1", 8080)
		if !proceed {
			t.Error("should proceed to bind when port is free")
		}
	})
}

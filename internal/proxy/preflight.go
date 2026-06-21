package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PortState classifies what (if anything) occupies a TCP port.
type PortState int

const (
	// PortFree means nothing is listening on the port.
	PortFree PortState = iota
	// PortOurs means a codex-converter instance is already serving the port.
	PortOurs
	// PortBusyOther means some other program holds the port.
	PortBusyOther
)

// ProbePort reports what is occupying host:port.
//
// It connects DIRECTLY via raw TCP and an explicitly proxy-less HTTP client so
// that a system HTTP(S)_PROXY (a common cause of localhost confusion) cannot
// mask the result. A refused connection means the port is free; an open port
// whose /health identifies itself as codex-converter is ours; anything else is
// treated as occupied by another program.
func ProbePort(host string, port int, timeout time.Duration) PortState {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return PortFree
	}
	_ = conn.Close()

	client := &http.Client{
		Timeout: timeout,
		// Proxy: nil — never route a localhost health probe through a proxy.
		Transport: &http.Transport{Proxy: nil},
	}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", addr))
	if err != nil {
		return PortBusyOther
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return PortBusyOther
	}
	var body struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body); err == nil &&
		body.Service == healthServiceName {
		return PortOurs
	}
	return PortBusyOther
}

// IsAddrInUse reports whether err is an "address already in use" bind failure.
func IsAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	return strings.Contains(err.Error(), "address already in use")
}

// StartupAdvice maps a PortState to a user-facing message, a process exit code,
// and whether startup should proceed to bind the port.
func StartupAdvice(state PortState, host string, port int) (msg string, exitCode int, proceed bool) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	switch state {
	case PortFree:
		return "", 0, true
	case PortOurs:
		return fmt.Sprintf(
			"✅ codex-converter 已在 %s 运行,无需重复启动。\n   直接在另一个终端运行 `codex` 即可。",
			addr), 0, false
	default: // PortBusyOther
		return fmt.Sprintf(
			"❌ 端口 %s 已被另一个程序占用(不是 codex-converter),无法启动。\n"+
				"   排查与修复:\n"+
				"     • 查看占用进程:  lsof -i:%d   (或 ss -ltnp | grep :%d)\n"+
				"     • 释放该端口:    fuser -k %d/tcp\n"+
				"     • 或换端口启动:  codex-converter --port <PORT>\n"+
				"       (换端口后需同步把 ~/.codex/config.toml 里 codex-converter 的 base_url 改成同一端口)",
			addr, port, port, port), 1, false
	}
}

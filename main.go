package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/strings77wzq/codex-converter/internal/config"
	"github.com/strings77wzq/codex-converter/internal/proxy"
	"github.com/strings77wzq/codex-converter/internal/setup"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	port := flag.Int("port", 0, "server port (overrides config)")
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *showVersion {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			fmt.Println("codex-converter", info.Main.Version)
		} else {
			fmt.Println("codex-converter (devel)")
		}
		return
	}

	var cfg *config.Config
	var err error

	// Check if first run or config file specified
	if *configPath == "" && setup.IsFirstRun() {
		// Interactive setup
		setupCfg, setupErr := setup.RunSetup()
		if setupErr != nil {
			log.Fatalf("setup failed: %v", setupErr)
		}

		// Convert setup config to config.Config
		cfg = &config.Config{
			Server: config.Server{
				Port: setupCfg.Server.Port,
				Host: setupCfg.Server.Host,
			},
			Providers: make([]config.Provider, len(setupCfg.Providers)),
		}

		for i, p := range setupCfg.Providers {
			cfg.Providers[i] = config.Provider{
				Name:      p.Name,
				BaseURL:   p.BaseURL,
				Model:     p.Model,
				APIKey:    p.APIKey,
				AuthStyle: p.AuthStyle,
			}
		}

		cfg.DefaultProvider = setupCfg.Default
	} else {
		// Load from config file
		if *configPath == "" {
			// Try to find config in home directory
			homeDir, err := os.UserHomeDir()
			if err == nil {
				homeConfig := filepath.Join(homeDir, ".codex-converter", "config.toml")
				if _, statErr := os.Stat(homeConfig); statErr == nil {
					*configPath = homeConfig
				}
			}
			if *configPath == "" {
				*configPath = "config.toml"
			}
		}
		cfg, err = config.Load(*configPath)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		// Show startup banner for existing config
		fmt.Println()
		fmt.Println("════════════════════════════════════════════════════")
		if len(cfg.Providers) > 0 {
			fmt.Printf("  Provider: %s\n", cfg.DefaultProvider)
			fmt.Printf("  Base URL: %s\n", cfg.Providers[0].BaseURL)
			fmt.Printf("  Model:    %s\n", cfg.Providers[0].Model)
		}
		fmt.Println("════════════════════════════════════════════════════")
		fmt.Println()
	}

	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Sync model config to Codex (non-fatal)
	if err := config.SyncCodexConfig(cfg); err != nil {
		log.Printf("sync codex config: %v", err)
	}

	// Create handler with API key from setup config
	handler := proxy.NewHandler(cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Preflight: classify the port before binding so users get an actionable
	// message instead of a cryptic "address already in use" crash.
	switch state := proxy.ProbePort(cfg.Server.Host, cfg.Server.Port, 2*time.Second); state {
	case proxy.PortFree:
		// proceed to bind
	default:
		msg, code, proceed := proxy.StartupAdvice(state, cfg.Server.Host, cfg.Server.Port)
		if !proceed {
			fmt.Println()
			fmt.Println(msg)
			fmt.Println()
			os.Exit(code)
		}
	}

	fmt.Printf("  🚀 服务已启动 %s\n", addr)
	fmt.Println()
	fmt.Println("  现在你可以直接运行: codex")
	fmt.Println("  按 Ctrl+C 停止服务")
	fmt.Println()

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Minute,  // generous for streaming requests
		WriteTimeout: 10 * time.Minute, // generous for streaming responses
		IdleTimeout:  2 * time.Minute,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	// First signal: drain in-flight requests (30s timeout).
	// Second signal: force exit immediately.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println("  ⏳ 等待请求完成后关闭（再按 Ctrl+C 强制退出）")
		go func() {
			<-sigCh
			os.Exit(1)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			fmt.Printf("  ✗ 关闭超时: %v\n", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// A race can still lose the bind between preflight and Listen; give the
		// same actionable guidance instead of a raw Go error.
		if proxy.IsAddrInUse(err) {
			msg, code, _ := proxy.StartupAdvice(proxy.PortBusyOther, cfg.Server.Host, cfg.Server.Port)
			fmt.Println()
			fmt.Println(msg)
			fmt.Println()
			os.Exit(code)
		}
		log.Fatalf("server error: %v", err)
	}
	fmt.Println("  ✓ 服务已关闭")
}

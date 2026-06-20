package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/codex-converter/internal/config"
	"github.com/codex-converter/internal/proxy"
	"github.com/codex-converter/internal/setup"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	port := flag.Int("port", 0, "server port (overrides config)")
	flag.Parse()

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
	}

	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Create handler with API key from setup config
	handler := proxy.NewHandler(cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("codex-converter listening on %s", addr)
	if len(cfg.Providers) > 0 {
		log.Printf("provider: %s (%s)", cfg.DefaultProvider, cfg.Providers[0].BaseURL)
	}

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

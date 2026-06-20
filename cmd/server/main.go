package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/codex-converter/internal/config"
	"github.com/codex-converter/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	port := flag.Int("port", 0, "server port (overrides config)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if *port > 0 {
		cfg.Server.Port = *port
	}

	handler := proxy.NewHandler(cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("codex-converter listening on %s", addr)
	log.Printf("provider: %s (%s)", cfg.DefaultProvider, cfg.Providers[0].BaseURL)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

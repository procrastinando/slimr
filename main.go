package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"slimr/internal/config"
	"slimr/internal/scheduler"
	"slimr/internal/server"
)

//go:embed web
var webFS embed.FS

func main() {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".slimr", "config.json")

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	cfg.ExpandPaths()

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Fatalf("mkdir config: %v", err)
	}
	if err := config.Save(configPath, cfg); err != nil {
		log.Fatalf("save default config: %v", err)
	}

	if err := os.MkdirAll(cfg.OutputPath, 0755); err != nil {
		log.Printf("warn: cannot create output dir %s: %v", cfg.OutputPath, err)
	}

	broadcaster := server.NewLogBroadcaster()
	sched := scheduler.New(cfg, configPath, broadcaster)
	srv := server.New(cfg, configPath, sched, webFS)

	addr := cfg.BindAddress + ":" + cfg.Port
	fmt.Printf("Slimr running on http://%s\n", addr)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		sched.Stop()
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

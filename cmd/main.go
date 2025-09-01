package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"mcp-xlsm-server/internal/server"
	"mcp-xlsm-server/pkg/config"
)

func main() {
	// Parse command line flags to determine mode
	var stdioMode bool
	var configPath string
	flag.BoolVar(&stdioMode, "stdio", false, "Run in stdio mode for MCP integration")
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	var cfg *config.Config
	var err error
	
	if configPath != "" {
		cfg, err = config.LoadFromPath(configPath)
	} else {
		cfg, err = config.Load()
	}
	
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if stdioMode {
		// Run in stdio mode for Claude Code MCP integration
		// Don't log to stdout to avoid interfering with MCP communication
		if err := srv.StartStdio(ctx); err != nil {
			log.Fatalf("Stdio server failed: %v", err)
		}
	} else {
		// Start HTTP server
		go func() {
			log.Printf("Starting MCP XLSM server on %s:%d", cfg.Server.Host, cfg.Server.Port)
			if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Server failed: %v", err)
			}
		}()

		// Wait for interrupt signal
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		log.Println("Shutting down server...")

		// Graceful shutdown with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownGracePeriod)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}

		log.Println("Server exited")
	}
}
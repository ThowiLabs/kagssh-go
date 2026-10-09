//go:build linux

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"github.com/ThowiLabs/kagssh-go/internal/mcp"
	"github.com/ThowiLabs/kagssh-go/internal/sshserver"
	"github.com/ThowiLabs/kagssh-go/internal/tunnel"
)

var version = "dev"

func main() {
	check := flag.Bool("check", false, "validar configuración y terminar")
	showVersion := flag.Bool("version", false, "mostrar versión")
	flag.Parse()
	if *showVersion {
		fmt.Println("kagmcp", version)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.FromEnv(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuración:", err)
		os.Exit(2)
	}
	if *check {
		fmt.Println("configuración válida para KagMCP")
		return
	}
	if err := config.SaveSettings(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "guardar preferencias KagMCP:", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if os.Geteuid() == 0 {
		log.Warn("KagMCP corre como root: las herramientas MCP/SSH tendrán privilegios del proceso")
	}
	results := make(chan error, 3)
	services := 0
	if cfg.SSHEnabled {
		ready := make(chan error, 1)
		services++
		go func() { results <- sshserver.Run(ctx, cfg, log, ready) }()
		if err := <-ready; err != nil {
			stop()
			log.Error("no se pudo iniciar SSH", "error", err)
			os.Exit(1)
		}
		services++
		go func() { results <- tunnel.Run(ctx, cfg, log) }()
	}
	if cfg.MCPEnabled {
		services++
		go func() { results <- mcp.Run(ctx, cfg, log) }()
	}
	completed := 0
	failed := false
	select {
	case err = <-results:
		completed++
		if err != nil && !errors.Is(err, context.Canceled) {
			failed = true
			log.Error("servicio KagMCP terminó con error", "error", err)
		}
	case <-ctx.Done():
	}
	stop()
	// Permitir que el servidor HTTP, SSH y el proceso cloudflared cierren
	// antes de que el binario termine: Detener en Kaggle cierra todo.
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	for completed < services {
		select {
		case <-results:
			completed++
		case <-deadline.C:
			log.Warn("tiempo de cierre agotado; terminando proceso")
			completed = services
		}
	}
	log.Info("KagMCP detenido")
	if failed {
		os.Exit(1)
	}
}

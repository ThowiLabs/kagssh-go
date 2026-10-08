//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"github.com/ThowiLabs/kagssh-go/internal/sshserver"
	"github.com/ThowiLabs/kagssh-go/internal/tunnel"
)

var version = "dev"

func main() {
	check := flag.Bool("check", false, "validar configuración y terminar")
	showVersion := flag.Bool("version", false, "mostrar versión")
	flag.Parse()
	if *showVersion {
		fmt.Println("kagssh", version)
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
		fmt.Println("configuración válida")
		return
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if os.Geteuid() == 0 {
		log.Warn("el proceso corre como root: las sesiones SSH tendrán privilegios root")
	}
	ready := make(chan error, 1)
	serverDone := make(chan error, 1)
	tunnelDone := make(chan error, 1)
	go func() { serverDone <- sshserver.Run(ctx, cfg, log, ready) }()
	if err := <-ready; err != nil {
		log.Error("no se pudo iniciar SSH", "error", err)
		os.Exit(1)
	}
	go func() { tunnelDone <- tunnel.Run(ctx, cfg, log) }()
	select {
	case err = <-serverDone:
		if err != nil {
			log.Error("servidor SSH", "error", err)
		}
	case err = <-tunnelDone:
		if err != nil {
			log.Error("túnel SSH", "error", err)
		}
	case <-ctx.Done():
	}
	stop()
	log.Info("cerrado")
	if err != nil {
		os.Exit(1)
	}
}

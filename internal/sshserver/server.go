//go:build linux

package sshserver

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"github.com/creack/pty"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const maxConnections = 24

func Run(ctx context.Context, cfg config.Config, log *slog.Logger, ready chan<- error) error {
	signer, err := loadOrCreateHostKey(cfg.HostKey)
	if err != nil {
		if ready != nil {
			ready <- err
		}
		return fmt.Errorf("clave del servidor: %w", err)
	}
	keys, err := readAuthorizedKeys(cfg.AuthorizedKeys)
	if err != nil {
		if ready != nil {
			ready <- err
		}
		return fmt.Errorf("authorized_keys: %w", err)
	}
	serverConfig := &ssh.ServerConfig{MaxAuthTries: 4}
	if cfg.LoginPassword != "" {
		serverConfig.PasswordCallback = func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if meta.User() != cfg.LoginUser || subtle.ConstantTimeCompare(password, []byte(cfg.LoginPassword)) != 1 {
				return nil, errors.New("credenciales incorrectas")
			}
			return nil, nil
		}
	}
	if len(keys) > 0 {
		serverConfig.PublicKeyCallback = func(meta ssh.ConnMetadata, supplied ssh.PublicKey) (*ssh.Permissions, error) {
			if meta.User() != cfg.LoginUser {
				return nil, errors.New("usuario incorrecto")
			}
			for _, key := range keys {
				if bytes.Equal(supplied.Marshal(), key.Marshal()) {
					return nil, nil
				}
			}
			return nil, errors.New("clave no autorizada")
		}
	}
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", cfg.Listen)
	if ready != nil {
		ready <- err
	}
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Info("servidor SSH activo", "listen", cfg.Listen, "fingerprint", ssh.FingerprintSHA256(signer.PublicKey()), "usuario_ssh", cfg.LoginUser)
	go func() { <-ctx.Done(); _ = listener.Close() }()
	sem := make(chan struct{}, maxConnections)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(time.Second)
				continue
			}
			return err
		}
		select {
		case sem <- struct{}{}:
			go func() { defer func() { <-sem }(); serveConn(ctx, conn, serverConfig, log) }()
		default:
			_ = conn.Close()
		}
	}
}

func readAuthorizedKeys(path string) ([]ssh.PublicKey, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > 1024*1024 {
		return nil, errors.New("archivo de claves demasiado grande")
	}
	keys := make([]ssh.PublicKey, 0)
	for len(bytes.TrimSpace(data)) > 0 {
		var key ssh.PublicKey
		var comment string
		var options []string
		key, comment, options, data, err = ssh.ParseAuthorizedKey(data)
		_ = comment
		if err != nil {
			return nil, err
		}
		if len(options) != 0 {
			return nil, errors.New("opciones authorized_keys no compatibles; elimina opciones o usa claves sin restricciones")
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errors.New("archivo sin claves autorizadas")
	}
	return keys, nil
}

func serveConn(ctx context.Context, raw net.Conn, cfg *ssh.ServerConfig, log *slog.Logger) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(20 * time.Second))
	conn, chans, requests, err := ssh.NewServerConn(raw, cfg)
	if err != nil {
		log.Warn("conexión SSH rechazada", "remote", raw.RemoteAddr().String(), "error", err)
		return
	}
	_ = raw.SetDeadline(time.Time{})
	defer conn.Close()
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-sessionCtx.Done():
		}
	}()
	go func() { _ = conn.Wait(); cancel() }()
	go ssh.DiscardRequests(requests)
	var wg sync.WaitGroup
	for nc := range chans {
		if nc.ChannelType() != "session" {
			_ = nc.Reject(ssh.Prohibited, "solo sesiones SSH")
			continue
		}
		ch, reqs, err := nc.Accept()
		if err != nil {
			continue
		}
		wg.Add(1)
		go func() { defer wg.Done(); serveSession(sessionCtx, ch, reqs, log) }()
	}
	cancel()
	wg.Wait()
}

func serveSession(ctx context.Context, ch ssh.Channel, reqs <-chan *ssh.Request, log *slog.Logger) {
	defer ch.Close()
	type dims struct{ Cols, Rows, Width, Height uint32 }
	var term string = "xterm"
	var requestedPTY bool
	var size dims
	var master *os.File
	started := false
	for req := range reqs {
		ok := false
		switch req.Type {
		case "pty-req":
			if !started {
				var payload struct {
					Term                      string
					Cols, Rows, Width, Height uint32
					Modes                     string
				}
				if ssh.Unmarshal(req.Payload, &payload) == nil && payload.Cols > 0 && payload.Rows > 0 {
					requestedPTY = true
					size = dims{payload.Cols, payload.Rows, payload.Width, payload.Height}
					if len(payload.Term) > 0 && len(payload.Term) <= 64 && validTerm(payload.Term) {
						term = payload.Term
					}
					ok = true
				}
			}
		case "window-change":
			if requestedPTY && master != nil {
				var payload dims
				if ssh.Unmarshal(req.Payload, &payload) == nil {
					_ = pty.Setsize(master, &pty.Winsize{Cols: uint16(min(payload.Cols, 65535)), Rows: uint16(min(payload.Rows, 65535))})
				}
			}
			continue
		case "shell", "exec":
			if !started {
				command := ""
				if req.Type == "exec" {
					var payload struct{ Command string }
					if ssh.Unmarshal(req.Payload, &payload) != nil || len(payload.Command) > 65536 {
						break
					}
					command = payload.Command
				} else if len(req.Payload) != 0 {
					break
				}
				shell := "/bin/bash"
				if _, err := os.Stat(shell); err != nil {
					shell = "/bin/sh"
				}
				args := []string{"-l"}
				if req.Type == "exec" {
					args = []string{"-lc", command}
				}
				cmd := exec.CommandContext(ctx, shell, args...)
				cmd.Env = append(safeShellEnvironment(), "TERM="+term)
				if requestedPTY {
					var err error
					master, err = pty.StartWithSize(cmd, &pty.Winsize{
						Cols: uint16(min(size.Cols, 65535)), Rows: uint16(min(size.Rows, 65535)),
					})
					if err != nil {
						log.Error("no se pudo abrir PTY", "error", err)
						break
					}
					go runPTY(ctx, cmd, master, ch)
				} else {
					cmd.Stdin = ch
					cmd.Stdout = ch
					cmd.Stderr = ch.Stderr()
					if err := cmd.Start(); err != nil {
						log.Error("falló el shell", "error", err)
						break
					}
					go finishCommand(cmd, ch)
				}
				ok = true
				started = true
			}
		case "subsystem":
			if !started {
				var payload struct{ Name string }
				if ssh.Unmarshal(req.Payload, &payload) == nil && payload.Name == "sftp" {
					server, err := sftp.NewServer(ch)
					if err != nil {
						log.Error("sftp", "error", err)
						break
					}
					go func() {
						defer ch.Close()
						if err := server.Serve(); err != nil && err != io.EOF {
							log.Warn("sesión SFTP", "error", err)
						}
						_ = server.Close()
					}()
					ok = true
					started = true
				}
			}
		}
		if req.WantReply {
			_ = req.Reply(ok, nil)
		}
	}
}
func runPTY(ctx context.Context, cmd *exec.Cmd, master *os.File, ch ssh.Channel) {
	done := make(chan struct{})
	defer close(done)
	defer master.Close()
	go func() { _, _ = io.Copy(master, ch) }()
	go func() {
		select {
		case <-ctx.Done():
			_ = master.Close()
		case <-done:
		}
	}()
	_, _ = io.Copy(ch, master)
	_ = cmd.Wait()
	sendExit(ch, cmd)
	_ = ch.Close()
}
func safeShellEnvironment() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "SSH_") && !strings.HasPrefix(entry, "KAGGLE_") {
			env = append(env, entry)
		}
	}
	return env
}
func finishCommand(cmd *exec.Cmd, ch ssh.Channel) { _ = cmd.Wait(); sendExit(ch, cmd); _ = ch.Close() }
func sendExit(ch ssh.Channel, cmd *exec.Cmd) {
	status := uint32(255)
	if cmd.ProcessState != nil {
		if code := cmd.ProcessState.ExitCode(); code >= 0 {
			status = uint32(code)
		}
	}
	_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
}
func validTerm(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("-_.", r)) {
			return false
		}
	}
	return true
}

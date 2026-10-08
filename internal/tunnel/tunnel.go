package tunnel

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	delay := cfg.Retry
	if delay <= 0 {
		delay = 5 * time.Second
	}
	for ctx.Err() == nil {
		err := connect(ctx, cfg, log)
		if ctx.Err() != nil {
			return nil
		}
		log.Warn("túnel desconectado; reintentando", "error", err, "espera", delay)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
		if delay < 60*time.Second {
			delay = min(delay*2, 60*time.Second)
		}
	}
	return nil
}
func connect(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	hosts, err := hostCallback(cfg)
	if err != nil {
		return err
	}
	var auth []ssh.AuthMethod
	if cfg.VPSKey != "" {
		data, err := os.ReadFile(cfg.VPSKey)
		if err != nil {
			return fmt.Errorf("leer clave del VPS: %w", err)
		}
		key, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return fmt.Errorf("clave del VPS no válida: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(key))
	}
	if cfg.VPSPassword != "" {
		auth = append(auth, ssh.Password(cfg.VPSPassword))
		auth = append(auth, ssh.KeyboardInteractive(func(_ string, _ string, questions []string, echo []bool) ([]string, error) {
			if len(questions) != 1 || len(echo) != 1 || echo[0] {
				return nil, errors.New("el VPS requiere un desafío adicional no compatible")
			}
			return []string{cfg.VPSPassword}, nil
		}))
	}
	sshCfg := &ssh.ClientConfig{User: cfg.VPSUser, Auth: auth, HostKeyCallback: hosts, Timeout: 15 * time.Second}
	target := net.JoinHostPort(cfg.VPSHost, strconv.Itoa(cfg.VPSPort))
	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, target, sshCfg)
	if err != nil {
		return fmt.Errorf("autenticación del VPS: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = sshConn.Close()
		return err
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()
	remoteAddr := net.JoinHostPort(cfg.RemoteBind, strconv.Itoa(cfg.RemotePort))
	listener, err := client.Listen("tcp", remoteAddr)
	if err != nil {
		return fmt.Errorf("publicar %s en VPS: %w", remoteAddr, err)
	}
	defer listener.Close()
	log.Info("túnel activo", "vps", target, "remote", remoteAddr, "local", cfg.Listen)
	ended := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
			_ = listener.Close()
		case <-ended:
		}
	}()
	defer close(ended)
	// Keepalives detect broken network paths even when there is no user traffic.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ended:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					_ = client.Close()
					return
				}
			}
		}
	}()
	sem := make(chan struct{}, 32)
	for {
		in, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case sem <- struct{}{}:
			go func() {
				defer func() { <-sem }()
				bridge(ctx, in, cfg.Listen)
			}()
		default:
			_ = in.Close()
		}
	}
}
func bridge(ctx context.Context, in net.Conn, localAddr string) {
	defer in.Close()
	dialer := net.Dialer{Timeout: 5 * time.Second}
	out, err := dialer.DialContext(ctx, "tcp", localAddr)
	if err != nil {
		return
	}
	defer out.Close()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(out, in)
		if c, ok := out.(*net.TCPConn); ok {
			_ = c.CloseWrite()
		}
		close(done)
	}()
	_, _ = io.Copy(in, out)
	if c, ok := in.(interface{ CloseWrite() error }); ok {
		_ = c.CloseWrite()
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}
func hostCallback(cfg config.Config) (ssh.HostKeyCallback, error) {
	if cfg.VPSFingerprint != "" {
		return func(_ string, _ net.Addr, key ssh.PublicKey) error {
			actual := ssh.FingerprintSHA256(key)
			if subtle.ConstantTimeCompare([]byte(actual), []byte(cfg.VPSFingerprint)) != 1 {
				return fmt.Errorf("huella SSH del VPS no coincide (obtenida %s)", actual)
			}
			return nil
		}, nil
	}
	if cfg.VPSKnownHosts == "" {
		return nil, errors.New("no se configuró known_hosts")
	}
	callback, err := knownhosts.New(cfg.VPSKnownHosts)
	if err != nil {
		return nil, fmt.Errorf("known_hosts del VPS: %w", err)
	}
	return callback, nil
}

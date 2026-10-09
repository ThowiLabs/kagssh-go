package tunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"golang.org/x/crypto/ssh"
)

// Verifica el bind enviado por el cliente Go en el protocolo SSH, sin VPS real.
func TestConnect_SolicitaBindPublicoVPS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}

	sshServer := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) != "prueba" {
				return nil, fmt.Errorf("contraseña incorrecta")
			}
			return nil, nil
		},
	}
	sshServer.AddHostKey(signer)

	received := make(chan string, 1)
	serverErr := make(chan error, 1)
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer raw.Close()
		conn, chans, requests, err := ssh.NewServerConn(raw, sshServer)
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		go func() {
			for ch := range chans {
				_ = ch.Reject(ssh.Prohibited, "no channels")
			}
		}()
		for req := range requests {
			if req.Type != "tcpip-forward" {
				_ = req.Reply(false, nil)
				continue
			}
			var data struct {
				Host string
				Port uint32
			}
			if err := ssh.Unmarshal(req.Payload, &data); err != nil {
				_ = req.Reply(false, nil)
				serverErr <- err
				return
			}
			received <- data.Host
			_ = req.Reply(true, nil)
		}
		serverErr <- nil
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.Config{
		VPSHost: "127.0.0.1", VPSPort: listener.Addr().(*net.TCPAddr).Port,
		VPSUser: "test", VPSPassword: "prueba", VPSFingerprint: ssh.FingerprintSHA256(signer.PublicKey()),
		RemotePort: 2223, Listen: "127.0.0.1:2224",
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := make(chan error, 1)
	go func() { done <- connect(ctx, cfg, log) }()

	select {
	case host := <-received:
		if host != "0.0.0.0" {
			t.Errorf("se pidió %q, debe ser 0.0.0.0", host)
		}
	case err := <-serverErr:
		t.Fatalf("falló servidor SSH falso antes de pedir forward: %v", err)
	case <-time.After(8 * time.Second):
		t.Fatal("no se solicitó puerto inverso SSH")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("el cliente no se cerró tras cancelar contexto")
	}
}

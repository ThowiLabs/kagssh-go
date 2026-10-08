package tunnel

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"golang.org/x/crypto/ssh"
)

func TestHostCallback_Fingerprint(t *testing.T) {
	// A private key is not required to test pinning; synthetic wire-format keys suffice.
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPQHGftYoaKE6d1dxbGlaPiovbd/QBMFHdN5nGAcuCwE generated\n"))
	if err != nil {
		t.Fatal(err)
	}
	fp := ssh.FingerprintSHA256(pub)
	tests := []struct {
		name, pin string
		valid     bool
	}{
		{"huella correcta", fp, true},
		{"huella equivocada", "SHA256:incorrecta", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb, err := hostCallback(config.Config{VPSFingerprint: tt.pin})
			if err != nil {
				t.Fatal(err)
			}
			got := cb("test", nil, pub)
			if (got == nil) != tt.valid {
				t.Fatalf("valid=%v, error=%v", tt.valid, got)
			}
		})
	}
}
func TestBridge_TransfiereDatos(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("pong"))
	}()
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go bridge(ctx, a, listener.Addr().String())
	buf := make([]byte, 4)
	if _, err := b.Read(buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "pong" {
		t.Fatalf("datos incorrectos: %q", buf)
	}
}

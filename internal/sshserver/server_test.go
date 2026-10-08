//go:build linux

package sshserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestHostKey_CreaYReutiliza(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host_ed25519")
	first, err := loadOrCreateHostKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreateHostKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if ssh.FingerprintSHA256(first.PublicKey()) != ssh.FingerprintSHA256(second.PublicKey()) {
		t.Fatal("huella de host cambió")
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0600 {
		t.Fatalf("permisos inseguros: %v", stat.Mode())
	}
}

func TestReadAuthorizedKeys_RechazaOpciones(t *testing.T) {
	_, key, err := loadPublicTestKey(t)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(path, []byte("command=\"id\" "+strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAuthorizedKeys(path); err == nil {
		t.Fatal("se aceptaron restricciones no implementadas")
	}
}
func loadPublicTestKey(t *testing.T) (ssh.Signer, ssh.PublicKey, error) {
	t.Helper()
	signer, err := loadOrCreateHostKey(filepath.Join(t.TempDir(), "host"))
	if err != nil {
		return nil, nil, err
	}
	return signer, signer.PublicKey(), nil
}

func TestSafeShellEnvironment_OcultaCredenciales(t *testing.T) {
	t.Setenv("SSH_PASSWORD", "no-revelar")
	t.Setenv("SSH_LOGIN_PASSWORD", "no-revelar")
	t.Setenv("KAGGLE_USER_SECRETS_TOKEN", "token-no-revelar")
	for _, entry := range safeShellEnvironment() {
		if strings.HasPrefix(entry, "SSH_") || strings.HasPrefix(entry, "KAGGLE_") {
			t.Fatalf("credencial expuesta: %s", strings.SplitN(entry, "=", 2)[0])
		}
	}
}

func TestSSH_ExecYSFTP(t *testing.T) {
	addrListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := addrListener.Addr().String()
	_ = addrListener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.Config{
		Listen: addr, HostKey: filepath.Join(t.TempDir(), "host"),
		LoginUser: "tester", LoginPassword: "test-password",
	}
	ready := make(chan error, 1)
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), ready) }()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("servidor no inició")
	}
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: "tester", Auth: []ssh.AuthMethod{ssh.Password("test-password")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	out, err := session.Output("printf 'hola'")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hola" {
		t.Fatalf("respuesta inesperada: %q", out)
	}
	ptySession, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := ptySession.RequestPty("xterm", 25, 80, ssh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	ptyOut, err := ptySession.Output("printf 'pty-ok'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ptyOut), "pty-ok") {
		t.Fatalf("PTY no entregó salida: %q", ptyOut)
	}
	path := filepath.Join(t.TempDir(), "archivo-sftp.txt")
	if err := os.WriteFile(path, []byte("sftp-ok"), 0600); err != nil {
		t.Fatal(err)
	}
	remote, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	file, err := remote.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sftpOut, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(sftpOut) != "sftp-ok" {
		t.Fatalf("SFTP no devolvió datos esperados: %q", sftpOut)
	}
	cancel()
	_ = client.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el servidor no se detuvo")
	}
}

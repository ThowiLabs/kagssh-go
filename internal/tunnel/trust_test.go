package tunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ThowiLabs/kagssh-go/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestHostCallback_TOFUGuardaYVerifica(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ssh", "known_hosts")
	cfg := config.Config{
		VPSHost: "vps.example", VPSPort: 22,
		VPSKnownHosts: path,
	}
	callback, err := hostCallback(cfg)
	if err != nil {
		t.Fatal(err)
	}
	key, otherKey := testPublicKey(t), testPublicKey(t)
	addr := "vps.example:22"
	remote := &net.TCPAddr{IP: net.ParseIP("192.0.2.3"), Port: 22}

	if err := callback("otro.example:22", remote, key); err == nil {
		t.Fatal("aceptó un host inesperado")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("guardó una clave de un host no autorizado")
	}
	if err := callback(addr, remote, key); err != nil {
		t.Fatalf("alta automática falló: %v", err)
	}
	initial, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(initial), "vps.example") || !strings.Contains(string(initial), "ssh-ed25519") {
		t.Fatal("no registró la clave del VPS en known_hosts")
	}
	if err := callback(addr, remote, key); err != nil {
		t.Fatalf("rechazó la misma clave: %v", err)
	}
	if err := callback(addr, remote, otherKey); err == nil {
		t.Fatal("aceptó un cambio de clave del VPS")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(initial) {
		t.Fatal("modificó known_hosts cuando la clave cambió")
	}

}

func TestHostCallback_TOFUAgregaHostSinCambiarOtro(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	old := testPublicKey(t)
	line := knownhosts.Line([]string{"otro.example"}, old) + "\n"
	if err := os.WriteFile(path, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{VPSHost: "nuevo.example", VPSPort: 2222, VPSKnownHosts: path}
	cb, err := hostCallback(cfg)
	if err != nil {
		t.Fatal(err)
	}
	key := testPublicKey(t)
	if err := cb("nuevo.example:2222", &net.TCPAddr{IP: net.ParseIP("192.0.2.4"), Port: 2222}, key); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(contents), line) {
		t.Fatal("se sobrescribió la clave previa")
	}
}

func TestHostCallback_SinRutaNiHuella(t *testing.T) {
	_, err := hostCallback(config.Config{})
	if err == nil || !strings.Contains(err.Error(), "SSH_FINGERPRINT") {
		t.Fatalf("se esperaba indicación para ruta vacía: %v", err)
	}
}

func TestHostCallback_ArchivoKnownHostsImposible(t *testing.T) {
	cfg := config.Config{VPSHost: "vps.example", VPSPort: 22,
		VPSKnownHosts: filepath.Join(t.TempDir(), "carpeta"),
	}
	if err := os.Mkdir(cfg.VPSKnownHosts, 0700); err != nil {
		t.Fatal(err)
	}
	callback, err := hostCallback(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := callback("vps.example:22", &net.TCPAddr{Port: 22}, testPublicKey(t)); err == nil {
		t.Fatal("no rechazó una ruta que es directorio")
	}
}

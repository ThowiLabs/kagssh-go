package gradiolive

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestOfficialPinnedFRPChecksums(t *testing.T) {
	for _, p := range []string{"linux_amd64", "linux_arm64", "windows_amd64", "darwin_amd64", "darwin_arm64"} {
		var parts = strings.SplitN(p, "_", 2)
		name, hash, err := binaryName(parts[0], parts[1])
		if err != nil {
			t.Fatal(err)
		}
		raw, err := hex.DecodeString(hash)
		if err != nil || len(raw) != sha256.Size {
			t.Errorf("hash FRP inválido %s", p)
		}
		if !strings.HasPrefix(name, "frpc_") {
			t.Fatalf("nombre binario inválido: %s", name)
		}
	}
	if _, _, err := binaryName("linux", "mips"); err == nil {
		t.Fatal("aceptó plataforma sin checksum")
	}
}
func TestOnlyAcceptCanonicalGradioURL(t *testing.T) {
	good := []string{"start proxy success: https://abc123.gradio.live", "2026 INFO start proxy success: https://random-test.gradio.live/"}
	for _, line := range good {
		if extractURL(line) == "" {
			t.Fatalf("rechazada URL legítima: %q", line)
		}
	}
	bad := []string{
		"login to server failed", "start proxy success: http://abc.gradio.live",
		"start proxy success: https://abc.gradio.live.evil.test",
		"start proxy success: https://abc.gradio.live:4443",
		"start proxy success: https://abc.gradio.live/mcp",
		"start proxy success: https://abc.gradio.live?token=1",
		"start proxy success: https://user@abc.gradio.live",
	}
	for _, line := range bad {
		if x := extractURL(line); x != "" {
			t.Fatalf("aceptó URL no válida: %q", x)
		}
	}
}
func TestServerHostSecurity(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "a..b", "gradio.live:999", "", "-evil.localhost"} {
		if validHost(host) {
			t.Errorf("host no permitido %q", host)
		}
	}
	for _, host := range []string{"frp.gradio.live", "tunnel.gradio.live"} {
		if !validHost(host) {
			t.Errorf("host oficial razonable inválido: %q", host)
		}
	}
}

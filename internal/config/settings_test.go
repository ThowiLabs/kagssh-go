package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsPersistOnlyNonSecretValues(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".kagmcp")
	cfg := Config{DataDir: dir, MCPEnabled: true, MCPTunnel: "cloudflare", MCPPort: 8181,
		MCPAccessPIN: "test-pin-do-not-save", GitHubToken: "test-token-do-not-save"}
	if err := SaveSettings(cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "test-pin") || strings.Contains(string(raw), "test-token") ||
		strings.Contains(string(raw), "MCP_ACCESS_PIN") || strings.Contains(string(raw), "GITHUB_TOKEN") {
		t.Fatal("config.json filtró credenciales")
	}
	parsed, err := readSettings(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed["MCP_ENABLED"] != "true" || parsed["MCP_TUNNEL"] != "cloudflare" {
		t.Fatal("config.json no preservó configuración")
	}
}
func TestSettingsCannotInjectCredentialsAndPriority(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(file, []byte(`{"MCP_ENABLED":"true","SSH_ENABLED":"false","GITHUB_TOKEN":"no-leer","MCP_ACCESS_PIN":"no-leer","MCP_TUNNEL":"none"}`), 0600); err != nil {
		t.Fatal(err)
	}
	settings, err := readSettings(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := settings["GITHUB_TOKEN"]; ok {
		t.Fatal("config acepta GITHUB_TOKEN")
	}
	if _, ok := settings["MCP_ACCESS_PIN"]; ok {
		t.Fatal("config acepta MCP_ACCESS_PIN")
	}
	fake := &fakeSecrets{values: map[string]string{"MCP_TUNNEL": "cloudflare", "MCP_ACCESS_PIN": "seguridad-pin-12345"}}
	cfg, err := load(context.Background(), func(k string) (string, bool) {
		if k == "MCP_ENABLED" {
			return "true", true
		}
		return "", false
	}, fake, settings)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MCPEnabled || cfg.MCPTunnel != "cloudflare" || cfg.GitHubToken != "" || cfg.MCPAccessPIN != "seguridad-pin-12345" {
		t.Fatal("precedencia incorrecta")
	}
}

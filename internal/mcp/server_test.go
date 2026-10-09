package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ThowiLabs/kagssh-go/internal/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Config{MCPEnabled: true, MCPPort: 8181, MCPTunnel: "none",
		MCPPublicURL: "https://sample.trycloudflare.com", MCPAccessPIN: "LONG-TEST-PIN-ABC",
		DataDir: filepath.Join(t.TempDir(), ".kagmcp")}
	s, err := New(cfg, cfg.MCPPublicURL)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestOAuthRequiredForToolAccess(t *testing.T) {
	s := newTestServer(t)
	for _, tc := range []struct {
		method, path, origin string
		status               int
	}{
		{"POST", "/mcp", "", http.StatusUnauthorized},
		{"POST", "/mcp", "https://attacker.example", http.StatusForbidden},
		{"GET", "/mcp", "", http.StatusMethodNotAllowed},
		{"GET", "/.well-known/oauth-authorization-server", "", http.StatusOK},
		{"GET", "/.well-known/oauth-protected-resource/mcp", "", http.StatusOK},
		{"GET", "/health", "", http.StatusOK},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		if tc.method == "POST" {
			req.Header.Set("Content-Type", "application/json")
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Errorf("%s %s status=%d want=%d response=%s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}
		if tc.path == "/mcp" && tc.status == http.StatusUnauthorized &&
			!strings.Contains(rec.Header().Get("WWW-Authenticate"), "resource_metadata") {
			t.Fatal("OAuth discovery missing in challenge")
		}
	}
}
func TestDefinitionsAndNoSecrets(t *testing.T) {
	s := newTestServer(t)
	s.cfg.GitHubToken = "TEST_SECRET_TOKEN"
	raw, err := json.Marshal(s.definitions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), s.cfg.GitHubToken) {
		t.Fatal("GitHub token leaked into MCP tool schemas")
	}
	var defs []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &defs); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, d := range defs {
		found[d.Name] = true
	}
	for _, name := range []string{"exec", "read_file", "write_file", "list_dir", "github_status", "github_repositories", "github_branches", "github_file_read", "github_file_write"} {
		if !found[name] {
			t.Errorf("missing tool %s", name)
		}
	}
}
func TestStateInPrivateDir(t *testing.T) {
	s := newTestServer(t)
	if s.auth == nil {
		t.Fatal("OAuth no inicializado")
	}
	path := filepath.Join(s.cfg.DataDir, "state", "oauth.json")
	req := httptest.NewRequest("GET", "/health", nil)
	if _, err := s.readFile(path); err == nil {
		t.Fatal("el servidor debe impedir leer sus propias credenciales")
	}
	_ = req
}

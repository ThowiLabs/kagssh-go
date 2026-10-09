package dashboard

import (
	"github.com/ThowiLabs/kagssh-go/internal/projectstate"
	"github.com/ThowiLabs/kagssh-go/internal/skills"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

const baseURL = "https://sample.trycloudflare.com"

func testHandler(t *testing.T) *Handler {
	store, err := projectstate.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return New(baseURL, "TEST_LONG_ACCESS_PIN", store, skills.Registry{Dir: t.TempDir()}, nil, func() bool { return false }, nil, nil)
}
func call(h http.Handler, method, path string, values url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	body := ""
	if values != nil {
		body = values.Encode()
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == "POST" {
		req.Header.Set("Origin", baseURL)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
func TestLoginCookieCSRFAndProjects(t *testing.T) {
	h := testHandler(t)
	page := call(h, "GET", "/login", nil)
	if page.Code != 200 {
		t.Fatalf("login GET=%d", page.Code)
	}
	first := page.Result().Cookies()[0]
	if !first.Secure || !first.HttpOnly || first.SameSite != http.SameSiteStrictMode {
		t.Fatal("cookie insegura")
	}
	ok := call(h, "POST", "/login", url.Values{"pin": {"TEST_LONG_ACCESS_PIN"}, "csrf": {first.Value}}, first)
	if ok.Code != http.StatusSeeOther {
		t.Fatalf("login POST=%d: %s", ok.Code, ok.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, c := range ok.Result().Cookies() {
		if c.Name == "__Host-kagmcp-session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || !sessionCookie.Secure || !sessionCookie.HttpOnly {
		t.Fatal("sesión insegura")
	}
	csrf := h.sessions[key(sessionCookie.Value)].CSRF
	invalid := call(h, "POST", "/project", url.Values{"csrf": {"falso"}, "id": {"alpha"}, "name": {"nombre"}}, sessionCookie)
	if invalid.Code != 403 {
		t.Fatalf("CSRF no rechazado=%d", invalid.Code)
	}
	valid := call(h, "POST", "/project", url.Values{"csrf": {csrf}, "id": {"alpha"}, "name": {"Proyecto <script>alert(1)</script>"}}, sessionCookie)
	if valid.Code != 303 {
		t.Fatalf("crear proyecto: %d, %s", valid.Code, valid.Body.String())
	}
	home := call(h, "GET", "/", nil, sessionCookie)
	if home.Code != 200 || !strings.Contains(home.Body.String(), "Ponytail") || strings.Contains(home.Body.String(), "<script>alert(1)</script>") {
		t.Fatalf("panel inválido: %s", home.Body.String()[:min(home.Body.Len(), 400)])
	}
	logout := call(h, "POST", "/logout", url.Values{"csrf": {csrf}}, sessionCookie)
	if logout.Code != 303 {
		t.Fatalf("logout %d", logout.Code)
	}
	denied := call(h, "GET", "/", nil, sessionCookie)
	if denied.Code != 303 {
		t.Fatal("sesión no revocada")
	}
}
func TestLoginRateLimits(t *testing.T) {
	h := testHandler(t)
	page := call(h, "GET", "/login", nil)
	c := page.Result().Cookies()[0]
	for i := 0; i < 5; i++ {
		wrong := call(h, "POST", "/login", url.Values{"pin": {"INVALID"}, "csrf": {c.Value}}, c)
		if wrong.Code != 401 {
			t.Fatalf("intento %d: %d", i, wrong.Code)
		}
	}
	blocked := call(h, "POST", "/login", url.Values{"pin": {"TEST_LONG_ACCESS_PIN"}, "csrf": {c.Value}}, c)
	if blocked.Code != 429 {
		t.Fatalf("rate limit inefectivo: %d", blocked.Code)
	}
}

func TestImportRequiresSessionAndCSRF(t *testing.T) {
	h := testHandler(t)
	called := false
	h.Import = func(id string) (any, error) { called = id == "alpha"; return map[string]bool{"imported": true}, nil }
	anonymous := call(h, "POST", "/import", url.Values{"project": {"alpha"}, "csrf": {"x"}})
	if anonymous.Code != 401 || called {
		t.Fatalf("acceso no autorizado: %d", anonymous.Code)
	}
	page := call(h, "GET", "/login", nil)
	c := page.Result().Cookies()[0]
	authenticated := call(h, "POST", "/login", url.Values{"csrf": {c.Value}, "pin": {"TEST_LONG_ACCESS_PIN"}}, c)
	var session *http.Cookie
	for _, item := range authenticated.Result().Cookies() {
		if item.Name == "__Host-kagmcp-session" {
			session = item
		}
	}
	if session == nil {
		t.Fatal("faltó sesión")
	}
	invalid := call(h, "POST", "/import", url.Values{"project": {"alpha"}, "csrf": {"incorrecto"}}, session)
	if invalid.Code != 403 || called {
		t.Fatal("importación sin CSRF permitida")
	}
	csrf := h.sessions[key(session.Value)].CSRF
	valid := call(h, "POST", "/import", url.Values{"project": {"alpha"}, "csrf": {csrf}}, session)
	if valid.Code != 303 || !called {
		t.Fatalf("importación autenticada: %d, called=%v", valid.Code, called)
	}
}

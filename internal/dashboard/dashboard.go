package dashboard

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/ThowiLabs/kagssh-go/internal/pinauth"
	"github.com/ThowiLabs/kagssh-go/internal/projectstate"
	"github.com/ThowiLabs/kagssh-go/internal/skills"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type session struct {
	CSRF    string
	Expires time.Time
}
type attempts struct {
	Count int
	Until time.Time
}
type Handler struct {
	Public           string
	pin              *pinauth.State
	revokePIN        func() error
	Projects         *projectstate.Store
	Skills           skills.Registry
	GitHub           func(context.Context, string) (string, error)
	GitHubConfigured func() bool
	Export           func(string) (any, error)
	Import           func(string) (any, error)
	mu               sync.Mutex
	sessions         map[string]session
	failures         map[string]attempts
	global           attempts
}

func New(public string, pin *pinauth.State, revokePIN func() error, store *projectstate.Store, reg skills.Registry, github func(context.Context, string) (string, error), configured func() bool, export func(string) (any, error), importProject func(string) (any, error)) *Handler {
	return &Handler{Public: public, pin: pin, revokePIN: revokePIN, Projects: store, Skills: reg, GitHub: github, GitHubConfigured: configured, Export: export, Import: importProject,
		sessions: map[string]session{}, failures: map[string]attempts{}}
}
func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("random unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func key(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: age, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}
func (h *Handler) security(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	w.Header().Set("Cache-Control", "no-store")
}

// La cabecera Origin puede ser modificada u omitida por los túneles
// Gradio FRP/Cloudflare. La autenticación del navegador descansa en una
// cookie __Host privada SameSite=Strict y un token CSRF aleatorio que debe
// coincidir con el formulario; nunca en Origin/Host reescritos por el proxy.
func ip(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func (h *Handler) allowed(r *http.Request) bool {
	now := time.Now()
	k := ip(r)
	h.mu.Lock()
	defer h.mu.Unlock()
	for addr, v := range h.failures {
		if now.After(v.Until) {
			delete(h.failures, addr)
		}
	}
	if len(h.failures) > 1000 {
		return false
	}
	if v, ok := h.failures[k]; ok && v.Count >= 5 && now.Before(v.Until) {
		return false
	}
	if h.global.Count >= 50 && now.Before(h.global.Until) {
		return false
	}
	if now.After(h.global.Until) {
		h.global = attempts{Until: now.Add(15 * time.Minute)}
	}
	return true
}
func (h *Handler) fail(r *http.Request) {
	now := time.Now()
	k := ip(r)
	h.mu.Lock()
	defer h.mu.Unlock()
	v := h.failures[k]
	if now.After(v.Until) {
		v = attempts{Until: now.Add(15 * time.Minute)}
	}
	v.Count++
	h.failures[k] = v
	h.global.Count++
}
func (h *Handler) loginPage(w http.ResponseWriter, status int, message string) {
	csrf := random()
	cookie(w, "__Host-kagmcp-login", csrf, 600)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTemplate.Execute(w, view{Login: true, CSRF: csrf, Message: message})
}
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.loginPage(w, http.StatusOK, "")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido; abre /login", http.StatusMethodNotAllowed)
		return
	}
	if !h.allowed(r) {
		w.Header().Set("Retry-After", "900")
		h.loginPage(w, http.StatusTooManyRequests, "Demasiados intentos. Vuelve a probar después de 15 minutos.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		slog.Warn("formulario de login inválido", "reason", "parse_or_size", "origin_present", r.Header.Get("Origin") != "")
		h.loginPage(w, http.StatusBadRequest, "No se pudo leer el formulario. Abre de nuevo la URL pública de KagMCP y prueba otra vez.")
		return
	}
	c, err := r.Cookie("__Host-kagmcp-login")
	if err != nil || c.Value == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(r.FormValue("csrf"))) != 1 {
		slog.Warn("formulario de login rechazado", "reason", "csrf_cookie_or_token", "cookie_present", err == nil, "origin_present", r.Header.Get("Origin") != "")
		h.loginPage(w, http.StatusForbidden, "No se pudo comprobar la cookie de seguridad. Abre la URL pública directamente en una pestaña nueva y permite sus cookies; vuelve a introducir el PIN.")
		return
	}
	// Un proxy HTTP legítimo puede cambiar Origin o suprimirlo; el CSRF
	// secreto verificado arriba impide que otra web envíe el formulario.
	if r.Header.Get("Origin") != h.Public {
		slog.Info("login recibido a través de proxy con Origin diferente", "origin_present", r.Header.Get("Origin") != "")
	}
	if !h.pin.Verify(r.FormValue("pin")) {
		h.fail(r)
		h.loginPage(w, http.StatusUnauthorized, "PIN incorrecto. Usa el PIN temporal de los logs de Kaggle o el que configuraste desde el panel.")
		return
	}
	token := random()
	csrf := random()
	h.mu.Lock()
	for id, current := range h.sessions {
		if time.Now().After(current.Expires) {
			delete(h.sessions, id)
		}
	}
	if len(h.sessions) >= 100 {
		h.mu.Unlock()
		http.Error(w, "demasiadas sesiones", 429)
		return
	}
	h.sessions[key(token)] = session{CSRF: csrf, Expires: time.Now().Add(8 * time.Hour)}
	delete(h.failures, ip(r))
	h.mu.Unlock()
	cookie(w, "__Host-kagmcp-session", token, 8*3600)
	cookie(w, "__Host-kagmcp-login", "", -1)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (h *Handler) authorize(r *http.Request) (session, bool) {
	c, err := r.Cookie("__Host-kagmcp-session")
	if err != nil {
		return session{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[key(c.Value)]
	if !ok || time.Now().After(s.Expires) {
		delete(h.sessions, key(c.Value))
		return session{}, false
	}
	return s, true
}
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) (session, bool) {
	s, ok := h.authorize(r)
	if !ok {
		if r.Method == http.MethodGet {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		} else {
			http.Error(w, "sesión requerida", 401)
		}
		return session{}, false
	}
	if r.Method == http.MethodPost {
		// Validación mediante cookie de sesión privada + CSRF ligado a ella;
		// no asumir Origin intacto a través de Gradio/Cloudflare.
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if err := r.ParseForm(); err != nil || subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(s.CSRF)) != 1 {
			http.Error(w, "Formulario rechazado: token de seguridad inválido o expirado. Actualiza el panel y vuelve a intentarlo.", 403)
			return session{}, false
		}
	}
	return s, true
}

type view struct {
	Login        bool
	CSRF         string
	Message      string
	Projects     []projectstate.Project
	History      []projectstate.Event
	SkillList    []skills.Info
	SkillContent string
	SkillName    string
	NextOffset   int
	GitHubOn     bool
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.security(w)
	if r.URL.Path == "/login" {
		h.login(w, r)
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/skills" && r.URL.Path != "/project" && r.URL.Path != "/memory" && r.URL.Path != "/task" && r.URL.Path != "/task-status" && r.URL.Path != "/github" && r.URL.Path != "/export" && r.URL.Path != "/import" && r.URL.Path != "/change-pin" && r.URL.Path != "/logout" {
		http.NotFound(w, r)
		return
	}
	sess, ok := h.Verify(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		if r.URL.Path != "/" && r.URL.Path != "/skills" {
			http.NotFound(w, r)
			return
		}
		state := h.Projects.Snapshot()
		v := view{CSRF: sess.CSRF, Projects: state.Projects, History: state.History, GitHubOn: h.GitHubConfigured()}
		v.SkillList, _ = h.Skills.List()
		if r.URL.Path == "/skills" {
			name := r.URL.Query().Get("name")
			if name == "" {
				name = skills.Active
			}
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			body, err := h.Skills.Read(name, offset, 7000)
			if err != nil {
				http.Error(w, "skill no encontrada", 404)
				return
			}
			v.SkillName = name
			v.SkillContent = body["content"].(string)
			v.NextOffset = body["next_offset"].(int)
		}
		h.page(w, v)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "método inválido", 405)
		return
	}
	if r.URL.Path == "/change-pin" {
		if !h.allowed(r) {
			w.Header().Set("Retry-After", "900")
			http.Error(w, "intentos limitados", 429)
			return
		}
		err := h.pin.Change(r.FormValue("current_pin"), r.FormValue("new_pin"), h.revokePIN)
		if err != nil {
			h.fail(r)
			http.Error(w, err.Error(), 400)
			return
		}
		h.mu.Lock()
		h.sessions = make(map[string]session)
		h.failures = make(map[string]attempts)
		h.global = attempts{}
		h.mu.Unlock()
		cookie(w, "__Host-kagmcp-session", "", -1)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.URL.Path == "/logout" {
		c, _ := r.Cookie("__Host-kagmcp-session")
		if c != nil {
			h.mu.Lock()
			delete(h.sessions, key(c.Value))
			h.mu.Unlock()
		}
		cookie(w, "__Host-kagmcp-session", "", -1)
		http.Redirect(w, r, "/login", 303)
		return
	}
	var err error
	switch r.URL.Path {
	case "/project":
		err = h.Projects.Create(r.FormValue("id"), r.FormValue("name"), r.FormValue("repository"), r.FormValue("notebook"))
	case "/memory":
		err = h.Projects.Remember(r.FormValue("project"), r.FormValue("text"))
	case "/task":
		_, err = h.Projects.TaskAdd(r.FormValue("project"), r.FormValue("title"))
	case "/task-status":
		err = h.Projects.TaskSet(r.FormValue("project"), r.FormValue("task"), r.FormValue("status"))
	case "/export":
		if h.Export == nil {
			err = errors.New("exportación no disponible")
		} else {
			_, err = h.Export(r.FormValue("project"))
		}
	case "/import":
		if h.Import == nil {
			err = errors.New("restauración no disponible")
		} else {
			_, err = h.Import(r.FormValue("project"))
		}
	case "/github":
		if h.GitHub == nil {
			err = errors.New("GitHub no disponible")
		} else {
			_, err = h.GitHub(r.Context(), r.FormValue("token"))
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (h *Handler) page(w http.ResponseWriter, v view) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pageTemplate.Execute(w, v)
}

var pageTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>KagMCP · Control</title><style>
:root{color-scheme:dark;font-family:system-ui,-apple-system,sans-serif;background:#0a101b;color:#eaf1ff}body{margin:0;padding:25px;max-width:1150px;margin:auto}h1{font-size:2.1rem;margin:0 0 8px}h2{font-size:1.1rem}p,small{color:#a9b7cd}a{color:#9dc5ff;text-decoration:none}.muted{font-size:.85rem;color:#91a3be}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(310px,1fr));gap:16px;margin-top:20px}.card{background:#111b2c;border:1px solid #2c3b52;border-radius:15px;padding:20px;overflow-wrap:anywhere}.panel{margin-top:20px;background:#101a2b;border:1px solid #293852;border-radius:15px;padding:18px}input,textarea,select,button{box-sizing:border-box;font:inherit;color:#eaf1ff}input,textarea,select{background:#081223;border:1px solid #42536c;border-radius:9px;padding:10px;width:100%;margin:6px 0 12px}textarea{min-height:83px}button{background:#447be8;border:0;border-radius:9px;cursor:pointer;padding:10px 15px}button:hover{background:#6799fb}form{margin:0}header{display:flex;justify-content:space-between;align-items:center;gap:20px}.badge{font-size:.8rem;background:#21374d;border-radius:30px;padding:6px 10px}.entry{border-top:1px solid #26364c;padding:10px 0}.entry:first-of-type{border-top:0}pre{white-space:pre-wrap;word-break:break-word;background:#081223;border-radius:10px;padding:14px;font-size:.78rem;max-height:520px;overflow:auto}.login{max-width:440px;margin:12vh auto}.flex{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.flex form{display:inline-block}.light{border:1px solid #40516d;background:none}code{font-size:.85rem;color:#bcd4ff}hr{border:0;border-top:1px solid #2c3b52}label{font-size:.85rem;color:#b8c8e2}
</style></head><body>{{if .Login}}<main class="card login"><h1>🔐 KagMCP</h1><p>Acceso administrativo al runtime autorizado</p>{{if .Message}}<p role="alert" style="color:#ffbbb5">{{.Message}}</p>{{end}}<form method="post" action="/login"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="pin">PIN de administración</label><input id="pin" name="pin" type="password" autocomplete="current-password" maxlength="128" required><button type="submit">Iniciar sesión</button></form><p class="muted">Sesiones privadas, caducidad de 8 horas y protección contra intentos repetidos.</p></main>{{else}}<header><div><h1>KagMCP <span class="badge">Ponytail v2 activa</span></h1><p>Proyectos, tareas, memoria e historial compartido</p></div><form method="post" action="/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button class="light">Cerrar sesión</button></form></header>
<section class="cards"><article class="card"><h2>Nuevo proyecto</h2><form action="/project" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label>ID (a-z, 0-9, - y _)</label><input name="id" placeholder="proyecto-demo" required><label>Nombre</label><input name="name" required><label>Repositorio Git</label><input name="repository" placeholder="owner/repo"><label>Cuaderno</label><input name="notebook" placeholder="notebooks/ejemplo.ipynb"><button>Crear proyecto</button></form></article>
<article class="card"><h2>Cambiar PIN</h2><p>PIN actual obligatorio. Nuevo PIN de 6 a 128 caracteres. Al cambiarlo se revocan las conexiones OAuth y las sesiones web; deberás volver a autenticar los agentes.</p><form action="/change-pin" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label>PIN actual</label><input type="password" name="current_pin" autocomplete="current-password" required maxlength="128"><label>Nuevo PIN</label><input type="password" name="new_pin" autocomplete="new-password" minlength="6" maxlength="128" required><button>Cambiar PIN y cerrar sesiones</button></form></article><article class="card"><h2>GitHub</h2><p>{{if .GitHubOn}}Token activo solo en memoria{{else}}Token no configurado{{end}}. Se comprueba antes de guardarlo; jamás se escribe en disco.</p><form action="/github" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label>Personal Access Token</label><input type="password" name="token" autocomplete="off" placeholder="ghp_… / github_pat_…" maxlength="4096"><button>Configurar PAT</button></form><p class="muted">Para desactivar, envía el campo vacío. El token se perderá al reiniciar el proceso.</p></article>
<article class="card"><h2>Skills</h2><p><strong>Ponytail v2 siempre activa</strong></p>{{range .SkillList}}<div class="entry"><a href="/skills?name={{.Name}}">{{.Name}}</a>{{if .AlwaysActive}} <span class="badge">Siempre activa</span>{{end}}<div class="muted">{{.Description}}</div></div>{{end}}</article></section>
{{range .Projects}}{{$project := .ID}}<section class="panel"><h2>📁 {{.Name}} <code>{{.ID}}</code></h2><p>Repositorio: {{.Repository}} · Cuaderno: {{.Notebook}}</p>{{if .Verification}}<p class="muted">Pruebas registradas: {{.Verification.TestedAt.Format "2006-01-02 15:04 UTC"}} · commit {{.Verification.GitHead}}</p>{{else}}<p class="muted">Pendiente: ejecutar project_verify antes de generar Gradio.</p>{{end}}<form method="post" action="/export"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="project" value="{{.ID}}"><button class="light">Exportar memoria y tareas al repositorio</button></form><form method="post" action="/import"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="project" value="{{.ID}}"><button class="light">Restaurar memoria desde el repositorio</button></form><div class="cards"><article><h2>Memoria compartida</h2>{{range .Memory}}<p class="entry">{{.}}</p>{{else}}<p>Sin notas todavía.</p>{{end}}<form method="post" action="/memory"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="project" value="{{.ID}}"><textarea name="text" placeholder="Decisión técnica o contexto persistente" maxlength="6000" required></textarea><button>Guardar memoria</button></form></article><article><h2>Lista de tareas</h2>{{range .Tasks}}<div class="entry"><strong>{{.Title}}</strong> <span class="muted">{{.Status}}</span><form method="post" action="/task-status" class="flex"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="project" value="{{$project}}"><input type="hidden" name="task" value="{{.ID}}"><select name="status"><option value="pending">Pendiente</option><option value="in_progress">En progreso</option><option value="done">Completa</option></select><button>Actualizar</button></form></div>{{else}}<p>Sin tareas.</p>{{end}}<form method="post" action="/task"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="project" value="{{.ID}}"><input name="title" maxlength="300" placeholder="Nueva tarea" required><button>Añadir tarea</button></form></article></div></section>{{end}}
<section class="panel"><h2>Historial de comandos</h2><p>Registro compartido y acotado. Evita introducir secretos en comandos.</p>{{range .History}}<div class="entry"><strong>{{.Description}}</strong> <span class="badge">{{.Status}}</span> <span class="muted">{{.When.Format "2006-01-02 15:04:05"}} · {{.Project}}</span><pre>{{.Command}}</pre></div>{{else}}<p>Aún no hay comandos registrados.</p>{{end}}</section>
{{if .SkillName}}<section class="panel"><h2>Skill · {{.SkillName}}</h2><pre>{{.SkillContent}}</pre><a href="/skills?name={{.SkillName}}&amp;offset={{.NextOffset}}">Siguiente fragmento →</a></section>{{end}}
<footer><p>Los datos se almacenan en /kaggle/working/.kagmcp. Versiona el contexto relevante antes de destruir el runtime.</p><p><a href="/">Volver al inicio</a></p></footer>{{end}}</body></html>`))

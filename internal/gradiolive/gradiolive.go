package gradiolive

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	apiURL         = "https://api.gradio.app/v3/tunnel-request"
	cdnURL         = "https://cdn-media.huggingface.co/frpc-gradio-0.3/"
	maxFRPBytes    = int64(35 << 20)
	startupTimeout = 45 * time.Second
)

// Huellas v0.3 verificadas contra gradio/gradio/tunneling.py, no son de terceros.
var checksums = map[string]string{
	"windows_amd64": "14bc0ea470be5d67d79a07412bd21de8a0a179c6ac1116d7764f68e942dc9ceb",
	"linux_amd64":   "c791d1f047b41ff5885772fc4bf20b797c6059bbd82abb9e31de15e55d6a57c4",
	"linux_arm64":   "823ced25104de6dc3c9f4798dbb43f20e681207279e6ab89c40e2176ccbf70cd",
	"darwin_arm64":  "dfac50c690aca459ed5158fad8bfbe99f9282baf4166cf7c410a6673fbc1f327",
	"darwin_amd64":  "930f8face3365810ce16689da81b7d1941fda4466225a7bbcbced9a2916a6e15",
}

type serverInfo struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	RootCA string `json:"root_ca"`
}
type Tunnel struct {
	URL    string
	cancel context.CancelFunc
	done   chan error
	once   sync.Once
}

func (t *Tunnel) Close() error {
	if t == nil {
		return nil
	}
	t.once.Do(func() { t.cancel() })
	select {
	case err, ok := <-t.done:
		if !ok || err == nil {
			return nil
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil
		}
		return err
	case <-time.After(5 * time.Second):
		return errors.New("FRP no terminó en 5 segundos")
	}
}
func (t *Tunnel) Done() <-chan error {
	if t == nil {
		return nil
	}
	return t.done
}
func client() *http.Client { return &http.Client{Timeout: 35 * time.Second} }
func getServer(ctx context.Context) (serverInfo, error) {
	var result serverInfo
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return result, err
	}
	res, err := client().Do(req)
	if err != nil {
		return result, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return result, fmt.Errorf("Gradio API: HTTP %d", res.StatusCode)
	}
	var servers []serverInfo
	if err = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&servers); err != nil || len(servers) == 0 {
		return result, errors.New("respuesta inválida de Gradio")
	}
	result = servers[0]
	if result.Port < 1 || result.Port > 65535 || !validHost(result.Host) {
		return serverInfo{}, errors.New("host/puerto FRP no válido")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(result.RootCA)) {
		return serverInfo{}, errors.New("CA TLS FRP inválida")
	}
	return result, nil
}

var domain = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]{0,251}[a-zA-Z0-9]$`)

func validHost(host string) bool {
	if len(host) > 253 || !domain.MatchString(host) || strings.Contains(host, "..") {
		return false
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsUnspecified()
	}
	return !strings.EqualFold(host, "localhost") && strings.Contains(host, ".")
}
func binaryName(goos, goarch string) (name, digest string, err error) {
	platform := goos + "_" + goarch
	digest, ok := checksums[platform]
	if !ok {
		return "", "", fmt.Errorf("FRP sin versión verificada para %s", platform)
	}
	name = "frpc_" + platform
	if goos == "windows" {
		name += ".exe"
	}
	return name, digest, nil
}
func cachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	if info, e := os.Stat("/kaggle/working"); e == nil && info.IsDir() {
		dir = filepath.Join("/kaggle/working", ".kagmcp", "cache")
	}
	return filepath.Join(dir, "kagmcp", "frp-v0.3"), nil
}
func verifyFile(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() < 1024 || st.Size() > maxFRPBytes {
		return errors.New("FRP en caché inválido")
	}
	h := sha256.New()
	if _, err = io.Copy(h, io.LimitReader(f, maxFRPBytes+1)); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return errors.New("FRP checksum incorrecto")
	}
	return nil
}
func downloadFRP(ctx context.Context) (string, error) {
	name, expected, err := binaryName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	dir, err := cachePath()
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, name)
	if err = verifyFile(dest, expected); err == nil {
		return dest, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdnURL+name, nil)
	if err != nil {
		return "", err
	}
	res, err := client().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("descarga FRP: HTTP %d", res.StatusCode)
	}
	if res.ContentLength > maxFRPBytes {
		return "", errors.New("FRP más grande que límite")
	}
	f, err := os.CreateTemp(dir, ".frp-download-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0700); err != nil {
		f.Close()
		return "", err
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, maxFRPBytes+1))
	closeErr := f.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n < 1024 || n > maxFRPBytes || hex.EncodeToString(h.Sum(nil)) != expected {
		return "", errors.New("FRP tamaño o SHA-256 incorrecto")
	}
	if err = os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("instalar FRP: %w", err)
	}
	return dest, nil
}
func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

var gradioURL = regexp.MustCompile(`^https://[a-zA-Z0-9-]+\.gradio\.live/?$`)

func extractURL(line string) string {
	const marker = "start proxy success:"
	i := strings.Index(line, marker)
	if i < 0 {
		return ""
	}
	tail := strings.Fields(strings.TrimSpace(line[i+len(marker):]))
	if len(tail) == 0 {
		return ""
	}
	if !gradioURL.MatchString(tail[0]) {
		return ""
	}
	return strings.TrimSuffix(tail[0], "/")
}
func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !gradioURL.MatchString(raw) {
		return errors.New("URL Gradio inesperada")
	}
	return nil
}

// Start publica el HTTP local mediante FRP oficial v0.3.
// Comprueba la salud HTTP pública en el orquestador; aquí solo se confirma
// que FRP anuncie un nombre .gradio.live verificable.
func Start(ctx context.Context, localPort int) (*Tunnel, error) {
	if localPort < 1 || localPort > 65535 {
		return nil, errors.New("puerto Gradio inválido")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	binary, err := downloadFRP(ctx)
	if err != nil {
		return nil, err
	}
	info, err := getServer(ctx)
	if err != nil {
		return nil, err
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "kagmcp-frp-ca-*")
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	ca := filepath.Join(dir, "root-ca.pem")
	if err = os.WriteFile(ca, []byte(info.RootCA), 0600); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	args := []string{"http", "-n", token, "-l", fmt.Sprint(localPort), "-i", "127.0.0.1", "--uc", "--sd", "random", "--ue", "--server_addr", net.JoinHostPort(info.Host, fmt.Sprint(info.Port)), "--disable_log_color", "--tls_enable", "--tls_trusted_ca_file", ca}
	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		os.RemoveAll(dir)
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		os.RemoveAll(dir)
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		cancel()
		os.RemoveAll(dir)
		return nil, fmt.Errorf("arrancar Gradio FRP: %w", err)
	}
	lines := make(chan string, 64)
	done := make(chan error, 1)
	scan := func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			default:
			}
		}
	}
	go scan(stdout)
	go scan(stderr)
	go func() { result := cmd.Wait(); _ = os.RemoveAll(dir); done <- result; close(done) }()
	tunnel := &Tunnel{cancel: cancel, done: done}
	timer := time.NewTimer(startupTimeout)
	defer timer.Stop()
	for {
		select {
		case line := <-lines:
			if strings.Contains(strings.ToLower(line), "login to server failed") {
				_ = tunnel.Close()
				return nil, errors.New("servidor Gradio rechazó conexión FRP")
			}
			if public := extractURL(line); public != "" {
				if err = validateURL(public); err != nil {
					_ = tunnel.Close()
					return nil, err
				}
				tunnel.URL = public
				return tunnel, nil
			}
		case <-done:
			cancel()
			return nil, errors.New("FRP terminó sin publicar URL")
		case <-timer.C:
			_ = tunnel.Close()
			return nil, errors.New("timeout (45 s) esperando URL gradio.live")
		case <-ctx.Done():
			_ = tunnel.Close()
			return nil, ctx.Err()
		}
	}
}

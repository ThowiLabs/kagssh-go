package config

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Las mismas claves se usan como variables exportadas o etiquetas de Kaggle Secrets.
var names = []string{
	"SSH_HOST", "SSH_USER", "SSH_PASSWORD", "SSH_KEY", "SSH_FINGERPRINT",
	"SSH_KNOWN_HOSTS", "SSH_PORT_REMOTE", "SSH_PORT_KAGGLE", "SSH_PORT_LOCAL",
	"SSH_LOGIN_USER", "SSH_LOGIN_PASSWORD",
	"SSH_AUTHORIZED_KEYS", "SSH_HOST_KEY",
}

type Config struct {
	Listen, HostKey, LoginUser, LoginPassword, AuthorizedKeys            string
	VPSHost, VPSUser, VPSPassword, VPSKey, VPSKnownHosts, VPSFingerprint string
	VPSPort, RemotePort                                                  int
	Retry                                                                time.Duration
}

// SecretReader permite probar Kaggle Secrets sin una conexión real.
type SecretReader interface {
	Get(context.Context, string) (string, bool, error)
}

func FromEnv(ctx context.Context) (Config, error) {
	var reader SecretReader
	if token := os.Getenv("KAGGLE_USER_SECRETS_TOKEN"); token != "" {
		reader = newKaggleSecrets(token, os.Getenv("KAGGLE_IAP_TOKEN"))
	}
	return Load(ctx, os.LookupEnv, reader)
}

// Load: export explícito > secret individual de Kaggle > valor predeterminado.
func Load(ctx context.Context, lookup func(string) (string, bool), secrets SecretReader) (Config, error) {
	values := make(map[string]string, len(names))
	for _, name := range names {
		if value, exists := lookup(name); exists {
			values[name] = value
			continue
		}
		if secrets == nil {
			continue
		}
		value, found, err := secrets.Get(ctx, name)
		if err != nil {
			return Config{}, fmt.Errorf("leyendo el Secret %s de Kaggle: %w", name, err)
		}
		if found {
			values[name] = value
		}
	}
	return parse(values)
}

func parse(v map[string]string) (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	login := "root"
	if u, err := user.Current(); err == nil && u.Username != "" {
		login = u.Username
	}
	value := func(name, fallback string) string {
		if v, ok := v[name]; ok {
			return v
		}
		return fallback
	}
	port := func(name string, fallback int) (int, error) {
		raw, ok := v[name]
		if !ok {
			return fallback, nil
		}
		p, err := strconv.Atoi(raw)
		if err != nil || p < 1 || p > 65535 {
			return 0, fmt.Errorf("%s debe estar entre 1 y 65535", name)
		}
		return p, nil
	}
	vpsPort, err := port("SSH_PORT_REMOTE", 22)
	if err != nil {
		return Config{}, err
	}
	remotePort, err := port("SSH_PORT_KAGGLE", 2223)
	if err != nil {
		return Config{}, err
	}
	localPort, err := port("SSH_PORT_LOCAL", 2224)
	if err != nil {
		return Config{}, err
	}
	c := Config{
		Listen:         net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)),
		HostKey:        value("SSH_HOST_KEY", filepath.Join(home, ".config", "kagssh", "host_ed25519")),
		LoginUser:      value("SSH_LOGIN_USER", login),
		LoginPassword:  value("SSH_LOGIN_PASSWORD", ""),
		AuthorizedKeys: value("SSH_AUTHORIZED_KEYS", ""),
		VPSHost:        value("SSH_HOST", ""),
		VPSUser:        value("SSH_USER", "root"),
		VPSPassword:    value("SSH_PASSWORD", ""),
		VPSKey:         value("SSH_KEY", ""),
		VPSKnownHosts:  value("SSH_KNOWN_HOSTS", filepath.Join(home, ".ssh", "known_hosts")),
		VPSFingerprint: value("SSH_FINGERPRINT", ""),
		VPSPort:        vpsPort,
		RemotePort:     remotePort,
		Retry:          5 * time.Second,
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.LoginUser == "" || strings.ContainsAny(c.LoginUser, "\r\n\x00") {
		return errors.New("usuario local inválido")
	}
	if c.LoginPassword == "" && c.AuthorizedKeys == "" {
		return errors.New("configura SSH_LOGIN_PASSWORD o SSH_AUTHORIZED_KEYS en Secrets o export")
	}
	if c.VPSPassword == "" && c.VPSKey == "" {
		return errors.New("configura SSH_PASSWORD o SSH_KEY en Secrets o export")
	}
	if c.VPSHost == "" || strings.ContainsAny(c.VPSHost, "\r\n\x00") {
		return errors.New("SSH_HOST: configura la IP pública o dominio de tu VPS en Kaggle Secrets o en el entorno")
	}
	if c.VPSUser == "" || strings.ContainsAny(c.VPSUser, "\r\n\x00") {
		return errors.New("SSH_USER inválido")
	}
	if c.VPSFingerprint == "" && c.VPSKnownHosts == "" {
		return errors.New("SSH_KNOWN_HOSTS o SSH_FINGERPRINT debe tener una ruta/huella; se configura ruta predeterminada automáticamente")
	}
	if c.VPSFingerprint == "" {
		if info, err := os.Stat(c.VPSKnownHosts); err == nil && info.IsDir() {
			return errors.New("SSH_KNOWN_HOSTS debe ser un archivo, no un directorio")
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("comprobar known_hosts del VPS: %w", err)
		}
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return fmt.Errorf("SSH_PORT_LOCAL inválido: %w", err)
	}
	if c.VPSPort < 1 || c.VPSPort > 65535 || c.RemotePort < 1 || c.RemotePort > 65535 {
		return errors.New("puerto fuera de rango")
	}
	return nil
}

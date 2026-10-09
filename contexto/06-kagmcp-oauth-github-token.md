# Contexto 06 — KagMCP: MCP, OAuth, túneles HTTPS y GitHub PAT

## Fecha y objetivo
2026-10-09. Evolución de KagSSH Go a **KagMCP**, implementación independiente y no oficial de Model Context Protocol para trabajar en Kaggle desde ChatGPT y clientes MCP compatibles, usando un único binario Go y sin obligar a tener VPS.

## Decisiones

1. **Conservar temporalmente GitHub `ThowiLabs/kagssh-go`** y el module path `github.com/ThowiLabs/kagssh-go` para no romper el notebook ni los clones. El nombre funcional del programa pasa a ser **KagMCP**. El usuario decidirá después cuándo renombrar el repositorio.
2. Se usa **una única ejecución interactiva de notebook**: el proceso Go entrega logs por stdout/stderr a Python de forma bloqueante y el botón Detener/Interruptar termina todos los servicios. No dejar servidores independientes en segundo plano.
3. **Servicios independientes:** MCP se activa con `MCP_ENABLED=true`, el SSH/SFTP heredado con `SSH_ENABLED=true`. Cuando MCP está activo y no se configuró `SSH_HOST`, SSH se desactiva por defecto para no requerir VPS. Si no se activa MCP, permanece el comportamiento previo SSH.
4. **OAuth independiente de GitHub:** el HTTP MCP necesita autenticación OAuth con PIN secreto `MCP_ACCESS_PIN` (12–128), PKCE, discovery y estado privado; se trasplantaron módulos de la rama `portable-shell` de Lilith-MCP (propiedad del usuario), con adaptación de importaciones y nombre.
5. **GitHub exclusivamente mediante PAT:** `GITHUB_TOKEN` como Kaggle Secret o variable. **No se crea ni instala GitHub App**; no usa client ID/secret ni login GitHub. El token se envía solo en cabecera HTTPS Authorization Bearer y nunca en URLs, argumentos, respuestas MCP ni logs.
6. **Túnel HTTPS:** `MCP_TUNNEL=cloudflare` es automático (Quick Tunnel, URL temporal). El servidor HTTP está enlazado a `127.0.0.1:MCP_LISTEN_PORT` y no requiere abrir puerto público Kaggle. `MCP_TUNNEL=none` requiere `MCP_PUBLIC_URL=https://...` y un proxy externo correctamente configurado.
7. **localhost.run no implementado en esta primera entrega:** su CLI usa un túnel SSH; requiere verificación segura de host, extracción robusta de URL de SSH y pruebas reales antes de ofrecerlo como transporte.
8. **SSH preservado:** servidor Go shell/PTY/SFTP y túnel inverso al VPS con escucha solicitada `0.0.0.0:SSH_PORT_KAGGLE`. El cambio de URL MCP no afecta la opción SSH.
9. **Estado privado bajo `/kaggle/working/.kagmcp`:** preferencias no sensibles en `config.json`, tokens emitidos OAuth en `state/oauth.json`, caché de conector Cloudflare en `cache`, identidad de host SSH y known_hosts en `ssh`. No se guardan PIN, claves o GITHUB_TOKEN en JSON. La persistencia del runtime Kaggle no está garantizada y perder el estado o cambiar la URL implica reautorizar.
10. Herramientas MCP iniciales: `environment_info`, `list_dir`, `read_file`, `write_file`, `exec`, `github_status`, `github_repositories`, `github_branches`, `github_file_read`, `github_file_write`.
11. Solo los métodos MCP autorizados reciben acceso a herramientas. Se controlan los orígenes HTTP, el tamaño de solicitudes y archivos, directorios permitidos (`/kaggle/working` lectura/escritura y `/kaggle/input` lectura) y se deniega lectura MCP del directorio privado. Los subprocesos no heredan variables `SSH_*`, `KAGGLE_*`, `MCP_*` o `GITHUB_*`. **El agente autorizado dispone de shell con los privilegios del proceso**, lo cual requiere confianza explícita.
12. Se reutilizan pruebas unitarias OAuth y Quick Tunnel del mismo Lilith-MCP (sin tocar sus archivos originales), además de crear pruebas del cliente GitHub PAT, rechazo de MCP sin OAuth y detección de herramientas.

## Arquitectura

```text
ChatGPT/cliente MCP --HTTPS OAuth--> Quick Tunnel cloudflare
                                  --> 127.0.0.1:8181/mcp
                                       KagMCP (Go)
                                       ├─ MCP (archivos, exec)
                                       ├─ GitHub REST (PAT opcional)
                                       ├─ OAuth state: .kagmcp/state
                                       ├─ conector: .kagmcp/cache
                                       └─ SSH/SFTP + reverse VPS:0.0.0.0:2223 (opcional)
```

## Dependencias y procedencia

- Código OAuth, protección de archivos privados y descarga verificada de cloudflared adaptados de la rama local `portable-shell` de `YahirHub/Lilith-MCP`. La rama de origen tenía cambios locales preexistentes no relacionados; **no se modificó Lilith-MCP**.
- No se introdujeron nuevas dependencias en go.mod, solamente nuevos paquetes del módulo existente y la stdlib Go.
- GitHub PAT usa `net/http` y `encoding/json`; no requiere `git` instalado en Kaggle para las herramientas remotas de lectura/escritura ofrecidas por MCP.
- El binario KagMCP todavía utiliza `./cmd/kagssh` como package compilado hasta un cambio de repositorio futuro; el ejecutable resultante se llama `kagmcp-linux-amd64`.

## Pruebas realizadas

- `go test -count=1 ./...` (config, GitHub PAT, MCP, OAuth, quicktunnel y túnel SSH).
- `go vet ./...`.
- `python scripts/check_notebook.py` con tests simulados de logs e interrupción.
- Compilación cruzada Linux amd64 con `CGO_ENABLED=0` y `-buildvcs=false`.
- `git diff --check`.

**Limitación:** faltan pruebas reales de Cloudflare desde un runtime de Kaggle, autorización de ChatGPT por OAuth y API GitHub contra una cuenta real con permisos específicos. La suite local no garantiza conectividad de producción.

## Riesgos y próximos pasos

- Probar URL temporal en Kaggle y autorizar desde un cliente MCP real.
- Confirmar que los procesos auxiliares desaparecen al pulsar Stop en Kaggle.
- Registrar fallos de red de Kaggle y restricciones de Cloudflare sin exponer Secrets.
- Implementar localhost.run solo con validación segura de huella y URL pública.
- Evaluar GitHub operations avanzadas (branches, clone/commit/push de múltiples archivos) manteniendo PAT sin App.
- Tras pruebas reales, decidir renombrar el repositorio GitHub y module path a KagMCP.

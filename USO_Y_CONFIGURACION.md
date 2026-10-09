# Uso y configuración de KagMCP en Kaggle

## Proyecto y alcance

KagMCP es un servidor MCP **no oficial**, integrado en un binario Go ejecutado directamente en un runtime Kaggle. Está diseñado para conectar ChatGPT y agentes compatibles con MCP mediante HTTPS/OAuth, habilitar trabajo autorizado en notebooks y datasets, administrar código y acceder a GitHub mediante un token personal.

**No está afiliado a Kaggle, Google ni OpenAI.** El usuario es responsable de las condiciones del runtime.

El repositorio remoto sigue temporalmente en `ThowiLabs/kagssh-go`. Todavía no se ha renombrado la URL del repositorio ni el módulo Go. El nombre público del programa es **KagMCP**.

## 1. Notebook

Abre [notebooks/kagssh_kaggle_chatgpt_agentes.ipynb](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb) en Kaggle, habilita Internet y autoriza Secrets. El notebook:

1. Clona o actualiza `main` sin eliminar modificaciones locales.
2. Comprueba la versión Go requerida y descarga una versión oficial verificando el hash si corresponde.
3. Compila el ejecutable `/kaggle/working/kagmcp-linux-amd64` con `./cmd/kagssh`.
4. Ejecuta `-check` para validar Secrets; no abre conexiones durante la comprobación.
5. Arranca **una celda bloqueante** que muestra stdout/stderr en vivo. El botón de **Detener/Interruptar** cierra el proceso y los servicios dependientes.

## 2. Secrets

En Kaggle: **Add-ons → Secrets**, crea uno por nombre. Orden de precedencia:

```text
export de la variable > Secret individual Kaggle > `.kagmcp/config.json` (solo preferencias no sensibles) > predeterminado
```

Nunca incluyas secretos reales en archivos Git ni en el propio notebook.

### MCP mínimo, sin VPS

| Nombre | Ejemplo |
|---|---|
| `MCP_ENABLED` | `true` |
| `MCP_ACCESS_PIN` | valor privado aleatorio de 12–128 caracteres |
| `MCP_TUNNEL` | `cloudflare` (opcional: es el predeterminado) |
| `SSH_ENABLED` | `false` (opcional: se selecciona automáticamente sin SSH_HOST) |

El proceso abre HTTP en `127.0.0.1:8181`, instala o reutiliza la utilidad verificada de Cloudflare y muestra un URL parecido a `https://nombre.trycloudflare.com/mcp`.

Este hostname **no se elige manualmente** en Quick Tunnel, y puede cambiar en cada arranque. Cuando la URL cambie, el cliente MCP puede necesitar volver a autorizarse.

### GitHub mediante Personal Access Token

Solo añade:

```text
GITHUB_TOKEN = (token personal configurado como Kaggle Secret)
```

No hay GitHub App, client ID, instalación de App ni OAuth GitHub. El servidor llama la REST API de GitHub con cabecera Bearer.

Herramientas:

- `github_status`: verificar usuario autenticado, sin devolver el token.
- `github_repositories`: listar hasta 100 repositorios recientes accesibles.
- `github_branches`: listar las ramas de un repositorio.
- `github_file_read`: leer contenido y SHA de un archivo de repositorio.
- `github_file_write`: crear o actualizar contenido haciendo un commit; al actualizar se debe enviar el SHA actual obtenido de `github_file_read`.

Usa un token fine-grained con solo el alcance y repositorios que necesitas. Para escrituras se necesita permiso **Contents: Read and write**; para operaciones de solo lectura basta **Contents: Read**.

### SSH opcional mediante VPS

Si también quieres SSH/SFTP:

```text
SSH_ENABLED=true
SSH_HOST=IP_PUBLICA_O_DOMINIO_DEL_VPS
SSH_USER=root
SSH_PASSWORD=SECRETO_VPS
SSH_LOGIN_PASSWORD=OTRO_SECRETO_DE_ACCESO_A_KAGGLE
```

Puedes emplear `SSH_KEY` y `SSH_AUTHORIZED_KEYS` como alternativas. Parámetros adicionales: `SSH_PORT_REMOTE=22`, `SSH_PORT_KAGGLE=2223`, `SSH_PORT_LOCAL=2224`, `SSH_LOGIN_USER`, `SSH_FINGERPRINT`, `SSH_KNOWN_HOSTS` y `SSH_HOST_KEY`.

El servidor OpenSSH del VPS debe permitir `AllowTcpForwarding yes` y `GatewayPorts clientspecified` o `yes`. El túnel inverso solicita siempre `0.0.0.0:2223` para que sea externo. Protege el puerto en el firewall.

## 3. Estados y archivos

```text
/kaggle/working/
  .kagmcp/
    config.json              # preferencias no sensibles
    state/oauth.json         # sesiones OAuth, no el PIN ni el PAT
    cache/lilith-mcp/bin/     # copia verificada de cloudflared
  kagmcp-linux-amd64
  kagssh-go/                  # clon temporal del repositorio actual
```

El programa emplea `/kaggle/working/.kagmcp` para estado privado y caché. Utiliza permisos privados de directorio y archivo. El archivo `config.json` se escribe automáticamente cuando arranca el CLI y solo conserva flags, puerto y URL propia. Los Secrets siguen siendo la fuente de las credenciales; no se serializan a JSON local.

**Persistencia:** Kaggle puede perder archivos de `/kaggle/working` al cambiar de runtime, por lo que no se garantiza persistencia entre ejecuciones. Cuando se pierde el estado OAuth o cambia la URL pública, se vuelve a conectar el cliente.

## 4. Conectar un cliente MCP

Después del arranque, copia la URL completa del log:

```text
https://subdominio.trycloudflare.com/mcp
```

Configura esa URL como servidor MCP remoto en el cliente compatible. El servidor exige autenticación OAuth y anuncia:

- `/.well-known/oauth-protected-resource/mcp`
- `/.well-known/oauth-authorization-server`
- `/oauth/register`, `/oauth/authorize`, `/oauth/token`

Durante la autorización, introduce el **PIN MCP** para permitir que ese cliente ejecute las herramientas. Acepta PKCE S256 y el descubrimiento moderno de clientes soportado por el módulo OAuth de Lilith.

El tráfico HTTPS termina en Cloudflare; el origen de KagMCP se expone en loopback y no se publica directamente en `0.0.0.0`. Ese modo de escucha es **independiente** del bind público solicitado al VPS para SSH.

## 5. Herramientas de Kaggle

| Herramienta | Alcance |
|---|---|
| `environment_info` | Información general del runtime sin secretos |
| `list_dir` | Listar rutas permitidas, máximo 100 entradas |
| `read_file` | Leer archivos de texto de hasta 1 MiB |
| `write_file` | Crear/modificar hasta 1 MiB en `/kaggle/working` |
| `exec` | Shell de Kaggle, máximo 45 s y 64 KiB de logs |
| GitHub tools | Repositorios, ramas, lectura y escritura de archivos |

Los datasets de `/kaggle/input` son de solo lectura. Las herramientas de archivos rechazan las rutas fuera de los árboles permitidos y deniegan explícitamente acceso al directorio privado `.kagmcp`. Los comandos sí disponen de los privilegios del proceso: concede OAuth únicamente a clientes de confianza.

## 6. Otros túneles

- **`MCP_TUNNEL=cloudflare` (implementado):** túnel temporal Cloudflare con URL HTTPS pública automática; requiere conectividad saliente y la descarga verificada del conector.
- **`MCP_TUNNEL=none` (implementado):** KagMCP escucha en loopback. Configura `MCP_PUBLIC_URL=https://dominio-propio` y un proxy externo que reenvíe HTTPS hacia el puerto local.
- **`localhost.run` (pendiente):** servicio basado en SSH; su integración interna exigirá verificar la identidad del servidor y descubrir la URL publicada sin guardar contraseñas ni romper el binario único.

El servicio no necesita una GitHub App para ninguno de estos transportes.

## 7. Compilación y pruebas

```bash
go test ./...
go vet ./...
python scripts/check_notebook.py

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -o kagmcp-linux-amd64 ./cmd/kagssh
```

La prueba real debe cubrir: inicio Cloudflare, URL HTTPS, descubrimiento OAuth, autorización, `tools/list`, lectura/escritura Kaggle, llamadas GitHub con token de pruebas y detención desde la celda. Hasta entonces no consideres verificada la integración extremo a extremo.

## 8. Riesgos y límites

- KagMCP **no** conserva un runtime Kaggle vivo indefinidamente; debes cumplir sus límites de sesión.
- Las herramientas autorizadas pueden ejecutar código dentro de Kaggle con los permisos reales del proceso, que en algunos runtimes pueden ser root.
- `MCP_ACCESS_PIN`, `GITHUB_TOKEN`, `SSH_PASSWORD` y claves privadas nunca deben registrarse en notebooks públicos ni logs.
- El cliente SSH con huella no configurada usa TOFU (confianza en primer uso); verifica esa primera huella fuera de banda.
- No expongas el puerto de origen HTTP directamente a Internet; utiliza HTTPS y OAuth.

## Solucionar «Authentication succeeded, action discovery failed»

**Segundo problema corregido:** el endpoint `tools/list` emitía esquemas inválidos con `"required": null` para las herramientas sin argumentos. El validador detectó esta incompatibilidad de JSON Schema, y la nueva versión **omite** esa propiedad cuando no corresponde. También hay pruebas contra la respuesta HTTP después de OAuth.

**Actualizar el notebook no es suficiente y tampoco es necesario si sus celdas ya ejecutan Git clone/fetch y build:** es indispensable ejecutar nuevamente las celdas `clonar` (actualizar código de `main`), `compilar` (crear binario nuevo) y finalmente `run`. Si solo repites la celda `run`, inicias el mismo binario anterior.

**SSH opcional:** si no lo utilizas, crea un Secret `SSH_ENABLED=false` para evitar que servicios SSH guardados se activen automáticamente. La presencia de logs SSH es independiente de la corrección del esquema MCP.


KagMCP admite tanto el protocolo MCP **2026-07-28** con `server/discover` (stateless) como los clientes que usan `initialize` y `tools/list` de 2025. Los cambios en la autenticación OAuth y los Secrets no son necesarios.

Si autorizaste la versión anterior y ChatGPT muestra ese error, **detén la celda actual**, vuelve a ejecutar las celdas del notebook **clonar → compilar → validación → run** y utiliza la URL HTTPS `.../mcp` que aparece en los nuevos logs. Repite el análisis de herramientas en ChatGPT; si el Quick Tunnel creó otra URL, actualiza/recrea esa conexión.

La salida del binario registra `descubrimiento MCP` con el nombre de los métodos `server/discover`, `initialize` o `tools/list`, sin registrar tokens ni argumentos. Si vuelve a fallar, aporta solo esos logs y el estado HTTP, nunca credenciales. La compatibilidad está verificada localmente con pruebas del protocolo y flujo real OAuth/PKCE, pero no sustituye una prueba de extremo a extremo en el runtime Kaggle.

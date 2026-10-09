# KagMCP — MCP no oficial para Kaggle

**KagMCP** es una implementación **independiente y no oficial** de Model Context Protocol (MCP) para conectar ChatGPT y otros agentes compatibles a una sesión autorizada de Kaggle. Está pensado para **investigación, análisis de datos, preparación de datasets, creación y reparación de notebooks, desarrollo, entrenamiento y depuración de proyectos de machine learning**.

No está afiliado, respaldado ni patrocinado por Kaggle, Google u OpenAI. El usuario es responsable de los permisos, cuotas y políticas del runtime.

> **Estado:** base de MCP/OAuth, Cloudflare Quick Tunnel, herramientas de Kaggle, GitHub PAT y SSH opcional implementada y cubierta por pruebas locales. La compatibilidad extremo a extremo con un runtime Kaggle, un túnel público y un cliente ChatGPT real **todavía requiere validación**.

## Inicio en Kaggle — configuración en Python (recomendado)

**Notebook:** [Instalar y ejecutar KagMCP](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb).

1. En Kaggle habilita **Internet**. **No necesitas GPU ni configurar Kaggle Secrets** para MCP puro.
2. Ejecuta `clonar`, `instalar-go` y `compilar`, en ese orden.
3. Ejecuta la celda **configurar**. Por defecto trae `MCP_ENABLED=True` y `SSH_ENABLED=False`; solicita mediante `getpass` un **PIN OAuth secreto de 12–128 caracteres**. Para GitHub, cambia `USE_GITHUB=True` y escribe el PAT de forma oculta en el mismo paso.
4. Ejecuta **validación** y después **run**. El proceso Go recibe las opciones como variables del proceso; **no consulta Kaggle Secrets** para MCP ni GitHub. La celda run permanece activa con logs en vivo.
5. Copia la URL `https://....trycloudflare.com/mcp` y conecta ChatGPT; aprueba OAuth con el PIN que introdujiste en Python.
6. Para terminar, pulsa **Detener/Interruptar** en la celda run.

**SSH/SFTP opcional:** activa `SSH_ENABLED=True` en la celda de configuración e indica allí el host, usuario, puertos y huella del VPS. Solo entonces Python lee desde Kaggle Secrets `SSH_PASSWORD` (contraseña del VPS) y `SSH_LOGIN_PASSWORD` (contraseña de entrada al servidor SSH local). Si SSH está deshabilitado, el notebook no invoca la API de Secrets.

**Importante al actualizar:** ejecutar `clonar` actualiza el código Go del repositorio, **no modifica automáticamente la copia del notebook que tienes abierta en Kaggle**. Para incorporar esta nueva celda Python, abre/importa la versión actual del notebook o copia manualmente la celda; después recompila. `run` por sí solo tampoco actualiza Go.

## Arquitectura

```text
ChatGPT / cliente MCP autorizado
          |
          | OAuth + HTTPS
          v
  Cloudflare Quick Tunnel (URL HTTPS temporal)
          |
          v
 /kaggle/working / kagmcp (un único ejecutable Go)
   |-- HTTP Streamable MCP: 127.0.0.1:8181/mcp
   |-- herramientas de archivos, entorno, comandos
   |-- GitHub REST mediante token PERSONAL
   |-- SSH/SFTP + túnel inverso al VPS (opcional)
   \-- estado OAuth y binario cloudflared en .kagmcp/
```

Cloudflare termina HTTPS y reenvía al servidor HTTP que escucha **solamente en loopback**. Esto es correcto y distinto de la opción SSH: si se habilita el túnel SSH, el puerto solicitado al VPS sigue siendo **`0.0.0.0:SSH_PORT_KAGGLE`** para conexiones externas.

## Configuración: Python primero y Secrets de SSH únicamente

En el notebook recomendado se prepara una copia llamada `RUNTIME_ENV` para cada invocación del binario Go. Sus variables **sobrescriben las preferencias persistidas** y se omiten de esa copia `KAGGLE_USER_SECRETS_TOKEN` y `KAGGLE_IAP_TOKEN`. Así, el cargador automático de Go **no realiza solicitudes a Kaggle Secrets** al ejecutar `-check` o `run`.

| Opción en celda `configurar` | Valor por defecto | Uso |
|---|---|---|
| `MCP_ENABLED` | `True` | Inicia MCP |
| `SSH_ENABLED` | `False` | Nunca inicia SSH involuntariamente |
| `MCP_TUNNEL` | `cloudflare` | URL HTTPS temporal; también `none` con proxy externo |
| `MCP_LISTEN_PORT` | `8181` | Puerto interno MCP |
| `MCP_PUBLIC_URL` | vacío | Origen HTTPS si se usa `none` |
| `USE_GITHUB` | `False` | Solicita PAT con `getpass` |
| `SSH_HOST`, `SSH_USER`, `SSH_PORT_REMOTE` | vacío, `root`, `22` | Dirección y acceso al VPS opcional |
| `SSH_PORT_KAGGLE`, `SSH_PORT_LOCAL` | `2223`, `2224` | Puertos SSH |
| `SSH_FINGERPRINT` | vacío | Huella del VPS verificada por canal seguro |

**Nunca escribas `MCP_ACCESS_PIN`, `GITHUB_TOKEN`, `SSH_PASSWORD` o `SSH_LOGIN_PASSWORD` como texto visible en el notebook.** El PIN y el PAT se piden con `getpass`; solo las dos contraseñas SSH opcionales provienen de Kaggle Secrets. Todos se almacenan **en la memoria de Python y del proceso Go durante la sesión**. `.kagmcp/config.json` conserva solo configuraciones públicas. No se promete aislamiento absoluto frente a otros procesos con privilegios elevados en el mismo runtime.

La ejecución directa del binario Go sin la celda Python conserva el mecanismo anterior de lectura automática por entorno/Kaggle Secrets. Para evitar HTTP 429 en el notebook, usa siempre el entorno ya preparado.

### Conectar GitHub con token personal

Crea un **fine-grained Personal Access Token** de GitHub con acceso a los repositorios deseados y permisos de `Contents: read` o `Contents: read and write`, dependiendo de las operaciones. En el notebook, cambia **`USE_GITHUB=True`** e introduce el PAT mediante `getpass` (no Kaggle Secrets). KagMCP lo envía por HTTPS únicamente en la cabecera `Authorization` al servicio GitHub API; el token no se pasa al cliente MCP como resultado.

No existe GitHub App, manifiesto de App, callback de instalación ni autenticación GitHub OAuth. El OAuth de KagMCP **protege a ChatGPT** y no tiene relación con el acceso al API de GitHub.

Herramientas GitHub iniciales: `github_status`, `github_repositories`, `github_branches`, `github_file_read` y `github_file_write`. La última crea un commit mediante GitHub Contents API y necesita el SHA previo para actualizar un archivo existente.

### Herramientas MCP

`environment_info`, `list_dir`, `read_file`, `write_file`, `exec` y herramientas GitHub anteriores. Las herramientas de archivos permiten lectura dentro de `/kaggle/working` y `/kaggle/input`, escritura solo en `/kaggle/working`, y bloquean el directorio privado `.kagmcp`.

El comando `exec` admite procesos del runtime Kaggle con timeout de 45 segundos y salida acotada. Los programas lanzados no heredan las variables `KAGGLE_*`, `MCP_*`, `SSH_*` ni `GITHUB_*`. **Advertencia:** el acceso autorizado a terminal implica confiar en ese cliente, porque el proceso tiene los permisos del runtime.

### SSH/SFTP por VPS (opcional)

Para habilitar SSH en el notebook cambia `SSH_ENABLED=True` e introduce `SSH_HOST`, usuario, puertos y huella en la celda Python. Autoriza solo los Secrets `SSH_PASSWORD` y `SSH_LOGIN_PASSWORD`. El túnel reverso publica desde tu VPS el puerto solicitado `0.0.0.0:2223` (o el puerto personalizado), siempre que OpenSSH tenga:

```text
AllowTcpForwarding yes
GatewayPorts clientspecified
```

Y el firewall permita TCP 2223. Desde tu PC:

```bash
ssh -p 2223 usuario_kaggle@IP_PUBLICA_DEL_VPS
sftp -P 2223 usuario_kaggle@IP_PUBLICA_DEL_VPS
```

El SSH integrado continúa funcionando sin instalar OpenSSH Server dentro de Kaggle. Consulta [USO_Y_CONFIGURACION.md](USO_Y_CONFIGURACION.md) para el uso avanzado.

## Compilar y probar

El repositorio sigue temporalmente en [ThowiLabs/kagssh-go](https://github.com/ThowiLabs/kagssh-go); **no se ha renombrado aún GitHub ni el módulo Go**.

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags="-s -w" -o kagmcp-linux-amd64 ./cmd/kagssh
go test ./...
go vet ./...
python scripts/check_notebook.py
```

Compila un único ejecutable; cuando se usa Cloudflare, KagMCP descarga y verifica la utilidad oficial `cloudflared` como proceso auxiliar de transporte. No necesita Python para el servidor: Python prepara de forma segura la configuración y ejecuta el binario desde el notebook.

## Solucionar «Authentication succeeded, action discovery failed»

**Segundo problema corregido:** el endpoint `tools/list` emitía esquemas inválidos con `"required": null` para las herramientas sin argumentos. El validador detectó esta incompatibilidad de JSON Schema, y la nueva versión **omite** esa propiedad cuando no corresponde. También hay pruebas contra la respuesta HTTP después de OAuth.

**Si cambió el código Go**, ejecuta `clonar` y `compilar`. **Si cambió el propio notebook**, vuelve a importar el notebook actualizado o copia sus nuevas celdas: `clonar` no actualiza el notebook que tienes abierto. Finalmente ejecuta `configurar`, `validación` y `run`.

**SSH opcional:** el nuevo notebook trae `SSH_ENABLED=False` explícitamente en Python; no hay que crear el Secret de ese nombre. El error de esquema MCP es independiente de SSH.


KagMCP admite tanto el protocolo MCP **2026-07-28** con `server/discover` (stateless) como los clientes que usan `initialize` y `tools/list` de 2025. Los cambios en la autenticación OAuth y los Secrets no son necesarios.

Si autorizaste la versión anterior y ChatGPT muestra ese error, **detén la celda actual**, vuelve a ejecutar las celdas del notebook **clonar → compilar → validación → run** y utiliza la URL HTTPS `.../mcp` que aparece en los nuevos logs. Repite el análisis de herramientas en ChatGPT; si el Quick Tunnel creó otra URL, actualiza/recrea esa conexión.

La salida del binario registra `descubrimiento MCP` con el nombre de los métodos `server/discover`, `initialize` o `tools/list`, sin registrar tokens ni argumentos. Si vuelve a fallar, aporta solo esos logs y el estado HTTP, nunca credenciales. La compatibilidad está verificada localmente con pruebas del protocolo y flujo real OAuth/PKCE, pero no sustituye una prueba de extremo a extremo en el runtime Kaggle.

## Seguridad y límites

- OAuth con PIN, PKCE, metadata de descubrimiento, protección de autorización y límites de solicitudes, heredados y adaptados de [Lilith-MCP](https://github.com/YahirHub/Lilith-MCP).
- No publiques PIN, claves privadas ni tokens en celdas públicas, commits, URLs o logs.
- Cloudflare Quick Tunnel publica una URL **temporal** que puede cambiar en cada sesión; `MCP_TUNNEL=none` requiere un proxy HTTPS propio y `MCP_PUBLIC_URL`.
- **localhost.run y otros transportes aún no están implementados**; se contemplan para extensiones futuras tras verificar el flujo público y la identidad del servidor SSH.
- La integración real con ChatGPT y un runtime Kaggle activo permanece pendiente de prueba extremo a extremo.
- Los permisos de las herramientas MCP son los del proceso KagMCP: evita autorizar clientes no confiables.

## Licencias y procedencia

El código de servidor OAuth, almacenamiento privado y Cloudflare Quick Tunnel procede de la variante `portable-shell` de **Lilith-MCP**, adaptada a este módulo y al flujo de Kaggle. KagMCP mantiene su propia interfaz de herramientas, sistema de configuración y cliente personal de GitHub.

## Ponytail v2, proyectos y panel web

La metodología **Ponytail v2** está integrada **íntegra** en el binario, a partir del archivo local `codex-ponytail-v2.md` (no procede de Lilith). El archivo integrado es `internal/skills/ponytail-v2.md` e incluye una extensión obligatoria para Kaggle: cada proyecto debe terminar con **repositorio Git y notebook funcional**, comprobados mediante pruebas; **después** se genera e integra una UI **Gradio**, con dependencias fijadas, lock de transitivas con hashes y validación de arranque. También obliga a vigilar continuamente el disco para prevenir el estado `readonly`.

Herramientas MCP añadidas:

- `skills_list`, `skills_read`, `skills_search` y `skills_install`. `ponytail-v2` es siempre activa y no se puede sustituir; se puede consultar por fragmentos.
- `projects_list`, `project_create`, `project_memory_add`, `project_export`, `tasks_add`, `tasks_update`, `history_list`, `gradio_scaffold`.
- `exec` admite `project` y `description` para auditar las operaciones de cada agente. Los comandos que posiblemente contienen credenciales se ocultan por completo en el historial, y no se almacena stdout.

Al arrancar, el log muestra dos enlaces **del mismo túnel**: `https://...trycloudflare.com/mcp` para clientes MCP y **`https://...trycloudflare.com/`** para el panel. En la raíz `/`, inicia sesión con el **mismo PIN MCP**. El panel permite crear/consultar proyectos, memoria, tareas, historial, leer Skills y **configurar o desconectar el PAT GitHub**. El formulario usa cookies `Secure` + `HttpOnly` + `SameSite=Strict`, protección CSRF y verificación de origen, sesiones de 8 horas, revocación al cerrar sesión y límites de intentos.

La web está implementada en el servidor Go, sin añadir una dependencia Python al servicio principal. **Gradio está reservado a las interfaces funcionales de los proyectos Kaggle**, posteriores a verificar notebook y Git. El generador fija `gradio==6.30.0` y documenta la compilación de `requirements-gradio.lock` con hashes mediante `pip-tools==7.6.2`. La instalación reproducible exige el lock creado y versionado; fijar solamente Gradio no inmoviliza todas sus transitivas.

**Persistencia:** el estado compartido por todos los agentes conectados a **este mismo proceso** se almacena con escritura atómica en `/kaggle/working/.kagmcp/projects.json`, con límites de tamaño (100 proyectos, 200 notas/tareas por proyecto, 1 000 eventos). Persiste entre reinicios del binario mientras siga existiendo el volumen; **no está garantizado entre sesiones de Kaggle**. Ejecuta `project_export` para generar `/kaggle/working/<project_id>/contexto/kagmcp-proyecto.md` y haz commit/push de ese contenido para tener recuperación fuera del runtime. No se guardan contraseñas ni PAT en el estado.

**Disco:** el servidor comprueba periódicamente el almacenamiento de `/kaggle/working`, antes de ejecutar comandos o escribir archivos, y cada 2 segundos durante comandos. Detiene operaciones cuando quedan menos de 512 MiB o el 3 % del volumen. Es protección preventiva, **no** una cuota real de disco ni una garantía absoluta ante programas externos o escrituras muy rápidas.

**Seguridad:** el panel y el MCP comparten PIN pero mantienen autenticación y sesiones diferentes. Las credenciales de GitHub introducidas por web se verifican contra el servicio GitHub y residen solo en memoria del proceso (se pierden al reiniciarlo). Los clientes MCP autorizados pueden ejecutar comandos con los permisos del runtime, incluso root: autoriza exclusivamente agentes de confianza y no expongas el PIN. El origen Cloudflare usa HTTPS; el servidor escucha en loopback.

### Recuperación comprobable al cambiar de runtime

`project_export` (también disponible en el panel web) escribe **dos archivos** dentro del repositorio del proyecto: `contexto/kagmcp-proyecto.md` legible y `contexto/kagmcp-proyecto.json` restaurable. Tras revisarlos para evitar subir notas sensibles, añade ambos a Git y haz commit/push. En otra sesión, clona ese repositorio y ejecuta **`project_import`** con el ID de la carpeta del proyecto; memoria y tareas vuelven al estado compartido. El historial de comandos permanece local en `.kagmcp/projects.json` y no se exporta automáticamente por seguridad.

El formulario GitHub del panel permite cambiar o quitar el token sin modificar `config.json`; se pierde al reiniciar el proceso. La exportación no realiza `git push` automáticamente.

### Comprobar realmente un proyecto antes de generar Gradio

La herramienta `project_verify` sustituye la declaración manual `tests_passed=true`. Debes registrar primero el proyecto (`project_create`), crear o corregir el repositorio bajo `/kaggle/working/<project_id>/` y versionar sus cambios. Antes de emitir un comprobante, se verifica que el repositorio **esté limpio y tenga commit** y que el cuaderno sea `.ipynb` válido con celdas de código.

Ejemplo de argumentos para `project_verify` (adapta las rutas y los comandos a cada proyecto):

```json
{
  "project": "mi-proyecto",
  "test_command": "python -m pytest -q",
  "notebook_command": "python -m jupyter nbconvert --to notebook --execute notebooks/run.ipynb --output-dir /tmp --output kagmcp-verificado.ipynb"
}
```

Ambos comandos deben concluir satisfactoriamente; los ejecuta el agente autorizado con límites de tiempo y controles del disco, no se confía en la respuesta del cliente. Luego compara el commit de Git y SHA-256 del notebook antes/después y registra un comprobante privado por proyecto. Si cambia el notebook, el commit o hay modificaciones pendientes en Git, `gradio_scaffold` exige **repetir `project_verify`**. `tests_passed` en clientes anteriores se ignora.

La herramienta **no puede evaluar la calidad científica de las pruebas** ni garantiza que un comando exitoso cubra todas las funcionalidades: quien configura los comandos debe seleccionar verificaciones reales del proyecto. El notebook generado por `gradio_scaffold` sigue siendo una **plantilla que hay que conectar a la lógica funcional** y probar después.

El portal `/` ofrece ahora **Exportar** y **Restaurar** la memoria del proyecto desde los archivos JSON/Markdown del repositorio. La restauración recupera notas y tareas, pero invalida comprobantes importados para obligar a repetir las pruebas en el nuevo runtime. Los archivos de contexto pueden contener datos privados y deben revisarse antes de publicarlos.

# Uso y configuración de KagMCP en Kaggle

## Proyecto y alcance

KagMCP es un servidor MCP **no oficial**, integrado en un binario Go ejecutado directamente en un runtime Kaggle. Está diseñado para conectar ChatGPT y agentes compatibles con MCP mediante HTTPS/OAuth, habilitar trabajo autorizado en notebooks y datasets, administrar código y acceder a GitHub mediante un token personal.

**No está afiliado a Kaggle, Google ni OpenAI.** El usuario es responsable de las condiciones del runtime.

El repositorio remoto sigue temporalmente en `ThowiLabs/kagssh-go`. Todavía no se ha renombrado la URL del repositorio ni el módulo Go. El nombre público del programa es **KagMCP**.

## 1. Notebook — ejecución Python-first

Abre [notebooks/kagssh_kaggle_chatgpt_agentes.ipynb](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb) en Kaggle, habilita Internet, y ejecuta las siguientes celdas:

1. **clonar:** descarga `main` sin sobreescribir cambios locales.
2. **instalar-go:** comprueba Go requerido o descarga la versión oficial validada con SHA256.
3. **compilar:** genera `/kaggle/working/kagmcp-linux-amd64`.
4. **configurar:** define opciones no sensibles y `pin = ""` (Go genera un PIN temporal al arrancar `run`) o establece un PIN propio de 6–128 caracteres. El PAT opcional se pide con `getpass`; con SSH activado se consultan únicamente dos contraseñas de Kaggle Secrets.
5. **validación:** ejecuta `-check` con el entorno preparado **sin consultar Secrets**.
6. **run:** inicia Go en primer plano, muestra sus logs y termina al pulsar **Detener/Interruptar**.

El notebook que está abierto en Kaggle es una **copia independiente** del archivo en GitHub. Para utilizar nuevas celdas que añadimos al archivo del repositorio debes volver a importar el notebook o copiar sus celdas; correr **clonar** no lo actualiza en la interfaz.

## 2. Configurar servicios y credenciales

### Solo MCP (modo por defecto, sin Secrets)

En la celda Python:

```python
MCP_ENABLED = True
SSH_ENABLED = False
MCP_TUNNEL = "auto"  # Gradio → Cloudflare en caso de fallos
GRADIO_RETRIES = 3
MCP_LISTEN_PORT = 8181
USE_GITHUB = False
```

Con `pin = ""`, Go genera al iniciar un PIN temporal criptográficamente aleatorio (20 caracteres) y lo imprime una sola vez en la salida privada de la celda `run`; **Python no genera ni solicita el PIN**. Si configuras un valor propio, debe tener entre **6 y 128 caracteres**. Inicia HTTP en `127.0.0.1:8181`, crea el túnel HTTPS y muestra `/mcp` y el panel `/`. Tras iniciar sesión con el PIN actual, puedes modificarlo en la web. El cambio revoca las sesiones web y autorizaciones OAuth previas, por lo que los clientes deberán autorizarse de nuevo.

**La GPU no es necesaria para MCP.** Con SSH deshabilitado **no se hace ninguna llamada a Kaggle Secrets**, aunque en la cuenta existan Secrets antiguos. La celda retira de `RUNTIME_ENV` las credenciales internas `KAGGLE_USER_SECRETS_TOKEN` y `KAGGLE_IAP_TOKEN` antes de llamar a Go. El entorno global de Kaggle no se altera.

### MCP con GitHub PAT (sin Kaggle Secrets)

Cambia `USE_GITHUB=True`; la celda solicita el GitHub Personal Access Token mediante entrada oculta, en memoria. No se necesita ninguna GitHub App ni etiqueta `GITHUB_TOKEN` en Secrets. Usa un PAT fine-grained limitado a los repositorios y permisos `Contents` necesarios.

Herramientas: `github_status`, `github_repositories`, `github_branches`, `github_file_read` y `github_file_write`. La última necesita el SHA actual si actualiza un archivo existente.

### SSH/VPS opcional (solo dos Secrets)

Cambia `SSH_ENABLED=True` y define en Python `SSH_HOST`, `SSH_USER`, `SSH_PORT_REMOTE`, `SSH_PORT_KAGGLE`, `SSH_PORT_LOCAL`, `SSH_LOGIN_USER` y opcionalmente `SSH_FINGERPRINT`. Crea y autoriza únicamente:

| Secret en Kaggle | Finalidad |
|---|---|
| `SSH_PASSWORD` | Contraseña para conectar al VPS |
| `SSH_LOGIN_PASSWORD` | Contraseña para entrar por SSH al runtime Kaggle |

El cliente `kaggle_secrets.UserSecretsClient` lee estos dos valores **una sola vez** cuando ejecutas la celda configurar con SSH activado, no durante `-check` ni `run`. En caso de HTTP 429 no hagas reintentos compulsivos. No guardes contraseñas en el notebook.

En el VPS, `sshd` necesita `AllowTcpForwarding yes` y `GatewayPorts clientspecified` o `yes`. El túnel inverso solicita el puerto externo `0.0.0.0:SSH_PORT_KAGGLE`; protege ese puerto con firewall. Si no necesitas SSH, mantenlo desactivado.

### Prioridad y persistencia

El ejecutable Go sigue soportando, cuando se utiliza fuera de este notebook, sus fuentes anteriores (`env > Secrets > .kagmcp/config.json > default`). **En este notebook**, Python construye las variables explícitamente, elimina el token Kaggle Secrets del entorno del subprocess y evita consultas adicionales. El PIN, el PAT y las contraseñas solo viven en memoria durante el kernel, y los Secrets no se escriben en JSON ni Git.

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

El programa emplea `/kaggle/working/.kagmcp` para estado privado y caché. Utiliza permisos privados de directorio y archivo. El archivo `config.json` se escribe automáticamente cuando arranca el CLI y solo conserva flags, puerto y URL propia. La celda Python entrega un PIN explícito o vacío; **Go genera el temporal cuando el valor llega vacío**. El PAT continúa usándose mediante `getpass`, y Kaggle Secrets se consulta solo para las dos contraseñas SSH cuando corresponde. Ninguna credencial se serializa a JSON local.

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

El tráfico HTTPS llega a través de Gradio FRP o Cloudflare; el origen de KagMCP se expone en loopback y no se publica directamente en `0.0.0.0`. Ese modo de escucha es **independiente** del bind público solicitado al VPS para SSH.

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

- **`MCP_TUNNEL=auto` (predeterminado):** Gradio FRP TLS prioritario y fallback a Cloudflare después de los intentos configurados.
- **`MCP_TUNNEL=gradio` (estricto):** URL Gradio sin Cloudflare de respaldo.
- **`MCP_TUNNEL=cloudflare` (estricto):** túnel temporal Cloudflare con URL HTTPS pública automática; requiere conectividad saliente y la descarga verificada del conector.
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
- No compartas notebooks que tengan un PIN explícito ni los logs privados donde Go muestra **una sola vez** el PIN temporal generado; no publiques `GITHUB_TOKEN`, `SSH_PASSWORD` ni claves privadas.
- El cliente SSH con huella no configurada usa TOFU (confianza en primer uso); verifica esa primera huella fuera de banda.
- No expongas el puerto de origen HTTP directamente a Internet; utiliza HTTPS y OAuth.

## Solucionar «Authentication succeeded, action discovery failed»

**Segundo problema corregido:** el endpoint `tools/list` emitía esquemas inválidos con `"required": null` para las herramientas sin argumentos. El validador detectó esta incompatibilidad de JSON Schema, y la nueva versión **omite** esa propiedad cuando no corresponde. También hay pruebas contra la respuesta HTTP después de OAuth.

**Actualizaciones:** `clonar` actualiza el repositorio pero no las celdas del notebook ya abierto. Reimporta el notebook cuando cambien sus celdas; ejecuta `clonar → compilar → configurar → validación → run` para la versión actual.

**SSH opcional:** la nueva celda Python establece `SSH_ENABLED=False` por defecto, incluso con Secrets SSH antiguos.


KagMCP admite tanto el protocolo MCP **2026-07-28** con `server/discover` (stateless) como los clientes que usan `initialize` y `tools/list` de 2025. Los cambios en la autenticación OAuth y los Secrets no son necesarios.

Si autorizaste la versión anterior y ChatGPT muestra ese error, **detén la celda actual**, vuelve a ejecutar las celdas del notebook **clonar → compilar → validación → run** y utiliza la URL HTTPS `.../mcp` que aparece en los nuevos logs. Repite el análisis de herramientas en ChatGPT; si el Quick Tunnel creó otra URL, actualiza/recrea esa conexión.

La salida del binario registra `descubrimiento MCP` con el nombre de los métodos `server/discover`, `initialize` o `tools/list`, sin registrar tokens ni argumentos. Si vuelve a fallar, aporta solo esos logs y el estado HTTP, nunca credenciales. La compatibilidad está verificada localmente con pruebas del protocolo y flujo real OAuth/PKCE, pero no sustituye una prueba de extremo a extremo en el runtime Kaggle.

## 9. Portal web y proyectos

En la salida de la celda `run` se mostrarán:

```text
url=https://subdominio.gradio.live/mcp
panel_web=https://subdominio.gradio.live/
```

La segunda URL abre la administración del servidor. Usa **el PIN MCP introducido en la celda configurar**. Después del login puedes crear proyectos (ID igual al nombre de su carpeta bajo `/kaggle/working`), consultar sus notas, crear/cambiar tareas, leer historial, recorrer Ponytail v2 y configurar o borrar el PAT de GitHub sin almacenarlo en disco.

El panel **no es Gradio**: está implementado en Go para compartir el mismo HTTP/OAuth y no instalar paquetes Python innecesarios en el proceso principal. Los proyectos que desarrolles mediante KagMCP sí deberán terminar con una **interfaz Gradio real después de verificar** repositorio Git y notebook.

Login: máximo cinco intentos fallidos por origen en ventana de 15 minutos; límite global, sesiones de ocho horas (máximo cien simultáneas), CSRF, cookies seguras y bloqueo de frames. El PIN no se escribe en ningún archivo. Si pierdes la sesión, vuelve a la raíz y autentícate otra vez.

## 10. Sistema de Skills y memoria multiagente

`ponytail-v2` es la Skill integrada siempre activa, copiada íntegramente de `C:\Users\Admin\Documents\codex-ponytail-v2.md` y ampliada con la regla específica de Kaggle. El contenido está empaquetado dentro del ejecutable Go. Se carga en fragmentos mediante `skills_read`; otras Skills se instalan con `skills_install` y se guardan en `/kaggle/working/.kagmcp/skills`.

Reglas obligatorias durante trabajos de Kaggle:

1. Registrar **proyecto**, objetivo, repositorio, notebook, tareas, decisiones y descripción de cada comando.
2. Crear/corregir **el repositorio Git y el notebook `.ipynb`** de manera coherente. Ejecutar y registrar pruebas reales.
3. Bloquear dependencias directas y transitivas, versiones de Python/modelos y SHA de artefactos cuando estén disponibles; no instalar versiones abiertas que cambian cada día.
4. Vigilar `df -h` y el disco durante todo el proceso: si se agota puede activarse el modo `readonly`.
5. Una vez que repositorio y notebook estén verificados, crear y **conectar una UI Gradio a la lógica del proyecto**. El tool `gradio_scaffold` comprueba Git y JSON de notebook y genera plantilla con `gradio==6.30.0`; debe personalizarse y comprobarse, no constituye una UI terminada.
6. Exportar memoria y tareas vía `project_export` al contexto del repositorio y versionarlo en GitHub.

Estado privado en `/kaggle/working/.kagmcp/projects.json`. Varios agentes conectados al **mismo KagMCP** ven el mismo almacenamiento y las escrituras quedan serializadas. Para continuidad real entre sesiones Kaggle, sincronizar el contexto relevante en Git (no se automatiza un push sin petición explícita). Memoria: hasta 100 proyectos, 200 notas y 200 tareas por proyecto; el historial conserva 1 000 acciones recientes, sin stdout y con ocultación preventiva de comandos potencialmente sensibles.

## 11. Seguridad del almacenamiento y dependencias Gradio

Los guardarraíles de disco rechazan escrituras o ejecución si el volumen de trabajo tiene menos de 512 MiB libres o menos del 3 % de capacidad disponible. El monitor vuelve a revisar cada 5 segundos y controla comandos cada 2 segundos. **No sustituye las cuotas del proveedor ni puede impedir al 100 % escrituras rápidas o externas**.

Para Gradio, primero genera y personaliza `app.py`, `requirements-gradio.in` y `GRADIO_REPRODUCIBLE.md` tras verificar el proyecto. Bloquea dependencias transitivas con hashes desde entorno limpio y registra el archivo `requirements-gradio.lock` en el repositorio. Ejecuta `python -m pip install --require-hashes --no-cache-dir -r requirements-gradio.lock`; no instales desde un requerimiento abierto sin versión.

El PAT de GitHub configurado desde el panel se verifica contra la API y solo queda en memoria. Para recuperarlo en una nueva sesión del notebook, vuelve a introducirlo mediante `getpass` con `USE_GITHUB=True` o desde el panel después de conectar. Nunca pongas el PAT en el repo.

### Recuperación comprobable al cambiar de runtime

`project_export` (también disponible en el panel web) escribe **dos archivos** dentro del repositorio del proyecto: `contexto/kagmcp-proyecto.md` legible y `contexto/kagmcp-proyecto.json` restaurable. Tras revisarlos para evitar subir notas sensibles, añade ambos a Git y haz commit/push. En otra sesión, clona ese repositorio y ejecuta **`project_import`** con el ID de la carpeta del proyecto; memoria y tareas vuelven al estado compartido. El historial de comandos permanece local en `.kagmcp/projects.json` y no se exporta automáticamente por seguridad.

El formulario GitHub del panel permite cambiar o quitar el token sin modificar `config.json`; se pierde al reiniciar el proceso. La exportación no realiza `git push` automáticamente.

## 12. Verificación ejecutada de proyecto (necesaria antes de Gradio)

Una marca `tests_passed=true` escrita por un agente **ya no es suficiente** para permitir la generación de Gradio.

1. Crea el proyecto en el panel o usando `project_create`. El ID coincide con la carpeta `/kaggle/working/<project_id>`, que debe ser un repositorio Git con commit. Configura el notebook relativo al repo.
2. Corrige el código y notebook, realiza las pruebas pertinentes y haz commit. Debe quedar limpio `git status --porcelain`.
3. Invoca `project_verify` indicando `project`, `test_command` (tests reales) y `notebook_command` (ejecución real del notebook). Cada uno se ejecuta en la carpeta del proyecto con los límites de ejecución establecidos.
4. Si los dos terminan sin errores, y el repositorio Git y notebook siguen intactos, KagMCP guarda un comprobante en `.kagmcp/projects.json`: fecha, commit Git y SHA256 del notebook y de los comandos de prueba. **No guarda el contenido de los comandos de verificación** en el comprobante, pero sí quedan registrados en el historial de `exec` con filtrado preventivo de secretos.
5. Solo entonces puede ejecutarse `gradio_scaffold(project)`. Si cambió el commit o notebook, si hay archivos Git pendientes o si se importó el proyecto a un runtime nuevo, deben repetirse las pruebas.
6. Conecta `app.py` a la lógica REAL del proyecto, genera y versiona `requirements-gradio.lock` con hashes, instala desde el lock, ejecuta y prueba Gradio antes de entregar.

Ejemplo de verificación: `test_command="python -m pytest -q"`, `notebook_command="python -m jupyter nbconvert --to notebook --execute notebooks/run.ipynb --output-dir /tmp --output kagmcp-verificado.ipynb"`. Ajusta las opciones para no escribir dentro del repositorio durante las pruebas. El servidor valida éxito técnico, no garantiza que los tests elegidos cubran los casos esenciales.

El panel `/` muestra el estado de verificación y dispone de **Restaurar memoria** desde el contexto JSON que previamente exportaste y versionaste. Importar nunca supone pruebas realizadas en esta nueva sesión.

## Selección de túneles HTTP (Gradio primero, Cloudflare de respaldo)

Por defecto el notebook pasa `MCP_TUNNEL="auto"` y `MCP_GRADIO_RETRIES=3`, con este orden:

1. El binario Go consulta la API oficial `https://api.gradio.app/v3/tunnel-request` y establece una sesión **FRP nativa en Go** (protocolo 0.44 compatible con Gradio, TLS, yamux y mensajes cifrados). **No descarga ni ejecuta `frpc` ni instala Gradio Python**. Las versiones de las bibliotecas Go están fijadas en `go.mod` y `go.sum`.
2. Arranca el servidor HTTP local `127.0.0.1:8181` para ofrecer una prueba temporal `/api/health`, después inicia FRP, obtiene la URL `https://<aleatorio>.gradio.live` y verifica por HTTPS que dicha ruta responde el desafío único de **este proceso KagMCP**.
3. Si no se pudo abrir el enlace o la ruta pública no responde, cierra FRP y repite hasta tres intentos (editable 1–5). Si todos fallan, utiliza automáticamente **cloudflared**, verificando también que su URL publique el mismo proceso. En los logs se muestra qué proveedor se eligió.
4. Una vez confirmada la URL, activa OAuth y el panel web. El log anuncia `url=https://.../mcp` y `panel_web=https://.../`.

| Variable `MCP_TUNNEL` | Comportamiento |
|---|---|
| `auto` (predeterminado) | Gradio primero; Cloudflare al agotar los intentos |
| `gradio` | Solo Gradio; tras agotar intentos, devuelve error sin cambiar a Cloudflare |
| `cloudflare` | Solo Cloudflare, sin iniciar FRP |
| `none` | Sin túnel integrado; requiere proxy HTTPS propio y `MCP_PUBLIC_URL` |

En el notebook, `GRADIO_RETRIES=3` se traduce a `MCP_GRADIO_RETRIES=3`. Si necesitas otra cifra, configura entre 1 y 5. **No** utilices `gradio` si deseas que cambie de proveedor automáticamente: usa `auto`.

El túnel Gradio **no utiliza ejecutables FRP externos**: el protocolo está implementado en `internal/gradiolive/native_client.go` y los mensajes FRP compatibles se conservan con atribución Apache 2.0 en `internal/frpmsg`. Las dependencias directas están fijadas a `github.com/fatedier/golib v0.1.1-0.20220321042308-c306138b83ac` y `github.com/hashicorp/yamux v0.1.1`. El TLS verifica la CA entregada por la API oficial. Las futuras actualizaciones deben ser explícitas y probadas.

**Aclaración:** compartir el servidor Go de KagMCP a través de `gradio.live` **no crea una interfaz de Gradio en Python**. El panel sigue siendo Go, las herramientas siguen siendo MCP y OAuth/PIN siguen protegiendo las rutas administrativas. Las URL temporales pueden caducar o interrumpirse; una caída *posterior* a la conexión no cambia silenciosamente la URL OAuth del cliente ya vinculado. Si el túnel muere después del arranque, reinicia la celda y vuelve a conectar el cliente si cambió el enlace. No compartas el PIN ni los logs que lo incluyen.

### Corrección del login web con túneles Gradio/Cloudflare

El formulario del panel `https://.../` ya **no exige que la cabecera HTTP `Origin` sea exactamente igual a la URL pública**: el proxy de Gradio/Cloudflare puede omitirla o transformarla, provocando el antiguo error genérico «solicitud inválida» **antes de revisar el PIN**.

**No se deshabilitó la protección CSRF:** el servidor exige simultáneamente una cookie `__Host-kagmcp-login` privada `Secure`, `HttpOnly`, `SameSite=Strict` y el token aleatorio correspondiente en el formulario. Tras iniciar sesión, cada formulario administrativo exige la cookie de sesión privada y un token CSRF específico de esa sesión. Ningún formulario sin token válido se acepta, aunque llegue por un túnel legítimo. El control de PIN, rate limits, límites de sesiones y revocación OAuth permanece.

Si el navegador no guarda la cookie, el formulario devuelve una explicación accionable. Si el PIN es erróneo, muestra «PIN incorrecto» sin revelar datos privados. Para probar después de actualizar el servidor Go, abre la **URL raíz anunciada en el log** (`panel_web=https://...gradio.live/` o `panel_web=https://...trycloudflare.com/`) directamente en una pestaña del navegador, introduce el PIN activo y evita abrir el panel dentro de un iframe que bloquee cookies. No es necesario cambiar el PIN ni deshabilitar su seguridad.

Pruebas automatizadas de login a través de proxy: `Origin` vacío, `null`, reescrito y extraño con cookie/token correctos; se rechazan cookies ausentes y tokens falsos con HTTP 403; formularios protegidos después del login requieren su CSRF; se informa de errores de parseo sin mostrar credenciales.

## Integración ampliada de GitHub desde MCP (PAT)

Los agentes OAuth autorizados pueden acceder **directamente a la API REST y GraphQL de GitHub**, sin necesitar instalar el ejecutable `git` para esas operaciones. El cliente envía el PAT únicamente en `Authorization: Bearer ...`, fija `X-GitHub-Api-Version: 2022-11-28`, rechaza redirecciones y rutas fuera del servidor configurado de GitHub. El PAT sigue pudiéndose configurar desde el panel web `/` o en la celda privada `configurar`.

**Herramientas nuevas:**

| Grupo | Herramientas MCP |
|---|---|
| API general | `github_api` (REST con GET, POST, PUT, PATCH, DELETE), `github_graphql` (queries y mutaciones) |
| Repositorios | `github_repo_create` (obliga a declarar `private: true/false`), `github_repo_delete` |
| Ramas | `github_branch_create`, `github_branch_delete` |
| Archivos | `github_file_delete`, junto a `github_file_read` y `github_file_write` existentes |
| Pull Requests | `github_pr_create`, `github_pr_merge` |
| Issues | `github_issue_create`, `github_issue_update` |
| Releases y Actions | `github_release_create`, `github_workflow_dispatch`, `github_workflows` |

La herramienta `github_api` permite además acceder a los endpoints REST que no cuentan con una herramienta especializada, por ejemplo administrar comentarios, revisiones de PR, etiquetas, colaboradores, equipos, reglas de rama, forks, Git Trees/Commits, etiquetas, deployments, webhooks, acciones de workflow, repositorios de organización y sus configuraciones. `github_graphql` permite consultas y mutaciones GraphQL. Ninguna herramienta salta restricciones de permisos: **la operación solo puede realizarse si el PAT tiene los permisos requeridos y GitHub la permite**. Algunas rutas requieren paginación, cuerpos JSON específicos, cifrado de secretos o subida binaria fuera de la API JSON: consulta los requisitos del endpoint.

Ejemplo de herramienta genérica `github_api` para listar PR:

```json
{"method":"GET","endpoint":"/repos/owner/repo/pulls?state=open&per_page=100"}
```

Ejemplo de `github_api` para actualizar un repositorio:

```json
{"method":"PATCH","endpoint":"/repos/owner/repo","json_body":"{\"description\":\"Proyecto verificado en Kaggle\"}"}
```

Crear un repo **privado** requiere `github_repo_create` con `name` y `private=true`. Se rechaza si `private` no está especificado, evitando publicar accidentalmente un proyecto.

Eliminar repositorios, ramas, archivos, fusionar PR y cambiar permisos son acciones potencialmente irreversibles: los agentes deben **pedir autorización explícita** antes de realizarlas. No compartas PAT ni lo insertes en `endpoint` o en registros de comandos. Un PAT con poderes administrativos compartido con agentes tiene un riesgo alto: limita repositorios y permisos cuando sea posible.

**Límites actuales:** máximo de 1 MiB para solicitudes JSON `github_api` y 2 MiB para la respuesta JSON por llamada. Para repositorios completos y commits múltiples, usa los endpoints Git Trees/Git Commits con varias llamadas o Git de la shell local. Release assets binarios y ciertos endpoints de cargas multimedia necesitan lógica de subida distinta y no están cubiertos por el cliente JSON genérico.

## GPU en Kaggle mediante MCP

KagMCP incorpora cuatro herramientas de telemetría NVIDIA sin dependencias Python, sin instalar CUDA ni ejecutar comandos arbitrarios. El backend utiliza `nvidia-smi` **solo en lectura**, con argumentos fijos, una consulta por operación y un límite de 6 segundos para cada llamada al programa.

| Herramienta MCP | Uso |
|---|---|
| `gpu_detect` | Detecta todas las GPU NVIDIA visibles: índice, nombre, UUID, versión de controlador, bus PCI y VRAM total (MiB) |
| `gpu_metrics` | Consulta cada GPU: utilización de núcleos y memoria (%), VRAM total/usada/libre (MiB), VRAM usada (%), temperatura (°C), energía/potencia (W), límite de potencia, ventilador y estado de rendimiento |
| `gpu_processes` | Enumera procesos de cómputo que NVIDIA permite consultar: UUID, PID, nombre y VRAM utilizada (MiB) |
| `gpu_monitor` | Devuelve entre 1 y 10 muestras consecutivas de `gpu_metrics`, separadas entre 1 y 5 segundos, con duración nominal no superior a 20 segundos y límite de 30 segundos en total |

**Kaggle con GPU desactivada:** estas herramientas no activan la GPU automáticamente. En la configuración del notebook selecciona **Accelerator → GPU**, reinicia el entorno y vuelve a ejecutar `clonar → compilar → configurar → validación → run`. Si `nvidia-smi` falta o no puede comunicarse con el controlador, la respuesta se entrega estructurada con `available=false`, `status`, `reason` y `hint` en lugar de inventar métricas. Un estado `driver_or_query_error` no demuestra por sí solo que Kaggle haya desactivado la GPU.

Los campos no soportados por una tarjeta/controlador (`N/A`) se muestran como valores omitidos o nulos; **no se convierten falsamente en 0**. Una GPU puede existir, pero no mostrar mediciones como potencia, ventilador o utilización bajo MIG. Los porcentajes y potencias son lecturas puntuales, no un historial permanente.

Ejemplos para agentes MCP:

```json
{"name":"gpu_detect","arguments":{}}
{"name":"gpu_metrics","arguments":{}}
{"name":"gpu_processes","arguments":{}}
{"name":"gpu_monitor","arguments":{"samples":5,"interval_seconds":2}}
```

En runtimes con varios agentes, la GPU puede estar compartida: `gpu_processes` muestra los procesos de cómputo accesibles a `nvidia-smi` del entorno, **no atribuye propiedad ni permisos al agente que los creó**. Las herramientas nunca matan procesos, cambian potencia, hacen overclock o escriben archivos. No existe un monitor residente ni se crea historial permanente en disco.

Se probó localmente la lógica contra respuestas simuladas de 2 Tesla T4, ausencia de `nvidia-smi`, errores del driver, mediciones N/A, varias aplicaciones y cancelación del monitor. El consumo real de una GPU Kaggle requiere probar el notebook con Accelerator GPU activado.

## Diagnóstico: OAuth correcto pero descubrimiento de herramientas fallido

Si ChatGPT indica `Authentication succeeded, action discovery failed`, el intercambio OAuth pudo haber terminado bien mientras la solicitud posterior de `/mcp` (por ejemplo `server/discover` o `tools/list`) fue rechazada.

Se identificó en código una causa **probable**: `internal/mcp/server.go` exigía que `Origin` fuese **idéntico** a la URL pública del túnel antes de validar el Bearer token. Los clientes ChatGPT pueden enviar `Origin: https://chatgpt.com` o `https://chat.openai.com`, mientras Gradio/Cloudflare utilizan otra URL. Esto bloqueaba el descubrimiento con HTTP 403 aun teniendo una autorización OAuth válida. Los mensajes `login recibido a través de proxy con Origin diferente` son **informativos del panel** y no demuestran un fallo de PIN ni de OAuth.

**Corrección:** el endpoint `/mcp` conserva la validación obligatoria de `Origin` para prevenir DNS rebinding, pero permite únicamente (a) ausencia de cabecera, habitual en clientes no navegador; (b) el origen de la URL pública del túnel; y (c) los orígenes conocidos `https://chatgpt.com`, `https://chat.openai.com` y `https://www.chatgpt.com`. Todo origen ajeno se rechaza con HTTP 403. Todas las solicitudes MCP válidas requieren además un Bearer OAuth vigente. El dashboard conserva su cookie y CSRF independientes.

La respuesta a una versión MCP desconocida ahora incluye `error.data.supported` y `error.data.requested`, permitiendo al cliente negociar con `2026-07-28` o `2025-11-25` en vez de quedarse sin alternativa.

**Logs de diagnóstico sin secretos:** una conexión válida debe mostrar `descubrimiento MCP method=server/discover` o `method=tools/list`, seguido de `MCP respuesta de descubrimiento ... http_status=200` y `MCP herramientas disponibles count=42`. En caso de rechazo, `MCP rechazado reason=origin_not_allowed` indica un Origin distinto del permitido; `bearer_invalid` indica token inválido; `content_type` el tipo HTTP; `mcp_method_header_mismatch`, `modern_meta_missing_or_mismatch` o `protocol_version_unsupported` señalan las condiciones concretas del protocolo moderno. No se registran PIN, Bearer tokens, cookies, cuerpos de peticiones ni valores íntegros de la cabecera Origin.

**Cómo volver a probar en Kaggle:** detener la celda `run`, ejecutar `clonar → compilar → configurar → validación → run` con la URL pública actual. Si `pin = ""`, Go genera un PIN nuevo al reiniciar. En ChatGPT, utiliza la nueva URL `https://...gradio.live/mcp` o `https://...trycloudflare.com/mcp`, vuelve a conectar si cambió la URL, y autoriza con el PIN vigente. El log `panel_web` es para el navegador; `url` termina en `/mcp` y es la dirección de la conexión ChatGPT.

Se probaron los métodos de descubrimiento 2026 y 2025 a través de un reverse proxy HTTP real simulado con `Origin: https://chatgpt.com`, token OAuth conseguido mediante PKCE, esquemas de todas las herramientas y negociación de versiones. **Sin ensayo real de tu sesión Kaggle o ChatGPT** no puede afirmarse que ese haya sido el único factor de la incidencia.

Referencia de implementación: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http

## Supervisión y autorrecuperación de Gradio Live y Cloudflare

Desde la revisión de reconexión de octubre 2026, KagMCP ya no deja indefinidamente una dirección `gradio.live` publicada pero inservible. La mejora se inspiró en la implementación madura del cliente FRP nativo de `Goradio/gradio_native.go`, consultada **solo en lectura**, sin modificar Goradio.

**Qué vigila ahora KagMCP**
1. La vida del cliente nativo FRP / proceso de Cloudflare: si termina el canal de control, detecta el cierre y reconecta inmediatamente.
2. La salud de la **URL HTTPS pública**, no solo el servicio HTTP local: revisa cada **20 segundos** `/api/health` y exige recibir el nonce criptográfico de esta ejecución, sin seguir redirecciones. Tras **5 fallos consecutivos** (aproximadamente 100 segundos si no responde) declara el enlace caído, cancela el túnel e intenta otro. Los fallos intermitentes se reinician a cero tras una respuesta válida.
3. Si Gradio queda inservible, con `MCP_TUNNEL=auto` los reintentos seleccionan Gradio primero y después recurren a Cloudflare según `GRADIO_RETRIES`; `gradio` explícito no cambia de proveedor, ni `cloudflare` explícito activa Gradio. Los fallos posteriores a startup y los de arranque se reintentan con backoff **1, 2, 4, 8, 16 segundos** hasta recuperar enlace o detener `run`. No crea un proceso FRP externo: sigue siendo Go nativo.
4. Al renovar la URL, desactiva temporalmente las rutas del panel/OAuth hasta validar el túnel nuevo, reutiliza **el mismo PIN (incluso si fue cambiado en el panel)**, memoria, proyectos, tareas, skills y cliente PAT GitHub configurado. Regenera el servidor OAuth para la URL actual; los tokens de la URL antigua se invalidan en el cambio de identidad.
5. Se registran la caída, motivo FRP/health, intentos, proveedor y `url=https://.../mcp` nueva. La URL actual también queda en el archivo local privado `/kaggle/working/.kagmcp/state/tunnel-status.json`, junto a `provider`, `status` y `updated_at`. No hay PIN ni PAT ni tokens en ese archivo.

**Limitación importante de Gradio Live:** la nueva URL pública puede ser **distinta**. Ni el cliente FRP ni KagMCP pueden conservar una dirección `xxxxx.gradio.live` previamente asignada cuando el servidor externo la retira. **ChatGPT no se reconecta automáticamente a un hostname distinto.** Cuando el log diga `KagMCP restablecido con URL pública nueva; vuelve a conectar y autorizar ChatGPT`, actualiza la dirección MCP en ChatGPT y vuelve a autorizar mediante el PIN vigente. El servidor de Kaggle sigue encendido y sus proyectos no se borran. Para evitar esta interrupción de la configuración de ChatGPT, utiliza un proxy o dominio HTTPS **estable propio** con `MCP_TUNNEL=none` y `MCP_PUBLIC_URL` configurada correctamente; los enlaces gratuitos temporales no garantizan continuidad.

Un corte prolongado de Internet puede dejar el servicio MCP local funcionando sin URL pública hasta que haya conectividad; los reintentos continúan mientras la celda `run` esté activa. Este supervisor no sustituye el tiempo máximo de sesión o la suspensión del propio entorno Kaggle.

**Pruebas locales:** canal FRP cerrado, salud fallando de forma consecutiva, recuperación tras fallos intermitentes, backoff y cancelación, nueva URL OAuth, revocación de token antiguo, uso del PIN cambiado para un nuevo OAuth, conservación del PAT y de proyectos y publicación del estado privado. Aún falta una medición real continua durante horas desde una instancia Kaggle y una conexión ChatGPT.

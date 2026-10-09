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
4. **configurar:** define opciones no sensibles y pide con `getpass` el PIN OAuth y el PAT opcional; con SSH activado consulta únicamente dos contraseñas SSH de Kaggle Secrets.
5. **validación:** ejecuta `-check` con el entorno preparado **sin consultar Secrets**.
6. **run:** inicia Go en primer plano, muestra sus logs y termina al pulsar **Detener/Interruptar**.

El notebook que está abierto en Kaggle es una **copia independiente** del archivo en GitHub. Para utilizar nuevas celdas que añadimos al archivo del repositorio debes volver a importar el notebook o copiar sus celdas; correr **clonar** no lo actualiza en la interfaz.

## 2. Configurar servicios y credenciales

### Solo MCP (modo por defecto, sin Secrets)

En la celda Python:

```python
MCP_ENABLED = True
SSH_ENABLED = False
MCP_TUNNEL = "cloudflare"
MCP_LISTEN_PORT = 8181
USE_GITHUB = False
```

El notebook solicita el PIN con `getpass.getpass`, sin guardarlo en el código. Inicia HTTP en `127.0.0.1:8181`, crea el túnel Cloudflare verificado y muestra la URL `https://...trycloudflare.com/mcp`.

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

El programa emplea `/kaggle/working/.kagmcp` para estado privado y caché. Utiliza permisos privados de directorio y archivo. El archivo `config.json` se escribe automáticamente cuando arranca el CLI y solo conserva flags, puerto y URL propia. La celda Python aporta el PIN y PAT temporalmente con entrada oculta, mientras que Kaggle Secrets se consulta solo para las dos contraseñas SSH cuando corresponde. Ninguna credencial se serializa a JSON local.

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

**Actualizaciones:** `clonar` actualiza el repositorio pero no las celdas del notebook ya abierto. Reimporta el notebook cuando cambien sus celdas; ejecuta `clonar → compilar → configurar → validación → run` para la versión actual.

**SSH opcional:** la nueva celda Python establece `SSH_ENABLED=False` por defecto, incluso con Secrets SSH antiguos.


KagMCP admite tanto el protocolo MCP **2026-07-28** con `server/discover` (stateless) como los clientes que usan `initialize` y `tools/list` de 2025. Los cambios en la autenticación OAuth y los Secrets no son necesarios.

Si autorizaste la versión anterior y ChatGPT muestra ese error, **detén la celda actual**, vuelve a ejecutar las celdas del notebook **clonar → compilar → validación → run** y utiliza la URL HTTPS `.../mcp` que aparece en los nuevos logs. Repite el análisis de herramientas en ChatGPT; si el Quick Tunnel creó otra URL, actualiza/recrea esa conexión.

La salida del binario registra `descubrimiento MCP` con el nombre de los métodos `server/discover`, `initialize` o `tools/list`, sin registrar tokens ni argumentos. Si vuelve a fallar, aporta solo esos logs y el estado HTTP, nunca credenciales. La compatibilidad está verificada localmente con pruebas del protocolo y flujo real OAuth/PKCE, pero no sustituye una prueba de extremo a extremo en el runtime Kaggle.

## 9. Portal web y proyectos

En la salida de la celda `run` se mostrarán:

```text
url=https://subdominio.trycloudflare.com/mcp
panel_web=https://subdominio.trycloudflare.com/
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

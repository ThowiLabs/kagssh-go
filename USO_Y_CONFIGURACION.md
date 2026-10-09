# Uso y configuración de KagSSH Go

**Estado: 9 de octubre de 2026.** Esta guía describe el código que ya existe. La lectura de Kaggle Secrets y el túnel completo **todavía no se han probado en una notebook real**: las pruebas actuales son unitarias/simuladas y de compilación.

**Caso de uso con agentes:** el objetivo es facilitar que **ChatGPT u otro agente que sí tenga una herramienta SSH/terminal** pueda acceder al runtime de Kaggle, **reparar notebooks rotos** y **crear o depurar proyectos de machine learning**. Esta herramienta es el medio de acceso remoto, no una conexión nativa del chat de ChatGPT, ni una API de ChatGPT, ni un servidor MCP, ni una función que repare notebooks por sí sola. Hay un [notebook de demostración con pasos de arranque y cierre](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb). Las ediciones por SSH a archivos `.ipynb` tampoco sincronizan automáticamente las celdas abiertas en la interfaz de Kaggle.

## 1. Qué hace el programa

KagSSH Go reúne **servidor SSH + cliente SSH inverso + PTY + SFTP** en un único binario Go estático para Linux. No requiere Python, Go ni OpenSSH instalado en Kaggle para ejecutarse; para abrir una terminal sí necesita una shell Linux.

```text
PC ──SSH──> VPS:2223 ──forward inverso──> Kaggle:127.0.0.1:2224
              ▲
              └───── KagSSH Go conecta con el SSH del VPS en puerto 22
```

**Tres puertos diferentes:** `SSH_PORT_REMOTE=22` conecta al SSH normal del VPS; `SSH_PORT_KAGGLE=2223` publica el túnel en el VPS; `SSH_PORT_LOCAL=2224` escucha dentro de Kaggle.

**Alcance:** este proyecto todavía **no** es un servidor MCP. La propuesta Kaggle MCP queda pospuesta, sin implementación.

## 2. Antes de comenzar

Necesitas un notebook de Kaggle con Internet para contactar con el VPS; tu VPS debe tener servidor SSH en funcionamiento, autorizar TCP reverse forwarding y disponer de las credenciales y la huella del host previamente verificada.

**Forma recomendada:** abre el [notebook de instalación y conexión](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb) en Kaggle. Este clona `https://github.com/ThowiLabs/kagssh-go`, comprueba la versión requerida en `go.mod`, descarga **Go desde go.dev con verificación SHA-256** si hace falta, y compila el binario como `/kaggle/working/kagssh-linux-amd64`. Requiere Internet y no necesita adjuntar un ejecutable ni escribir Secrets en las celdas.

**Alternativa manual:** el ejecutable precompilado para notebooks Kaggle x86_64 es `dist/kagssh-linux-amd64`. Puedes adjuntarlo/subirlo al notebook y ejecutarlo desde `/kaggle/working/kagssh-linux-amd64`. Utiliza una compilación actualizada con Go 1.26.9 o posterior; los binarios anteriores no se parchean automáticamente.

Si adjuntas el binario como Kaggle Dataset/Input, copiarlo desde el Input (solo lectura) al directorio de trabajo:

```python
%%bash
set -e
cp /kaggle/input/NOMBRE_DEL_DATASET/kagssh-linux-amd64 /kaggle/working/kagssh-linux-amd64
chmod +x /kaggle/working/kagssh-linux-amd64
/kaggle/working/kagssh-linux-amd64 -version
```

Cambia `NOMBRE_DEL_DATASET` por el directorio real. Si el archivo ya está en `/kaggle/working`, omite el paso de copia.

## 3. Forma recomendada: Kaggle Secrets individuales

Abre **Add-ons → Secrets** en la notebook y crea **una etiqueta (Label) y valor (Value) por entrada**, autorizando su uso para ese notebook. El nombre de cada Secret es exactamente el nombre de la variable; **no** hay un Secret JSON ni prefijos `KAGSSH_`.

| Label / variable | Valor de ejemplo | Qué representa |
|---|---|---|
| `SSH_HOST` | `mi-vps.example.com` | Dirección del VPS |
| `SSH_USER` | `usuario_vps` | Usuario para entrar al SSH del VPS |
| `SSH_PASSWORD` | contraseña privada del VPS | Autenticación de la conexión saliente al VPS |
| `SSH_LOGIN_USER` | `usuario_kaggle` | Usuario admitido por el SSH integrado de Kaggle |
| `SSH_LOGIN_PASSWORD` | **otra** contraseña privada | Contraseña para entrar a Kaggle |
| `SSH_FINGERPRINT` | `SHA256:HUELLA_VERIFICADA` | Huella confiable de la clave SSH del VPS |
| `SSH_PORT_REMOTE` | `22` | Puerto del daemon SSH del VPS |
| `SSH_PORT_KAGGLE` | `2223` | Puerto publicado en el VPS |
| `SSH_PORT_LOCAL` | `2224` | Puerto interno de Kaggle |
| `SSH_REMOTE_BIND` | `127.0.0.1` | Puerto remoto accesible solo desde el VPS (recomendado) |

Los cuatro últimos ajustes de puertos/bind pueden omitirse cuando te sirven sus valores predeterminados. También puedes usar `SSH_KEY` (ruta a clave privada) en lugar de `SSH_PASSWORD`, o `SSH_AUTHORIZED_KEYS` (ruta a archivo de claves públicas) en lugar de `SSH_LOGIN_PASSWORD`.

Desde un acceso confiable al VPS, obtén/verifica su huella de host, por ejemplo:

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub -E sha256
```

**Celda para iniciar sin exports ni Python que lea Secrets:**

```python
%%bash
set -e
chmod +x /kaggle/working/kagssh-linux-amd64
exec /kaggle/working/kagssh-linux-amd64
```

El binario intenta obtener por HTTPS cada Secret con su etiqueta mediante el token `KAGGLE_USER_SECRETS_TOKEN` suministrado por Kaggle. La API en vivo aún necesita comprobación. La celda permanecerá ocupada mientras se mantenga conectado el túnel; detener la celda o terminar Kaggle corta el proceso.

## 4. Alternativa: todas las variables en la celda

**Solo para pruebas en notebooks privados**, evitando guardar contraseñas en código compartido.

```python
%%bash
set -e

# Credenciales para la conexión saliente al VPS
export SSH_HOST="mi-vps.example.com"
export SSH_USER="usuario_vps"
export SSH_PASSWORD="PASSWORD_PRIVADA_VPS"
export SSH_FINGERPRINT="SHA256:HUELLA_VERIFICADA"
export SSH_PORT_REMOTE="22"

# Puerto de retorno en el VPS
export SSH_PORT_KAGGLE="2223"
export SSH_REMOTE_BIND="127.0.0.1"

# Servidor SSH embebido de Kaggle
export SSH_PORT_LOCAL="2224"
export SSH_LOGIN_USER="usuario_kaggle"
export SSH_LOGIN_PASSWORD="PASSWORD_PRIVADA_DIFERENTE"

chmod +x /kaggle/working/kagssh-linux-amd64
exec /kaggle/working/kagssh-linux-amd64
```

No incluyas valores reales en un repositorio o notebook público. El programa **no carga automáticamente** el archivo `.env.example`: es solo una plantilla de referencia.

## 5. Alternativa mixta: Secrets + exports

Puedes guardar todas las contraseñas en Kaggle Secrets y cambiar solo una configuración desde la celda:

```python
%%bash
set -e
export SSH_PORT_KAGGLE="3333"
export SSH_REMOTE_BIND="127.0.0.1"
chmod +x /kaggle/working/kagssh-linux-amd64
exec /kaggle/working/kagssh-linux-amd64
```

En este caso, el puerto solicitado al VPS será **3333**, aunque el Secret `SSH_PORT_KAGGLE` contenga `2223`.

**Prioridad real:** variable exportada explícitamente → Secret de Kaggle con **la misma etiqueta** → valor predeterminado.

Si no hay token de Secrets se usan exports/defaults. Si el servicio de Secrets devuelve un error distinto de etiqueta ausente (p. ej., error de red, autorización o cuota), el arranque puede fallar mostrando la etiqueta afectada sin revelar su contenido. El código no vuelve a pedir Secrets durante una reconexión del túnel.

## 6. Catálogo completo de configuración

| Variable | Valor predeterminado | Significado |
|---|---|---|
| `SSH_HOST` | `fp.thowilabs.com` | Host del VPS; puedes sobreescribirlo |
| `SSH_USER` | `root` | Usuario SSH de salida al VPS |
| `SSH_PASSWORD` | vacío | Password del VPS |
| `SSH_KEY` | vacío | **Ruta a archivo** de clave privada para acceder al VPS |
| `SSH_FINGERPRINT` | vacío | Huella SHA256 confiable del SSH del VPS |
| `SSH_KNOWN_HOSTS` | HOME + `/.ssh/known_hosts` | Archivo alternativo de claves SSH confiables |
| `SSH_PORT_REMOTE` | `22` | Puerto SSH real del VPS |
| `SSH_PORT_KAGGLE` | `2223` | Puerto publicado por el túnel en VPS |
| `SSH_PORT_LOCAL` | `2224` | Puerto del SSH local en Kaggle |
| `SSH_REMOTE_BIND` | `127.0.0.1` | IP de escucha del VPS; `0.0.0.0` permite acceso público si el VPS lo acepta |
| `SSH_LOGIN_USER` | usuario Linux del proceso | Identidad admitida en el SSH integrado |
| `SSH_LOGIN_PASSWORD` | vacío | Password para entrar a Kaggle |
| `SSH_AUTHORIZED_KEYS` | vacío | **Ruta a archivo** con claves públicas para acceder a Kaggle |
| `SSH_HOST_KEY` | HOME + `/.config/kagssh/host_ed25519` | Ruta a clave Ed25519 del servidor integrado, que se crea si falta |

Se exige alguna autenticación para **ambos lados**: `SSH_PASSWORD` o `SSH_KEY` hacia el VPS; `SSH_LOGIN_PASSWORD` o `SSH_AUTHORIZED_KEYS` para entrar a Kaggle. Para verificar el VPS usa `SSH_FINGERPRINT` o un archivo de `SSH_KNOWN_HOSTS` existente y válido.

**Los valores de SSH_KEY y SSH_AUTHORIZED_KEYS son rutas de archivos**, no texto PEM ni texto de claves. La versión actual no convierte automáticamente el contenido de un Secret en un archivo de claves. El nombre `SSH_LOGIN_USER` es una identidad SSH, **no cambia al usuario Unix**: la shell remota utiliza los permisos del proceso KagSSH Go.

## 7. Acceso desde tu computadora

### Forma segura: puerto remoto privado

Con `SSH_REMOTE_BIND=127.0.0.1`, el puerto 2223 solo escucha en el VPS. Abre una terminal de tu computadora (no en Kaggle):

```bash
ssh -N -L 2223:127.0.0.1:2223 -p 22 usuario_vps@mi-vps.example.com
```

Mantén esa terminal abierta. En **una segunda terminal** de tu computadora:

```bash
ssh -p 2223 usuario_kaggle@127.0.0.1
sftp -P 2223 usuario_kaggle@127.0.0.1
```

Usa `SSH_LOGIN_USER` para el segundo comando. Si cambiaste `SSH_PORT_KAGGLE` o `SSH_PORT_REMOTE`, adapta los puertos. Si el puerto local 2223 de tu PC está ocupado, elige otro solo para el lado izquierdo de `-L` y úsalo en el segundo comando.

### Acceso público directo (opcional, no recomendado de entrada)

Si deseas llegar a Kaggle directamente a través del VPS:

- Define `SSH_REMOTE_BIND=0.0.0.0`.
- Habilita en el servidor SSH del VPS `AllowTcpForwarding yes` y `GatewayPorts clientspecified`, según las reglas de su instalación.
- Abre el puerto publicado, por ejemplo TCP 2223, en el firewall del VPS.

Después, desde tu computadora:

```bash
ssh -p 2223 usuario_kaggle@mi-vps.example.com
sftp -P 2223 usuario_kaggle@mi-vps.example.com
```

**Seguridad:** publicar el puerto expone la autenticación SSH integrada de Kaggle a Internet. Mantén acceso privado siempre que puedas; usa contraseñas diferentes, preferentemente claves, y restringe el firewall.

## 8. Validación y solución de problemas

Antes de abrir el túnel puedes validar la carga y coherencia de la configuración:

```python
%%bash
set -e
chmod +x /kaggle/working/kagssh-linux-amd64
/kaggle/working/kagssh-linux-amd64 -check
```

Resultado esperado: **configuración válida**. Esto **no** prueba que el SSH de VPS ni el túnel funcionen; tampoco equivale a una prueba completa de Secrets reales.

| Síntoma | Revisión |
|---|---|
| Binario sin permisos | Copiar a `/kaggle/working` y aplicar `chmod +x` |
| `Exec format error` | Seleccionar binario Linux AMD64 en Kaggle x86_64 |
| Error de Kaggle Secrets | Label exacto, autorización del notebook, Internet, token, cuota |
| Password obligatoria ausente | Crear el Secret individual o definir export |
| Huella SSH del VPS no coincide | Comprobar huella real desde canal seguro; no desactivar validación |
| Error al publicar puerto remoto | VPS con SSH, Forwarding habilitado, puerto libre y permisos |
| Llega desde VPS pero no desde PC | Con bind loopback, usar el túnel local de la sección 7 |
| Bind público no recibe conexiones | GatewayPorts, firewall, IP/dominio y puerto |
| Sin shell/PTY | Shell Linux instalada y permisos del usuario que ejecutó KagSSH |
| Desconexión al detener la notebook | Esperado: KagSSH necesita sesión activa |

El programa reintenta el túnel ante desconexiones, pero **no** puede mantener en ejecución una sesión que Kaggle haya terminado.

## 9. Compilar y probar desde Windows

Directorio del repositorio: `C:\Users\Admin\Documents\GitHub\kagssh-go`.

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/kagssh-linux-amd64 ./cmd/kagssh

# Volver al destino nativo Windows antes de correr los tests
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
go test ./...
```

Para el build alternativo Linux ARM64, cambia `GOARCH` a `arm64` y salida a `dist/kagssh-linux-arm64`. Para pruebas Windows habituales, quita las variables de cross-build antes de ejecutar `go test ./...`.

## 10. Seguridad y pendientes

- No compartas passwords, tokens o claves privadas dentro del repositorio, logs o notebook público; rota claves/contraseñas que se hayan expuesto previamente.
- La huella del VPS se verifica: no hay modo previsto para saltarla.
- SSH de Kaggle escucha solo en loopback; el puerto de salida remoto también es privado por defecto.
- Se filtran variables `SSH_*` y `KAGGLE_*` del entorno de las shells lanzadas por SSH integrado, pero no se pueden deshacer secretos ya copiados a notebooks.
- La clave de host de Kaggle persiste solamente si su ruta de almacenamiento sobrevive la sesión; si se recrea, la huella SSH del entorno podría cambiar.
- Pruebas existentes: `go test ./...` (config/túnel) pasado en Windows, `go vet` cruzado a Linux pasado, builds Linux AMD64/ARM64 estáticos completados, pruebas SSH Linux compiladas pero no ejecutadas.
- **Siguiente paso real:** correr en Kaggle con Secrets autorizados, probar SSH/PTY/SFTP hacia un VPS propio y verificar reconexión y STOP. No se declara probado hasta hacerlo.

Consulta también [README.md](README.md), [contexto/03-secrets-individuales-kaggle.md](contexto/03-secrets-individuales-kaggle.md) y [tareas/pendiente-03-prueba-integracion-kaggle-vps.md](tareas/pendiente-03-prueba-integracion-kaggle-vps.md).

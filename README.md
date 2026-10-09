# KagMCP — MCP no oficial para Kaggle

**KagMCP** es una implementación **independiente y no oficial** de Model Context Protocol (MCP) para conectar ChatGPT y otros agentes compatibles a una sesión autorizada de Kaggle. Está pensado para **investigación, análisis de datos, preparación de datasets, creación y reparación de notebooks, desarrollo, entrenamiento y depuración de proyectos de machine learning**.

No está afiliado, respaldado ni patrocinado por Kaggle, Google u OpenAI. El usuario es responsable de los permisos, cuotas y políticas del runtime.

> **Estado:** base de MCP/OAuth, Cloudflare Quick Tunnel, herramientas de Kaggle, GitHub PAT y SSH opcional implementada y cubierta por pruebas locales. La compatibilidad extremo a extremo con un runtime Kaggle, un túnel público y un cliente ChatGPT real **todavía requiere validación**.

## Inicio en Kaggle

**Notebook:** [Instalar y ejecutar KagMCP](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb).

1. En Kaggle habilita Internet y ve a **Add-ons → Secrets**.
2. Crea y autoriza los Secrets `MCP_ENABLED=true` y `MCP_ACCESS_PIN=<PIN largo y secreto>`. Utiliza un PIN aleatorio de al menos 12 caracteres.
3. Si quieres GitHub, añade **`GITHUB_TOKEN` con tu Personal Access Token** (preferentemente fine-grained, limitado a los repositorios necesarios). **No se crea ni instala ninguna GitHub App**.
4. Ejecuta en orden las celdas del notebook: clonar el repositorio, disponer de Go, compilar, ejecutar `-check` e iniciar KagMCP.
5. La última celda **permanece en ejecución** y muestra todos los logs; localiza la URL `https://....trycloudflare.com/mcp`.
6. Configura esa URL como servidor remoto MCP en un cliente compatible (incluido ChatGPT, cuando la conexión MCP esté disponible) y aprueba el flujo OAuth con el `MCP_ACCESS_PIN`.
7. Para desconectar, pulsa **Detener/Interruptar** en esa misma celda; finalizan Go, MCP, el túnel y SSH opcional.

**No necesitas VPS para usar MCP.** La conexión SSH/SFTP por VPS se mantiene como opción independiente.

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

## Configuración: Secrets individuales o entorno

Las variables y los nombres de Kaggle Secrets son **idénticos**. Prioridad: **variable exportada explícitamente > Secret individual > opción no sensible de `.kagmcp/config.json` > valor predeterminado**. No se necesita exportar en Python ni instalar un servidor SSH externo dentro de Kaggle.

| Etiqueta | Predeterminado | Descripción |
|---|---|---|
| `MCP_ENABLED` | `false` | Activa servidor MCP |
| `MCP_ACCESS_PIN` | obligatorio si MCP | PIN de autorización OAuth; 12–128 caracteres |
| `MCP_TUNNEL` | `cloudflare` | `cloudflare` crea URL HTTPS temporal; `none` usa proxy HTTPS externo |
| `MCP_PUBLIC_URL` | vacío | URL pública https obligatoria cuando `MCP_TUNNEL=none` |
| `MCP_LISTEN_PORT` | `8181` | Puerto HTTP interno loopback |
| `GITHUB_TOKEN` | vacío | Token personal de GitHub; **sin App** |
| `SSH_ENABLED` | automático | Con MCP y sin SSH_HOST: falso. Con SSH_HOST configurado: verdadero. Sin MCP: verdadero |
| `SSH_HOST` | sin valor | Host del VPS cuando se habilita SSH |
| `SSH_USER` | `root` | Usuario SSH del VPS |
| `SSH_PASSWORD` / `SSH_KEY` | vacío | Contraseña o ruta a clave privada para VPS |
| `SSH_LOGIN_PASSWORD` / `SSH_AUTHORIZED_KEYS` | vacío | Autenticación SSH entrante en Kaggle |
| `SSH_PORT_REMOTE` | `22` | Puerto del SSH del VPS |
| `SSH_PORT_KAGGLE` | `2223` | Puerto externo solicitado en el VPS |
| `SSH_PORT_LOCAL` | `2224` | Puerto SSH local |
| `SSH_FINGERPRINT` | vacío | Huella verificada del VPS; sin ella TOFU con known_hosts |
| `SSH_KNOWN_HOSTS` | `/kaggle/working/.kagmcp/ssh/known_hosts` | Claves conocidas del VPS |
| `SSH_LOGIN_USER` | usuario del proceso | Identidad de login SSH en Kaggle |
| `SSH_HOST_KEY` | `/kaggle/working/.kagmcp/ssh/host_ed25519` | Identidad de host del SSH integrado |

**Carpeta interna de KagMCP:** `/kaggle/working/.kagmcp/` guarda `config.json` con preferencias no sensibles, el estado privado de OAuth y la caché de túneles. **No incluye `GITHUB_TOKEN`, el PIN ni otros Secrets en archivos de configuración.** El proveedor Kaggle no garantiza que `/kaggle/working` persista al cambiar o destruir el runtime; vuelve a autorizar clientes si cambia la URL o se pierde el estado.

### Conectar GitHub con token personal

Crea un **fine-grained Personal Access Token** de GitHub con acceso a los repositorios deseados y permisos de `Contents: read` o `Contents: read and write`, dependiendo de las operaciones. Introduce el valor en un Secret llamado exactamente **`GITHUB_TOKEN`**. KagMCP lo envía por HTTPS únicamente en la cabecera `Authorization` al servicio GitHub API; el token no se pasa al cliente MCP como resultado.

No existe GitHub App, manifiesto de App, callback de instalación ni autenticación GitHub OAuth. El OAuth de KagMCP **protege a ChatGPT** y no tiene relación con el acceso al API de GitHub.

Herramientas GitHub iniciales: `github_status`, `github_repositories`, `github_branches`, `github_file_read` y `github_file_write`. La última crea un commit mediante GitHub Contents API y necesita el SHA previo para actualizar un archivo existente.

### Herramientas MCP

`environment_info`, `list_dir`, `read_file`, `write_file`, `exec` y herramientas GitHub anteriores. Las herramientas de archivos permiten lectura dentro de `/kaggle/working` y `/kaggle/input`, escritura solo en `/kaggle/working`, y bloquean el directorio privado `.kagmcp`.

El comando `exec` admite procesos del runtime Kaggle con timeout de 45 segundos y salida acotada. Los programas lanzados no heredan las variables `KAGGLE_*`, `MCP_*`, `SSH_*` ni `GITHUB_*`. **Advertencia:** el acceso autorizado a terminal implica confiar en ese cliente, porque el proceso tiene los permisos del runtime.

### SSH/SFTP por VPS (opcional)

Para conservar el comportamiento previo define `SSH_ENABLED=true` y los Secrets SSH correspondientes. El túnel reverso publica desde tu VPS el puerto solicitado `0.0.0.0:2223` (o el puerto personalizado), siempre que OpenSSH tenga:

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

Compila un único ejecutable; cuando se usa Cloudflare, KagMCP descarga y verifica la utilidad oficial `cloudflared` como proceso auxiliar de transporte. No necesita Python para el servidor: Python solo prepara/ejecuta el binario desde el notebook.

## Seguridad y límites

- OAuth con PIN, PKCE, metadata de descubrimiento, protección de autorización y límites de solicitudes, heredados y adaptados de [Lilith-MCP](https://github.com/YahirHub/Lilith-MCP).
- No publiques PIN, claves privadas ni tokens en celdas públicas, commits, URLs o logs.
- Cloudflare Quick Tunnel publica una URL **temporal** que puede cambiar en cada sesión; `MCP_TUNNEL=none` requiere un proxy HTTPS propio y `MCP_PUBLIC_URL`.
- **localhost.run y otros transportes aún no están implementados**; se contemplan para extensiones futuras tras verificar el flujo público y la identidad del servidor SSH.
- La integración real con ChatGPT y un runtime Kaggle activo permanece pendiente de prueba extremo a extremo.
- Los permisos de las herramientas MCP son los del proceso KagMCP: evita autorizar clientes no confiables.

## Licencias y procedencia

El código de servidor OAuth, almacenamiento privado y Cloudflare Quick Tunnel procede de la variante `portable-shell` de **Lilith-MCP**, adaptada a este módulo y al flujo de Kaggle. KagMCP mantiene su propia interfaz de herramientas, sistema de configuración y cliente personal de GitHub.

# KagSSH Go

**Notebook de ejemplo:** [Conectar Kaggle con ChatGPT u otros agentes mediante SSH](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb).

**¿Para qué está pensado?** Permite que un agente de IA, como ChatGPT **cuando dispone de una herramienta SSH/terminal autorizada**, acceda a un entorno de Kaggle para **diagnosticar y corregir cuadernos dañados**, depurar código y desarrollar proyectos de **machine learning y aprendizaje**. KagSSH Go proporciona el acceso SSH/SFTP al runtime mediante un VPS; no automatiza por sí mismo la reparación ni el entrenamiento.

**Para evitar confusiones:** **no** conecta automáticamente el chat de ChatGPT a Kaggle, **no** es una integración oficial, **no** es una API de ChatGPT y **no** implementa un servidor MCP. Hace falta un VPS con SSH y un agente/cliente capaz de usar SSH de forma autorizada. La ejecución real con Kaggle y un VPS aún está pendiente de pruebas de integración.

**Guía completa de instalación, Secrets y ejecución en Kaggle:** [USO_Y_CONFIGURACION.md](USO_Y_CONFIGURACION.md).

Un solo binario Linux estático, sin dependencias de Python, OpenSSH Server ni cliente SSH en **Kaggle**. Incluye servidor SSH con shell/PTY/SFTP y cliente SSH para publicar un túnel inverso en tu VPS.

## Inicio recomendado: notebook que clona el repositorio y compila en Kaggle

Abre [`notebooks/kagssh_kaggle_chatgpt_agentes.ipynb`](notebooks/kagssh_kaggle_chatgpt_agentes.ipynb) en Kaggle con acceso a Internet. Ejecuta sus celdas en orden: **clona** `https://github.com/ThowiLabs/kagssh-go`, utiliza el Go instalado si cumple la versión mínima de `go.mod` o **descarga y verifica** el Go oficial necesario, compila el binario Linux y luego valida configuración/inicia el túnel. **No hace falta adjuntar un binario previamente compilado.**

Puedes configurar los parámetros como **Kaggle Secrets individuales** (Add-ons → Secrets) o **variables de entorno `SSH_*`**; usa Secrets para contraseñas. Las etiquetas, alternativas, puertos y prioridades se detallan abajo y en la guía.

## Arranque sin exports desde Kaggle Secrets

1. En tu notebook, abre **Add-ons → Secrets**.
2. Crea Secrets separados, cada uno con etiqueta y valor, y autorízalos para ese notebook.
3. Ejecuta el binario. Este consulta las etiquetas directamente en Kaggle por HTTPS, usando el token entregado automáticamente al notebook. No necesita `kaggle_secrets` ni un JSON tuyo.

Configura al menos:

| Secret / variable | Ejemplo | Significado |
|---|---|---|
| `SSH_HOST` | `fp.thowilabs.com` | Host del VPS |
| `SSH_USER` | `root` | Usuario SSH del VPS |
| `SSH_PASSWORD` | (valor privado) | Contraseña SSH del VPS; alternativa: `SSH_KEY` |
| `SSH_LOGIN_PASSWORD` | (valor privado diferente) | Contraseña para conectarse al SSH incorporado en Kaggle; alternativa: `SSH_AUTHORIZED_KEYS` |
| `SSH_FINGERPRINT` | `SHA256:...` | **Opcional pero más seguro:** huella verificada del VPS. Sin ella, KagSSH registra automáticamente la primera clave recibida en `known_hosts` |

Puedes añadir estas etiquetas para cambiar los puertos y comportamiento:

| Secret / export | Default | Significado |
|---|---|---|
| `SSH_PORT_REMOTE` | `22` | **Puerto del SSH real del VPS** para establecer el túnel saliente |
| `SSH_PORT_KAGGLE` | `2223` | **Puerto publicado en el VPS** para entrar a Kaggle |
| `SSH_PORT_LOCAL` | `2224` | Puerto interno donde escucha el SSH embebido en Kaggle |
| `SSH_REMOTE_BIND` | `127.0.0.1` | IP de escucha en VPS; `0.0.0.0` habilita acceso público |
| `SSH_LOGIN_USER` | usuario Linux del proceso | Usuario admitido por el SSH de Kaggle |
| `SSH_KEY` | vacío | Ruta a clave privada SSH del VPS, si no usas contraseña |
| `SSH_KNOWN_HOSTS` | `~/.ssh/known_hosts` | Archivo alternativo de claves confiables del VPS |
| `SSH_AUTHORIZED_KEYS` | vacío | Ruta a las claves públicas permitidas en el servidor integrado |
| `SSH_HOST_KEY` | `~/.config/kagssh/host_ed25519` | Ruta persistente de la identidad Ed25519 del servidor de Kaggle |

**Estas etiquetas son idénticas a las variables `export`**. No hay prefijo propio, Secret JSON, servicio auxiliar ni archivo obligatorio de configuración. **No necesitas habilitar ningún modo adicional ni crear `known_hosts` a mano.** Si no defines `SSH_FINGERPRINT`, KagSSH crea el archivo y guarda automáticamente la primera clave pública que entregue el VPS, sin depender de OpenSSH dentro de Kaggle. Comprueba la huella registrada por otro canal seguro: la primera conexión no puede garantizar la identidad del VPS frente a un posible intermediario. Una vez registrada, las conexiones posteriores rechazan cambios de clave.

**Prioridad:** variable de entorno declarada explícitamente → Kaggle Secret con la misma etiqueta → valor predeterminado. Si se encuentra un token Kaggle pero el servicio falla por autenticación/red/rate limit, el arranque falla explicando qué etiqueta falló; si no existe token Kaggle, funciona solo con `export`. Si falta una contraseña o clave obligatoria se muestra un error sin revelar valores.

### Ejecutar en Kaggle sin configurar variables en la celda

Tras compilar con el notebook recomendado (o subir manualmente un binario actualizado) en `/kaggle/working/`:

```bash
!chmod +x /kaggle/working/kagssh-linux-amd64
!/kaggle/working/kagssh-linux-amd64
```

O ejecuta el mismo binario desde una celda `%%bash`. Permanecerá en primer plano mostrando logs. STOP detiene el proceso y cierra el túnel. El binario es Go puro; el notebook usa su interfaz habitual para lanzar ejecutables.

### Ejecutar en otro servidor con export

```bash
export SSH_HOST=mi-vps.example
export SSH_USER=root
export SSH_PORT_REMOTE=22
export SSH_PORT_KAGGLE=2223
export SSH_FINGERPRINT='SHA256:HUELLA_VERIFICADA'
export SSH_PASSWORD='...'
export SSH_LOGIN_PASSWORD='...'
./kagssh-linux-amd64
```

No publiques exportaciones con contraseñas en notebooks o logs compartidos. Evita incluir secretos en historial de terminal.

## Topología

```text
PC --ssh--> VPS:2223 --reverse forward--> Kaggle:127.0.0.1:2224
               ^
               | SSH saliente desde Kaggle al VPS:22
               |
        KagSSH Go (cliente integrado)
```

Este proyecto elimina OpenSSH **en Kaggle**. El VPS sigue necesitando un servidor SSH escuchando en `SSH_PORT_REMOTE` (normalmente 22).

Por defecto, el puerto remoto 2223 solo acepta conexiones **desde el mismo VPS** porque `SSH_REMOTE_BIND=127.0.0.1`. Para entrar desde Internet, debes guardar `SSH_REMOTE_BIND=0.0.0.0` como Secret; además, el VPS requiere:

```text
AllowTcpForwarding yes
GatewayPorts clientspecified
```

y permitir TCP/2223 en el firewall. Exponer SSH a Internet requiere autenticación fuerte; recomendamos claves autorizadas y protección adicional.

Para dejarlo privado, realiza desde tu PC un túnel hasta el VPS y después accede a Kaggle:

```bash
ssh -N -L 2223:127.0.0.1:2223 root@fp.thowilabs.com
ssh -p 2223 root@127.0.0.1
```

Para acceso público (si lo habilitas):

```bash
ssh -p 2223 root@fp.thowilabs.com
sftp -P 2223 root@fp.thowilabs.com
```

Sustituye `root` por `SSH_LOGIN_USER` si usas otro usuario en Kaggle.

## Instalación/compilación

Desde el proyecto, con Go 1.26.9 o superior:

```powershell
$env:CGO_ENABLED="0"
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/kagssh-linux-amd64 ./cmd/kagssh
```

Para ARM64, cambia `GOARCH` y el destino. El ejecutable no requiere librerías compartidas de OpenSSH ni CGO. Sí requiere Linux con `/bin/sh` o `/bin/bash` y permisos suficientes para ejecutar la shell deseada.

Comprobaciones:

```bash
go test ./...
go vet ./...
govulncheck ./...
```

GitHub Actions realiza pruebas Linux de SSH, PTY y SFTP usando localhost. Las pruebas de conexión real al VPS requieren un entorno autorizado.

## Seguridad y límites

- Las variables `SSH_*` y los tokens `KAGGLE_*` no se transmiten al entorno de las shells iniciadas por el servidor integrado.
- La clave de host Ed25519 se crea con permisos 0600 y se reutiliza.
- El cliente verifica la huella explícita, si existe. Sin ella, registra la primera clave del VPS automáticamente (**TOFU**), advierte que aún no ha sido verificada externamente y rechaza cambios posteriores. Para máxima protección contra un ataque en el primer contacto, configura `SSH_FINGERPRINT` verificada.
- No modifica la contraseña del usuario root del sistema.
- El usuario indicado por `SSH_LOGIN_USER` es una identidad SSH: la shell hereda los permisos del **proceso**, no cambia de cuenta Linux automáticamente. Evita ejecutarlo como root salvo necesidad.
- Límites de conexiones y reintentos de autenticación.
- No implementa X11, agent forwarding ni port forwarding arbitrario hacia otros destinos.
- Cada arranque hace una consulta por cada etiqueta que no esté exportada, con timeout. Kaggle puede aplicar límites de API. No vuelve a consultar Secrets durante la reconexión.
- El servidor SSH embebido escucha únicamente en 127.0.0.1. Para almacenamiento persistente de host keys usa `SSH_HOST_KEY`.

### Verificar la huella confiable del VPS

Desde la consola confiable del VPS:

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub -E sha256
```

Asegúrate de que corresponda a la clave que el VPS realmente presenta en su SSH. Configura esa huella como `SSH_FINGERPRINT` en Kaggle Secrets.

**Importante:** rota las contraseñas que se hayan pegado previamente en texto plano. El repositorio no debe guardar claves reales.

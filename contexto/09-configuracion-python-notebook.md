# Contexto 09 — Configuración Python del notebook, Secrets solo para SSH

Fecha: 2026-10-09
Repositorio y **único workspace modificado**: `C:\Users\Admin\Documents\GitHub\kagssh-go`.
Runtime Kaggle: `/kaggle/working`; clone `/kaggle/working/kagssh-go`.
No se altera la carpeta `C:\Users\Admin\Documents\GitHub\Lilith-MCP` ni proyectos de otros agentes.

## Motivación

Al iniciar o validar, la versión previa de Go consultaba 20+ claves de Kaggle Secrets. Se observó `HTTP 429` al buscar tanto `SSH_HOST_KEY` como `MCP_ACCESS_PIN`. El usuario pidió cambiar el enfoque: definir la mayoría de parámetros en Python dentro del notebook y usar Kaggle Secrets solamente para las contraseñas SSH/VPS.

## Diseño implementado

El notebook tiene la celda nueva **configurar**, anterior a validación, con:

- `MCP_ENABLED=True`, `SSH_ENABLED=False`, `MCP_TUNNEL=cloudflare`, `MCP_LISTEN_PORT=8181` como valores normales de Python.
- `MCP_PUBLIC_URL` opcional y parámetros VPS `SSH_HOST`, `SSH_USER`, puertos, `SSH_LOGIN_USER`, `SSH_FINGERPRINT` editables en Python, no Secrets.
- Solicitud privada `getpass.getpass` del `MCP_ACCESS_PIN` (12–128 caracteres), en memoria. `USE_GITHUB=True` pide token personal PAT por getpass, no Secret.
- Solamente cuando `SSH_ENABLED=True`, se consultan **una vez** `SSH_PASSWORD` y `SSH_LOGIN_PASSWORD` mediante `kaggle_secrets.UserSecretsClient`; si SSH está apagado, no se importa siquiera el cliente de Secrets.
- `RUNTIME_ENV` es una copia del entorno Kaggle, saneada de `MCP_*`, `SSH_*`, `GITHUB_*`, `KAGGLE_USER_SECRETS_TOKEN` y `KAGGLE_IAP_TOKEN`. Se introducen solo los valores explícitos elegidos.
- Las celdas `validacion` y `run` pasan **la misma copia** mediante `env=RUNTIME_ENV`, evitando que Go descubra el token de Secrets y vuelva a consultar etiquetas.
- `SSH_ENABLED=False` explícito previene reactivar un túnel SSH previo; `MCP_PUBLIC_URL=""` explícito invalida preferencias URL anteriores.
- El PIN y el PAT no se persisten en Git/notebook ni config.json. El entorno en memoria y procesos del mismo runtime no constituyen aislamiento frente a un adversario con acceso root.
- Reutiliza exactamente el protocolo/OAuth Go y código de carga legacy; ningún cambio del módulo Lilith-MCP.

## Verificaciones

`scripts/check_notebook.py` verifica sintaxis de todas las celdas, simulación Python sin Secrets en MCP-only, consulta de exactamente dos claves en SSH, PIN/PAT distintos por getpass, limpieza de entorno, igualdad de env en `-check` y `run`, y comportamiento Interruptar. Comprobar también Go test, race, vet y compilación Linux; no se realiza acceso real a Secrets de Kaggle durante pruebas.

## Actualización en Kaggle

**Importante:** ejecutar celda `clonar` descarga cambios Go, pero NO edita el notebook ya abierto. Para tener la celda `configurar`, volver a importar en Kaggle el notebook actualizado o copiar manualmente la celda. Orden: clonar → instalar-go (si hace falta) → compilar → configurar → validación → run. El PIN se introducirá en cada kernel nuevo, no en cada llamada a la API.

## Limitaciones

Si SSH opcional está habilitado y Kaggle Secrets devuelve 429, la celda configurar informa del fallo sin reintentar masivamente. Si SSH está deshabilitado, este recorrido elimina por completo las consultas a Secrets por parte del proceso Go del notebook. Prueba Kaggle/ChatGPT real aún necesaria.

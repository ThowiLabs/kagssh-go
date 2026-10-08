# Objetivo
KagSSH Go lee **Secrets independientes** con exactamente los mismos nombres SSH_* que las variables exportadas; sin prefijos propios y sin un Secret JSON.

# Etiquetas prioritarias
SSH_HOST, SSH_USER, SSH_PASSWORD, SSH_LOGIN_PASSWORD, SSH_FINGERPRINT, SSH_PORT_REMOTE y SSH_PORT_KAGGLE.

# Semántica
SSH_PORT_REMOTE = puerto SSH real del VPS (por defecto 22).
SSH_PORT_KAGGLE = puerto publicado por el reverse forward en el VPS (por defecto 2223).
SSH_PORT_LOCAL = puerto SSH interno del binario (por defecto 2224).

# Flujo
export > consulta directa a Secrets por etiqueta > valor predeterminado.
Sin token Kaggle, funciona con export.
HTTP estándar Go; sin Python ni otro binario ni un archivo de configuración.

# Seguridad y pruebas
Validar HTTP errors, secretos inexistentes, prioridad, puertos, no filtrar credenciales.
Completar builds Linux AMD64/ARM64, go test, go vet y govulncheck antes de marcar completada.

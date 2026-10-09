# Fecha
2026-10-08

# Objetivo
Incorporar lectura nativa de Kaggle Secrets individuales y usar exactamente los mismos nombres SSH_* en Secrets y en exports, sin fichero JSON ni prefijo propio.

# Decisiones tomadas
- SSH_PORT_REMOTE = puerto del daemon SSH real del VPS (por defecto 22).
- SSH_PORT_KAGGLE = puerto publicado por el reverse SSH en el VPS (por defecto 2223).
- SSH_PORT_LOCAL = puerto servidor SSH interno de Kaggle (por defecto 2224).
- Resto: SSH_HOST, SSH_USER, SSH_PASSWORD, SSH_LOGIN_PASSWORD, SSH_FINGERPRINT, SSH_KEY, SSH_KNOWN_HOSTS, SSH_LOGIN_USER, SSH_AUTHORIZED_KEYS, SSH_HOST_KEY.
- Prioridad: export explícito > Secret por etiqueta > default seguro. Sin token Kaggle, solo export/default.
- Leer por HTTPS POST /requests/GetUserSecretByLabelRequest siguiendo el cliente oficial de Kaggle. Se usa token de entorno KAGGLE_USER_SECRETS_TOKEN y, si está presente, KAGGLE_IAP_TOKEN, asignados por Kaggle, no por el usuario.
- No añadir dependencias nuevas: solo net/http, encoding/json, context y paquete estándar.
- No exponer SSH_* o KAGGLE_* en el ambiente de las shells SSH hijas.
- No reconsultar Secrets ante reconexiones del túnel.
- En el servicio de Secrets, error de etiqueta inexistente (code 5) se interpreta como ausente; otro error falla el arranque.
- El protocolo HTTPS usa JSON, pero NO hay documento JSON de configuración ni Secret único.

# Arquitectura actual
cmd/kagssh/main.go pasa context.Context al cargador.
internal/config/config.go resuelve variables/Secrets/valores por defecto.
internal/config/kaggle.go integra cliente HTTPS Kaggle.
internal/sshserver/server.go limpia tokens/credenciales antes de lanzar shell.
internal/tunnel/tunnel.go sigue creando reverse port forwarding.

# Librerías usadas
Go 1.26.6 + x/crypto/ssh, creack/pty, pkg/sftp. Sin dependencias nuevas.

# Archivos importantes modificados
internal/config/config.go, kaggle.go, config_test.go, kaggle_test.go, cmd/kagssh/main.go, internal/sshserver/server.go y server_test.go, README.md, .env.example, tareas y contexto.

# Problemas encontrados
La API documentada de Kaggle es por etiqueta y aplica rate limits (HTTP 429).
La máquina desarrolladora Windows no tiene WSL; no se puede hacer aquí una conexión SSH real en Linux.
Los valores antiguos KAGSSH_* ya no se aceptan como claves de configuración.

# Soluciones implementadas
Cliente HTTPS puro Go con token inyectado, timeout y límite de respuesta; tests httptest, pruebas de precedencia, parseo estricto de puertos y filtrado de credenciales.

# Estado de tests
go test ./... pasó en Windows para config y tunnel.
go vet ./... en target Linux pasó.
Compilación Linux AMD64 y ARM64 con CGO_ENABLED=0 y Go 1.26.6 pasó.
Compilación de tests SSH/PTT/SFTP Linux pasó; ejecución en Linux pendiente.
govulncheck con GOOS=linux reportó cero vulnerabilidades alcanzables; una advertencia a nivel de módulo no usada por el código.

# Estado de seguridad
Secrets nunca incluidos en mensajes de error ni entorno de shells autenticadas.
Sin verificar Kaggle en vivo aún, protocolo extraído del repositorio oficial.
Huella SSH del VPS debe configurarse por valor confiable o known_hosts.
Actualización: puerto público solicitado siempre en 0.0.0.0. El VPS requiere GatewayPorts y firewall.
No almacenar contraseñas reales en repo; rotar las anteriormente expuestas.

# Pendientes
Ejecutar en Kaggle real, comprobar Secrets, túnel VPS y SSH/PTT/SFTP extremo a extremo. Ejecutar CI Linux.
El uso frecuente puede superar la cuota de Kaggle Secrets; no hacer consultas en bucle.

# Próximos pasos
Instalar/adjuntar el binario a Kaggle; crear los Secrets individuales, autorizar el notebook y lanzar el binario. Hacer pruebas de conexión con un VPS autorizado.

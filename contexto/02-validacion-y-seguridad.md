# Fecha
2026-10-08

# Objetivo
Registrar verificaciones de implementación y seguridad para la primera versión.

# Decisiones tomadas
- Exigir Go 1.26.6 por la vulnerabilidad GO-2026-5972 del paquete estándar encoding/asn1 presente en Go 1.26.4.
- No integrar el paquete openpgp de x/crypto; no aparece en las dependencias compiladas.
- CI en Linux para ejecutar pruebas reales SSH exec, PTY y SFTP que no se pueden ejecutar en esta máquina Windows sin WSL.

# Arquitectura actual
Un solo ejecutable Linux incorpora servidor SSH (shell/PTT/SFTP) y cliente SSH (remote forward) con configuración por entorno.

# Librerías usadas
x/crypto/ssh v0.57.0; creack/pty v1.1.24; pkg/sftp v1.13.11.

# Archivos importantes modificados
go.mod, go.sum, internal/sshserver, internal/tunnel, cmd/kagssh, .github/workflows/ci.yml.

# Problemas encontrados
- El primer govulncheck con Go 1.26.4 encontró GO-2026-5972 en llamadas alcanzables.
- govulncheck en modo binario (con símbolos eliminados) señala GO-2026-5932 en openpgp/*, aunque la inspección de go list -deps del binario confirma que no se importa openpgp. Es una alerta heurística de dependencia del módulo y no hay trazas de código fuente para ese paquete.

# Soluciones implementadas
- Binarios rehechos con GOTOOLCHAIN=go1.26.6, CGO_ENABLED=0.
- Auth del VPS con validación de huella y deadline del handshake.
- Secrets KAGSSH_* excluidos del entorno de shell; contraseña en variables, no en repo.
- CI Linux permite correr tests de integración cuando se publique en GitHub.

# Estado de tests
- go test ./... Windows: aprobado (config y túnel).
- go vet ./... Linux cross-target: aprobado.
- go test -c ./internal/sshserver para Linux: compila correctamente, pendiente ejecutar en Linux.
- go build linux/amd64 y linux/arm64: aprobados.
- govulncheck con GOOS=linux, Go 1.26.6: 0 vulnerabilidades alcanzables.

# Estado de seguridad
Se mitigó riesgo del runtime identificado por GO-2026-5972. Queda una advertencia a nivel de módulo por GO-2026-5932 que no se usa directamente. Exposición pública del puerto remoto desactivada por defecto. Rotar contraseñas anteriormente pegadas en texto plano.

# Pendientes
- Ejecutar SSH exec/PTT/SFTP en Kaggle o Linux CI y probar conexión al VPS con credenciales de prueba.
- Validar comportamiento ante caída del VPS/reconexión y cortes del notebook.
- No publicar el binario sin ejecutar integración Linux.

# Próximos pasos
Probar `ssh -p 2223` y `sftp -P 2223` con el forward activo; luego tag de versión v0.1.0 si se completa la integración.

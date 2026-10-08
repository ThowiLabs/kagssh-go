# Objetivo
Crear el primer binario Go Linux con servidor SSH, shell PTY, SFTP y cliente de túnel reverso al VPS.

# Estado
Completado en código y compilación para Linux AMD64/ARM64.

# Cambios
cmd/kagssh, internal/sshserver, internal/tunnel, internal/config, README, pruebas y CI.

# Pruebas
Build estático y tests compatibles con Windows pasan. Tests Linux SSH/PTT/SFTP compilan, pendientes de ejecutar físicamente en Linux.

# Seguridad
Autenticación, host key, pin fingerprint y mitigación GO-2026-5972.

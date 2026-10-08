# Fecha
2026-10-08

# Objetivo
Sustituir una celda Python de Kaggle que instalaba OpenSSH Server y ejecutaba SSH inverso mediante un único binario Go estático, sin procesos cliente/servidor OpenSSH locales.

# Decisiones tomadas
- Proyecto nuevo, no migración de un repositorio anterior.
- SSH servidor y SSH cliente usando golang.org/x/crypto/ssh.
- PTY con github.com/creack/pty; SFTP con github.com/pkg/sftp.
- Configuración sencilla por variables de entorno. Nada hardcodeado salvo puertos y host predeterminados.
- Clave de host Ed25519 persistente; pin de fingerprint o known_hosts para el VPS.
- Contraseña/clave pública configurable; no modificar el password root del sistema.
- Servidor loopback predeterminado, bind remoto loopback salvo opción explícita.
- Proceso en primer plano con logs y reinicio del túnel.

# Arquitectura actual
cmd/kagssh/main.go coordina SSH local y túnel.
internal/config valida variables.
internal/sshserver sirve sesiones, shell PTY y SFTP en Linux.
internal/tunnel conecta al VPS y crea reverse forwards.

# Librerías usadas
Go 1.26.6 (mínimo por corrección GO-2026-5972); x/crypto/ssh; creack/pty; pkg/sftp.

# Archivos importantes modificados
go.mod, go.sum, cmd/kagssh/main.go, internal/config, internal/sshserver, internal/tunnel, README.md.

# Problemas encontrados
El script original embebía contraseñas, confiaba indiscriminadamente en hosts SSH y cambiaba password root del sistema.

# Soluciones implementadas
Variables de entorno, verificación de huella, identidades locales separadas de las cuentas del sistema, cierre por contexto.

# Estado de tests
Pruebas unitarias y de integración incluidas. Verificar sobre Linux además de build cruzado Windows.

# Estado de seguridad
Revisión inicial: credenciales excluidas del entorno de shells, validación de huella host, auth SSH, acceso por loopback por defecto.
La exposición pública 0.0.0.0 debe ser explícita. Recomendación: rotar contraseñas del script original.

# Pendientes
Probar túnel extremo a extremo con VPS de prueba, validar reconexión ante cortes de red, comprobar compatibilidad Kaggle real sin exponer credenciales de producción.

# Próximos pasos
Compilación estática, go test, go vet, análisis de dependencias, test de smoke en Linux.

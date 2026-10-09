# Fecha
2026-10-08

# Objetivo
Cerrar la sesión actual de KagSSH Go con una guía de uso y configuración suficientemente detallada para ejecutarlo en Kaggle sin depender de esta conversación.

# Alcance y decisiones
- No cambiar implementación, dependencias, binarios ni infraestructura.
- Crear USO_Y_CONFIGURACION.md en la raíz y enlazarlo desde README.md.
- Documentar tres modalidades: Secrets individuales de Kaggle, exports en celda y mezcla con precedencia export > Secret > default.
- Registrar SSH_PORT_REMOTE=22 (SSH del VPS), SSH_PORT_KAGGLE=2223 (puerto inverso del VPS) y SSH_PORT_LOCAL=2224 (servidor local Kaggle).
- Mostrar preparación del binario en /kaggle/working, celda %%bash, comando -check, acceso externo directo por IP pública del VPS.
- Actualización posterior: túnel solicitado siempre en 0.0.0.0; huella verificada o primera clave registrada automáticamente.
- No confundir KagSSH Go con la idea posterior de Kaggle MCP: queda explícitamente pospuesta, sin código ni planificación comprometida.

# Arquitectura en este cierre
- cmd/kagssh/main.go: coordina SSH servidor y SSH inverso.
- internal/config: lee variables SSH_* o Secrets de Kaggle por etiqueta exacta.
- internal/sshserver: terminal SSH/PTY/SFTP sobre Linux.
- internal/tunnel: publicacion de puerto SSH inverso a través del VPS.
- dist/kagssh-linux-amd64 y dist/kagssh-linux-arm64: binarios estáticos generados anteriormente.

# Documentación modificada
- USO_Y_CONFIGURACION.md (nueva guía completa).
- README.md (enlace a la guía).
- contexto/04-guia-uso-y-cierre-sesion.md (este registro).
- tareas/completado-04-guia-uso-configuracion.md (registro de tarea de documentación).

# Estado de pruebas y seguridad
- Esta sesión de cierre solo cambia archivos Markdown; no modifica la lógica del programa.
- De la fase anterior: go test en Windows para config/tunnel pasó; go vet Linux pasó; builds Linux AMD64/ARM64 pasaron; tests SSH/PTY/SFTP Linux compilaron pero todavía no se ejecutaron en Linux real.
- La integración nativa con Kaggle Secrets se probó con mocks HTTP, NO con una notebook Kaggle real.
- Evitar contraseñas en notebook o repositorio; rotar contraseñas antiguas que aparecieran en texto plano.
- SSH de Kaggle escucha en loopback interno; el puerto del VPS se solicita en 0.0.0.0.
- El VPS todavía necesita un servidor SSH que acepte forward inverso.

# Pendientes para la siguiente sesión
1. Cargar el binario Linux AMD64 en una notebook autorizada y habilitar Secrets individuales e Internet.
2. Probar en Kaggle real lectura de Secrets, inicio del túnel, SSH, shell/PTY/SFTP y cierre.
3. Validar reconexión y observar posibles límites de API Secrets.
4. Revisar resultados, actualizar documentación y evaluar release/tag de versión solo después de la prueba real.

# Próximo paso
Consultar primero USO_Y_CONFIGURACION.md y tareas/pendiente-03-prueba-integracion-kaggle-vps.md, sin empezar Kaggle MCP.

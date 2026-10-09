# 2026-10-09 — Ejecución interactiva del SSH en Kaggle

## Motivo del cambio
En el notebook anterior el proceso Go se lanzaba con `Popen` desacoplado y la celda terminaba. Eso impedía utilizar el botón nativo **Detener/Interruptar** del notebook para cerrar la sesión SSH, lo que no respondía al flujo esperado.

## Decisión vigente
- La celda `run` **permanece ejecutándose** mientras KagSSH Go siga activo; el propio botón de Kaggle es el control de detención.
- `Popen` se espera con `wait()` en la **misma celda**, sin `start_new_session=True`, hilos de logs ni visor HTML.
- `stdout` y `stderr` de Go se combinan en un `PIPE` y se imprimen **línea por línea** desde la misma celda usando `print(..., flush=True)`; así se entregan al frontend de Jupyter sin depender de la captura de descriptores de archivo del kernel.
- Al recibir `KeyboardInterrupt` de la interfaz de Kaggle, el código solicita `terminate()` y espera hasta 10 segundos; solo usa `kill()` como último recurso.
- Se eliminan las celdas independientes `estado`, `cierre` y `stop`, que eran necesarias únicamente para gestionar un proceso separado.
- El SSH de Kaggle sigue siendo integrado en Go; el túnel inverso al VPS sigue solicitando `0.0.0.0` (el VPS requiere `GatewayPorts` y firewall).
- `SSH_HOST` es obligatorio: jamás usar un dominio corporativo como valor predeterminado.

## Archivos afectados
- `notebooks/kagssh_kaggle_chatgpt_agentes.ipynb`: arranque, salida de logs y control con interrupción nativa.
- `scripts/check_notebook.py`: validaciones de estructura y señal de detención.
- `README.md` y `USO_Y_CONFIGURACION.md`: guía de operación interactiva.
- `contexto/05-celda-interactiva-kaggle.md`: registro de la decisión técnica.

## Pruebas y límites
Comprobar con el validador Python y `go test ./...` que no se alteró el servidor SSH ni el código del túnel; validar la compilación Linux del ejecutable como en CI. No declarar verificado el botón real de Kaggle sin ejecutar el notebook en Kaggle: depende de la interfaz y la señal de interrupción del kernel.

## Git
A petición expresa del usuario, **enmendar el último commit** que había introducido el inicio desacoplado y publicar con `--force-with-lease`, verificando antes y después el SHA del remoto. No crear un commit separado para este cambio.

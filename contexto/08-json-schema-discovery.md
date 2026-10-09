# Contexto 08 — Descubrimiento rechaza JSON Schema de las herramientas

Fecha: 2026-10-09

## Incidente
ChatGPT terminó correctamente OAuth pero volvió a mostrar "Authentication succeeded, action discovery failed". En Kaggle se observó el acceso real a `server/discover` y `tools/list` con `MCP-Protocol-Version: 2026-07-28`, por lo que la versión nueva del servidor **sí estaba ejecutándose**. Los logs muestran SSH opcional activado y GitHub token no configurado; ninguno explica por sí mismo este error de acciones.

## Causa concreta reproducida
En `internal/mcp/server.go`, `def(name,desc,properties,required ...string)` incluía la propiedad `required` sin condiciones. Cuando un tool no requiere parámetros, el slice de Go es `nil` y `encoding/json` lo serializa como **`"required":null`**. Sin embargo, `required` en JSON Schema debe ser un arreglo de nombres de propiedades, nunca `null`. Esto hace inválidos los esquemas de `environment_info`, `list_dir`, `github_status` y `github_repositories`; el cliente puede abortar el descubrimiento aunque `tools/list` haya devuelto HTTP 200.

## Reproducción y solución
- Se agregó `TestAllToolSchemasJSONSchemaRequiredArrays` y se ejecutó inicialmente contra el código original: falló específicamente en las cuatro herramientas con `required:null`.
- El generador de schemas ahora omite `required` cuando no hay campos requeridos y lo mantiene como lista de cadenas cuando sí los hay.
- Se reforzó `TestOAuthThenActionDiscoveryModern` para inspeccionar **el JSON auténtico emitido por HTTP** después de pasar DCR, OAuth con PIN, PKCE, `server/discover` y `tools/list`.
- Se verificó que las pruebas dejan de fallar con la corrección y no se alteró la autenticación OAuth ni GitHub PAT.

## Operación en Kaggle
El notebook existente clona/actualiza el repositorio únicamente al ejecutar la celda **clonar**; la celda **compilar** genera un nuevo ejecutable; la celda **run** solo lanza el binario ya construido, no descarga cambios de forma automática. Para probar la revisión nueva: detener run, volver a ejecutar clonar → compilar → validación → run y restablecer la conexión cuando cambie la URL `trycloudflare`.

**Observación SSH independiente:** en los logs `SSH_ENABLED` estaba efectivo en `true`, conectando al VPS `209.46.122.18:22` con remote `0.0.0.0:2223`; para ejecutar solo MCP, establecer explícitamente el Kaggle Secret `SSH_ENABLED=false` y reiniciar. Esto también evita exponer SSH accidentalmente. La huella del VPS se registró con TOFU sin verificación externa; verificarla si se vuelve a utilizar SSH.

## Evidencia y limitaciones
El esquema inválido se detectó inequívocamente en pruebas locales. Esto es una causa viable del error externo, pero falta repetir en ChatGPT/Kaggle para confirmar que es la única. Los resultados de las pruebas se registran en la tarea completada.

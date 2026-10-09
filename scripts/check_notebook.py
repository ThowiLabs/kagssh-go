"""Validación sin dependencias externas del notebook entregado para Kaggle."""
import ast
import json
import re
from pathlib import Path

notebook = Path(__file__).resolve().parents[1] / "notebooks" / "kagssh_kaggle_chatgpt_agentes.ipynb"
data = json.loads(notebook.read_text(encoding="utf-8"))
if data.get("nbformat") != 4:
    raise SystemExit("Formato .ipynb incompatible: se esperaba nbformat 4.")
ids = set()
code_count = 0
for index, cell in enumerate(data["cells"], start=1):
    cell_id = cell["id"]
    if cell_id in ids:
        raise SystemExit(f"ID de celda repetido: {cell_id}")
    ids.add(cell_id)
    source = cell["source"]
    content = "".join(source) if isinstance(source, list) else source
    if cell["cell_type"] == "code":
        ast.parse(content, filename=f"{notebook.name}:celda-{index}")
        code_count += 1
required = {"clonar", "instalar-go", "compilar", "validacion", "run"}
if missing := required - ids:
    raise SystemExit(f"Celdas faltantes: {sorted(missing)}")

# Prueba de comportamiento: compilar sintácticamente la celda no detecta
# expresiones regulares válidas que nunca coinciden (error anterior \\s / \\d).
install_cell = next(c for c in data["cells"] if c["id"] == "instalar-go")
install_source = "".join(install_cell["source"])
patterns = {}
for node in ast.walk(ast.parse(install_source)):
    if not isinstance(node, ast.Assign) or len(node.targets) != 1:
        continue
    target = node.targets[0]
    call = node.value
    if (
        isinstance(target, ast.Name)
        and isinstance(call, ast.Call)
        and isinstance(call.func, ast.Attribute)
        and isinstance(call.func.value, ast.Name)
        and call.func.value.id == "re"
        and call.func.attr == "search"
        and call.args
        and isinstance(call.args[0], ast.Constant)
        and isinstance(call.args[0].value, str)
    ):
        patterns[target.id] = call.args[0].value

mod_pattern = patterns.get("match")
version_pattern = patterns.get("found")
if not mod_pattern or not version_pattern:
    raise SystemExit("No se encontraron los patrones de go.mod y go version.")

mod_text = (notebook.parents[1] / "go.mod").read_text(encoding="utf-8")
mod_match = re.search(mod_pattern, mod_text)
installed_match = re.search(version_pattern, "go version go1.26.9 linux/amd64")
if mod_match is None or mod_match.groups() != ("1", "26", "9"):
    raise SystemExit("La regex del notebook no reconoce 'go 1.26.9' en go.mod.")
if installed_match is None or installed_match.groups() != ("1", "26", "9"):
    raise SystemExit("La regex del notebook no reconoce 'go version go1.26.9'.")
print(f"Notebook válido: {len(data['cells'])} celdas, {code_count} celdas de Python.")
print("Pruebas de regex go.mod y go version: correctas.")

# La celda debe mantenerse en ejecución con los logs heredados del proceso,
# hasta que el usuario pulse Detener/Interruptar en Kaggle.
run_source = "".join(next(c for c in data["cells"] if c["id"] == "run")["source"])
run_tree = ast.parse(run_source)
calls = [node for node in ast.walk(run_tree) if isinstance(node, ast.Call)]
def has_call(name, attr):
    return any(
        isinstance(node.func, ast.Attribute)
        and isinstance(node.func.value, ast.Name)
        and node.func.value.id == name and node.func.attr == attr
        for node in calls
    )
if not has_call("subprocess", "Popen") or not has_call("kagssh", "wait"):
    raise SystemExit("La celda debe lanzar Go con Popen y esperar su terminación (espera bloqueante).")
if not all(has_call("kagssh", name) for name in ("poll", "terminate", "kill")):
    raise SystemExit("Falta limpieza del proceso al interrumpir la celda.")
if not any(isinstance(node, ast.ExceptHandler) and isinstance(node.type, ast.Name)
           and node.type.id == "KeyboardInterrupt" for node in ast.walk(run_tree)):
    raise SystemExit("La celda debe atender KeyboardInterrupt de Kaggle.")
if any(value in run_source for value in (
    "start_new_session=True", "stdout=logfile",
    "threading.Thread", "display_id=True",
)):
    raise SystemExit("La celda no puede desacoplar el proceso ni capturar/ocultar logs.")
if ("stdout=subprocess.PIPE" not in run_source
        or "stderr=subprocess.STDOUT" not in run_source
        or 'for line in kagssh.stdout:' not in run_source
        or 'print(line, end="", flush=True)' not in run_source):
    raise SystemExit("Los logs deben imprimirse línea por línea en la celda, sin hilos.")
if any(c["id"] in {"estado", "cierre", "stop"} for c in data["cells"]):
    raise SystemExit("El notebook no debe depender de celdas STOP/estado adicionales.")
print("Inicio interactivo, logs directos y parada por Interruptar: estructura correcta.")

# Prueba dinámica local: ejecuta el código de la celda con Popen simulado.
# Comprueba que el bucle muestra logs, espera al proceso y libera el recurso
# cuando Jupyter envía KeyboardInterrupt. No abre un servidor ni un túnel real.
import contextlib
import io
import subprocess
from unittest.mock import patch

class FakeGoProcess:
    def __init__(self, interrupt):
        self.interrupt = interrupt
        self.stdout = self
        self.returncode = None
        self.lines = iter(["Go: servidor SSH activo\\n", "Go: túnel activo\\n"])
        self.terminated = False

    def __enter__(self):
        return self

    def __exit__(self, *args):
        return False

    def __iter__(self):
        return self

    def __next__(self):
        if self.interrupt:
            raise KeyboardInterrupt
        return next(self.lines)

    def poll(self):
        return self.returncode

    def terminate(self):
        self.terminated = True
        self.returncode = -15

    def kill(self):
        self.returncode = -9

    def wait(self, timeout=None):
        if self.returncode is None:
            self.returncode = 0
        return self.returncode

for interrupted in (False, True):
    fake = FakeGoProcess(interrupted)
    def fake_popen(*args, **kwargs):
        if not args or not args[0] or kwargs.get("start_new_session"):
            raise AssertionError("Ejecución desacoplada o comando vacío")
        if kwargs.get("stdout") is not subprocess.PIPE or kwargs.get("stderr") is not subprocess.STDOUT:
            raise AssertionError("No captura los logs para mostrarlos en la celda")
        return fake
    output = io.StringIO()
    with patch.object(subprocess, "Popen", fake_popen), contextlib.redirect_stdout(output):
        exec(compile(run_source, "celda-run", "exec"), {
            "BINARY": "/kaggle/working/kagssh-linux-amd64",
            "WORKING": "/kaggle/working",
        })
    text = output.getvalue()
    if interrupted:
        if not fake.terminated or "KagSSH detenido" not in text:
            raise SystemExit("Interrumpir la celda no cierra KagSSH.")
    elif fake.terminated or "servidor SSH activo" not in text or "túnel activo" not in text:
        raise SystemExit("La celda no muestra los logs en ejecución.")
print("Pruebas simuladas de logs y KeyboardInterrupt: correctas.")

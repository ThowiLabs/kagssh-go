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
required = {"clonar", "instalar-go", "compilar", "configurar", "validacion", "run"}
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
import getpass
import os
import sys
import types

# Pruebas de la celda configurar: con SSH apagado no se invoca Kaggle Secrets.
config_source = "".join(next(c for c in data["cells"] if c["id"] == "configurar")["source"])
test_pin = "TEST_ONLY_PIN_12345678"
test_pat = "TEST_ONLY_GITHUB_PAT_ABC"
test_vps_password = "TEST_ONLY_VPS_SECRET"
test_local_password = "TEST_ONLY_LOGIN_SECRET"

def simulate_setup(source, *, secret_module=None):
    global fake_secret_reads
    fake_secret_reads = []
    module = types.ModuleType("kaggle_secrets")
    class FakeSecrets:
        def get_secret(self, label):
            fake_secret_reads.append(label)
            if label == "SSH_PASSWORD":
                return test_vps_password
            if label == "SSH_LOGIN_PASSWORD":
                return test_local_password
            raise AssertionError(f"Consulta inesperada a Kaggle Secrets: {label}")
    module.UserSecretsClient = FakeSecrets
    local = {
        "WORKING": Path("/kaggle/working"),
        "REPO_DIR": Path("/kaggle/working/kagssh-go"),
    }
    output = io.StringIO()
    with (patch.dict(os.environ, {
        "KAGGLE_USER_SECRETS_TOKEN": "TEST_ONLY_KAGGLE_INTERNAL_SECRET",
        "KAGGLE_IAP_TOKEN": "TEST_ONLY_KAGGLE_IAP",
        "SSH_ENABLED": "true", "SSH_HOST": "old-vps",
        "MCP_ENABLED": "false", "GITHUB_TOKEN": "stale-token",
    }), patch.dict(sys.modules, {"kaggle_secrets": module}),
        patch.object(getpass, "getpass", side_effect=lambda prompt: test_pat if "GitHub" in prompt else test_pin),
        contextlib.redirect_stdout(output)):
        exec(compile(source, "celda-configurar", "exec"), local)
    return local["RUNTIME_ENV"], output.getvalue(), list(fake_secret_reads)

normal_env, normal_output, calls = simulate_setup(config_source)
if calls or normal_env.get("SSH_ENABLED") != "false" or normal_env.get("MCP_ENABLED") != "true":
    raise SystemExit("El perfil MCP-only no debe consultar Secrets ni habilitar SSH.")
if (normal_env.get("MCP_ACCESS_PIN") != ""
    or "KAGGLE_USER_SECRETS_TOKEN" in normal_env or "KAGGLE_IAP_TOKEN" in normal_env
    or "GITHUB_TOKEN" in normal_env or "SSH_HOST" in normal_env):
    raise SystemExit("El entorno de Go contiene valores antiguos o tokens internos Kaggle.")
if any(value in normal_output for value in (test_pin, test_pat, test_vps_password, test_local_password)):
    raise SystemExit("La celda configurar mostró secretos en stdout.")

# Un PIN explícito de seis caracteres se pasa sin generación en Python.
custom_source = config_source.replace('pin = ""', 'pin = "ABCDEF"')
custom_env, custom_output, custom_reads = simulate_setup(custom_source)
if custom_env.get("MCP_ACCESS_PIN") != "ABCDEF" or custom_reads:
    raise SystemExit("El PIN opcional de 6 caracteres no se pasó a Go.")
if "ABCDEF" in custom_output:
    raise SystemExit("No debe imprimirse el PIN definido en Python.")

# Activar VPS debe consultar exactamente las dos contraseñas solicitadas.
ssh_source = config_source.replace("SSH_ENABLED = False", "SSH_ENABLED = True")
ssh_source = ssh_source.replace('SSH_HOST = ""', 'SSH_HOST = "vps.example"')
ssh_env, ssh_output, ssh_reads = simulate_setup(ssh_source)
if ssh_reads != ["SSH_PASSWORD", "SSH_LOGIN_PASSWORD"]:
    raise SystemExit(f"Se consultaron Secrets ajenos al VPS: {ssh_reads}")
if ssh_env.get("SSH_PASSWORD") != test_vps_password or ssh_env.get("SSH_LOGIN_PASSWORD") != test_local_password:
    raise SystemExit("Las contraseñas no se entregaron al proceso SSH/MCP.")
if any(value in ssh_output for value in (test_pin, test_vps_password, test_local_password)):
    raise SystemExit("La celda con SSH mostró contraseñas.")

# Confirmar uso opcional de PAT introducido sin Kaggle Secrets.
github_source = config_source.replace("USE_GITHUB = False", "USE_GITHUB = True")
github_env, _, github_reads = simulate_setup(github_source)
if github_reads or github_env.get("GITHUB_TOKEN") != test_pat:
    raise SystemExit("El PAT debe solicitarse mediante getpass, sin Secret de Kaggle.")

# La validación y ejecución deben usar el MISMO entorno y no consultar Secrets.
validation_source = "".join(next(c for c in data["cells"] if c["id"] == "validacion")["source"])
observed = {}
class FakeCheck:
    returncode = 0
    stdout = "configuración válida"
    stderr = ""

def fake_check(*args, **kwargs):
    observed.update(kwargs)
    if not args or args[0][-1] != "-check":
        raise AssertionError("Se esperaba el comando de validación -check")
    return FakeCheck()

with patch.object(subprocess, "run", fake_check), contextlib.redirect_stdout(io.StringIO()):
    exec(compile(validation_source, "celda-validacion", "exec"), {
        "BINARY": "/kaggle/working/kagmcp-linux-amd64",
        "WORKING": Path("/kaggle/working"),
        "RUNTIME_ENV": normal_env,
        "subprocess": subprocess,
    })
if observed.get("env") is not normal_env:
    raise SystemExit("La celda -check no usó el entorno Python configurado.")
print("Python configura MCP/SSH y Secrets selectivos: pruebas correctas.")

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
        if kwargs.get("env") is not normal_env or "KAGGLE_USER_SECRETS_TOKEN" in kwargs["env"]:
            raise AssertionError("El binario no recibió el entorno seguro de la celda configurar")
        return fake
    output = io.StringIO()
    with patch.object(subprocess, "Popen", fake_popen), contextlib.redirect_stdout(output):
        exec(compile(run_source, "celda-run", "exec"), {
            "BINARY": "/kaggle/working/kagssh-linux-amd64",
            "WORKING": "/kaggle/working",
            "RUNTIME_ENV": normal_env,
        })
    text = output.getvalue()
    if interrupted:
        if not fake.terminated or "KagMCP detenido" not in text:
            raise SystemExit("Interrumpir la celda no cierra KagMCP.")
    elif fake.terminated or "servidor SSH activo" not in text or "túnel activo" not in text:
        raise SystemExit("La celda no muestra los logs en ejecución.")
print("Pruebas simuladas de logs y KeyboardInterrupt: correctas.")

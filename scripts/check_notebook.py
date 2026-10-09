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
required = {"clonar", "instalar-go", "compilar", "validacion", "run", "stop"}
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

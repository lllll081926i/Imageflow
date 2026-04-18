import importlib
import sys
from functools import lru_cache
from pathlib import Path
from typing import Any


def ensure_engine_scripts_path() -> Path:
    scripts_dir = Path(__file__).resolve().parents[1] / "engines"
    scripts_path = str(scripts_dir)
    if scripts_path not in sys.path:
        sys.path.insert(0, scripts_path)
    return scripts_dir


@lru_cache(maxsize=None)
def load_engine_module(module_name: str):
    ensure_engine_scripts_path()
    return importlib.import_module(module_name)


def invoke_engine_process(module_name: str, payload: dict[str, Any]) -> dict[str, Any]:
    module = load_engine_module(module_name)
    process = getattr(module, "process", None)
    if process is None:
        raise AttributeError(f"{module_name} 缺少 process()")
    result = process(payload)
    if not isinstance(result, dict):
        raise TypeError(f"{module_name}.process() 未返回 dict")
    return result

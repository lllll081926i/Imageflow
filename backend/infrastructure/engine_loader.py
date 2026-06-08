import importlib.util
import sys
from functools import lru_cache
from pathlib import Path
from typing import Any

ALLOWED_ENGINES = frozenset({
    "converter", "compressor", "filter", "adjuster",
    "watermark", "pdf_generator", "gif_splitter",
    "metadata_tool", "info_viewer", "subtitle_stitcher",
})

ENGINES_REQUIRING_CONVERTER = frozenset({
    "adjuster",
    "filter",
    "info_viewer",
    "pdf_generator",
    "watermark",
})


def ensure_engine_scripts_path() -> Path:
    scripts_dir = Path(__file__).resolve().parents[1] / "engines"
    scripts_path = str(scripts_dir)
    if scripts_path not in sys.path:
        sys.path.append(scripts_path)
    return scripts_dir


def _load_module_from_engine_file(module_name: str):
    scripts_dir = ensure_engine_scripts_path()
    module_path = scripts_dir / f"{module_name}.py"
    if not module_path.is_file():
        raise ImportError(f"Engine module '{module_name}' was not found at {module_path}")

    existing = sys.modules.get(module_name)
    if existing is not None:
        existing_file = getattr(existing, "__file__", None)
        if existing_file and Path(existing_file).resolve() == module_path.resolve():
            return existing

    spec = importlib.util.spec_from_file_location(module_name, module_path)
    if spec is None or spec.loader is None:
        raise ImportError(f"Failed to create module spec for '{module_name}'")

    module = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = module
    try:
        spec.loader.exec_module(module)
    except Exception:
        sys.modules.pop(module_name, None)
        raise
    return module


@lru_cache(maxsize=None)
def load_engine_module(module_name: str):
    if module_name not in ALLOWED_ENGINES:
        raise ImportError(f"Module '{module_name}' is not in the allowed engines list")
    if module_name in ENGINES_REQUIRING_CONVERTER:
        load_engine_module("converter")
    return _load_module_from_engine_file(module_name)


def invoke_engine_process(module_name: str, payload: dict[str, Any]) -> dict[str, Any]:
    module = load_engine_module(module_name)
    process = getattr(module, "process", None)
    if process is None:
        raise AttributeError(f"{module_name} 缺少 process()")
    result = process(payload)
    if not isinstance(result, dict):
        raise TypeError(f"{module_name}.process() 未返回 dict")
    return result

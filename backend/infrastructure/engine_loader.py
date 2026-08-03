import importlib
import importlib.util
import sys
from functools import lru_cache
from pathlib import Path
from typing import Any

ALLOWED_ENGINES = frozenset({
    "converter", "compressor", "image_processor",
    "watermark", "pdf_generator", "gif_splitter",
    "subtitle_stitcher",
})

ENGINES_REQUIRING_CONVERTER = frozenset({
    "pdf_generator",
    "watermark",
})


def _is_frozen() -> bool:
    """True when running inside a PyInstaller/PyOxide bundle."""
    return getattr(sys, "frozen", False) or hasattr(sys, "_MEIPASS")


def _qualified_engine_module_name(module_name: str) -> str:
    return f"backend.engines.{module_name}"


def _register_engine_alias(module_name: str, module) -> None:
    """Expose the engine under its bare name so in-package engines that do
    ``from converter import ...`` keep resolving after freezing, where the
    engines no longer live as loose .py files on sys.path."""
    existing = sys.modules.get(module_name)
    if existing is not None and existing is not module:
        # Keep the already-loaded object to preserve module identity.
        return
    sys.modules[module_name] = module


def _load_engine_package_module(module_name: str):
    """Import the engine as a regular package member (``backend.engines.<name>``).

    This path works both in development and inside a PyInstaller bundle, where
    engines are collected into the PYZ archive instead of being shipped as
    loose source files under ``_internal/backend/engines``.
    """
    qualified = _qualified_engine_module_name(module_name)
    try:
        module = importlib.import_module(qualified)
    except ImportError:
        return None
    _register_engine_alias(module_name, module)
    return module


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

    # Frozen builds ship engines inside the PYZ archive as package members
    # (``backend.engines.<name>``) instead of loose .py files on disk, so the
    # file-based loader below would raise "was not found at ...converter.py".
    # Prefer the package import whenever it succeeds; fall back to file-based
    # loading for the development layout where the bare ``converter`` module
    # is expected on sys.path.
    module = _load_engine_package_module(module_name)
    if module is not None:
        return module
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

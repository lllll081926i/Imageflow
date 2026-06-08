import json
import os
from dataclasses import asdict, fields
from pathlib import Path
from typing import Any

from backend.contracts.settings import AppSettings, default_app_settings

MAX_RECENT_PATHS = 4


def _clamp(value: int, min_value: int, max_value: int) -> int:
    return max(min_value, min(value, max_value))


def _coerce_int(value: Any, fallback: int) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return fallback


def _coerce_bool(value: Any, fallback: bool) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        normalized = value.strip().lower()
        if normalized in {"1", "true", "yes", "on"}:
            return True
        if normalized in {"0", "false", "no", "off"}:
            return False
    return fallback


def _normalize_saved_path(value: Any) -> str:
    trimmed = str(value or "").strip()
    if not trimmed:
        return ""
    if trimmed in {"/", "\\"}:
        return trimmed
    cleaned = trimmed.rstrip("/\\")
    if len(cleaned) == 2 and cleaned[1] == ":":
        return trimmed
    return cleaned or trimmed


def _saved_path_key(value: str) -> str:
    return value.replace("\\", "/").casefold()


def _normalize_recent_paths(values: Any) -> list[str]:
    if not isinstance(values, list):
        return []

    normalized: list[str] = []
    seen: set[str] = set()
    for item in values:
        path = _normalize_saved_path(item)
        if not path:
            continue
        key = _saved_path_key(path)
        if key in seen:
            continue
        seen.add(key)
        normalized.append(path)
        if len(normalized) >= MAX_RECENT_PATHS:
            break
    return normalized


def normalize_settings(settings: AppSettings) -> AppSettings:
    defaults = default_app_settings()
    output_prefix = str(settings.output_prefix or "").strip() or defaults.output_prefix
    output_template = str(settings.output_template or "").strip() or defaults.output_template
    conflict_strategy = str(settings.conflict_strategy or "").strip() or defaults.conflict_strategy
    if conflict_strategy != "rename":
        conflict_strategy = defaults.conflict_strategy

    return AppSettings(
        max_concurrency=_clamp(_coerce_int(settings.max_concurrency, defaults.max_concurrency), 1, 32),
        output_prefix=output_prefix,
        output_template=output_template,
        preserve_folder_structure=_coerce_bool(settings.preserve_folder_structure, defaults.preserve_folder_structure),
        conflict_strategy=conflict_strategy,
        default_output_dir=_normalize_saved_path(settings.default_output_dir),
        recent_input_dirs=_normalize_recent_paths(settings.recent_input_dirs),
        recent_output_dirs=_normalize_recent_paths(settings.recent_output_dirs),
    )


def settings_from_dict(data: dict[str, Any]) -> AppSettings:
    defaults = asdict(default_app_settings())
    known_fields = {field.name for field in fields(AppSettings)}
    values = {name: data.get(name, defaults[name]) for name in known_fields}
    return AppSettings(**values)


def _settings_file_path() -> tuple[Path, bool]:
    override = os.getenv("IMAGEFLOW_SETTINGS_FILE", "").strip()
    if override:
        path = Path(override).resolve()
        if path.suffix.lower() != ".json":
            raise ValueError("IMAGEFLOW_SETTINGS_FILE must point to a .json file")
        if not path.parent.exists():
            raise ValueError("IMAGEFLOW_SETTINGS_FILE parent directory must already exist")
        return path, True

    appdata = os.getenv("APPDATA", "").strip()
    if appdata:
        return Path(appdata) / "imageflow" / "settings.json", False

    return Path.home() / ".config" / "imageflow" / "settings.json", False


def load_settings() -> AppSettings:
    try:
        path, _is_override = _settings_file_path()
    except (OSError, ValueError):
        return default_app_settings()
    if not path.exists():
        return default_app_settings()

    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(data, dict):
            return default_app_settings()
        settings = settings_from_dict(data)
        return normalize_settings(settings)
    except (json.JSONDecodeError, OSError, TypeError, ValueError):
        return default_app_settings()


def save_settings(settings: AppSettings) -> AppSettings:
    import tempfile
    normalized = normalize_settings(settings)
    path, is_override = _settings_file_path()
    if not is_override:
        path.parent.mkdir(parents=True, exist_ok=True)
    tmp_fd, tmp_path = tempfile.mkstemp(dir=path.parent, suffix=".tmp")
    try:
        with os.fdopen(tmp_fd, "w", encoding="utf-8") as f:
            json.dump(asdict(normalized), f, ensure_ascii=False, indent=2)
        os.replace(tmp_path, path)
    except BaseException:
        try:
            os.unlink(tmp_path)
        except OSError:
            pass
        raise
    return normalized

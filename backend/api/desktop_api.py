from __future__ import annotations

from dataclasses import asdict
from pathlib import Path
from threading import Lock
from typing import Any

from backend.application.task_manager import TaskManager


def expand_input_paths(paths: list[str]) -> dict:
    from backend.domain.paths import expand_input_paths as expand_paths

    return expand_paths(paths)


def list_system_fonts() -> list[str]:
    from backend.domain.paths import list_system_fonts as load_fonts

    return load_fonts()


def normalize_optional_user_supplied_path(value: str) -> str:
    from backend.domain.paths import normalize_optional_user_supplied_path as normalize_optional_path

    return normalize_optional_path(value)


def normalize_user_supplied_path(value: str) -> str:
    from backend.domain.paths import normalize_user_supplied_path as normalize_path

    return normalize_path(value)


def resolve_output_path(base_path: str, reserved: list[str]) -> str:
    from backend.domain.paths import resolve_output_path as resolve_path

    return resolve_path(base_path, reserved)


def open_file_dialog(options: dict | None = None):
    from backend.infrastructure.dialogs import open_file_dialog as show_file_dialog

    return show_file_dialog(options)


def open_directory_dialog(options: dict | None = None):
    from backend.infrastructure.dialogs import open_directory_dialog as show_directory_dialog

    return show_directory_dialog(options)


def open_path_in_explorer(path: str) -> dict:
    from backend.infrastructure.dialogs import open_path_in_explorer as reveal

    return reveal(path)


def load_settings():
    from backend.infrastructure.settings_store import load_settings as read_settings

    return read_settings()


def save_settings(settings):
    from backend.infrastructure.settings_store import save_settings as write_settings

    return write_settings(settings)


def settings_from_dict(payload: dict):
    from backend.infrastructure.settings_store import settings_from_dict as build_settings

    return build_settings(payload)


def runtime_quit() -> None:
    from backend.infrastructure.window_ops import runtime_quit as quit_window

    quit_window()


def runtime_window_minimise() -> None:
    from backend.infrastructure.window_ops import runtime_window_minimise as minimise_window

    minimise_window()


def runtime_window_toggle_maximise() -> None:
    from backend.infrastructure.window_ops import runtime_window_toggle_maximise as toggle_window_maximise

    toggle_window_maximise()


def execute_engine(module_name: str, payload: dict, task_manager: Any, task_id: int | None = None) -> dict:
    from backend.application.image_ops import execute_engine as run_engine

    return run_engine(module_name, payload, task_manager, task_id=task_id)


def execute_engine_batch(
    module_name: str,
    payloads: list[dict],
    settings: Any,
    task_manager: Any,
    task_id: int | None = None,
) -> list[dict]:
    from backend.application.image_ops import execute_engine_batch as run_engine_batch

    return run_engine_batch(module_name, payloads, settings, task_manager, task_id=task_id)


def _normalize_recent_path(value: str) -> str:
    trimmed = str(value or "").strip()
    if not trimmed:
        return ""
    if trimmed in {"/", "\\"}:
        return trimmed
    cleaned = trimmed.rstrip("/\\")
    if len(cleaned) == 2 and cleaned[1] == ":":
        return trimmed
    return cleaned or trimmed


def _recent_path_key(value: str) -> str:
    return value.replace("\\", "/").casefold()


def _merge_recent_paths(current: list[str], next_value: str) -> list[str]:
    normalized = _normalize_recent_path(next_value)
    if not normalized:
        return list(current)

    merged: list[str] = [normalized]
    seen = {_recent_path_key(normalized)}
    for item in current:
        candidate = _normalize_recent_path(item)
        if not candidate:
            continue
        key = _recent_path_key(candidate)
        if key in seen:
            continue
        seen.add(key)
        merged.append(candidate)
        if len(merged) >= 4:
            break
    return merged


def _normalize_payload_paths(payload: Any) -> Any:
    if isinstance(payload, list):
        return [_normalize_payload_paths(item) for item in payload]
    if not isinstance(payload, dict):
        return payload

    normalized = dict(payload)
    for field in ("input_path", "output_path", "output_dir", "watermark_path", "image_path"):
        if field in normalized:
            normalized[field] = normalize_optional_user_supplied_path(str(normalized.get(field) or ""))
    for field in ("input_paths", "image_paths"):
        if field in normalized and isinstance(normalized[field], list):
            normalized[field] = [normalize_user_supplied_path(str(item)) for item in normalized[field] if str(item).strip()]
    if isinstance(normalized.get("images"), list):
        normalized["images"] = [_normalize_payload_paths(item) for item in normalized["images"]]
    return normalized


def _extract_runtime_file_path(file_ref: Any) -> str:
    candidates: list[str] = []
    if isinstance(file_ref, dict):
        candidates.extend(
            str(file_ref.get(key) or "")
            for key in ("pywebviewFullPath", "path")
        )
    else:
        candidates.extend(str(getattr(file_ref, key, "") or "") for key in ("pywebviewFullPath", "path"))

    for candidate in candidates:
        trimmed = candidate.strip()
        if not trimmed:
            continue
        if not Path(trimmed).is_absolute():
            continue
        return normalize_optional_user_supplied_path(trimmed)
    return ""


def _probe_animated_path(input_path: str) -> dict[str, Any]:
    from PIL import Image, UnidentifiedImageError

    raw_path = str(input_path or "").strip()
    normalized_path = raw_path
    try:
        normalized_path = normalize_user_supplied_path(raw_path)
        # Only need container metadata; avoid full pixel decode when possible.
        with Image.open(normalized_path) as image:
            frame_count = int(getattr(image, "n_frames", 1) or 1)
            return {
                "input_path": normalized_path,
                "frame_count": frame_count,
                "is_animated": frame_count > 1,
                "format": str(image.format or "").upper(),
            }
    except ValueError as exc:
        return {
            "input_path": raw_path,
            "frame_count": 0,
            "is_animated": False,
            "error": str(exc),
        }
    except FileNotFoundError:
        return {
            "input_path": normalized_path,
            "frame_count": 0,
            "is_animated": False,
            "error": f"Input file not found: {normalized_path}",
        }
    except UnidentifiedImageError:
        return {
            "input_path": normalized_path,
            "frame_count": 0,
            "is_animated": False,
            "error": f"Unsupported image format: {normalized_path}",
        }
    except Exception as exc:
        return {
            "input_path": normalized_path,
            "frame_count": 0,
            "is_animated": False,
            "error": str(exc),
        }


_probe_cache_lock = Lock()
_probe_cache: dict[tuple[str, int, int], dict[str, Any]] = {}
_PROBE_CACHE_MAX = 256


def _probe_cache_key(path: str) -> tuple[str, int, int] | None:
    try:
        st = Path(path).stat()
    except OSError:
        return None
    return (path, int(getattr(st, "st_mtime_ns", int(st.st_mtime * 1_000_000_000))), int(st.st_size))


def _probe_animated_path_cached(input_path: str) -> dict[str, Any]:
    key = _probe_cache_key(str(input_path or "").strip())
    if key is not None:
        with _probe_cache_lock:
            cached = _probe_cache.get(key)
            if cached is not None:
                return dict(cached)
    result = _probe_animated_path(input_path)
    if key is not None and not result.get("error"):
        with _probe_cache_lock:
            _probe_cache[key] = dict(result)
            if len(_probe_cache) > _PROBE_CACHE_MAX:
                # Drop an arbitrary old entry (FIFO-ish via iterator order).
                _probe_cache.pop(next(iter(_probe_cache)), None)
    return result


class DesktopAPI:
    def __init__(self, task_manager: Any | None = None):
        self._task_manager_instance = task_manager
        self._task_manager_lock = Lock()
        self._info_task_lock = Lock()
        self._settings_lock = Lock()
        self._active_info_task_id: int | None = None

    @property
    def _task_manager(self):
        if self._task_manager_instance is None:
            with self._task_manager_lock:
                if self._task_manager_instance is None:
                    # Resolve through module globals so tests can substitute TaskManager.
                    self._task_manager_instance = globals()["TaskManager"]()
        return self._task_manager_instance

    def _settings(self):
        return load_settings()

    def _run_operation(self, handler):
        """Run a single engine operation under its own task id.

        ``handler`` receives the task id so cancellation always targets this
        operation instead of whatever task happens to be "current".
        """
        task_id = self._task_manager.begin_task("operation")
        try:
            return handler(task_id)
        except Exception as exc:
            return {"success": False, "error": str(exc)}
        finally:
            self._task_manager.finish_task(task_id)

    def _run_batch_operation(self, payloads: list[dict], handler):
        """Always return list[dict] so frontend never mistakes an error envelope for success."""
        items = list(payloads or [])
        if not items:
            return []
        task_id = self._task_manager.begin_task("operation")
        try:
            result = handler(task_id)
            if isinstance(result, list):
                return result
            if isinstance(result, dict):
                error = str(result.get("error") or "批处理失败")
                return [
                    {
                        "success": False,
                        "error": error,
                        "input_path": str(item.get("input_path") or ""),
                    }
                    for item in items
                ]
            return [
                {
                    "success": False,
                    "error": "批处理返回格式异常",
                    "input_path": str(item.get("input_path") or ""),
                }
                for item in items
            ]
        except Exception as exc:
            error = str(exc)
            return [
                {
                    "success": False,
                    "error": error,
                    "input_path": str(item.get("input_path") or ""),
                }
                for item in items
            ]
        finally:
            self._task_manager.finish_task(task_id)

    def _run_engine_operation(self, module_name: str, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(
            lambda task_id: execute_engine(module_name, normalized, self._task_manager, task_id=task_id)
        )

    def _run_engine_batch(self, module_name: str, payloads: list[dict]) -> list[dict]:
        normalized = [_normalize_payload_paths(item) for item in payloads]
        return self._run_batch_operation(
            normalized,
            lambda task_id: execute_engine_batch(
                module_name, normalized, self._settings(), self._task_manager, task_id=task_id
            ),
        )

    def ping(self) -> str:
        return "pong"

    def get_settings(self) -> dict:
        return asdict(self._settings())

    def save_settings(self, payload: dict) -> dict:
        with self._settings_lock:
            saved = save_settings(settings_from_dict(payload))
        return asdict(saved)

    def update_recent_paths(self, payload: dict) -> dict:
        with self._settings_lock:
            current = self._settings()
            current.recent_input_dirs = _merge_recent_paths(current.recent_input_dirs, str(payload.get("input_dir") or ""))
            current.recent_output_dirs = _merge_recent_paths(current.recent_output_dirs, str(payload.get("output_dir") or ""))
            saved = save_settings(current)
        return asdict(saved)

    def select_input_files(self, options: dict | None = None) -> list[str]:
        dialog_options = dict(options) if isinstance(options, dict) else {}
        dialog_options.setdefault("title", "选择文件")
        dialog_options["allowsMultipleSelection"] = bool(
            dialog_options.get("allowsMultipleSelection", True)
        )
        result = open_file_dialog(dialog_options)
        if isinstance(result, list):
            return [str(item) for item in result if str(item).strip()]
        if isinstance(result, str) and result.strip():
            return [result.strip()]
        return []

    def select_input_directory(self) -> str:
        return str(open_directory_dialog({"title": "选择文件夹"}) or "")

    def select_output_directory(self) -> str:
        return str(open_directory_dialog({"title": "选择输出文件夹"}) or "")

    def expand_dropped_paths(self, paths: list[str]) -> dict:
        filtered = [str(path).strip() for path in paths if str(path).strip()]
        return expand_input_paths(filtered)

    def resolve_output_path(self, payload: dict) -> dict:
        try:
            base = normalize_user_supplied_path(str(payload.get("base_path") or ""))
            reserved = [str(item) for item in payload.get("reserved") or []]
            output_path = resolve_output_path(base, reserved)
            return {"success": True, "output_path": output_path}
        except Exception as exc:
            return {"success": False, "error": str(exc)}

    def resolve_output_paths(self, payload: dict) -> dict:
        """Batch resolve unique output paths to avoid N bridge round-trips."""
        try:
            items = payload.get("items") if isinstance(payload, dict) else None
            if not isinstance(items, list):
                return {"success": False, "error": "Missing items list", "paths": []}
            reserved = [str(item) for item in (payload.get("reserved") or []) if str(item).strip()]
            resolved: list[str] = []
            for raw in items:
                base = normalize_user_supplied_path(str(raw or ""))
                output_path = resolve_output_path(base, reserved)
                resolved.append(output_path)
                reserved.append(output_path)
            return {"success": True, "paths": resolved}
        except Exception as exc:
            return {"success": False, "error": str(exc), "paths": []}

    def list_system_fonts(self) -> list[str]:
        return list_system_fonts()

    def get_image_preview(self, payload: dict) -> dict:
        from backend.application.preview import build_image_preview_smart

        normalized = _normalize_payload_paths(payload)
        input_path = normalized.get("input_path")
        if not input_path:
            return {"success": False, "error": "Missing input_path in payload"}
        return build_image_preview_smart(str(input_path))


    def convert(self, payload: dict) -> dict:
        return self._run_engine_operation("converter", payload)

    def convert_batch(self, payloads: list[dict]) -> list[dict]:
        return self._run_engine_batch("converter", payloads)

    def compress(self, payload: dict) -> dict:
        return self._run_engine_operation("compressor", payload)

    def compress_batch(self, payloads: list[dict]) -> list[dict]:
        return self._run_engine_batch("compressor", payloads)

    def generate_pdf(self, payload: dict) -> dict:
        return self._run_engine_operation("pdf_generator", payload)

    def split_gif(self, payload: dict) -> dict:
        return self._run_engine_operation("gif_splitter", payload)

    def probe_animated_paths(self, paths: list[str]) -> list[dict]:
        normalized_paths = [str(path) for path in paths if str(path).strip()]
        return [_probe_animated_path_cached(path) for path in normalized_paths]

    def generate_subtitle_long_image(self, payload: dict) -> dict:
        return self._run_engine_operation("subtitle_stitcher", payload)

    def add_watermark(self, payload: dict) -> dict:
        return self._run_engine_operation("watermark", payload)

    def add_watermark_batch(self, payloads: list[dict]) -> list[dict]:
        return self._run_engine_batch("watermark", payloads)

    def adjust(self, payload: dict) -> dict:
        return self._run_engine_operation("image_processor", payload)

    def adjust_batch(self, payloads: list[dict]) -> list[dict]:
        return self._run_engine_batch("image_processor", payloads)

    def apply_filter(self, payload: dict) -> dict:
        return self._run_engine_operation("image_processor", payload)

    def apply_filter_batch(self, payloads: list[dict]) -> list[dict]:
        return self._run_engine_batch("image_processor", payloads)

    def process_image(self, payload: dict) -> dict:
        return self._run_engine_operation("image_processor", payload)

    def process_image_batch(self, payloads: list[dict]) -> list[dict]:
        return self._run_engine_batch("image_processor", payloads)


    def get_app_version(self) -> dict:
        from backend.infrastructure.app_version import read_local_version
        return {"success": True, "version": read_local_version()}

    def check_for_updates(self, payload: dict | None = None) -> dict:
        from backend.infrastructure.app_version import check_for_updates as do_check
        timeout = 8.0
        if isinstance(payload, dict):
            try:
                timeout = float(payload.get("timeout_seconds") or 8.0)
            except (TypeError, ValueError):
                timeout = 8.0
        return do_check(timeout_seconds=timeout)


    def open_path(self, payload: dict | None = None) -> dict:
        path = ""
        if isinstance(payload, dict):
            path = str(payload.get("path") or payload.get("dir") or "").strip()
        return open_path_in_explorer(path)

    def cancel_processing(self) -> bool:
        return self._task_manager.cancel_current_task()

    def open_file_dialog(self, options: dict | None = None):
        return open_file_dialog(options or {})

    def open_directory_dialog(self, options: dict | None = None):
        return open_directory_dialog(options or {})

    def runtime_quit(self) -> None:
        runtime_quit()

    def runtime_window_minimise(self) -> None:
        runtime_window_minimise()

    def runtime_window_toggle_maximise(self) -> None:
        runtime_window_toggle_maximise()

    def resolve_file_paths(self, files: list[Any]) -> list[str]:
        resolved: list[str] = []
        for item in files:
            path = _extract_runtime_file_path(item)
            if not path:
                continue
            resolved.append(path)
        return resolved

    def can_resolve_file_paths(self) -> bool:
        return True

    def on_file_drop(self, *_args, **_kwargs) -> None:
        return None

    def on_file_drop_off(self) -> None:
        return None


# PascalCase aliases kept for the frontend bridge contract (historical Wails
# naming). pywebview exposes them like regular methods because the alias shares
# the underlying function (and therefore its signature).
_PASCAL_ALIASES = {
    "Ping": "ping",
    "GetSettings": "get_settings",
    "GetAppVersion": "get_app_version",
    "CheckForUpdates": "check_for_updates",
    "OpenPath": "open_path",
    "SaveSettings": "save_settings",
    "UpdateRecentPaths": "update_recent_paths",
    "SelectInputFiles": "select_input_files",
    "SelectInputDirectory": "select_input_directory",
    "SelectOutputDirectory": "select_output_directory",
    "ExpandDroppedPaths": "expand_dropped_paths",
    "ResolveOutputPath": "resolve_output_path",
    "ResolveOutputPaths": "resolve_output_paths",
    "ListSystemFonts": "list_system_fonts",
    "GetImagePreview": "get_image_preview",
    "Convert": "convert",
    "ConvertBatch": "convert_batch",
    "Compress": "compress",
    "CompressBatch": "compress_batch",
    "GeneratePDF": "generate_pdf",
    "SplitGIF": "split_gif",
    "ProbeAnimatedPaths": "probe_animated_paths",
    "GenerateSubtitleLongImage": "generate_subtitle_long_image",
    "AddWatermark": "add_watermark",
    "AddWatermarkBatch": "add_watermark_batch",
    "Adjust": "adjust",
    "AdjustBatch": "adjust_batch",
    "ApplyFilter": "apply_filter",
    "ApplyFilterBatch": "apply_filter_batch",
    "ProcessImage": "process_image",
    "ProcessImageBatch": "process_image_batch",
    "CancelProcessing": "cancel_processing",
    "OpenFileDialog": "open_file_dialog",
    "OpenDirectoryDialog": "open_directory_dialog",
    "Quit": "runtime_quit",
    "WindowMinimise": "runtime_window_minimise",
    "WindowToggleMaximise": "runtime_window_toggle_maximise",
    "ResolveFilePaths": "resolve_file_paths",
    "CanResolveFilePaths": "can_resolve_file_paths",
}

for _alias, _target in _PASCAL_ALIASES.items():
    setattr(DesktopAPI, _alias, getattr(DesktopAPI, _target))
del _alias, _target

from __future__ import annotations

import base64
import io
import os
from collections import OrderedDict
from dataclasses import asdict
from pathlib import Path
from threading import Lock
from typing import Any

from backend.application.task_manager import TaskManager


def expand_input_paths(paths: list[str]) -> dict:
    from backend.domain.paths import expand_input_paths as expand_paths

    return expand_paths(paths)



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
    for field in ("input_path", "output_path", "output_dir", "image_path", "path"):
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
_probe_cache: OrderedDict[tuple[str, int, int], dict[str, Any]] = OrderedDict()
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
                _probe_cache.move_to_end(key)
                return dict(cached)
    result = _probe_animated_path(input_path)
    if key is not None and not result.get("error"):
        with _probe_cache_lock:
            _probe_cache[key] = dict(result)
            _probe_cache.move_to_end(key)
            if len(_probe_cache) > _PROBE_CACHE_MAX:
                _probe_cache.popitem(last=False)
    return result


_THUMBNAIL_CACHE_LOCK = Lock()
_THUMBNAIL_CACHE: OrderedDict[tuple[str, int, int, int], dict[str, Any]] = OrderedDict()
_THUMBNAIL_CACHE_MAX = 256
_THUMBNAIL_SOURCE_SUFFIXES = {
    ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp",
    ".avif", ".heic", ".heif", ".ico", ".tiff", ".tif",
}
_HEIF_MAIN_REGISTERED = False
_DISK_CACHE_WRITE_COUNTER = {"n": 0}
_DISK_CACHE_KEEP_FILES = 800
_DISK_CACHE_PRUNE_EVERY = 128


def _thumbnail_disk_dir() -> Path | None:
    base = os.getenv("APPDATA") or str(Path.home() / ".config")
    directory = Path(base) / "imageflow" / "thumb_cache"
    try:
        directory.mkdir(parents=True, exist_ok=True)
        return directory
    except OSError:
        return None


def _prune_thumbnail_disk_cache(directory: Path) -> None:
    try:
        entries = [entry for entry in directory.iterdir() if entry.is_file()]
        if len(entries) <= _DISK_CACHE_KEEP_FILES:
            return
        entries.sort(key=lambda entry: entry.stat().st_mtime)
        for stale in entries[: len(entries) - _DISK_CACHE_KEEP_FILES]:
            try:
                stale.unlink()
            except OSError:
                pass
    except OSError:
        pass


def _ensure_main_process_heif() -> None:
    global _HEIF_MAIN_REGISTERED
    if _HEIF_MAIN_REGISTERED:
        return
    try:
        import pillow_heif

        pillow_heif.register_heif_opener()
    except Exception:
        pass
    _HEIF_MAIN_REGISTERED = True


def _thumbnail_cache_key(path: str, max_edge: int) -> tuple[str, int, int, int] | None:
    try:
        st = Path(path).stat()
    except OSError:
        return None
    mtime_ns = int(getattr(st, "st_mtime_ns", int(st.st_mtime * 1_000_000_000)))
    return (path, mtime_ns, int(st.st_size), max_edge)


def _render_thumbnail_bytes(path: str, suffix: str, max_edge: int) -> tuple[bytes, str, int, int]:
    from PIL import Image, ImageOps

    if suffix == ".svg":
        from backend.engines._svg_ops import assert_svg_render_safe

        assert_svg_render_safe(path)
        import resvg_py

        png_bytes = resvg_py.svg_to_bytes(svg_path=path, width=max_edge)
        img = Image.open(io.BytesIO(bytes(png_bytes)))
    else:
        _ensure_main_process_heif()
        img = Image.open(path)

    original_img = img
    with original_img:
        img.load()
        oriented = ImageOps.exif_transpose(img)
        if oriented is not img:
            img.close()
            img = oriented
        has_alpha = img.mode in ("RGBA", "LA") or (img.mode == "P" and "transparency" in img.info)
        img.thumbnail((max_edge, max_edge))
        encode = img
        if has_alpha and img.mode != "RGBA":
            encode = img.convert("RGBA")
        elif not has_alpha and img.mode not in ("RGB", "L"):
            encode = img.convert("RGB")
        fmt = "PNG" if has_alpha else "JPEG"
        mime = "image/png" if has_alpha else "image/jpeg"
        buffer = io.BytesIO()
        encode.save(buffer, format=fmt, quality=80)
        out_width, out_height = encode.size
        if encode is not img:
            encode.close()
        if img is not original_img:
            img.close()
        return buffer.getvalue(), mime, out_width, out_height


def _load_or_render_thumbnail(key: tuple[str, int, int, int], suffix: str) -> dict[str, Any]:
    path, mtime_ns, size, max_edge = key
    import hashlib

    digest = hashlib.md5(f"{path}|{mtime_ns}|{size}|{max_edge}".encode("utf-8")).hexdigest()

    directory = _thumbnail_disk_dir()
    disk_file: Path | None = None
    if directory is not None:
        disk_file = directory / f"{digest}.bin"

    if disk_file is not None and disk_file.is_file():
        try:
            raw = disk_file.read_bytes()
            if raw.startswith(b"\x89PNG"):
                mime = "image/png"
            elif raw.startswith(b"\xff\xd8"):
                mime = "image/jpeg"
            else:
                raise ValueError("unknown thumbnail blob")
            with _THUMBNAIL_CACHE_LOCK:
                _DISK_CACHE_WRITE_COUNTER["n"] += 1  # touch counter so pruning stays active
            data_url = f"data:{mime};base64,{base64.b64encode(raw).decode('ascii')}"
            # Dimensions are unknown without decoding; the browser scales anyway.
            return {"data_url": data_url, "width": 0, "height": 0}
        except Exception:
            try:
                disk_file.unlink()
            except OSError:
                pass

    raw, mime, width, height = _render_thumbnail_bytes(path, suffix, max_edge)

    if disk_file is not None:
        try:
            disk_file.write_bytes(raw)
            _DISK_CACHE_WRITE_COUNTER["n"] += 1
            if _DISK_CACHE_WRITE_COUNTER["n"] % _DISK_CACHE_PRUNE_EVERY == 0 and directory is not None:
                _prune_thumbnail_disk_cache(directory)
        except OSError:
            pass

    data_url = f"data:{mime};base64,{base64.b64encode(raw).decode('ascii')}"
    return {"data_url": data_url, "width": width, "height": height}


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
            strategy = str(payload.get("strategy") or "rename").strip().lower()
            if strategy == "overwrite":
                return {"success": True, "output_path": base, "exists": os.path.exists(base)}
            if strategy == "skip":
                return {
                    "success": True,
                    "output_path": base,
                    "exists": os.path.exists(base),
                }
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
            strategy = str(payload.get("strategy") or "rename").strip().lower()
            reserved = [str(item) for item in (payload.get("reserved") or []) if str(item).strip()]
            resolved: list[str] = []
            exists_flags: list[bool] | None = [] if strategy in ("overwrite", "skip") else None
            for raw in items:
                base = normalize_user_supplied_path(str(raw or ""))
                if strategy in ("overwrite", "skip"):
                    output_path = base
                else:
                    output_path = resolve_output_path(base, reserved)
                    reserved.append(output_path)
                resolved.append(output_path)
                if exists_flags is not None:
                    exists_flags.append(os.path.exists(output_path))
            result = {"success": True, "paths": resolved}
            if exists_flags is not None:
                result["exists"] = exists_flags
            return result
        except Exception as exc:
            return {"success": False, "error": str(exc), "paths": []}

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

    def cut_image_grid(self, payload: dict) -> dict:
        return self._run_engine_operation("grid_cutter", payload)

    def split_image(self, payload: dict) -> dict:
        return self._run_engine_operation("image_splitter", payload)

    def add_watermark(self, payload: dict) -> dict:
        return self._run_engine_operation("watermark", payload)

    def resize_image(self, payload: dict) -> dict:
        return self._run_engine_operation("resizer", payload)

    def palette_tool(self, payload: dict) -> dict:
        return self._run_engine_operation("palette", payload)

    def stitch_images(self, payload: dict) -> dict:
        return self._run_engine_operation("stitcher", payload)

    def render_rich_text(self, payload: dict) -> dict:
        return self._run_engine_operation("richtext", payload)

    def probe_animated_paths(self, paths: list[str]) -> list[dict]:
        normalized_paths = [str(path) for path in paths if str(path).strip()]
        return [_probe_animated_path_cached(path) for path in normalized_paths]

    def get_thumbnail(self, payload: dict) -> dict:
        try:
            data = payload if isinstance(payload, dict) else {}
            path = normalize_user_supplied_path(str(data.get("path") or ""))
            try:
                max_edge = int(data.get("max_edge") or 160)
            except (TypeError, ValueError):
                max_edge = 160
            max_edge = max(32, min(1536, max_edge))
            suffix = Path(path).suffix.lower()
            if suffix != ".svg" and suffix not in _THUMBNAIL_SOURCE_SUFFIXES:
                return {"success": False, "error": "[UNSUPPORTED_FORMAT] unsupported thumbnail format"}

            key = _thumbnail_cache_key(path, max_edge)
            if key is not None:
                with _THUMBNAIL_CACHE_LOCK:
                    cached = _THUMBNAIL_CACHE.get(key)
                    if cached is not None:
                        _THUMBNAIL_CACHE.move_to_end(key)
                        return {"success": True, **cached}

            value = _load_or_render_thumbnail(key or ("", 0, 0, max_edge), suffix)
            if key is not None and value.get("width"):
                with _THUMBNAIL_CACHE_LOCK:
                    _THUMBNAIL_CACHE[key] = value
                    _THUMBNAIL_CACHE.move_to_end(key)
                    if len(_THUMBNAIL_CACHE) > _THUMBNAIL_CACHE_MAX:
                        _THUMBNAIL_CACHE.popitem(last=False)
            return {"success": True, **value}
        except FileNotFoundError:
            return {"success": False, "error": "[NOT_FOUND] file not found"}
        except Exception as exc:
            return {"success": False, "error": str(exc)}


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
        from backend.infrastructure.tray import stop_tray

        stop_tray()
        import webview

        for window in list(webview.windows):
            try:
                window.destroy()
            except Exception:
                pass

    def close_request(self) -> dict:
        """Close button behavior: hide to tray when background_running is on."""
        background = False
        try:
            from backend.infrastructure.settings_store import load_settings

            settings = load_settings()
            background = bool(getattr(settings, "background_running", False))
        except Exception:
            background = False

        if not background:
            self.runtime_quit()
            return {"success": True, "hidden": False}

        try:
            import webview

            from backend.infrastructure.tray import ensure_tray

            def _show() -> None:
                if webview.windows:
                    target = webview.windows[0]
                    try:
                        target.restore()
                    except Exception:
                        pass
                    target.show()

            def _quit() -> None:
                self.runtime_quit()

            tray_ready = ensure_tray(_show, _quit)
            if not tray_ready:
                # Never hide the last window unless a working tray entry is
                # available to restore it.
                self.runtime_quit()
                return {"success": True, "hidden": False}
            if webview.windows:
                webview.windows[0].hide()
            return {"success": True, "hidden": True}
        except Exception:
            self.runtime_quit()
            return {"success": True, "hidden": False}

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
    "Convert": "convert",
    "ConvertBatch": "convert_batch",
    "Compress": "compress",
    "CompressBatch": "compress_batch",
    "GeneratePDF": "generate_pdf",
    "SplitGIF": "split_gif",
    "CutImageGrid": "cut_image_grid",
    "SplitImage": "split_image",
    "AddWatermark": "add_watermark",
    "ResizeImage": "resize_image",
    "PaletteTool": "palette_tool",
    "StitchImages": "stitch_images",
    "RenderRichText": "render_rich_text",
    "ProbeAnimatedPaths": "probe_animated_paths",
    "GetThumbnail": "get_thumbnail",
    "CancelProcessing": "cancel_processing",
    "OpenFileDialog": "open_file_dialog",
    "OpenDirectoryDialog": "open_directory_dialog",
    "Quit": "runtime_quit",
    "CloseRequest": "close_request",
    "WindowMinimise": "runtime_window_minimise",
    "WindowToggleMaximise": "runtime_window_toggle_maximise",
    "ResolveFilePaths": "resolve_file_paths",
    "CanResolveFilePaths": "can_resolve_file_paths",
}

for _alias, _target in _PASCAL_ALIASES.items():
    setattr(DesktopAPI, _alias, getattr(DesktopAPI, _target))
del _alias, _target

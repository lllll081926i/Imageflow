from __future__ import annotations

from dataclasses import asdict
from pathlib import Path
from typing import Any

from PIL import Image, UnidentifiedImageError

from backend.application.image_ops import execute_engine, execute_engine_batch
from backend.application.preview import build_image_preview
from backend.application.task_manager import TaskManager
from backend.contracts.settings import AppSettings
from backend.domain.paths import (
    expand_input_paths,
    list_system_fonts,
    normalize_optional_user_supplied_path,
    normalize_user_supplied_path,
    resolve_output_path,
)
from backend.infrastructure.dialogs import open_directory_dialog, open_file_dialog
from backend.infrastructure.settings_store import load_settings, save_settings, settings_from_dict
from backend.infrastructure.window_ops import (
    runtime_quit,
    runtime_window_minimise,
    runtime_window_toggle_maximise,
)


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
    for field in ("input_paths", "image_paths", "images"):
        if field in normalized and isinstance(normalized[field], list):
            normalized[field] = [normalize_user_supplied_path(str(item)) for item in normalized[field] if str(item).strip()]
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
    raw_path = str(input_path or "").strip()
    try:
        normalized_path = normalize_user_supplied_path(raw_path)
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


class DesktopAPI:
    def __init__(self, task_manager: TaskManager | None = None):
        self._task_manager = task_manager or TaskManager()

    def _settings(self) -> AppSettings:
        return load_settings()

    def _run_operation(self, handler):
        task_id = self._task_manager.begin_task("operation")
        try:
            return handler()
        finally:
            self._task_manager.finish_task(task_id)

    def ping(self) -> str:
        return "pong"

    def get_settings(self) -> dict:
        return asdict(self._settings())

    def save_settings(self, payload: dict) -> dict:
        saved = save_settings(settings_from_dict(payload))
        return asdict(saved)

    def update_recent_paths(self, payload: dict) -> dict:
        current = self._settings()
        current.recent_input_dirs = _merge_recent_paths(current.recent_input_dirs, str(payload.get("input_dir") or ""))
        current.recent_output_dirs = _merge_recent_paths(current.recent_output_dirs, str(payload.get("output_dir") or ""))
        saved = save_settings(current)
        return asdict(saved)

    def select_input_files(self) -> list[str]:
        result = open_file_dialog({"title": "选择文件", "allowsMultipleSelection": True})
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
        normalized = [normalize_user_supplied_path(path) for path in paths if str(path).strip()]
        return expand_input_paths(normalized)

    def resolve_output_path(self, payload: dict) -> dict:
        try:
            base = normalize_user_supplied_path(str(payload.get("base_path") or ""))
            reserved = [str(item) for item in payload.get("reserved") or []]
            output_path = resolve_output_path(base, reserved)
            return {"success": True, "output_path": output_path}
        except Exception as exc:
            return {"success": False, "error": str(exc)}

    def list_system_fonts(self) -> list[str]:
        return list_system_fonts()

    def get_image_preview(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return build_image_preview(normalized["input_path"])

    def get_info(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("info_viewer", normalized, self._task_manager))

    def edit_metadata(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        normalized["action"] = "edit_exif"
        return self._run_operation(lambda: execute_engine("info_viewer", normalized, self._task_manager))

    def strip_metadata(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        normalized["action"] = "strip_metadata"
        return self._run_operation(lambda: execute_engine("metadata_tool", normalized, self._task_manager))

    def convert(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("converter", normalized, self._task_manager))

    def convert_batch(self, payloads: list[dict]) -> list[dict]:
        normalized = [_normalize_payload_paths(item) for item in payloads]
        return self._run_operation(
            lambda: execute_engine_batch("converter", normalized, self._settings(), self._task_manager)
        )

    def compress(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("compressor", normalized, self._task_manager))

    def compress_batch(self, payloads: list[dict]) -> list[dict]:
        normalized = [_normalize_payload_paths(item) for item in payloads]
        return self._run_operation(
            lambda: execute_engine_batch("compressor", normalized, self._settings(), self._task_manager)
        )

    def generate_pdf(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("pdf_generator", normalized, self._task_manager))

    def split_gif(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("gif_splitter", normalized, self._task_manager))

    def probe_animated_paths(self, paths: list[str]) -> list[dict]:
        normalized_paths = [str(path) for path in paths if str(path).strip()]
        return [_probe_animated_path(path) for path in normalized_paths]

    def generate_subtitle_long_image(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("subtitle_stitcher", normalized, self._task_manager))

    def add_watermark(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("watermark", normalized, self._task_manager))

    def add_watermark_batch(self, payloads: list[dict]) -> list[dict]:
        normalized = [_normalize_payload_paths(item) for item in payloads]
        return self._run_operation(
            lambda: execute_engine_batch("watermark", normalized, self._settings(), self._task_manager)
        )

    def adjust(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("adjuster", normalized, self._task_manager))

    def adjust_batch(self, payloads: list[dict]) -> list[dict]:
        normalized = [_normalize_payload_paths(item) for item in payloads]
        return self._run_operation(
            lambda: execute_engine_batch("adjuster", normalized, self._settings(), self._task_manager)
        )

    def apply_filter(self, payload: dict) -> dict:
        normalized = _normalize_payload_paths(payload)
        return self._run_operation(lambda: execute_engine("filter", normalized, self._task_manager))

    def apply_filter_batch(self, payloads: list[dict]) -> list[dict]:
        normalized = [_normalize_payload_paths(item) for item in payloads]
        return self._run_operation(
            lambda: execute_engine_batch("filter", normalized, self._settings(), self._task_manager)
        )

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
                return []
            resolved.append(path)
        return resolved

    def can_resolve_file_paths(self) -> bool:
        return True

    def on_file_drop(self, *_args, **_kwargs) -> None:
        return None

    def on_file_drop_off(self) -> None:
        return None

    def Ping(self) -> str:
        return self.ping()

    def GetSettings(self) -> dict:
        return self.get_settings()

    def SaveSettings(self, payload: dict) -> dict:
        return self.save_settings(payload)

    def UpdateRecentPaths(self, payload: dict) -> dict:
        return self.update_recent_paths(payload)

    def SelectInputFiles(self) -> list[str]:
        return self.select_input_files()

    def SelectInputDirectory(self) -> str:
        return self.select_input_directory()

    def SelectOutputDirectory(self) -> str:
        return self.select_output_directory()

    def ExpandDroppedPaths(self, paths: list[str]) -> dict:
        return self.expand_dropped_paths(paths)

    def ResolveOutputPath(self, payload: dict) -> dict:
        return self.resolve_output_path(payload)

    def ListSystemFonts(self) -> list[str]:
        return self.list_system_fonts()

    def GetImagePreview(self, payload: dict) -> dict:
        return self.get_image_preview(payload)

    def GetInfo(self, payload: dict) -> dict:
        return self.get_info(payload)

    def EditMetadata(self, payload: dict) -> dict:
        return self.edit_metadata(payload)

    def StripMetadata(self, payload: dict) -> dict:
        return self.strip_metadata(payload)

    def Convert(self, payload: dict) -> dict:
        return self.convert(payload)

    def ConvertBatch(self, payloads: list[dict]) -> list[dict]:
        return self.convert_batch(payloads)

    def Compress(self, payload: dict) -> dict:
        return self.compress(payload)

    def CompressBatch(self, payloads: list[dict]) -> list[dict]:
        return self.compress_batch(payloads)

    def GeneratePDF(self, payload: dict) -> dict:
        return self.generate_pdf(payload)

    def SplitGIF(self, payload: dict) -> dict:
        return self.split_gif(payload)

    def ProbeAnimatedPaths(self, paths: list[str]) -> list[dict]:
        return self.probe_animated_paths(paths)

    def GenerateSubtitleLongImage(self, payload: dict) -> dict:
        return self.generate_subtitle_long_image(payload)

    def AddWatermark(self, payload: dict) -> dict:
        return self.add_watermark(payload)

    def AddWatermarkBatch(self, payloads: list[dict]) -> list[dict]:
        return self.add_watermark_batch(payloads)

    def Adjust(self, payload: dict) -> dict:
        return self.adjust(payload)

    def AdjustBatch(self, payloads: list[dict]) -> list[dict]:
        return self.adjust_batch(payloads)

    def ApplyFilter(self, payload: dict) -> dict:
        return self.apply_filter(payload)

    def ApplyFilterBatch(self, payloads: list[dict]) -> list[dict]:
        return self.apply_filter_batch(payloads)

    def CancelProcessing(self) -> bool:
        return self.cancel_processing()

    def OpenFileDialog(self, options: dict | None = None):
        return self.open_file_dialog(options)

    def OpenDirectoryDialog(self, options: dict | None = None):
        return self.open_directory_dialog(options)

    def Quit(self) -> None:
        self.runtime_quit()

    def WindowMinimise(self) -> None:
        self.runtime_window_minimise()

    def WindowToggleMaximise(self) -> None:
        self.runtime_window_toggle_maximise()

    def ResolveFilePaths(self, files: list[Any]) -> list[str]:
        return self.resolve_file_paths(files)

    def CanResolveFilePaths(self) -> bool:
        return self.can_resolve_file_paths()

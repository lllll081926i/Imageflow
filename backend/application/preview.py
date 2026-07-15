import base64
import io
import os
import threading
import time
from multiprocessing import Process, Queue
from pathlib import Path
from queue import Empty
from typing import Any

from backend.infrastructure.engine_loader import load_engine_module

DEFAULT_PREVIEW_MAX_BYTES = 4 * 1024 * 1024
PREVIEW_MAX_EDGE = 1280
PREVIEW_JPEG_QUALITY = 80
PREVIEW_PROCESS_TIMEOUT_SECONDS = 20.0
PREVIEW_CACHE_MAX_ENTRIES = 64
_ISOLATE_EXTENSIONS = {".svg"}

_preview_cache_lock = threading.Lock()
_preview_cache: dict[tuple[str, int, int], tuple[float, dict[str, Any]]] = {}


def _resolve_preview_max_bytes() -> int:
    raw_value = str(os.getenv("IMAGEFLOW_PREVIEW_MAX_BYTES", str(DEFAULT_PREVIEW_MAX_BYTES)) or "").strip()
    try:
        parsed = int(raw_value)
    except (TypeError, ValueError):
        return DEFAULT_PREVIEW_MAX_BYTES
    return parsed if parsed > 0 else DEFAULT_PREVIEW_MAX_BYTES


def _cache_key(source: Path) -> tuple[str, int, int] | None:
    try:
        stat = source.stat()
    except OSError:
        return None
    return (str(source), int(getattr(stat, "st_mtime_ns", int(stat.st_mtime * 1_000_000_000))), int(stat.st_size))


def _cache_get(key: tuple[str, int, int]) -> dict[str, Any] | None:
    with _preview_cache_lock:
        item = _preview_cache.get(key)
        if item is None:
            return None
        _preview_cache[key] = (time.monotonic(), item[1])
        return dict(item[1])


def _cache_put(key: tuple[str, int, int], value: dict[str, Any]) -> None:
    if not value.get("success"):
        return
    with _preview_cache_lock:
        _preview_cache[key] = (time.monotonic(), dict(value))
        if len(_preview_cache) > PREVIEW_CACHE_MAX_ENTRIES:
            oldest_key = min(_preview_cache.items(), key=lambda item: item[1][0])[0]
            _preview_cache.pop(oldest_key, None)


def _should_isolate(path: Path) -> bool:
    forced = str(os.getenv("IMAGEFLOW_PREVIEW_ISOLATE", "") or "").strip().lower()
    if forced in {"1", "true", "yes", "on", "always"}:
        return True
    if forced in {"0", "false", "no", "off", "never"}:
        return False
    return path.suffix.lower() in _ISOLATE_EXTENSIONS


def build_image_preview(input_path: str) -> dict[str, Any]:
    from PIL import Image

    # Preview path should never accept decompression bombs even if caller raised the global limit.
    Image.MAX_IMAGE_PIXELS = min(int(getattr(Image, "MAX_IMAGE_PIXELS", 0) or 64_000_000), 32_000_000)

    source = Path(input_path)
    if not source.exists():
        return {"success": False, "error": "文件不存在"}

    cache_key = _cache_key(source)
    if cache_key is not None:
        cached = _cache_get(cache_key)
        if cached is not None:
            return cached

    max_bytes = _resolve_preview_max_bytes()
    try:
        file_size = source.stat().st_size
    except OSError:
        return {"success": False, "error": "文件不存在"}
    if file_size > max_bytes:
        return {"success": False, "error": "PREVIEW_SKIPPED"}

    converter = load_engine_module("converter")
    open_image = getattr(converter, "open_image_with_svg_support")
    is_svg_path = getattr(converter, "is_svg_path", None)

    image: Image.Image | None = None
    raw: Image.Image | None = None
    try:
        if callable(is_svg_path) and is_svg_path(str(source)):
            raw = open_image(str(source), format_type="jpg")
        else:
            raw = Image.open(str(source))
            # Prefer decoder draft when available (JPEG) to reduce decode cost before thumbnail.
            try:
                raw.draft("RGB", (PREVIEW_MAX_EDGE, PREVIEW_MAX_EDGE))
            except Exception:
                pass

        if raw.mode not in ("RGB", "L"):
            image = raw.convert("RGB")
            raw.close()
            raw = None
        else:
            image = raw
            raw = None

        image.thumbnail((PREVIEW_MAX_EDGE, PREVIEW_MAX_EDGE), Image.Resampling.BILINEAR)
        buffer = io.BytesIO()
        image.save(buffer, format="JPEG", quality=PREVIEW_JPEG_QUALITY, optimize=False)
        encoded = base64.b64encode(buffer.getvalue()).decode("ascii")
        result = {"success": True, "data_url": f"data:image/jpeg;base64,{encoded}"}
        if cache_key is not None:
            _cache_put(cache_key, result)
        return result
    except Exception as exc:
        return {"success": False, "error": str(exc)}
    finally:
        if raw is not None:
            try:
                raw.close()
            except Exception:
                pass
        if image is not None:
            try:
                image.close()
            except Exception:
                pass


def _preview_worker(input_path: str, queue: Queue) -> None:
    try:
        queue.put(build_image_preview(input_path))
    except Exception as exc:
        queue.put({"success": False, "error": str(exc)})


def build_image_preview_isolated(input_path: str, timeout: float = PREVIEW_PROCESS_TIMEOUT_SECONDS) -> dict[str, Any]:
    """Generate previews in a child process so SVG/decode bombs cannot freeze the host."""
    queue: Queue = Queue()
    process = Process(target=_preview_worker, args=(str(input_path), queue))
    try:
        process.start()
        process.join(timeout)
        if process.is_alive():
            process.terminate()
            process.join(2)
            if process.is_alive():
                process.kill()
                process.join(1)
            return {"success": False, "error": "预览超时"}
        try:
            result = queue.get_nowait()
        except Empty:
            exitcode = process.exitcode
            return {"success": False, "error": f"预览失败 (exit code: {exitcode})"}
        if isinstance(result, dict):
            return result
        return {"success": False, "error": "预览返回格式异常"}
    finally:
        close = getattr(queue, "close", None)
        if callable(close):
            close()
        join_thread = getattr(queue, "join_thread", None)
        if callable(join_thread):
            join_thread()


def build_image_preview_smart(input_path: str) -> dict[str, Any]:
    """Prefer in-process preview for ordinary bitmaps; isolate only risky inputs."""
    source = Path(str(input_path or ""))
    if _should_isolate(source):
        return build_image_preview_isolated(str(source))
    return build_image_preview(str(source))

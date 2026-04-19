import base64
import io
import os
from pathlib import Path
from typing import Any

from PIL import Image

from backend.infrastructure.engine_loader import load_engine_module

DEFAULT_PREVIEW_MAX_BYTES = 4 * 1024 * 1024
PREVIEW_MAX_EDGE = 1280
PREVIEW_JPEG_QUALITY = 85


def _resolve_preview_max_bytes() -> int:
    raw_value = str(os.getenv("IMAGEFLOW_PREVIEW_MAX_BYTES", str(DEFAULT_PREVIEW_MAX_BYTES)) or "").strip()
    try:
        parsed = int(raw_value)
    except (TypeError, ValueError):
        return DEFAULT_PREVIEW_MAX_BYTES
    return parsed if parsed > 0 else DEFAULT_PREVIEW_MAX_BYTES


def build_image_preview(input_path: str) -> dict[str, Any]:
    source = Path(input_path)
    if not source.exists():
        return {"success": False, "error": "文件不存在"}

    max_bytes = _resolve_preview_max_bytes()
    if source.stat().st_size > max_bytes:
        return {"success": False, "error": "PREVIEW_SKIPPED"}

    converter = load_engine_module("converter")
    open_image = getattr(converter, "open_image_with_svg_support")

    image: Image.Image | None = None
    try:
        image = open_image(str(source), format_type="jpg")
        if image.mode not in ("RGB", "L"):
            image = image.convert("RGB")
        else:
            image = image.copy()

        image.thumbnail((PREVIEW_MAX_EDGE, PREVIEW_MAX_EDGE), Image.Resampling.LANCZOS)
        buffer = io.BytesIO()
        image.save(buffer, format="JPEG", quality=PREVIEW_JPEG_QUALITY, optimize=True)
        encoded = base64.b64encode(buffer.getvalue()).decode("ascii")
        return {"success": True, "data_url": f"data:image/jpeg;base64,{encoded}"}
    except Exception as exc:
        return {"success": False, "error": str(exc)}
    finally:
        if image is not None:
            try:
                image.close()
            except Exception:
                pass

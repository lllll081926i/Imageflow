#!/usr/bin/env python3
"""
Subtitle Stitcher

Generate one long image from sequential movie screenshots:
- First frame keeps full image
- Frames 2..N keep only the bottom subtitle strip
- Optional neighbor-strip deduplication
"""

import json
import logging
import os
import sys
from typing import List, Optional

from PIL import Image, UnidentifiedImageError

logger = logging.getLogger(__name__)

DEFAULT_SUBTITLE_RATIO = 0.18
MIN_SUBTITLE_RATIO = 0.10
MAX_SUBTITLE_RATIO = 0.35
DEFAULT_MIN_STRIP_HEIGHT = 24
DEFAULT_DEDUP_THRESHOLD = 2


def _error_response(code: str, message: str, detail: Optional[Exception] = None):
    payload = {"success": False, "error": message, "error_code": code}
    if detail is not None:
        payload["error_detail"] = str(detail)
    return payload


def _coerce_bool(value, default=False) -> bool:
    if value is None:
        return default
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        lowered = value.strip().lower()
        if lowered in ("1", "true", "yes", "on"):
            return True
        if lowered in ("0", "false", "no", "off"):
            return False
    return bool(value)


def _sanitize_ratio(value) -> float:
    try:
        ratio = float(value)
    except (TypeError, ValueError):
        ratio = DEFAULT_SUBTITLE_RATIO
    if ratio <= 0:
        ratio = DEFAULT_SUBTITLE_RATIO
    return max(MIN_SUBTITLE_RATIO, min(MAX_SUBTITLE_RATIO, ratio))


def _sanitize_int(value, default: int, minimum: int = 0) -> int:
    try:
        parsed = int(value)
    except (TypeError, ValueError):
        parsed = default
    if parsed < minimum:
        return minimum
    return parsed


def _coerce_input_paths(input_data) -> List[str]:
    paths = input_data.get("input_paths")
    if isinstance(paths, list):
        return [str(path).strip() for path in paths if str(path).strip()]
    single = str(input_data.get("input_path") or "").strip()
    if single:
        return [single]
    return []


def _ensure_parent_dir(path: str):
    parent = os.path.dirname(path)
    if parent:
        os.makedirs(parent, exist_ok=True)


def _dhash(image: Image.Image) -> int:
    resampling = getattr(Image, "Resampling", None)
    filter_name = resampling.LANCZOS if resampling is not None else Image.LANCZOS
    thumb = image.convert("L").resize((9, 8), filter_name)
    try:
        pixels = thumb.load()
        value = 0
        bit_index = 0
        for row in range(8):
            for col in range(8):
                left = pixels[col, row]
                right = pixels[col + 1, row]
                if left > right:
                    value |= (1 << bit_index)
                bit_index += 1
        return value
    finally:
        thumb.close()


def _hamming_distance(hash_a: int, hash_b: int) -> int:
    return (hash_a ^ hash_b).bit_count()


def _output_mode(output_path: str) -> str:
    ext = os.path.splitext(output_path)[1].lower()
    if ext in (".jpg", ".jpeg", ".bmp"):
        return "RGB"
    return "RGBA"


def _normalize_frame_mode(frame: Image.Image, mode: str) -> Image.Image:
    if mode == "RGB":
        if frame.mode == "RGB":
            return frame
        if frame.mode in ("RGBA", "LA"):
            base = Image.new("RGB", frame.size, (0, 0, 0))
            rgba = frame.convert("RGBA")
            rgb = frame.convert("RGB")
            alpha = rgba.split()[3]
            try:
                base.paste(rgb, mask=alpha)
                return base
            finally:
                alpha.close()
                rgb.close()
                rgba.close()
        return frame.convert("RGB")
    if frame.mode == "RGBA":
        return frame
    return frame.convert("RGBA")


def handle_request(input_data):
    action = str(input_data.get("action") or "subtitle_stitch").strip().lower()
    if action not in ("subtitle_stitch", "stitch", "subtitle"):
        return _error_response("SUBTITLE_UNSUPPORTED_ACTION", f"Unsupported action: {action}")

    input_paths = _coerce_input_paths(input_data)
    output_path = str(input_data.get("output_path") or "").strip()
    if not input_paths:
        return _error_response("SUBTITLE_BAD_REQUEST", "Missing input_paths")
    if not output_path:
        return _error_response("SUBTITLE_BAD_REQUEST", "Missing output_path")

    crop_ratio = _sanitize_ratio(input_data.get("subtitle_crop_ratio"))
    header_keep_full = _coerce_bool(input_data.get("header_keep_full"), default=True)
    dedup_enabled = _coerce_bool(input_data.get("dedup_enabled"), default=False)
    dedup_threshold = _sanitize_int(input_data.get("dedup_threshold"), DEFAULT_DEDUP_THRESHOLD, minimum=0)
    minimum_strip_height = _sanitize_int(
        input_data.get("minimum_strip_height"),
        DEFAULT_MIN_STRIP_HEIGHT,
        minimum=1,
    )

    render_mode = _output_mode(output_path)
    blocks: List[Image.Image] = []
    last_strip_hash: Optional[int] = None
    skipped_count = 0
    strip_height = 0
    canvas: Image.Image | None = None

    try:
        for idx, path in enumerate(input_paths):
            with Image.open(path) as image:
                if idx == 0 and header_keep_full:
                    frame = _normalize_frame_mode(image.copy(), render_mode)
                    blocks.append(frame)
                    if dedup_enabled:
                        header_strip_height = max(minimum_strip_height, int(round(frame.height * crop_ratio)))
                        header_strip_height = max(1, min(frame.height, header_strip_height))
                        header_strip = frame.crop((0, frame.height - header_strip_height, frame.width, frame.height))
                        try:
                            last_strip_hash = _dhash(header_strip)
                        finally:
                            header_strip.close()
                    continue

                # Non-first frames: extract strip first, then normalize (avoids full-image copy)
                current_strip_height = max(minimum_strip_height, int(round(image.height * crop_ratio)))
                current_strip_height = max(1, min(image.height, current_strip_height))
                strip_height = current_strip_height
                raw_strip = image.crop((0, image.height - current_strip_height, image.width, image.height))
                strip = _normalize_frame_mode(raw_strip.copy(), render_mode)
                raw_strip.close()

            current_hash = _dhash(strip)
            if dedup_enabled and last_strip_hash is not None:
                distance = _hamming_distance(last_strip_hash, current_hash)
                if distance <= dedup_threshold:
                    skipped_count += 1
                    strip.close()
                    continue

            last_strip_hash = current_hash
            blocks.append(strip)

        if not blocks:
            return _error_response("SUBTITLE_EMPTY_RESULT", "No image content available after processing")

        canvas_width = max(block.width for block in blocks)
        canvas_height = sum(block.height for block in blocks)
        background = (0, 0, 0, 0) if render_mode == "RGBA" else (0, 0, 0)
        canvas = Image.new(render_mode, (canvas_width, canvas_height), background)

        y = 0
        for block in blocks:
            x = max(0, (canvas_width - block.width) // 2)
            canvas.paste(block, (x, y))
            y += block.height

        _ensure_parent_dir(output_path)
        canvas.save(output_path)

        return {
            "success": True,
            "output_path": output_path,
            "input_count": len(input_paths),
            "kept_count": len(blocks),
            "skipped_count": skipped_count,
            "strip_height": strip_height,
        }
    except FileNotFoundError as exc:
        return _error_response("SUBTITLE_INPUT_NOT_FOUND", f"Input file not found: {exc.filename}")
    except UnidentifiedImageError:
        return _error_response("SUBTITLE_UNSUPPORTED_IMAGE", "Unsupported image format in input_paths")
    except Exception as exc:
        logger.error("subtitle stitch failed: %s", exc, exc_info=True)
        return _error_response("SUBTITLE_STITCH_FAILED", str(exc))
    finally:
        if canvas is not None:
            canvas.close()
        for block in blocks:
            block.close()


def process(input_data):
    try:
        return handle_request(input_data)
    except Exception as exc:
        logger.error("process failed: %s", exc, exc_info=True)
        return _error_response("SUBTITLE_INTERNAL_ERROR", str(exc))


def main():
    try:
        input_data = json.load(sys.stdin)
        result = handle_request(input_data)
        json.dump(result, sys.stdout)
    except json.JSONDecodeError as exc:
        json.dump(_error_response("SUBTITLE_INVALID_JSON", f"Invalid JSON input: {exc}"), sys.stdout)
    except Exception as exc:
        logger.error("unexpected error: %s", exc, exc_info=True)
        json.dump(_error_response("SUBTITLE_INTERNAL_ERROR", str(exc)), sys.stdout)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
Stitcher Engine — combine multiple images into one long image.

- direction: "vertical" (default) or "horizontal"
- target_width / target_height: every input is scaled to match (default:
  the median width/height across inputs so nothing is upscaled too hard)
- gap: px spacing filled with background color
- background: "#RRGGBB"
- border_mode "single": treat a lone input as "add simple frame to this image"

Hard limits: any output side <= 16000px and total area <= 256_000_000 px.
"""

import json
import logging
import os
import sys
from pathlib import Path

from PIL import Image, ImageOps

from backend.engines._image_formats import ensure_heif_supported

ensure_heif_supported()

logger = logging.getLogger(__name__)

MAX_SIDE = 16_000
MAX_AREA = 256_000_000


def _error(code, message):
    return {"success": False, "error": message, "error_code": code}


def _parse_color(value, default=(255, 255, 255)):
    text = str(value or "").strip().lstrip("#")
    if len(text) == 3:
        text = "".join(ch * 2 for ch in text)
    if len(text) != 6:
        return default
    try:
        return tuple(int(text[i:i + 2], 16) for i in (0, 2, 4))
    except ValueError:
        return default


def _load_scaled(path, target_length, direction):
    with Image.open(path) as img:
        img.load()
        oriented = ImageOps.exif_transpose(img)
        work = oriented.convert("RGB") if oriented is not img else oriented.copy().convert("RGB")
    width, height = work.size
    if direction == "vertical":
        scale = target_length / width if target_length else 1.0
    else:
        scale = target_length / height if target_length else 1.0
    scale = max(scale, 0.001)
    new_size = (max(1, round(width * scale)), max(1, round(height * scale)))
    return work.resize(new_size, Image.Resampling.LANCZOS)


class Stitcher:
    def stitch(self, payload):
        paths = payload.get("input_paths") or []
        paths = [str(p) for p in paths if str(p).strip()]
        if not paths:
            return _error("STITCH_NO_INPUT", "No input images")

        output_path = str(payload.get("output_path") or "")
        if not output_path:
            return _error("STITCH_BAD_REQUEST", "Missing output_path")

        direction = str(payload.get("direction") or "vertical").lower()
        if direction not in ("vertical", "horizontal"):
            direction = "vertical"
        try:
            gap = max(0, min(500, int(payload.get("gap", 0) or 0)))
        except (TypeError, ValueError):
            gap = 0
        background = _parse_color(payload.get("background"), (255, 255, 255))
        target_length = payload.get("target_width" if direction == "vertical" else "target_height")
        try:
            target_length = max(0, min(MAX_SIDE, int(target_length))) if target_length else 0
        except (TypeError, ValueError):
            target_length = 0

        # Frame mode: single image + optional border padding.
        border_only = bool(payload.get("border_mode")) or len(paths) == 1
        if border_only:
            try:
                border = max(0, min(1000, int(payload.get("border", 40) or 0)))
            except (TypeError, ValueError):
                border = 40

            with Image.open(paths[0]) as img:
                img.load()
                oriented = ImageOps.exif_transpose(img)
                base = oriented.convert("RGB") if oriented is not img else oriented.copy().convert("RGB")
            final = Image.new(
                "RGB",
                (base.width + border * 2, base.height + border * 2),
                background,
            )
            final.paste(base, (border, border))
            base.close()
            out_w, out_h = final.size
            if max(out_w, out_h) > MAX_SIDE or out_w * out_h > MAX_AREA:
                final.close()
                return _error("STITCH_TOO_LARGE", f"Output exceeds limits ({MAX_SIDE}px side / {MAX_AREA // 1_000_000}MP area)")
            parent = Path(output_path).parent
            if str(parent):
                os.makedirs(parent, exist_ok=True)
            fmt = str(payload.get("output_format") or Path(output_path).suffix.lstrip(".") or "png").lower()
            if fmt in ("jpg", "jpeg"):
                final.save(output_path, format="JPEG", quality=95)
            else:
                final.save(output_path, format="PNG")
            final.close()
            return {
                "success": True,
                "output_path": output_path,
                "width": out_w,
                "height": out_h,
                "image_count": 1,
                "mode": "frame",
            }

        missing = [p for p in paths if not Path(p).exists()]
        if missing:
            return _error("STITCH_INPUT_NOT_FOUND", f"Input not found: {missing[0]}")

        images = []
        try:
            probe_sizes = []
            for path in paths:
                with Image.open(path) as probe:
                    probe_sizes.append(probe.size)

            if not target_length:
                lengths = sorted(size[0] if direction == "vertical" else size[1] for size in probe_sizes)
                target_length = lengths[len(lengths) // 2]

            total_along = 0
            scaled_sizes = []
            for path, (w, h) in zip(paths, probe_sizes):
                if direction == "vertical":
                    scaled_h = max(1, round(h * target_length / w))
                    scaled_sizes.append((target_length, scaled_h))
                    total_along += scaled_h
                else:
                    scaled_w = max(1, round(w * target_length / h))
                    scaled_sizes.append((scaled_w, target_length))
                    total_along += scaled_w

            cross = target_length
            along = total_along + gap * (len(paths) - 1)
            out_w, out_h = (cross, along) if direction == "vertical" else (along, cross)
            if max(out_w, out_h) > MAX_SIDE or out_w * out_h > MAX_AREA:
                return _error(
                    "STITCH_TOO_LARGE",
                    f"Output {out_w}x{out_h} exceeds limits ({MAX_SIDE}px / {MAX_AREA // 1_000_000}MP)",
                )

            canvas = Image.new("RGB", (out_w, out_h), background)
            cursor = 0
            for path, size in zip(paths, scaled_sizes):
                scaled = _load_scaled(path, target_length, direction)
                if direction == "vertical":
                    canvas.paste(scaled, (0, cursor))
                else:
                    canvas.paste(scaled, (cursor, 0))
                cursor += (size[1] if direction == "vertical" else size[0]) + gap
                scaled.close()
                images.append(None)

            parent = Path(output_path).parent
            if str(parent):
                os.makedirs(parent, exist_ok=True)
            fmt = str(payload.get("output_format") or Path(output_path).suffix.lstrip(".") or "png").lower()
            if fmt in ("jpg", "jpeg"):
                canvas.save(output_path, format="JPEG", quality=95)
            else:
                canvas.save(output_path, format="PNG")
            canvas.close()

            return {
                "success": True,
                "output_path": output_path,
                "width": out_w,
                "height": out_h,
                "image_count": len(paths),
                "direction": direction,
            }
        except MemoryError:
            return _error("STITCH_MEMORY_LIMIT", "Images are too large to process safely")
        except Exception as exc:
            logger.error("Stitch failed: %s", exc, exc_info=True)
            return _error("STITCH_FAILED", str(exc))


_TOOL = Stitcher()


def process(input_data: dict) -> dict:
    return _TOOL.stitch(input_data)


def main():
    try:
        json.dump(process(json.load(sys.stdin)), sys.stdout)
    except Exception as exc:
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

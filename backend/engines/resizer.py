#!/usr/bin/env python3
"""
Resizer Engine — batch-friendly resize with three modes.

process(input_data) -> dict
    input_path, output_path,
    mode: "percent" (scale_percent 1..1000)
        | "long_edge" (long_edge px)
        | "short_edge" (short_edge px)
    output_format: keep original when omitted; jpg/png/webp supported.
"""

import json
import logging
import sys
from pathlib import Path

from PIL import Image, ImageOps

from backend.engines._image_formats import ensure_heif_supported

ensure_heif_supported()

logger = logging.getLogger(__name__)

SUPPORTED_SAVE = {"jpg", "jpeg", "png", "webp", "bmp", "tiff"}


def _error(code, message):
    return {"success": False, "error": message, "error_code": code}


class Resizer:
    def resize(self, input_path, output_path, mode="percent", value=100, output_format=None, quality=95):
        try:
            source = Path(input_path)
            if not source.exists():
                return _error("RESIZE_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
            if not output_path:
                return _error("RESIZE_BAD_REQUEST", "Missing output_path")
            mode = str(mode or "percent").lower()
            if mode not in ("percent", "long_edge", "short_edge"):
                return _error("RESIZE_BAD_MODE", f"Unsupported mode: {mode}")

            try:
                amount = int(value)
            except (TypeError, ValueError):
                return _error("RESIZE_BAD_VALUE", "Invalid size value")

            with Image.open(source) as img:
                img.load()
                oriented = ImageOps.exif_transpose(img)
                work = oriented.convert("RGBA") if oriented is not img else img.copy().convert("RGBA")
            try:
                width, height = work.size
                if mode == "percent":
                    percent = max(1, min(1000, amount))
                    new_w = max(1, round(width * percent / 100))
                    new_h = max(1, round(height * percent / 100))
                elif mode == "long_edge":
                    edge = max(1, min(30000, amount))
                    scale = edge / max(width, height)
                    new_w = max(1, round(width * scale))
                    new_h = max(1, round(height * scale))
                else:
                    edge = max(1, min(30000, amount))
                    scale = edge / min(width, height)
                    new_w = max(1, round(width * scale))
                    new_h = max(1, round(height * scale))

                resample = Image.Resampling.LANCZOS
                resized = work.resize((new_w, new_h), resample=resample)
            finally:
                work.close()

            out_format = str(output_format or source.suffix.lstrip(".") or "png").lower()
            if out_format == "jpeg":
                out_format = "jpg"
            if out_format not in SUPPORTED_SAVE:
                out_format = "png"

            parent = Path(output_path).parent
            if str(parent):
                import os

                os.makedirs(parent, exist_ok=True)

            save_image = resized
            if out_format in ("jpg", "jpeg"):
                flat = Image.new("RGB", resized.size, (255, 255, 255))
                flat.paste(resized, mask=resized.split()[-1])
                resized.close()
                save_image = flat
            try:
                if out_format in ("jpg", "jpeg"):
                    try:
                        q = max(1, min(100, int(quality)))
                    except (TypeError, ValueError):
                        q = 95
                    save_image.save(output_path, format="JPEG", quality=q)
                else:
                    save_image.save(output_path)
            finally:
                save_image.close()

            return {
                "success": True,
                "input_path": str(input_path),
                "output_path": str(output_path),
                "original_width": width,
                "original_height": height,
                "width": new_w,
                "height": new_h,
                "mode": mode,
            }
        except MemoryError:
            return _error("RESIZE_MEMORY_LIMIT", "Image is too large to process safely")
        except Exception as exc:
            logger.error("Resize failed: %s", exc, exc_info=True)
            return _error("RESIZE_FAILED", str(exc))


_TOOL = Resizer()


def process(input_data: dict) -> dict:
    return _TOOL.resize(
        input_data.get("input_path"),
        input_data.get("output_path"),
        mode=input_data.get("mode", "percent"),
        value=input_data.get("value", input_data.get("scale_percent", 100)),
        output_format=input_data.get("output_format"),
        quality=input_data.get("quality", 95),
    )


def main():
    try:
        json.dump(process(json.load(sys.stdin)), sys.stdout)
    except Exception as exc:
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

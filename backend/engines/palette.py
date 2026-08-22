#!/usr/bin/env python3
"""
Palette Engine — extract a theme color and a 10-swatch palette from a photo.

Actions:
- "extract"        -> {theme_color, colors: [hex x10]} (fast, downscaled probe)
- "export_palette" -> write swatch strip image to output_path
- "export_combined"-> original image with the strip appended below

Options for exports:
    swatch_height (default 96), include_hex_text (bool), output_format png/jpg
"""

import json
import logging
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageOps

from backend.engines._image_formats import ensure_heif_supported

ensure_heif_supported()

logger = logging.getLogger(__name__)

PALETTE_SIZE = 11  # 1 dominant + 10 palette


def _error(code, message):
    return {"success": False, "error": message, "error_code": code}


def _to_hex(rgb):
    return "#{:02X}{:02X}{:02X}".format(rgb[0], rgb[1], rgb[2])


def extract_palette(input_path):
    source = Path(input_path)
    if not source.exists():
        raise FileNotFoundError(f"Input file not found: {input_path}")
    with Image.open(source) as img:
        img.load()
        oriented = ImageOps.exif_transpose(img)
        work = oriented if oriented is not img else oriented.copy()
    try:
        probe = work.convert("RGB")
        probe.thumbnail((256, 256))
        quantized = probe.quantize(colors=PALETTE_SIZE, method=Image.Quantize.MEDIANCUT)
        palette = quantized.getpalette() or []
        counts = sorted(quantized.getcolors(maxcolors=PALETTE_SIZE * 2) or [], reverse=True)
        colors = []
        for _, index in counts:
            rgb = tuple(palette[index * 3:index * 3 + 3])
            if len(rgb) == 3:
                colors.append(_to_hex(rgb))
            if len(colors) >= PALETTE_SIZE:
                break
        while len(colors) < PALETTE_SIZE:
            colors.append(colors[-1] if colors else "#000000")
        return {"theme_color": colors[0], "colors": colors[1:]}
    finally:
        work.close()


def _strip_image(hex_colors, width, swatch_height, show_text, text_color="#FFFFFF"):
    count = max(1, len(hex_colors))
    strip = Image.new("RGB", (width, swatch_height), (255, 255, 255))
    draw = ImageDraw.Draw(strip)
    swatch_w = width / count
    try:
        from PIL import ImageFont

        font = ImageFont.truetype("segoeui.ttf", size=max(12, swatch_height // 5)) if show_text else None
    except Exception:
        font = None
    for i, hex_color in enumerate(hex_colors):
        rgb = tuple(int(hex_color.lstrip("#")[j:j + 2], 16) for j in (0, 2, 4))
        box = [round(i * swatch_w), 0, round((i + 1) * swatch_w), swatch_height]
        draw.rectangle(box, fill=rgb)
        if font is not None and swatch_w > 64:
            draw.text((box[0] + 8, box[3] - font.size - 6), hex_color.upper(), fill=text_color, font=font)
    return strip


class PaletteTool:
    def run(self, action, payload):
        action = str(action or "extract").lower()
        input_path = payload.get("input_path")
        if not input_path:
            return _error("PAL_BAD_REQUEST", "Missing input_path")
        try:
            result = extract_palette(input_path)
        except FileNotFoundError as exc:
            return _error("PAL_INPUT_NOT_FOUND", str(exc))
        except MemoryError:
            return _error("PAL_MEMORY_LIMIT", "Image is too large to process safely")
        except Exception as exc:
            return _error("PAL_FAILED", str(exc))

        if action == "extract":
            return {"success": True, **result}

        if action in ("export_palette", "export_combined"):
            output_path = str(payload.get("output_path") or "")
            if not output_path:
                return _error("PAL_BAD_REQUEST", "Missing output_path")
            try:
                swatch_height = max(24, min(600, int(payload.get("swatch_height", 96))))
            except (TypeError, ValueError):
                swatch_height = 96
            show_text = bool(payload.get("include_hex_text", True))
            all_colors = [result["theme_color"], *result["colors"]]

            try:
                with Image.open(input_path) as img:
                    img.load()
                    base = ImageOps.exif_transpose(img).convert("RGB").copy()
            except Exception as exc:
                return _error("PAL_FAILED", str(exc))

            try:
                out_format = str(payload.get("output_format") or Path(output_path).suffix.lstrip(".") or "png").lower()
                if out_format in ("jpg", "jpeg"):
                    out_format = "jpg"
                else:
                    out_format = "png"

                if action == "export_palette":
                    final = _strip_image(all_colors, base.width, swatch_height, show_text)
                else:
                    strip = _strip_image(all_colors, base.width, swatch_height, show_text)
                    final = Image.new("RGB", (base.width, base.height + strip.height), (255, 255, 255))
                    final.paste(base, (0, 0))
                    final.paste(strip, (0, base.height))
                    strip.close()
                base.close()

                parent = Path(output_path).parent
                if str(parent):
                    import os

                    os.makedirs(parent, exist_ok=True)
                if out_format == "jpg":
                    final.save(output_path, format="JPEG", quality=95)
                else:
                    final.save(output_path, format="PNG")
                final.close()
                return {
                    "success": True,
                    "action": action,
                    "output_path": output_path,
                    "theme_color": result["theme_color"],
                    "colors": result["colors"],
                }
            except MemoryError:
                return _error("PAL_MEMORY_LIMIT", "Image is too large to process safely")
            except Exception as exc:
                logger.error("Palette export failed: %s", exc, exc_info=True)
                return _error("PAL_EXPORT_FAILED", str(exc))

        return _error("PAL_BAD_ACTION", f"Unsupported action: {action}")


_TOOL = PaletteTool()


def process(input_data: dict) -> dict:
    return _TOOL.run(input_data.get("action", "extract"), input_data)


def main():
    try:
        json.dump(process(json.load(sys.stdin)), sys.stdout)
    except Exception as exc:
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

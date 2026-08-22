#!/usr/bin/env python3
"""
Watermark Engine — text watermark with rich styling.

Actions:
- "apply": stamp the watermark onto input_path and write output_path
- "preview": render a small watermarked preview and return it as a data URL

Style options:
    text            watermark content (multi-line supported via \n)
    font_name       微软雅黑 | 黑体 | 宋体 | 等线 | Arial  (system font lookup)
    size_mode       "px" (absolute) | "percent" (of image width)
    font_size       int, meaning depends on size_mode
    color           "#RRGGBB"
    opacity         0..100
    rotation        -180..180 degrees
    position        tl tc tr cl c cr bl bc br   (+ margin)
    tiled           bool — repeat across the whole image
    tile_gap_x/y    spacing for tiled mode
    stroke_width / stroke_color
    shadow          bool (soft dark shadow behind glyphs)
"""

import json
import logging
import math
import os
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

from backend.engines._image_formats import ensure_heif_supported

ensure_heif_supported()

logger = logging.getLogger(__name__)

POSITIONS = {"tl", "tc", "tr", "cl", "c", "cr", "bl", "bc", "br"}

_FONT_CANDIDATES = {
    "微软雅黑": ["msyh.ttc", "msyh.ttf", "msyhbd.ttc"],
    "黑体": ["simhei.ttf"],
    "宋体": ["simsun.ttc", "simsun.ttf"],
    "等线": ["Deng.ttf", "Dengb.ttf"],
    "Arial": ["arial.ttf", "arialbd.ttf"],
}
_FALLBACK_FONTS = ["msyhbd.ttc", "msyh.ttc", "simhei.ttf", "segoeui.ttf", "arial.ttf"]


def _error(code, message):
    return {"success": False, "error": message, "error_code": code}


def _load_font(font_name: str, size: int):
    candidates = _FONT_CANDIDATES.get(str(font_name), []) + [f"{font_name}.ttf"]
    for name in candidates + _FALLBACK_FONTS:
        try:
            return ImageFont.truetype(name, size=size)
        except Exception:
            continue
    return ImageFont.load_default()


def _parse_color(value, default=(0, 0, 0)):
    text = str(value or "").strip().lstrip("#")
    if len(text) == 3:
        text = "".join(ch * 2 for ch in text)
    if len(text) != 6:
        return default
    try:
        return tuple(int(text[i:i + 2], 16) for i in (0, 2, 4))
    except ValueError:
        return default


def _resolve_font_size(img_width, opts):
    size_mode = str(opts.get("size_mode") or "px").lower()
    raw = opts.get("font_size", 36)
    try:
        value = max(4, min(2000, int(raw)))
    except (TypeError, ValueError):
        value = 36
    if size_mode == "percent":
        percent = max(1, min(100, value))
        return max(8, round(img_width * percent / 100))
    return value


def _build_text_layer(width, height, opts, font_size):
    """Render the watermark into a transparent RGBA layer sized to the image."""
    text = str(opts.get("text") or "").replace("\r\n", "\n").strip()
    if not text:
        return None
    font = _load_font(str(opts.get("font_name") or "微软雅黑"), font_size)
    color = _parse_color(opts.get("color"), (0, 0, 0))
    try:
        alpha = max(1, min(100, int(opts.get("opacity", 50)))) * 255 // 100
    except (TypeError, ValueError):
        alpha = 128
    fill = (*color, alpha)

    stroke_width = 0
    stroke_fill = None
    try:
        stroke_width = max(0, min(20, int(opts.get("stroke_width", 0) or 0)))
    except (TypeError, ValueError):
        pass
    if stroke_width > 0:
        stroke_rgb = _parse_color(opts.get("stroke_color"), (255, 255, 255))
        stroke_fill = (*stroke_rgb, min(255, alpha + 60))

    lines = text.split("\n")
    probe = Image.new("RGBA", (1, 1))
    draw = ImageDraw.Draw(probe)

    def line_size(line_text):
        box = draw.textbbox((0, 0), line_text or " ", font=font, stroke_width=stroke_width)
        return box[2] - box[0], box[3] - box[1]

    widths = []
    heights = []
    for line_text in lines:
        w, h = line_size(line_text)
        widths.append(w)
        heights.append(h)
    line_gap = max(2, font_size // 6)
    total_w = max(widths)
    total_h = sum(heights) + line_gap * (len(lines) - 1)

    pad = stroke_width * 2 + (font_size // 3 if opts.get("shadow") else stroke_width)
    layer_w = min(width * 2, total_w + pad * 2)
    layer_h = min(height * 2, total_h + pad * 2)
    layer = Image.new("RGBA", (max(1, layer_w), max(1, layer_h)), (0, 0, 0, 0))
    ldraw = ImageDraw.Draw(layer)

    y = pad
    for i, line_text in enumerate(lines):
        x = pad
        if opts.get("shadow"):
            shadow_offset = max(1, font_size // 24)
            ldraw.text(
                (x + shadow_offset, y + shadow_offset), line_text,
                font=font, fill=(0, 0, 0, min(160, alpha)), stroke_width=stroke_width,
                stroke_fill=(0, 0, 0, 0),
            )
        ldraw.text(
            (x, y), line_text,
            font=font, fill=fill,
            stroke_width=stroke_width, stroke_fill=stroke_fill,
        )
        y += heights[i] + line_gap

    try:
        angle = float(opts.get("rotation", 0) or 0)
    except (TypeError, ValueError):
        angle = 0.0
    angle = ((angle % 360.0) + 360.0) % 360.0
    if angle > 0.01:
        layer = layer.rotate(angle, expand=True, resample=Image.Resampling.BICUBIC)
    return layer


def _paste_positions(width, height, layer_w, layer_h, opts):
    position = str(opts.get("position") or "br").lower()
    if position not in POSITIONS:
        position = "br"
    try:
        margin = max(0, min(2000, int(opts.get("margin", 40) or 40)))
    except (TypeError, ValueError):
        margin = 40

    if opts.get("tiled"):
        try:
            gap_x = max(10, min(4000, int(opts.get("tile_gap_x", 80) or 80)))
            gap_y = max(10, min(4000, int(opts.get("tile_gap_y", 80) or 80)))
        except (TypeError, ValueError):
            gap_x = gap_y = 80
        step_x = layer_w + gap_x
        step_y = layer_h + gap_y
        points = []
        offset_x = -layer_w
        row = 0
        while offset_x < width:
            offset_y = -layer_h + (row % 2) * (step_y // 2)
            while offset_y < height:
                points.append((offset_x, offset_y))
                offset_y += step_y
            offset_x += step_x
            row += 1
        return points

    xs = {"l": margin, "c": (width - layer_w) // 2, "r": width - layer_w - margin}
    ys = {"t": margin, "c": (height - layer_h) // 2, "b": height - layer_h - margin}
    return [(xs[position[1]], ys[position[0]])]


class WatermarkTool:
    def run(self, action, payload):
        action = str(action or "apply").lower()
        input_path = payload.get("input_path")
        if not input_path:
            return _error("WM_BAD_REQUEST", "Missing input_path")
        source = Path(input_path)
        if not source.exists():
            return _error("WM_INPUT_NOT_FOUND", f"Input file not found: {input_path}")

        opts = payload
        try:
            with Image.open(source) as img:
                img.load()
                oriented = ImageOps_exif(img)
                base = oriented.convert("RGBA")
        except MemoryError:
            return _error("WM_MEMORY_LIMIT", "Image is too large to process safely")
        except Exception as exc:
            return _error("WM_OPEN_FAILED", str(exc))

        try:
            font_size = _resolve_font_size(base.width, opts)
            layer = _build_text_layer(base.width, base.height, opts, font_size)
            if layer is None:
                base.close()
                return _error("WM_EMPTY_TEXT", "Watermark text is empty")

            watermarked = base.copy()
            base.close()
            # paste (with mask) tolerates negative offsets, required for tiled wrap-around
            for point in _paste_positions(watermarked.width, watermarked.height, layer.width, layer.height, opts):
                watermarked.paste(layer, point, layer)
            layer.close()

            if action == "preview":
                import io as _io
                import base64 as _b64

                # Downscale BEFORE compositing so huge inputs stay fast in live preview.
                preview_src = watermarked
                max_preview_edge = 1100
                if max(preview_src.size) > max_preview_edge:
                    scale = max_preview_edge / max(preview_src.size)
                    preview_src = preview_src.resize(
                        (max(1, round(preview_src.width * scale)), max(1, round(preview_src.height * scale))),
                        Image.Resampling.BILINEAR,
                    )
                buffer = _io.BytesIO()
                preview_src.save(buffer, format="PNG")
                out_w, out_h = watermarked.size
                watermarked.close()
                if preview_src is not watermarked:
                    preview_src.close()
                return {
                    "success": True,
                    "action": "preview",
                    "data_url": "data:image/png;base64," + _b64.b64encode(buffer.getvalue()).decode("ascii"),
                    "width": out_w,
                    "height": out_h,
                }

            output_path = str(payload.get("output_path") or "")
            if not output_path:
                watermarked.close()
                return _error("WM_BAD_REQUEST", "Missing output_path")
            parent = Path(output_path).parent
            if str(parent):
                os.makedirs(parent, exist_ok=True)

            out_format = str(payload.get("output_format") or Path(output_path).suffix.lstrip(".") or "png").lower()
            save_image = watermarked
            if out_format in ("jpg", "jpeg"):
                flat = Image.new("RGB", watermarked.size, (255, 255, 255))
                flat.paste(watermarked, mask=watermarked.split()[-1])
                watermarked.close()
                save_image = flat
            try:
                if out_format in ("jpg", "jpeg"):
                    try:
                        q = max(1, min(100, int(payload.get("quality", 95))))
                    except (TypeError, ValueError):
                        q = 95
                    save_image.save(output_path, format="JPEG", quality=q)
                else:
                    save_image.save(output_path, format="PNG")
            finally:
                save_image.close()

            return {
                "success": True,
                "input_path": str(input_path),
                "output_path": output_path,
                "width": save_image.width,
                "height": save_image.height,
            }
        except MemoryError:
            return _error("WM_MEMORY_LIMIT", "Image is too large to process safely")
        except Exception as exc:
            logger.error("Watermark failed: %s", exc, exc_info=True)
            return _error("WM_FAILED", str(exc))


def ImageOps_exif(img):
    from PIL import ImageOps

    oriented = ImageOps.exif_transpose(img)
    return oriented if oriented is not img else img.copy()


_TOOL = WatermarkTool()


def process(input_data: dict) -> dict:
    return _TOOL.run(input_data.get("action", "apply"), input_data)


def main():
    try:
        json.dump(process(json.load(sys.stdin)), sys.stdout)
    except Exception as exc:
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

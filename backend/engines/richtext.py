#!/usr/bin/env python3
"""
RichText Engine — render structured rich-text blocks into a PNG image.

Blocks:
    {type:"heading", text, level 1..3}
    {type:"paragraph", text}
    {type:"list", items:[...], ordered:bool}
    {type:"quote", text}
    {type:"image", path, width_percent 10..100}

Global style:
    width (canvas px, default 800), padding, background "#RRGGBB",
    text_color, heading_color, accent_color (quote bar / list markers),
    font_size (base px), line_height (1.2..2.5), font_name

CJK-aware greedy wrapping via font.getlength(). Canvas height is computed in
a dry pass so the output is exactly the content height.

Actions:
- "render"  -> writes output_path (+ optional preview data_url)
"""

import json
import logging
import os
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

logger = logging.getLogger(__name__)

_FONT_CANDIDATES = {
    "微软雅黑": ["msyh.ttc", "msyhbd.ttc"],
    "黑体": ["simhei.ttf"],
    "宋体": ["simsun.ttc"],
    "等线": ["Deng.ttf"],
    "Arial": ["arial.ttf"],
}
_FALLBACK = ["msyhbd.ttc", "msyh.ttc", "simhei.ttf", "segoeui.ttf", "arial.ttf"]

HEADING_SCALES = {1: 1.9, 2: 1.5, 3: 1.25}


def _error(code, message):
    return {"success": False, "error": message, "error_code": code}


def _load_font(name: str, size: int, bold: bool = False):
    candidates = list(_FONT_CANDIDATES.get(str(name), []))
    if bold:
        bold_first = [c.replace(".ttf", "bd.ttc").replace("msyhbd.ttc", "msyhbd.ttc") for c in _FONT_CANDIDATES.get(str(name), [])]
        candidates = bold_first + candidates
    for candidate in candidates + _FALLBACK:
        try:
            return ImageFont.truetype(candidate, size=size)
        except Exception:
            continue
    return ImageFont.load_default()


def _parse_color(value, default):
    text = str(value or "").strip().lstrip("#")
    if len(text) == 3:
        text = "".join(ch * 2 for ch in text)
    if len(text) != 6:
        return default
    try:
        return tuple(int(text[i:i + 2], 16) for i in (0, 2, 4))
    except ValueError:
        return default


def _wrap(draw, text, font, max_width):
    lines = []
    for raw_line in str(text).replace("\r\n", "\n").split("\n"):
        if not raw_line:
            lines.append("")
            continue
        current = ""
        for ch in raw_line:
            trial = current + ch
            if draw.textlength(trial, font=font) <= max_width or not current:
                current = trial
            else:
                lines.append(current)
                current = ch
        lines.append(current)
    return lines


class RichTextRenderer:
    def render(self, payload):
        blocks = payload.get("blocks")
        if not isinstance(blocks, list) or not blocks:
            return _error("RT_NO_BLOCKS", "blocks is empty")

        width = max(200, min(4000, int(payload.get("width", 800) or 800)))
        try:
            padding = max(16, min(400, int(payload.get("padding", 48) or 48)))
        except (TypeError, ValueError):
            padding = 48
        base_font_size = max(10, min(120, int(payload.get("font_size", 18) or 18)))
        try:
            line_height = max(1.2, min(2.5, float(payload.get("line_height", 1.6))))
        except (TypeError, ValueError):
            line_height = 1.6
        font_name = str(payload.get("font_name") or "微软雅黑")
        bg = _parse_color(payload.get("background"), (255, 255, 255))
        text_color = _parse_color(payload.get("text_color"), (31, 41, 55))
        heading_color = _parse_color(payload.get("heading_color"), (17, 24, 39))
        accent_color = _parse_color(payload.get("accent_color"), (0, 122, 255))

        content_width = width - padding * 2
        probe = Image.new("RGB", (4, 4))
        draw_probe = ImageDraw.Draw(probe)

        # ---- layout pass ----
        cursor_y = float(padding)
        plan = []  # (kind, params)
        image_paths_missing = []

        def para_height(lines, size):
            lh = size * line_height
            return len(lines) * lh

        for block in blocks:
            btype = str(block.get("type") or "paragraph").lower()
            if btype == "heading":
                level = max(1, min(3, int(block.get("level", 1) or 1)))
                size = round(base_font_size * HEADING_SCALES[level])
                font = _load_font(font_name, size, bold=True)
                lines = _wrap(draw_probe, block.get("text", ""), font, content_width)
                plan.append(("text", {"lines": lines, "font": font, "size": size,
                                      "color": heading_color, "space_before": size * 0.5,
                                      "bold_gap": True}))
            elif btype == "paragraph":
                size = base_font_size
                font = _load_font(font_name, size)
                lines = _wrap(draw_probe, block.get("text", ""), font, content_width)
                plan.append(("text", {"lines": lines, "font": font, "size": size,
                                      "color": text_color, "space_before": size * 0.4}))
            elif btype == "list":
                ordered = bool(block.get("ordered"))
                items = [str(item) for item in (block.get("items") or [])]
                size = base_font_size
                font = _load_font(font_name, size)
                marker_width = draw_probe.textlength("00.", font=font) + 10
                entry_lines = []
                for idx, item in enumerate(items, start=1):
                    wrapped = _wrap(draw_probe, item, font, content_width - marker_width)
                    marker = f"{idx}." if ordered else "•"
                    entry_lines.append((marker, wrapped))
                    para_h = para_height(wrapped, size)
                space_each = size * 0.25
                total_h = sum(para_height(wrapped, size) for _, wrapped in entry_lines) \
                    + space_each * max(0, len(entry_lines) - 1)
                plan.append(("list", {"entries": entry_lines, "font": font, "size": size,
                                      "color": text_color, "accent": accent_color,
                                      "marker_width": marker_width, "total_h": total_h,
                                      "space_before": size * 0.4}))
            elif btype == "quote":
                size = base_font_size
                font = _load_font(font_name, size)
                bar_w = max(3, size // 6)
                lines = _wrap(draw_probe, block.get("text", ""), font, content_width - bar_w - 14)
                plan.append(("quote", {"lines": lines, "font": font, "size": size,
                                       "color": text_color, "accent": accent_color,
                                       "bar_w": bar_w, "space_before": size * 0.5}))
            elif btype == "image":
                img_path = str(block.get("path") or "")
                if not img_path or not Path(img_path).exists():
                    image_paths_missing.append(img_path)
                    continue
                try:
                    percent = max(10, min(100, int(block.get("width_percent", 100))))
                except (TypeError, ValueError):
                    percent = 100
                with Image.open(img_path) as im:
                    im.load()
                    iw, ih = im.size
                target_w = round(content_width * percent / 100)
                target_h = max(1, round(ih * target_w / iw))
                plan.append(("image", {"path": img_path, "w": target_w, "h": target_h,
                                       "space_before": base_font_size * 0.5}))
            # unknown types ignored

        total_height = padding
        for kind, params in plan:
            total_height += params.get("space_before", 0)
            if kind == "text":
                total_height += para_height(params["lines"], params["size"])
            elif kind == "list":
                total_height += params["total_h"]
            elif kind == "quote":
                total_height += para_height(params["lines"], params["size"])
            elif kind == "image":
                total_height += params["h"]
        total_height += padding
        total_height = round(total_height)
        if total_height > 16000:
            return _error("RT_TOO_TALL", "富文本内容高度超过 16000px 安全上限")
        probe.close()

        canvas = Image.new("RGB", (width, total_height), bg)
        draw = ImageDraw.Draw(canvas)

        y = float(padding)
        for kind, params in plan:
            y += params.get("space_before", 0)
            if kind == "text":
                lh = params["size"] * line_height
                for line_text in params["lines"]:
                    draw.text((padding, y), line_text, font=params["font"], fill=params["color"])
                    y += lh
            elif kind == "list":
                lh = params["size"] * line_height
                for marker, wrapped in params["entries"]:
                    draw.text(
                        (padding, y), marker,
                        font=params["font"], fill=params["accent"],
                    )
                    inner_y = y
                    for line_text in wrapped:
                        draw.text((padding + params["marker_width"], inner_y), line_text,
                                  font=params["font"], fill=params["color"])
                        inner_y += lh
                    y += max(lh * len(wrapped), lh)
                    y += params["size"] * 0.25
                y -= params["size"] * 0.25
            elif kind == "quote":
                lh = params["size"] * line_height
                block_h = para_height(params["lines"], params["size"])
                draw.rectangle([padding, y, padding + params["bar_w"], y + block_h], fill=params["accent"])
                tx = padding + params["bar_w"] + 14
                inner_y = y
                for line_text in params["lines"]:
                    draw.text((tx, inner_y), line_text, font=params["font"], fill=params["color"])
                    inner_y += lh
                y += block_h
            elif kind == "image":
                try:
                    with Image.open(params["path"]) as im:
                        im.load()
                        content = im.convert("RGB").resize((params["w"], params["h"]), Image.Resampling.LANCZOS)
                        canvas.paste(content, (padding, round(y)))
                        content.close()
                except Exception:
                    pass
                y += params["h"]

        output_path = str(payload.get("output_path") or "")
        parent = Path(output_path).parent if output_path else None
        if parent and str(parent):
            os.makedirs(parent, exist_ok=True)

        result = {
            "success": not bool(image_paths_missing),
            "width": canvas.width,
            "height": canvas.height,
            "missing_images": image_paths_missing,
        }
        if image_paths_missing:
            result["error_code"] = "RT_MISSING_IMAGES"
            result["error"] = "存在无法读取的图片块"

        if payload.get("preview"):
            import io as _io
            import base64 as _b64

            preview_img = canvas.copy()
            preview_img.thumbnail((760, 1400))
            buffer = _io.BytesIO()
            preview_img.save(buffer, format="PNG")
            preview_img.close()
            result["data_url"] = "data:image/png;base64," + _b64.b64encode(buffer.getvalue()).decode("ascii")

        if output_path:
            fmt = Path(output_path).suffix.lstrip(".").lower()
            if fmt in ("jpg", "jpeg"):
                canvas.save(output_path, format="JPEG", quality=95)
            else:
                canvas.save(output_path, format="PNG")
            result["output_path"] = output_path
        canvas.close()
        return result


_TOOL = RichTextRenderer()


def process(input_data: dict) -> dict:
    return _TOOL.render(input_data)


def main():
    try:
        json.dump(process(json.load(sys.stdin)), sys.stdout)
    except Exception as exc:
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

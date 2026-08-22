#!/usr/bin/env python3
"""
Image Splitter Engine — cuts an image into rows x cols tiles.

Two fit modes:
- "square": prefer square tiles; the image is center-cropped to the largest
  region that divides evenly (cols*tile x rows*tile), excess is discarded.
- "full"  : prioritize covering the entire image; tile edges are computed with
  linspace so every pixel belongs to exactly one tile (tiles may be slightly
  non-uniform at the right/bottom edges).

Tiles are written row-major with zero-padded sequential names:
    {stem}_{index:02d}.{ext}

process(input_data) -> dict
    input_path, output_dir, rows (1..50), cols (1..50),
    mode ("square"|"full"), output_format ("png"|"jpg"), quality
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

MIN_COUNT, MAX_COUNT = 1, 50


def _error(code, message):
    return {"success": False, "error": message, "error_code": code}


def _clamp_count(value, default=1):
    try:
        count = int(value)
    except (TypeError, ValueError):
        return default
    return max(MIN_COUNT, min(MAX_COUNT, count))


class ImageSplitter:
    def split(self, input_path, output_dir, rows=2, cols=2, mode="square", output_format="png", quality=95):
        try:
            source = Path(input_path)
            if not source.exists():
                return _error("SPLIT_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
            if not output_dir:
                return _error("SPLIT_BAD_REQUEST", "Missing output_dir")

            rows = _clamp_count(rows)
            cols = _clamp_count(cols)
            mode = str(mode or "square").strip().lower()
            if mode not in ("square", "full"):
                mode = "square"
            fmt = "jpg" if str(output_format).lower() in ("jpg", "jpeg") else "png"

            os.makedirs(output_dir, exist_ok=True)

            with Image.open(source) as img:
                img.load()
                oriented = ImageOps.exif_transpose(img)
                work = oriented.convert("RGBA") if oriented is not img else img.copy().convert("RGBA")

            try:
                width, height = work.size
                if mode == "square":
                    tile_w = min(width // cols, height // rows) if cols and rows else 0
                    if tile_w <= 0:
                        return _error("SPLIT_TOO_SMALL", "Image is smaller than the requested grid")
                    usable_w, usable_h = cols * tile_w, rows * tile_w
                    left = (width - usable_w) // 2
                    top = (height - usable_h) // 2
                    col_edges = [left + c * tile_w for c in range(cols + 1)]
                    row_edges = [top + r * tile_w for r in range(rows + 1)]
                    uniform = True
                else:
                    # full coverage: linspace boundaries cover every pixel
                    if width < cols or height < rows:
                        return _error("SPLIT_TOO_SMALL", "Image is smaller than the requested grid")
                    col_edges = [round(c * width / cols) for c in range(cols + 1)]
                    row_edges = [round(r * height / rows) for r in range(rows + 1)]
                    col_edges[-1] = width
                    row_edges[-1] = height
                    uniform = (width % cols == 0) and (height % rows == 0)

                stem = source.stem or "output"
                pad = 2 if rows * cols <= 99 else 3
                flatten_alpha = fmt == "jpg"
                outputs = []
                index = 0
                for r in range(rows):
                    for c in range(cols):
                        index += 1
                        box = (col_edges[c], row_edges[r], col_edges[c + 1], row_edges[r + 1])
                        tile = work.crop(box)
                        try:
                            if flatten_alpha:
                                flat = Image.new("RGB", tile.size, (255, 255, 255))
                                flat.paste(tile, mask=tile.split()[-1])
                                tile.close()
                                tile = flat
                            out_path = os.path.join(output_dir, f"{stem}_{index:0{pad}d}.{fmt}")
                            if fmt == "png":
                                tile.save(out_path, format="PNG")
                            else:
                                try:
                                    q = max(1, min(100, int(quality)))
                                except (TypeError, ValueError):
                                    q = 95
                                tile.save(out_path, format="JPEG", quality=q)
                            outputs.append(out_path)
                        finally:
                            tile.close()

                return {
                    "success": True,
                    "input_path": str(input_path),
                    "output_dir": str(output_dir),
                    "rows": rows,
                    "cols": cols,
                    "mode": mode,
                    "tile_count": len(outputs),
                    "uniform_tiles": uniform,
                    "tile_width": col_edges[1] - col_edges[0],
                    "tile_height": row_edges[1] - row_edges[0],
                    "output_format": fmt,
                    "output_paths": outputs,
                }
            finally:
                work.close()
        except MemoryError:
            return _error("SPLIT_MEMORY_LIMIT", "Image is too large to process safely")
        except Exception as exc:
            logger.error("Split failed: %s", exc, exc_info=True)
            return _error("SPLIT_FAILED", str(exc))


_TOOL = ImageSplitter()


def process(input_data: dict) -> dict:
    input_path = input_data.get("input_path")
    output_dir = input_data.get("output_dir")
    if not input_path or not output_dir:
        return _error("SPLIT_BAD_REQUEST", "Missing input_path or output_dir")
    return _TOOL.split(
        input_path,
        output_dir,
        rows=input_data.get("rows", 2),
        cols=input_data.get("cols", 2),
        mode=input_data.get("mode", "square"),
        output_format=input_data.get("output_format", "png"),
        quality=input_data.get("quality", 95),
    )


def main():
    try:
        json.dump(process(json.load(sys.stdin)), sys.stdout)
    except Exception as exc:
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

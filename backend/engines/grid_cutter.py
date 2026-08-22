#!/usr/bin/env python3
"""
Grid Cutter Engine

Splits an image into an even grid (2x2 / 3x3 / 4x4) after centering it on a
square canvas crop. Tiles are written to the output directory with sequential
zero-padded names so they sort naturally in row-major reading order:

    {stem}_01.png ... {stem}_04.png   (2x2)
    {stem}_01.png ... {stem}_09.png   (3x3)
    {stem}_01.png ... {stem}_16.png   (4x4)

Usage (bridge):
    process(input_data) -> dict

Input keys:
    input_path   : str  - source image
    output_dir   : str  - destination folder (created when missing)
    grid_size    : int|str - 2, 3 or 4 (aliases: '2x2', '九宫格', ...)
    output_format: str  - 'png' (default, keeps alpha) or 'jpg'
    quality      : int  - JPEG quality when output_format is jpg (default 95)
"""

import json
import logging
import os
import re
import sys
from pathlib import Path

from PIL import Image, ImageOps

from backend.engines._image_formats import ensure_heif_supported

ensure_heif_supported()

logger = logging.getLogger(__name__)

VALID_GRID_SIZES = (2, 3, 4)
FLATTEN_FORMATS = {"jpg", "jpeg", "bmp"}
DEFAULT_JPEG_QUALITY = 95


def _error_response(code, message):
    return {"success": False, "error": message, "error_code": code}


_GRID_SIZE_ALIASES = (
    ("十六", 4),
    ("9宫", 3),
    ("九", 3),
    ("4宫", 4),
    ("四", 2),
    ("2x2", 2),
    ("3x3", 3),
    ("4x4", 4),
)


def _normalize_grid_size(value):
    if value is None:
        return None
    text = str(value).strip().lower()
    for alias, size in _GRID_SIZE_ALIASES:
        if alias in text:
            return size
    match = re.search(r"\d+", text)
    if not match:
        return None
    size = int(match.group())
    return size if size in VALID_GRID_SIZES else None


def _resolve_output_format(value):
    fmt = str(value or "png").strip().lower()
    if fmt == "jpeg":
        fmt = "jpg"
    return fmt


class GridCutter:
    """Cuts images into even grids with deterministic sequential naming."""

    def cut(self, input_path, output_dir, grid_size=3, output_format="png", quality=DEFAULT_JPEG_QUALITY):
        try:
            source = Path(input_path)
            if not source.exists():
                return _error_response("GRID_INPUT_NOT_FOUND", f"Input file not found: {input_path}")

            size = _normalize_grid_size(grid_size)
            if size is None:
                return _error_response(
                    "GRID_SIZE_INVALID",
                    f"Unsupported grid size: {grid_size}. Use 2 (2x2), 3 (3x3) or 4 (4x4).",
                )

            fmt = _resolve_output_format(output_format)
            if fmt not in ("png", "jpg"):
                return _error_response("GRID_OUTPUT_FORMAT_INVALID", f"Unsupported output format: {output_format}")

            quality_value = DEFAULT_JPEG_QUALITY
            if fmt == "jpg":
                try:
                    quality_value = max(1, min(100, int(quality)))
                except (TypeError, ValueError):
                    quality_value = DEFAULT_JPEG_QUALITY

            os.makedirs(output_dir, exist_ok=True)

            with Image.open(source) as img:
                img.load()
                oriented = ImageOps.exif_transpose(img)
                work = oriented if oriented is not img else img.copy()

            try:
                side = min(work.size)
                left = (work.width - side) // 2
                top = (work.height - side) // 2
                square = work.crop((left, top, left + side, top + side))
                tile_side = side // size
                if tile_side <= 0:
                    return _error_response("GRID_TOO_SMALL", "Image is too small for the requested grid")

                stem = source.stem or "output"
                flatten_alpha = fmt in FLATTEN_FORMATS
                pad = 2
                output_paths = []
                index = 0
                for row in range(size):
                    for col in range(size):
                        index += 1
                        box = (
                            col * tile_side,
                            row * tile_side,
                            (col + 1) * tile_side,
                            (row + 1) * tile_side,
                        )
                        tile = square.crop(box)
                        try:
                            if flatten_alpha:
                                flat = Image.new("RGB", tile.size, (255, 255, 255))
                                if tile.mode == "RGBA":
                                    flat.paste(tile, mask=tile.split()[-1])
                                else:
                                    flat.paste(tile.convert("RGB"))
                                tile.close()
                                tile = flat
                            out_name = f"{stem}_{index:0{pad}d}.{fmt}"
                            out_path = os.path.join(output_dir, out_name)
                            if fmt == "png":
                                tile.save(out_path, format="PNG")
                            else:
                                tile.save(out_path, format="JPEG", quality=quality_value)
                            output_paths.append(out_path)
                        finally:
                            tile.close()
                return {
                    "success": True,
                    "input_path": str(input_path),
                    "output_dir": str(output_dir),
                    "grid_size": size,
                    "tile_width": tile_side,
                    "tile_height": tile_side,
                    "tile_count": size * size,
                    "output_format": fmt,
                    "output_paths": output_paths,
                }
            finally:
                square.close()
                work.close()

        except MemoryError:
            return _error_response("GRID_MEMORY_LIMIT", "Image is too large to process safely")
        except Exception as exc:
            logger.error("Grid cut failed: %s", exc, exc_info=True)
            return _error_response("GRID_CUT_FAILED", str(exc))


_TOOL = GridCutter()


def handle_request(input_data: dict) -> dict:
    input_path = input_data.get("input_path")
    output_dir = input_data.get("output_dir")
    if not input_path or not output_dir:
        return _error_response("GRID_BAD_REQUEST", "Missing input_path or output_dir")
    return _TOOL.cut(
        input_path,
        output_dir,
        grid_size=input_data.get("grid_size", input_data.get("grid", 3)),
        output_format=input_data.get("output_format", "png"),
        quality=input_data.get("quality", DEFAULT_JPEG_QUALITY),
    )


def process(input_data: dict) -> dict:
    if isinstance(input_data, dict):
        return handle_request(input_data)
    return _error_response("GRID_BAD_REQUEST", "Invalid request payload")


def main():
    try:
        input_data = json.load(sys.stdin)
        result = process(input_data)
        json.dump(result, sys.stdout)
    except Exception as exc:
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

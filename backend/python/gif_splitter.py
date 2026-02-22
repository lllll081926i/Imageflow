#!/usr/bin/env python3
"""
GIF Tools Script

This script handles GIF operations using the Pillow library:
- export frames from a GIF
- reverse a GIF
- change GIF playback speed
- build a GIF from input images
- compress a GIF with adjustable quality

Usage:
    python gif_splitter.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import json
import logging
import os
import sys
from pathlib import Path

from PIL import Image, ImageSequence, UnidentifiedImageError

# Configure logging
logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

DEFAULT_FPS = 10
MIN_SPEED_FACTOR = 0.1
MAX_SPEED_FACTOR = 2.0
MIN_QUALITY = 1
MAX_QUALITY = 100
FRAME_OUTPUT_FORMATS = {"png", "bmp"}


class GIFTool:
    """Handles GIF operations."""

    def __init__(self):
        logger.info("GIFTool initialized")

    def export_frames(self, input_path, output_dir, output_format="png", frame_range="all"):
        try:
            output_format = (output_format or "png").lower()
            if output_format not in FRAME_OUTPUT_FORMATS:
                return {"success": False, "error": f"Unsupported output format: {output_format}"}

            with Image.open(input_path) as gif:
                if gif.format != "GIF":
                    raise ValueError("Input file is not a GIF")

                frame_count = self._get_frame_count(gif)
                frame_indices = self._parse_frame_range(frame_range, frame_count)
                frame_indices = [i for i in frame_indices if 0 <= i < frame_count]
                if not frame_indices:
                    return {"success": False, "error": "No frames selected for export"}

                os.makedirs(output_dir, exist_ok=True)
                base_name = Path(input_path).stem
                frame_files = []

                for frame_idx in frame_indices:
                    gif.seek(frame_idx)
                    frame = gif.copy()

                    output_filename = f"{base_name}_frame_{frame_idx:04d}.{output_format}"
                    output_path = os.path.join(output_dir, output_filename)

                    save_format = output_format.upper()
                    frame.save(output_path, format=save_format)
                    frame_files.append(output_path)

                return {
                    "success": True,
                    "input_path": input_path,
                    "output_dir": output_dir,
                    "frame_count": frame_count,
                    "export_count": len(frame_files),
                    "frame_paths": frame_files,
                }

        except FileNotFoundError:
            return {"success": False, "error": f"Input file not found: {input_path}"}
        except UnidentifiedImageError:
            return {"success": False, "error": f"Unsupported image format: {input_path}"}
        except Exception as exc:
            logger.error("GIF export failed: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}

    def reverse_gif(self, input_path, output_path, loop=None):
        try:
            with Image.open(input_path) as gif:
                if gif.format != "GIF":
                    raise ValueError("Input file is not a GIF")
                frames, durations, gif_loop = self._extract_gif_frames(gif)
            frames.reverse()
            durations.reverse()

            loop_value = gif_loop if loop is None else loop
            self._ensure_parent_dir(output_path)
            self._save_gif(frames, output_path, durations, loop_value)

            return {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
                "frame_count": len(frames),
            }
        except FileNotFoundError:
            return {"success": False, "error": f"Input file not found: {input_path}"}
        except UnidentifiedImageError:
            return {"success": False, "error": f"Unsupported image format: {input_path}"}
        except Exception as exc:
            logger.error("GIF reverse failed: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}

    def change_speed(self, input_path, output_path, speed_factor, loop=None):
        try:
            with Image.open(input_path) as gif:
                if gif.format != "GIF":
                    raise ValueError("Input file is not a GIF")
                frames, durations, gif_loop = self._extract_gif_frames(gif)
            factor = self._clamp_speed_factor(speed_factor)
            new_durations = [max(1, int(d / factor)) for d in durations]

            loop_value = gif_loop if loop is None else loop
            self._ensure_parent_dir(output_path)
            self._save_gif(frames, output_path, new_durations, loop_value)

            return {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
                "frame_count": len(frames),
                "speed_factor": factor,
            }
        except FileNotFoundError:
            return {"success": False, "error": f"Input file not found: {input_path}"}
        except UnidentifiedImageError:
            return {"success": False, "error": f"Unsupported image format: {input_path}"}
        except Exception as exc:
            logger.error("GIF speed change failed: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}

    def compress_gif(self, input_path, output_path, quality=90, loop=None):
        try:
            with Image.open(input_path) as gif:
                if gif.format != "GIF":
                    raise ValueError("Input file is not a GIF")
                frames, durations, gif_loop, disposals = self._extract_gif_frames_with_disposal(gif)

            quality_value = self._sanitize_quality(quality)
            palette_size = self._quality_to_palette_size(quality_value)
            compressed_frames = []
            for frame in frames:
                compressed_frames.append(self._quantize_rgba_frame(frame, palette_size))

            loop_value = gif_loop if loop is None else loop
            self._ensure_parent_dir(output_path)
            self._save_gif(
                compressed_frames,
                output_path,
                durations,
                loop_value,
                optimize=True,
                disposal=disposals,
                transparency=255,
            )

            in_size = os.path.getsize(input_path) if os.path.exists(input_path) else 0
            out_size = os.path.getsize(output_path) if os.path.exists(output_path) else 0
            return {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
                "frame_count": len(compressed_frames),
                "quality": quality_value,
                "input_size": in_size,
                "output_size": out_size,
            }
        except FileNotFoundError:
            return {"success": False, "error": f"Input file not found: {input_path}"}
        except UnidentifiedImageError:
            return {"success": False, "error": f"Unsupported image format: {input_path}"}
        except Exception as exc:
            logger.error("GIF compression failed: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}

    def build_gif(self, input_paths, output_path, fps=None, loop=0):
        try:
            paths = [p for p in (input_paths or []) if p]
            if not paths:
                return {"success": False, "error": "No input images provided"}

            fps_value = self._sanitize_fps(fps)
            duration_ms = max(1, int(1000 / fps_value))

            frames = []
            sizes = []
            for path in paths:
                with Image.open(path) as img:
                    ext = Path(path).suffix.lower()
                    if ext != ".png":
                        logger.info("Converting %s to PNG-compatible RGBA before GIF", path)
                    img = img.convert("RGBA")

                    frame = img.copy()
                frames.append(frame)
                sizes.append(frame.size)

            canvas_size = self._choose_canvas_size(sizes)
            frames = [self._pad_to_size(frame, canvas_size) for frame in frames]

            self._ensure_parent_dir(output_path)
            self._save_gif(frames, output_path, [duration_ms] * len(frames), loop)

            return {
                "success": True,
                "input_paths": paths,
                "output_path": output_path,
                "frame_count": len(frames),
                "fps": fps_value,
            }
        except FileNotFoundError as exc:
            return {"success": False, "error": f"Input file not found: {exc.filename}"}
        except UnidentifiedImageError as exc:
            return {"success": False, "error": f"Unsupported image format: {exc}"}
        except Exception as exc:
            logger.error("GIF build failed: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}

    def _open_gif(self, input_path):
        gif = Image.open(input_path)
        if gif.format != "GIF":
            raise ValueError("Input file is not a GIF")
        return gif

    def _get_frame_count(self, gif):
        frame_count = 0
        try:
            while True:
                gif.seek(frame_count)
                frame_count += 1
        except EOFError:
            pass
        return frame_count

    def _extract_gif_frames(self, gif):
        frames = []
        durations = []
        default_duration = gif.info.get("duration", 100)
        for frame in ImageSequence.Iterator(gif):
            frames.append(frame.copy())
            duration = frame.info.get("duration", default_duration)
            if not isinstance(duration, int) or duration <= 0:
                duration = default_duration
            durations.append(duration)
        loop = gif.info.get("loop", 0)
        return frames, durations, loop

    def _extract_gif_frames_with_disposal(self, gif):
        frames = []
        durations = []
        disposals = []
        default_duration = gif.info.get("duration", 100)
        for frame in ImageSequence.Iterator(gif):
            # Use RGBA to preserve alpha information during compression.
            frames.append(frame.convert("RGBA").copy())
            duration = frame.info.get("duration", default_duration)
            if not isinstance(duration, int) or duration <= 0:
                duration = default_duration
            durations.append(duration)
            disposal = frame.info.get("disposal", getattr(frame, "disposal_method", 0))
            if not isinstance(disposal, int) or disposal < 0:
                disposal = 0
            disposals.append(disposal)
        loop = gif.info.get("loop", 0)
        return frames, durations, loop, disposals

    def _parse_frame_range(self, frame_range, total_frames):
        if not frame_range or frame_range == "all":
            return list(range(total_frames))

        try:
            if "-" in frame_range:
                start, end = map(int, frame_range.split("-"))
                return list(range(start, min(end + 1, total_frames)))
            if ":" in frame_range:
                parts = frame_range.split(":")
                if len(parts) == 2:
                    start, step = map(int, parts)
                    return list(range(start, total_frames, step))
            return [int(frame_range)]
        except (ValueError, IndexError) as exc:
            logger.warning("Invalid frame range '%s': %s. Using all frames.", frame_range, exc)
            return list(range(total_frames))

    def _save_gif(self, frames, output_path, durations, loop, optimize=False, disposal=None, transparency=None):
        if not frames:
            raise ValueError("No frames to save")
        first, rest = frames[0], frames[1:]
        save_kwargs = {
            "save_all": True,
            "append_images": rest,
            "duration": durations,
            "loop": loop if loop is not None else 0,
            "optimize": bool(optimize),
        }
        if disposal is not None:
            save_kwargs["disposal"] = disposal
        if transparency is not None:
            save_kwargs["transparency"] = transparency
        first.save(
            output_path,
            **save_kwargs,
        )

    def _pad_to_size(self, img, size):
        if img.size == size:
            return img
        canvas = Image.new("RGBA", size, (0, 0, 0, 0))
        canvas.paste(img, (0, 0))
        return canvas

    def _choose_canvas_size(self, sizes):
        if not sizes:
            return (0, 0)
        max_w = max(size[0] for size in sizes)
        max_h = max(size[1] for size in sizes)
        return (max_w, max_h)

    def _clamp_speed_factor(self, value):
        try:
            factor = float(value)
        except (TypeError, ValueError):
            factor = 1.0
        if factor <= 0:
            factor = 1.0
        return max(MIN_SPEED_FACTOR, min(MAX_SPEED_FACTOR, factor))

    def _sanitize_fps(self, value):
        try:
            fps = float(value)
        except (TypeError, ValueError):
            fps = float(DEFAULT_FPS)
        if fps <= 0:
            fps = float(DEFAULT_FPS)
        return fps

    def _sanitize_quality(self, value):
        try:
            quality = int(round(float(value)))
        except (TypeError, ValueError):
            quality = 90
        return max(MIN_QUALITY, min(MAX_QUALITY, quality))

    def _quality_to_palette_size(self, quality):
        # Keep one palette index reserved for transparency.
        return max(16, min(255, int(round((quality / 100.0) * 255))))

    def _quantize_rgba_frame(self, frame, palette_size):
        rgba = frame.convert("RGBA")
        alpha = rgba.getchannel("A")
        # Reserve one index (255) for transparent pixels.
        colors = max(2, min(255, int(palette_size)))
        quantized = rgba.quantize(
            colors=colors,
            method=Image.FASTOCTREE,
            dither=Image.FLOYDSTEINBERG,
        )
        palette = quantized.getpalette() or []
        if len(palette) < 768:
            quantized.putpalette(palette + [0] * (768 - len(palette)))
        transparent_mask = alpha.point(lambda a: 255 if a <= 127 else 0, mode="L")
        quantized.paste(255, mask=transparent_mask)
        quantized.info["transparency"] = 255
        return quantized

    def _ensure_parent_dir(self, path):
        parent = os.path.dirname(path)
        if parent:
            os.makedirs(parent, exist_ok=True)


def _build_frame_range_from_request(input_data):
    frame_range = str(input_data.get("frame_range") or "").strip()
    if frame_range:
        return frame_range

    start = input_data.get("start_frame")
    end = input_data.get("end_frame")
    if isinstance(start, int) or isinstance(end, int):
        start_value = start if isinstance(start, int) else 0
        end_value = end if isinstance(end, int) else 0
        if start_value == 0 and end_value == 0:
            return "all"
        if end_value <= 0:
            return str(start_value)
        return f"{start_value}-{end_value}"

    return "all"


def _normalize_action(value):
    action = str(value or "").strip().lower()
    if action in ("", "split", "export", "export_frames"):
        return "export_frames"
    if action in ("reverse", "reverse_gif"):
        return "reverse"
    if action in ("change_speed", "change_frame_rate", "speed"):
        return "change_speed"
    if action in ("compress", "compress_gif"):
        return "compress"
    if action in ("build", "compose", "combine", "build_gif", "make_gif"):
        return "build_gif"
    if action == "get_frame_count":
        return "get_frame_count"
    return action


def _coerce_input_paths(input_data):
    paths = input_data.get("input_paths")
    if isinstance(paths, list):
        return [p for p in paths if p]
    single = input_data.get("input_path")
    if single:
        return [single]
    return []


def handle_request(input_data):
    tool = GIFTool()
    action = _normalize_action(input_data.get("action"))

    if action == "get_frame_count":
        input_path = input_data.get("input_path")
        if not input_path:
            return {"success": False, "error": "Missing input_path"}
        try:
            with Image.open(input_path) as gif:
                if gif.format != "GIF":
                    raise ValueError("Input file is not a GIF")
                return {
                    "success": True,
                    "input_path": input_path,
                    "frame_count": tool._get_frame_count(gif),
                }
        except Exception as exc:
            return {"success": False, "error": str(exc)}

    if action == "export_frames":
        input_path = input_data.get("input_path")
        output_dir = input_data.get("output_dir")
        if not input_path or not output_dir:
            return {"success": False, "error": "Missing input_path or output_dir"}
        output_format = input_data.get("output_format") or input_data.get("format") or "png"
        frame_range = _build_frame_range_from_request(input_data)
        return tool.export_frames(input_path, output_dir, output_format, frame_range)

    if action == "reverse":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return {"success": False, "error": "Missing input_path or output_path"}
        return tool.reverse_gif(input_path, output_path, loop)

    if action == "change_speed":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        speed_factor = input_data.get("speed_factor")
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return {"success": False, "error": "Missing input_path or output_path"}
        return tool.change_speed(input_path, output_path, speed_factor, loop)

    if action == "compress":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        quality = input_data.get("quality", 90)
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return {"success": False, "error": "Missing input_path or output_path"}
        return tool.compress_gif(input_path, output_path, quality, loop)

    if action == "build_gif":
        output_path = input_data.get("output_path")
        fps = input_data.get("fps")
        loop = input_data.get("loop", 0)
        input_paths = _coerce_input_paths(input_data)
        if not output_path:
            return {"success": False, "error": "Missing output_path"}
        return tool.build_gif(input_paths, output_path, fps, loop)

    return {"success": False, "error": f"Unsupported action: {action}"}


def process(input_data):
    try:
        return handle_request(input_data)
    except Exception as exc:
        logger.error("Process function error: %s", exc, exc_info=True)
        return {"success": False, "error": str(exc)}


def main():
    try:
        input_data = json.load(sys.stdin)
        logger.info("Received GIF request: %s", input_data.get("action") or "export_frames")
        result = handle_request(input_data)
        logger.info("GIF request completed: %s", result.get("success"))
        json.dump(result, sys.stdout)
    except json.JSONDecodeError as exc:
        logger.error("Invalid JSON input: %s", exc)
        json.dump({"success": False, "error": f"Invalid JSON input: {str(exc)}"}, sys.stdout)
    except Exception as exc:
        logger.error("Unexpected error: %s", exc, exc_info=True)
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

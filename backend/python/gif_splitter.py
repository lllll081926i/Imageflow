#!/usr/bin/env python3
"""
GIF Tools Script

This script handles GIF operations using the Pillow library:
- export frames from a GIF
- reverse a GIF
- change GIF playback speed
- build a GIF from input images
- compress a GIF with adjustable quality
- resize a GIF while keeping aspect ratio
- convert animated GIF/APNG/WEBP between each other

Usage:
    python gif_splitter.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import json
import logging
import os
import struct
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
MAX_FRAME_PIXEL_BUDGET = 120_000_000
ANIMATED_OUTPUT_FORMATS = {"gif", "apng", "webp"}


def _error_response(code, message, detail=None):
    payload = {"success": False, "error": message, "error_code": code}
    if detail:
        payload["error_detail"] = str(detail)
    return payload


class GIFTool:
    """Handles GIF operations."""

    def __init__(self):
        logger.info("GIFTool initialized")

    def export_frames(self, input_path, output_dir, output_format="png", frame_range="all"):
        try:
            output_format = (output_format or "png").lower()
            if output_format not in FRAME_OUTPUT_FORMATS:
                return _error_response("GIF_EXPORT_UNSUPPORTED_FORMAT", f"Unsupported output format: {output_format}")
            with Image.open(input_path) as animated:
                frames, _, _, _, _ = self._extract_animation_frames(
                    animated,
                    require_animated=True,
                    source_path=input_path,
                )
            frame_count = len(frames)
            frame_indices = self._parse_frame_range(frame_range, frame_count)
            frame_indices = [i for i in frame_indices if 0 <= i < frame_count]
            if not frame_indices:
                return _error_response("GIF_EXPORT_EMPTY_SELECTION", "No frames selected for export")

            os.makedirs(output_dir, exist_ok=True)
            base_name = Path(input_path).stem
            frame_files = []
            save_format = output_format.upper()

            for frame_idx in frame_indices:
                frame = frames[frame_idx]
                output_filename = f"{base_name}_frame_{frame_idx:04d}.{output_format}"
                output_path = os.path.join(output_dir, output_filename)
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
            return _error_response("GIF_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
        except UnidentifiedImageError:
            return _error_response("GIF_UNSUPPORTED_IMAGE", f"Unsupported image format: {input_path}")
        except Exception as exc:
            logger.error("GIF export failed: %s", exc, exc_info=True)
            return _error_response("GIF_EXPORT_FAILED", str(exc))

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
            return _error_response("GIF_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
        except UnidentifiedImageError:
            return _error_response("GIF_UNSUPPORTED_IMAGE", f"Unsupported image format: {input_path}")
        except Exception as exc:
            logger.error("GIF reverse failed: %s", exc, exc_info=True)
            return _error_response("GIF_REVERSE_FAILED", str(exc))

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
            return _error_response("GIF_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
        except UnidentifiedImageError:
            return _error_response("GIF_UNSUPPORTED_IMAGE", f"Unsupported image format: {input_path}")
        except Exception as exc:
            logger.error("GIF speed change failed: %s", exc, exc_info=True)
            return _error_response("GIF_SPEED_CHANGE_FAILED", str(exc))

    def compress_gif(self, input_path, output_path, quality=90, loop=None):
        try:
            with Image.open(input_path) as gif:
                if gif.format != "GIF":
                    raise ValueError("Input file is not a GIF")
                frames, durations, gif_loop, disposals = self._extract_gif_frames_with_disposal(gif)
                self._assert_frame_pixel_budget(len(frames), gif.size, "compress")

            quality_value = self._sanitize_quality(quality)
            palette_size = self._quality_to_palette_size(quality_value)
            for idx, frame in enumerate(frames):
                frames[idx] = self._quantize_rgba_frame(frame, palette_size)

            loop_value = gif_loop if loop is None else loop
            self._ensure_parent_dir(output_path)
            self._save_gif(
                frames,
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
                "frame_count": len(frames),
                "quality": quality_value,
                "input_size": in_size,
                "output_size": out_size,
            }
        except FileNotFoundError:
            return _error_response("GIF_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
        except UnidentifiedImageError:
            return _error_response("GIF_UNSUPPORTED_IMAGE", f"Unsupported image format: {input_path}")
        except MemoryError as exc:
            return _error_response("GIF_MEMORY_LIMIT", "GIF is too large to process safely", exc)
        except Exception as exc:
            logger.error("GIF compression failed: %s", exc, exc_info=True)
            return _error_response("GIF_COMPRESS_FAILED", str(exc))

    def build_gif(self, input_paths, output_path, fps=None, loop=0):
        try:
            paths = [p for p in (input_paths or []) if p]
            if not paths:
                return _error_response("GIF_BUILD_NO_INPUT", "No input images provided")

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
            return _error_response("GIF_INPUT_NOT_FOUND", f"Input file not found: {exc.filename}")
        except UnidentifiedImageError as exc:
            return _error_response("GIF_UNSUPPORTED_IMAGE", f"Unsupported image format: {exc}")
        except Exception as exc:
            logger.error("GIF build failed: %s", exc, exc_info=True)
            return _error_response("GIF_BUILD_FAILED", str(exc))

    def resize_gif(self, input_path, output_path, width=None, height=None, maintain_aspect=True, loop=None):
        try:
            with Image.open(input_path) as gif:
                if gif.format != "GIF":
                    raise ValueError("Input file is not a GIF")
                frames, durations, gif_loop, disposals = self._extract_gif_frames_with_disposal(gif)
                original_width, original_height = gif.size

            target_width = self._sanitize_dimension(width)
            target_height = self._sanitize_dimension(height)
            keep_aspect = self._coerce_bool(maintain_aspect, default=True)
            if target_width <= 0 and target_height <= 0:
                return _error_response("GIF_RESIZE_INVALID_SIZE", "Missing width or height for resize")

            resized_size = self._resolve_resize_size(
                (original_width, original_height),
                target_width,
                target_height,
                keep_aspect,
            )
            self._assert_frame_pixel_budget(
                len(frames),
                (max(original_width, resized_size[0]), max(original_height, resized_size[1])),
                "resize",
            )
            resample = self._get_resample_filter()
            for idx, frame in enumerate(frames):
                resized = frame.resize(resized_size, resample=resample)
                frames[idx] = self._quantize_rgba_frame(resized, 255)

            loop_value = gif_loop if loop is None else loop
            self._ensure_parent_dir(output_path)
            self._save_gif(
                frames,
                output_path,
                durations,
                loop_value,
                disposal=disposals,
                transparency=255,
            )

            return {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
                "frame_count": len(frames),
                "width": resized_size[0],
                "height": resized_size[1],
                "original_width": original_width,
                "original_height": original_height,
                "maintain_aspect": keep_aspect,
            }
        except FileNotFoundError:
            return _error_response("GIF_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
        except UnidentifiedImageError:
            return _error_response("GIF_UNSUPPORTED_IMAGE", f"Unsupported image format: {input_path}")
        except MemoryError as exc:
            return _error_response("GIF_MEMORY_LIMIT", "GIF is too large to process safely", exc)
        except Exception as exc:
            logger.error("GIF resize failed: %s", exc, exc_info=True)
            return _error_response("GIF_RESIZE_FAILED", str(exc))

    def convert_animation(self, input_path, output_path, output_format, quality=90, loop=None):
        try:
            target = self._sanitize_animation_output_format(output_format)
            with Image.open(input_path) as animated:
                frames, durations, animated_loop, disposals, source_format = self._extract_animation_frames(
                    animated,
                    require_animated=True,
                    source_path=input_path,
                )

            loop_value = animated_loop if loop is None else loop
            self._ensure_parent_dir(output_path)

            if target == "gif":
                quantized = [self._quantize_rgba_frame(frame, 255) for frame in frames]
                self._save_gif(
                    quantized,
                    output_path,
                    durations,
                    loop_value,
                    disposal=disposals,
                    transparency=255,
                )
            elif target == "apng":
                self._save_apng(frames, output_path, durations, loop_value, disposal=disposals)
            elif target == "webp":
                quality_value = self._sanitize_quality(quality)
                self._save_webp(frames, output_path, durations, loop_value, quality_value)
            else:
                return _error_response("ANIMATED_CONVERT_BAD_FORMAT", f"Unsupported output format: {target}")

            return {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
                "frame_count": len(frames),
                "output_format": target,
                "source_format": source_format,
            }
        except FileNotFoundError:
            return _error_response("GIF_INPUT_NOT_FOUND", f"Input file not found: {input_path}")
        except UnidentifiedImageError:
            return _error_response("GIF_UNSUPPORTED_IMAGE", f"Unsupported image format: {input_path}")
        except ValueError as exc:
            return _error_response("ANIMATED_CONVERT_BAD_INPUT", str(exc))
        except Exception as exc:
            logger.error("Animated convert failed: %s", exc, exc_info=True)
            return _error_response("ANIMATED_CONVERT_FAILED", str(exc))

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
        if gif.format != "GIF":
            raise ValueError("Input file is not a GIF")
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
        if gif.format != "GIF":
            raise ValueError("Input file is not a GIF")
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

    def _extract_animation_frames(self, animated, require_animated=False, source_path=None):
        image_format = str(animated.format or "").upper()
        if image_format not in {"GIF", "PNG", "WEBP"}:
            raise ValueError(f"Unsupported animated source format: {image_format or 'unknown'}")
        frame_count = int(getattr(animated, "n_frames", 1) or 1)
        if require_animated and frame_count <= 1:
            raise ValueError("Input image is not an animated image")

        frames = []
        durations = []
        disposals = []
        default_duration = animated.info.get("duration", 100)
        if not isinstance(default_duration, int) or default_duration <= 0:
            default_duration = 100

        for frame in ImageSequence.Iterator(animated):
            rgba = frame.convert("RGBA").copy()
            frames.append(rgba)
            duration = frame.info.get("duration", default_duration)
            if not isinstance(duration, int) or duration <= 0:
                duration = default_duration
            durations.append(duration)
            disposal = frame.info.get("disposal", getattr(frame, "disposal_method", 0))
            if not isinstance(disposal, int) or disposal < 0:
                disposal = 0
            disposals.append(disposal)

        if image_format == "WEBP" and source_path and len(frames) > 1:
            parsed = self._extract_webp_durations(source_path)
            if parsed and len(parsed) == len(frames):
                durations = parsed
        if image_format == "PNG" and source_path and len(frames) > 1:
            parsed = self._extract_apng_durations(source_path)
            if parsed and len(parsed) == len(frames):
                durations = parsed

        loop = animated.info.get("loop", 0)
        if not isinstance(loop, int) or loop < 0:
            loop = 0
        return frames, durations, loop, disposals, image_format

    def _extract_webp_durations(self, file_path):
        try:
            with open(file_path, "rb") as f:
                data = f.read()
            if len(data) < 12 or data[:4] != b"RIFF" or data[8:12] != b"WEBP":
                return None
            offset = 12
            durations = []
            data_len = len(data)
            while offset + 8 <= data_len:
                chunk_id = data[offset:offset + 4]
                chunk_size = struct.unpack_from("<I", data, offset + 4)[0]
                chunk_start = offset + 8
                chunk_end = chunk_start + chunk_size
                if chunk_end > data_len:
                    break
                if chunk_id == b"ANMF" and chunk_size >= 16:
                    payload = data[chunk_start:chunk_end]
                    duration = payload[12] | (payload[13] << 8) | (payload[14] << 16)
                    durations.append(max(1, int(duration)))
                offset = chunk_end + (chunk_size & 1)
            return durations or None
        except Exception:
            return None

    def _extract_apng_durations(self, file_path):
        try:
            with open(file_path, "rb") as f:
                data = f.read()
            if len(data) < 8 or data[:8] != b"\x89PNG\r\n\x1a\n":
                return None
            offset = 8
            durations = []
            data_len = len(data)
            while offset + 12 <= data_len:
                chunk_size = struct.unpack_from(">I", data, offset)[0]
                chunk_type = data[offset + 4:offset + 8]
                chunk_start = offset + 8
                chunk_end = chunk_start + chunk_size
                if chunk_end + 4 > data_len:
                    break
                if chunk_type == b"fcTL" and chunk_size >= 26:
                    payload = data[chunk_start:chunk_end]
                    delay_num = struct.unpack_from(">H", payload, 20)[0]
                    delay_den = struct.unpack_from(">H", payload, 22)[0]
                    if delay_den == 0:
                        delay_den = 100
                    if delay_num == 0:
                        delay_num = 1
                    duration_ms = max(1, int(round((delay_num / delay_den) * 1000.0)))
                    durations.append(duration_ms)
                offset = chunk_end + 4
            return durations or None
        except Exception:
            return None

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

    def _save_apng(self, frames, output_path, durations, loop, disposal=None):
        if not frames:
            raise ValueError("No frames to save")
        first, rest = frames[0], frames[1:]
        save_kwargs = {
            "save_all": True,
            "append_images": rest,
            "duration": durations,
            "loop": loop if loop is not None else 0,
            "optimize": False,
        }
        if disposal is not None:
            if isinstance(disposal, (list, tuple)):
                save_kwargs["disposal"] = [max(0, min(2, int(v))) for v in disposal]
            else:
                save_kwargs["disposal"] = max(0, min(2, int(disposal)))
        first.save(
            output_path,
            format="PNG",
            **save_kwargs,
        )

    def _save_webp(self, frames, output_path, durations, loop, quality):
        if not frames:
            raise ValueError("No frames to save")
        first, rest = frames[0], frames[1:]
        first.save(
            output_path,
            format="WEBP",
            save_all=True,
            append_images=rest,
            duration=durations,
            loop=loop if loop is not None else 0,
            quality=quality,
            lossless=True,
            method=6,
            background=(0, 0, 0, 0),
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

    def _sanitize_animation_output_format(self, value):
        text = str(value or "").strip().lower()
        if text in ("png", "apng"):
            return "apng"
        if text in ("gif", "webp"):
            return text
        raise ValueError(f"Unsupported output format: {value}")

    def _sanitize_dimension(self, value):
        try:
            dim = int(round(float(value)))
        except (TypeError, ValueError):
            dim = 0
        return max(0, dim)

    def _coerce_bool(self, value, default=False):
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

    def _resolve_resize_size(self, source_size, target_width, target_height, keep_aspect):
        src_width, src_height = source_size
        if src_width <= 0 or src_height <= 0:
            raise ValueError("Invalid source GIF size")

        if not keep_aspect:
            width = target_width if target_width > 0 else src_width
            height = target_height if target_height > 0 else src_height
            return max(1, width), max(1, height)

        if target_width > 0 and target_height > 0:
            scale = min(target_width / src_width, target_height / src_height)
            if scale <= 0:
                scale = 1.0
            width = max(1, int(round(src_width * scale)))
            height = max(1, int(round(src_height * scale)))
            return width, height

        if target_width > 0:
            height = max(1, int(round(src_height * (target_width / src_width))))
            return max(1, target_width), height

        width = max(1, int(round(src_width * (target_height / src_height))))
        return width, max(1, target_height)

    def _get_resample_filter(self):
        resampling = getattr(Image, "Resampling", None)
        if resampling is not None:
            return resampling.LANCZOS
        return Image.LANCZOS

    def _assert_frame_pixel_budget(self, frame_count, size, operation):
        if frame_count <= 0:
            return
        width, height = size
        if width <= 0 or height <= 0:
            raise ValueError("Invalid frame size")
        total_pixels = int(frame_count) * int(width) * int(height)
        if total_pixels > MAX_FRAME_PIXEL_BUDGET:
            raise MemoryError(
                f"{operation} requires {total_pixels} frame-pixels, exceeds budget {MAX_FRAME_PIXEL_BUDGET}"
            )

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
    if action in ("resize", "resize_gif", "scale", "scale_gif"):
        return "resize"
    if action in ("build", "compose", "combine", "build_gif", "make_gif"):
        return "build_gif"
    if action in ("convert", "convert_animation", "transcode", "convert_animated", "convert_anim"):
        return "convert_animation"
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
            return _error_response("GIF_BAD_REQUEST", "Missing input_path")
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
            return _error_response("GIF_GET_FRAME_COUNT_FAILED", str(exc))

    if action == "export_frames":
        input_path = input_data.get("input_path")
        output_dir = input_data.get("output_dir")
        if not input_path or not output_dir:
            return _error_response("GIF_BAD_REQUEST", "Missing input_path or output_dir")
        output_format = input_data.get("output_format") or input_data.get("format") or "png"
        frame_range = _build_frame_range_from_request(input_data)
        return tool.export_frames(input_path, output_dir, output_format, frame_range)

    if action == "reverse":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return _error_response("GIF_BAD_REQUEST", "Missing input_path or output_path")
        return tool.reverse_gif(input_path, output_path, loop)

    if action == "change_speed":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        speed_factor = input_data.get("speed_factor")
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return _error_response("GIF_BAD_REQUEST", "Missing input_path or output_path")
        return tool.change_speed(input_path, output_path, speed_factor, loop)

    if action == "compress":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        quality = input_data.get("quality", 90)
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return _error_response("GIF_BAD_REQUEST", "Missing input_path or output_path")
        return tool.compress_gif(input_path, output_path, quality, loop)

    if action == "resize":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        width = input_data.get("width")
        height = input_data.get("height")
        maintain_aspect = input_data.get("maintain_aspect", True)
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return _error_response("GIF_BAD_REQUEST", "Missing input_path or output_path")
        return tool.resize_gif(input_path, output_path, width, height, maintain_aspect, loop)

    if action == "build_gif":
        output_path = input_data.get("output_path")
        fps = input_data.get("fps")
        loop = input_data.get("loop", 0)
        input_paths = _coerce_input_paths(input_data)
        if not output_path:
            return _error_response("GIF_BAD_REQUEST", "Missing output_path")
        return tool.build_gif(input_paths, output_path, fps, loop)

    if action == "convert_animation":
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        output_format = input_data.get("output_format") or input_data.get("format")
        quality = input_data.get("quality", 90)
        loop = input_data.get("loop")
        if not input_path or not output_path:
            return _error_response("GIF_BAD_REQUEST", "Missing input_path or output_path")
        if not output_format:
            return _error_response("GIF_BAD_REQUEST", "Missing output_format")
        return tool.convert_animation(input_path, output_path, output_format, quality, loop)

    return _error_response("GIF_UNSUPPORTED_ACTION", f"Unsupported action: {action}")


def process(input_data):
    try:
        return handle_request(input_data)
    except Exception as exc:
        logger.error("Process function error: %s", exc, exc_info=True)
        return _error_response("GIF_INTERNAL_ERROR", str(exc))


def main():
    try:
        input_data = json.load(sys.stdin)
        logger.info("Received GIF request: %s", input_data.get("action") or "export_frames")
        result = handle_request(input_data)
        logger.info("GIF request completed: %s", result.get("success"))
        json.dump(result, sys.stdout)
    except json.JSONDecodeError as exc:
        logger.error("Invalid JSON input: %s", exc)
        json.dump(_error_response("GIF_INVALID_JSON", f"Invalid JSON input: {str(exc)}"), sys.stdout)
    except Exception as exc:
        logger.error("Unexpected error: %s", exc, exc_info=True)
        json.dump(_error_response("GIF_INTERNAL_ERROR", str(exc)), sys.stdout)


if __name__ == "__main__":
    main()




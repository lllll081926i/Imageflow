#!/usr/bin/env python3
"""
Image Compressor Script using MoZJPEG, PNGQuant, and OxiPNG

This script compresses images using advanced compression libraries:
- MoZJPEG: For JPEG compression with quality control
- PNGQuant (imagequant): For lossy PNG compression with quality control
- OxiPNG (pyoxipng): For lossless PNG optimization

Compression Levels:
- 1: Lossless - 100% quality, lossless algorithms
- 2: Light - 90% quality, minimal quality loss
- 3: Medium - 75% quality, balanced compression
- 4: Heavy - 60% quality, significant compression
- 5: Extreme - 40% quality, maximum compression

Usage:
    python compressor.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
import gc
import tempfile
from pathlib import Path
from PIL import Image
import logging

try:
    import mozjpeg_lossless_optimization

    HAS_MOZJPEG = True
except ImportError:
    HAS_MOZJPEG = False
    logging.warning("mozjpeg-lossless-optimization not available, using Pillow only")

try:
    import imagequant

    HAS_IMAGEQUANT = True
except ImportError:
    HAS_IMAGEQUANT = False
    logging.warning("imagequant not available, using Pillow only")

try:
    import oxipng

    HAS_OXIPNG = True
except ImportError:
    HAS_OXIPNG = False
    logging.warning("pyoxipng not available, using Pillow only")

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


class CompressionLevel:
    """Compression level constants."""

    LOSSLESS = 1  # 100% quality, lossless
    LIGHT = 2  # 90% quality
    MEDIUM = 3  # 75% quality
    HEAVY = 4  # 60% quality
    EXTREME = 5  # 40% quality

    @classmethod
    def get_quality(cls, level):
        """Get quality percentage for a given level."""
        quality_map = {
            cls.LOSSLESS: 100,
            cls.LIGHT: 90,
            cls.MEDIUM: 75,
            cls.HEAVY: 60,
            cls.EXTREME: 40,
        }
        return quality_map.get(level, 75)

    @classmethod
    def validate(cls, level):
        """Validate compression level."""
        return level in [cls.LOSSLESS, cls.LIGHT, cls.MEDIUM, cls.HEAVY, cls.EXTREME]


class ImageCompressor:
    """Handles image compression operations using MoZJPEG, PNGQuant, and OxiPNG."""

    def __init__(self):
        """Initialize the compressor."""
        self.mozjpeg_available = HAS_MOZJPEG
        self.imagequant_available = HAS_IMAGEQUANT
        self.oxipng_available = HAS_OXIPNG

        logger.info(f"ImageCompressor initialized:")
        logger.info(f"  - MoZJPEG: {self.mozjpeg_available}")
        logger.info(f"  - PNGQuant (imagequant): {self.imagequant_available}")
        logger.info(f"  - OxiPNG (pyoxipng): {self.oxipng_available}")

    def compress(
        self,
        input_path,
        output_path,
        level=CompressionLevel.MEDIUM,
        engine="",
        target_size_kb=0,
        strip_metadata=False,
    ):
        """
        Compress an image.

        Args:
            input_path (str): Path to the input image
            output_path (str): Path to save the compressed image
            level (int): Compression level (1-5)

        Returns:
            dict: Compression result with success status and metadata
        """
        tmp_output_path = None
        try:
            strip_metadata = bool(strip_metadata)
            # Validate compression level
            if not CompressionLevel.validate(level):
                logger.warning(f"Invalid compression level: {level}, using MEDIUM")
                level = CompressionLevel.MEDIUM

            input_abs = os.path.abspath(input_path)
            output_abs = os.path.abspath(output_path)
            same_file = input_abs == output_abs

            work_output_path = output_path
            if same_file:
                final_dir = os.path.dirname(output_abs) or "."
                os.makedirs(final_dir, exist_ok=True)
                with tempfile.NamedTemporaryFile(
                    delete=False,
                    dir=final_dir,
                    suffix=Path(output_path).suffix or ".tmp",
                ) as tmp:
                    tmp_output_path = tmp.name
                work_output_path = tmp_output_path
                logger.info(f"Overwrite mode detected; writing to temp: {work_output_path}")

            # Open input image
            logger.info(f"Opening image: {input_path}")
            img = Image.open(input_path)

            # Get original file size
            original_size = os.path.getsize(input_path)
            logger.info(f"Original size: {original_size} bytes")

            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(work_output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)

            # Get image format and compress accordingly
            format_type = img.format or "PNG"
            logger.info(f"Image format: {format_type}")

            target_bytes = 0
            try:
                target_kb_int = int(target_size_kb or 0)
                if target_kb_int > 0:
                    target_bytes = target_kb_int * 1024
            except Exception:
                target_bytes = 0

            engine = str(engine or "").strip().lower()
            warning = ""

            if format_type in ["JPEG", "JPG"]:
                warning = self._compress_jpeg(
                    img,
                    input_path,
                    work_output_path,
                    level,
                    engine=engine,
                    target_bytes=target_bytes,
                    strip_metadata=bool(strip_metadata),
                )
            elif format_type == "PNG":
                warning = self._compress_png(
                    img,
                    input_path,
                    work_output_path,
                    level,
                    engine=engine,
                    target_bytes=target_bytes,
                    strip_metadata=bool(strip_metadata),
                )
            elif format_type == "WEBP":
                warning = self._compress_webp(
                    img,
                    input_path,
                    work_output_path,
                    level,
                    engine=engine,
                    target_bytes=target_bytes,
                    strip_metadata=bool(strip_metadata),
                )
            else:
                # Fallback to Pillow for other formats
                warning = self._compress_fallback(
                    img,
                    input_path,
                    work_output_path,
                    level,
                    engine=engine,
                    target_bytes=target_bytes,
                    strip_metadata=bool(strip_metadata),
                )

            # Explicitly close image to free memory
            img.close()
            del img

            if tmp_output_path:
                os.replace(tmp_output_path, output_path)
                tmp_output_path = None

            # Get compressed file size
            compressed_size = os.path.getsize(output_path)
            compression_rate = (
                (1 - compressed_size / original_size) * 100 if original_size > 0 else 0
            )

            logger.info(
                f"Compressed size: {compressed_size} bytes (saved {compression_rate:.2f}%)"
            )

            # Force garbage collection
            gc.collect()

            # Return success result
            out = {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
                "original_size": original_size,
                "compressed_size": compressed_size,
                "compression_rate": round(compression_rate, 2),
                "compression_level": level,
            }
            if warning:
                out["warning"] = warning
            return out
        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            return {"success": False, "error": f"[NOT_FOUND] Input file not found: {input_path}"}
        except PermissionError as e:
            logger.error(f"Permission denied: {e}", exc_info=True)
            return {"success": False, "error": f"[PERMISSION_DENIED] {str(e)}"}
        except Exception as e:
            logger.error(f"Compression failed: {e}", exc_info=True)
            return {"success": False, "error": f"[INTERNAL] {str(e)}"}
        finally:
            try:
                if tmp_output_path:
                    os.remove(tmp_output_path)
            except Exception:
                pass

    def _compress_jpeg(
        self,
        img,
        input_path,
        output_path,
        level,
        engine="",
        target_bytes=0,
        strip_metadata=False,
    ):
        """Compress JPEG using MoZJPEG or Pillow."""
        logger.info(f"Compressing JPEG (level: {level})")

        quality = CompressionLevel.get_quality(level)

        use_mozjpeg = self.mozjpeg_available and engine in ("", "auto", "mozjpeg")
        force_pillow = engine in ("pillow",)
        lossless_warning = ""
        if level == CompressionLevel.LOSSLESS and not use_mozjpeg and not force_pillow:
            lossless_warning = "MozJPEG 不可用，已使用字节复制模式（仅剥离隐私元数据）"
        if level == CompressionLevel.LOSSLESS and force_pillow:
            lossless_warning = "已选择 Pillow 引擎，改为字节复制模式以保持无损"

        def strip_jpeg_metadata_bytes(data: bytes) -> bytes:
            if not data or len(data) < 4:
                return data
            if not data.startswith(b"\xFF\xD8"):
                return data

            out = bytearray(b"\xFF\xD8")
            i = 2
            strip_markers = {0xE1, 0xED, 0xFE}
            data_len = len(data)
            while i < data_len:
                if data[i] != 0xFF:
                    out.extend(data[i:])
                    break
                while i < data_len and data[i] == 0xFF:
                    i += 1
                if i >= data_len:
                    break
                marker = data[i]
                i += 1
                if marker == 0xD9:
                    out.extend(b"\xFF\xD9")
                    break
                if marker == 0xD8 or (0xD0 <= marker <= 0xD7) or marker == 0x01:
                    out.extend(b"\xFF" + bytes([marker]))
                    continue
                if i + 2 > data_len:
                    break
                seg_len = (data[i] << 8) | data[i + 1]
                if seg_len < 2:
                    break
                seg = data[i : i + seg_len]
                if marker not in strip_markers:
                    out.extend(b"\xFF" + bytes([marker]) + seg)
                i += seg_len
            return bytes(out)

        def strip_jpeg_metadata_file(path: str) -> None:
            try:
                with open(path, "rb") as f:
                    raw = f.read()
                stripped = strip_jpeg_metadata_bytes(raw)
                if stripped != raw:
                    with open(path, "wb") as f:
                        f.write(stripped)
            except Exception:
                return

        def save_once(q):
            if img.mode in ("RGBA", "P", "LA"):
                work = img.convert("RGB")
            else:
                work = img
            if level == CompressionLevel.LOSSLESS and use_mozjpeg and not force_pillow and input_path.lower().endswith((".jpg", ".jpeg")):
                with open(input_path, "rb") as f:
                    jpeg_bytes = f.read()
                optimized_bytes = mozjpeg_lossless_optimization.optimize(jpeg_bytes)
                if strip_metadata:
                    optimized_bytes = strip_jpeg_metadata_bytes(optimized_bytes)
                with open(output_path, "wb") as f:
                    f.write(optimized_bytes)
                return
            if level == CompressionLevel.LOSSLESS and (force_pillow or not use_mozjpeg):
                with open(input_path, "rb") as f:
                    jpeg_bytes = f.read()
                if strip_metadata:
                    jpeg_bytes = strip_jpeg_metadata_bytes(jpeg_bytes)
                with open(output_path, "wb") as f:
                    f.write(jpeg_bytes)
                return

            save_quality = 100 if level == CompressionLevel.LOSSLESS else int(q)
            save_quality = max(1, min(100, save_quality))
            save_kwargs = {
                "format": "JPEG",
                "quality": save_quality,
                "optimize": True,
                "progressive": (level != CompressionLevel.LOSSLESS),
            }
            work.save(output_path, **save_kwargs)
            if use_mozjpeg and not force_pillow and self.mozjpeg_available:
                try:
                    with open(output_path, "rb") as f:
                        jpeg_bytes = f.read()
                    optimized_bytes = mozjpeg_lossless_optimization.optimize(jpeg_bytes)
                    with open(output_path, "wb") as f:
                        f.write(optimized_bytes)
                except Exception as e:
                    logger.warning(f"mozjpeg optimize failed: {e}")
            if strip_metadata:
                strip_jpeg_metadata_file(output_path)

        if target_bytes > 0 and level == CompressionLevel.LOSSLESS:
            save_once(100)
            try:
                if os.path.getsize(output_path) <= target_bytes:
                    return lossless_warning
            except Exception:
                pass
            return (lossless_warning + "，" if lossless_warning else "") + f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"

        if target_bytes > 0:
            best_q = None
            best_size = None
            low = 5
            high = int(quality)
            last_size = None
            stable_hits = 0
            last_q = None
            for _ in range(12):
                if low > high:
                    break
                q = (low + high) // 2
                last_q = q
                save_once(q)
                try:
                    size = os.path.getsize(output_path)
                except Exception:
                    break
                if last_size == size:
                    stable_hits += 1
                else:
                    stable_hits = 0
                last_size = size
                if size <= target_bytes:
                    best_q = q
                    best_size = size
                    low = q + 1
                else:
                    high = q - 1
                if stable_hits >= 2:
                    break
            if best_q is not None:
                if best_size is None or best_q != last_q:
                    save_once(best_q)
                return lossless_warning
            return f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"

        save_once(quality)
        return lossless_warning

    def _compress_png(
        self,
        img,
        input_path,
        output_path,
        level,
        engine="",
        target_bytes=0,
        strip_metadata=False,
    ):
        """Compress PNG using imagequant (lossy) or oxipng (lossless)."""
        logger.info(f"Compressing PNG (level: {level})")

        engine = str(engine or "").strip().lower()
        use_oxipng = self.oxipng_available and engine in ("", "auto", "oxipng")
        use_pngquant = self.imagequant_available and engine in ("", "auto", "pngquant", "imagequant")
        force_pillow = engine in ("pillow",)

        quality = CompressionLevel.get_quality(level)

        def oxipng_level_for(lvl):
            if lvl <= CompressionLevel.LIGHT:
                return 2
            if lvl == CompressionLevel.MEDIUM:
                return 4
            return 6

        def save_lossless():
            save_kwargs = {"format": "PNG", "optimize": True}
            img.save(output_path, **save_kwargs)
            if use_oxipng and not force_pillow:
                try:
                    strip_mode = oxipng.StripChunks.safe()
                    oxipng.optimize(
                        output_path,
                        output_path,
                        level=oxipng_level_for(level),
                        strip=strip_mode,
                    )
                except Exception as e:
                    logger.warning(f"OxiPNG optimization failed: {e}")

        def save_lossy(min_q, max_q, colors_hint=256):
            if use_pngquant and not force_pillow:
                try:
                    if img.mode in ("RGBA", "LA"):
                        quantized_img = imagequant.quantize_pil_image(
                            img,
                            min_quality=min_q,
                            max_quality=max_q,
                            max_colors=256,
                            dithering_level=1.0,
                        )
                        save_kwargs = {"format": "PNG"}
                        quantized_img.save(output_path, **save_kwargs)
                        if use_oxipng and not force_pillow:
                            try:
                                strip_mode = oxipng.StripChunks.safe()
                                oxipng.optimize(
                                    output_path,
                                    output_path,
                                    level=oxipng_level_for(level),
                                    strip=strip_mode,
                                )
                            except Exception as e:
                                logger.warning(f"OxiPNG optimization failed: {e}")
                        return
                    if img.mode == "RGB":
                        rgba_img = img.convert("RGBA")
                        quantized_img = imagequant.quantize_pil_image(
                            rgba_img,
                            min_quality=min_q,
                            max_quality=max_q,
                            max_colors=256,
                            dithering_level=1.0,
                        )
                        save_kwargs = {"format": "PNG"}
                        quantized_img.save(output_path, **save_kwargs)
                        if use_oxipng and not force_pillow:
                            try:
                                strip_mode = oxipng.StripChunks.safe()
                                oxipng.optimize(
                                    output_path,
                                    output_path,
                                    level=oxipng_level_for(level),
                                    strip=strip_mode,
                                )
                            except Exception as e:
                                logger.warning(f"OxiPNG optimization failed: {e}")
                        return
                    if img.mode == "P":
                        save_kwargs = {"format": "PNG", "optimize": True}
                        img.save(output_path, **save_kwargs)
                        if use_oxipng and not force_pillow:
                            try:
                                strip_mode = oxipng.StripChunks.safe()
                                oxipng.optimize(
                                    output_path,
                                    output_path,
                                    level=2,
                                    strip=strip_mode,
                                )
                            except Exception as e:
                                logger.warning(f"OxiPNG optimization failed: {e}")
                        return
                    save_kwargs = {"format": "PNG", "optimize": True}
                    img.save(output_path, **save_kwargs)
                    return
                except Exception as e:
                    logger.warning(f"pngquant compression failed: {e}")

            if img.mode in ("RGBA", "RGB", "L"):
                colors = int(colors_hint)
                colors = max(2, min(256, colors))
                quantized = img.quantize(colors=colors, method=0, dither=1)
                save_kwargs = {"format": "PNG"}
                quantized.save(output_path, **save_kwargs)
                if use_oxipng and not force_pillow:
                    try:
                        strip_mode = oxipng.StripChunks.safe()
                        oxipng.optimize(
                            output_path,
                            output_path,
                            level=oxipng_level_for(level),
                            strip=strip_mode,
                        )
                    except Exception as e:
                        logger.warning(f"OxiPNG optimization failed: {e}")
            else:
                save_kwargs = {"format": "PNG", "optimize": True, "compress_level": 9}
                img.save(output_path, **save_kwargs)

        if level == CompressionLevel.LOSSLESS or engine == "oxipng":
            save_lossless()
            if target_bytes > 0:
                try:
                    if os.path.getsize(output_path) <= target_bytes:
                        return ""
                except Exception:
                    pass
                return f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"
            return ""

        if target_bytes > 0:
            best_q = None
            best_size = None
            low = 10
            high = 95
            last_size = None
            stable_hits = 0
            last_q = None

            def save_for_quality(q):
                min_q = max(0, int(q) - 10)
                max_q = min(100, int(q) + 10)
                colors_hint = 256
                if max_q >= 90:
                    colors_hint = 256
                elif max_q >= 75:
                    colors_hint = 128
                elif max_q >= 60:
                    colors_hint = 64
                else:
                    colors_hint = 32
                save_lossy(min_q, max_q, colors_hint=colors_hint)

            for _ in range(12):
                if low > high:
                    break
                q = (low + high) // 2
                last_q = q
                save_for_quality(q)
                try:
                    size = os.path.getsize(output_path)
                except Exception:
                    break
                if last_size == size:
                    stable_hits += 1
                else:
                    stable_hits = 0
                last_size = size
                if size <= target_bytes:
                    best_q = q
                    best_size = size
                    low = q + 1
                else:
                    high = q - 1
                if stable_hits >= 2:
                    break

            if best_q is not None:
                if best_size is None or best_q != last_q:
                    save_for_quality(best_q)
                return ""
            return f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"

        min_quality = max(0, quality - 10)
        max_quality = min(100, quality + 10)
        save_lossy(min_quality, max_quality)
        return ""

    def _compress_webp(
        self,
        img,
        input_path,
        output_path,
        level,
        engine="",
        target_bytes=0,
        strip_metadata=False,
    ):
        """Compress WEBP using Pillow with quality control."""
        logger.info(f"Compressing WEBP (level: {level})")

        quality = CompressionLevel.get_quality(level)

        def save_once(q):
            if level == CompressionLevel.LOSSLESS:
                save_kwargs = {"format": "WEBP", "lossless": True, "method": 6}
            else:
                q2 = max(1, min(100, int(q)))
                save_kwargs = {"format": "WEBP", "quality": q2, "method": 6}
            img.save(output_path, **save_kwargs)

        if target_bytes > 0 and level == CompressionLevel.LOSSLESS:
            save_once(100)
            try:
                if os.path.getsize(output_path) <= target_bytes:
                    return ""
            except Exception:
                pass
            return f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"

        if target_bytes > 0 and level != CompressionLevel.LOSSLESS:
            best_q = None
            best_size = None
            low = 5
            high = int(quality)
            last_size = None
            stable_hits = 0
            last_q = None
            for _ in range(12):
                if low > high:
                    break
                q = (low + high) // 2
                last_q = q
                save_once(q)
                try:
                    size = os.path.getsize(output_path)
                except Exception:
                    break
                if last_size == size:
                    stable_hits += 1
                else:
                    stable_hits = 0
                last_size = size
                if size <= target_bytes:
                    best_q = q
                    best_size = size
                    low = q + 1
                else:
                    high = q - 1
                if stable_hits >= 2:
                    break
            if best_q is not None:
                if best_size is None or best_q != last_q:
                    save_once(best_q)
                return ""
            return f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"

        save_once(quality)
        return ""

    def _compress_fallback(
        self,
        img,
        input_path,
        output_path,
        level,
        engine="",
        target_bytes=0,
        strip_metadata=False,
    ):
        """Fallback compression for unsupported formats."""
        logger.warning(f"Unsupported format, using fallback compression")

        quality = CompressionLevel.get_quality(level)

        # Try to save with optimization
        try:
            save_kwargs = {"format": img.format, "optimize": True}
            if level != CompressionLevel.LOSSLESS:
                save_kwargs["quality"] = quality
            img.save(output_path, **save_kwargs)
        except Exception as e:
            # Just save without optimization
            logger.warning(f"Optimization failed: {e}, saving without optimization")
            img.save(output_path)

        if target_bytes > 0:
            try:
                if os.path.getsize(output_path) <= target_bytes:
                    return ""
            except Exception:
                pass
            return f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"
        return ""


def process(input_data):
    """
    Process function for worker mode.
    This function is called by the worker.py script for process reuse.

    Args:
        input_data (dict): Input parameters

    Returns:
        dict: Processing result
    """
    try:
        # Extract parameters
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        level = input_data.get("level", CompressionLevel.MEDIUM)
        engine = input_data.get("engine", "")
        target_size_kb = input_data.get("target_size_kb", 0)
        strip_metadata = input_data.get("strip_metadata", False)

        # Validate required parameters
        if not input_path or not output_path:
            return {
                "success": False,
                "error": "Missing required parameters: input_path or output_path",
            }

        # Create compressor and perform compression
        compressor = ImageCompressor()
        result = compressor.compress(
            input_path=input_path,
            output_path=output_path,
            level=level,
            engine=engine,
            target_size_kb=target_size_kb,
            strip_metadata=strip_metadata,
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {"success": False, "error": str(e)}


def main():
    """Main entry point for the compressor script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(
            f"Received compression request: {input_data.get('input_path')} -> {input_data.get('output_path')}"
        )

        # Extract parameters
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        level = input_data.get("level", CompressionLevel.MEDIUM)
        engine = input_data.get("engine", "")
        target_size_kb = input_data.get("target_size_kb", 0)
        strip_metadata = input_data.get("strip_metadata", False)

        # Validate required parameters
        if not input_path or not output_path:
            result = {
                "success": False,
                "error": "Missing required parameters: input_path or output_path",
            }
        else:
            # Create compressor and perform compression
            compressor = ImageCompressor()
            result = compressor.compress(
                input_path=input_path,
                output_path=output_path,
                level=level,
                engine=engine,
                target_size_kb=target_size_kb,
                strip_metadata=strip_metadata,
            )

        # Write result to stdout
        logger.info(f"Compression completed: {result.get('success')}")
        json.dump(result, sys.stdout)

    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON input: {e}")
        json.dump(
            {"success": False, "error": f"Invalid JSON input: {str(e)}"}, sys.stdout
        )
    except Exception as e:
        logger.error(f"Unexpected error: {e}", exc_info=True)
        json.dump({"success": False, "error": str(e)}, sys.stdout)


if __name__ == "__main__":
    main()

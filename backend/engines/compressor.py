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
import tempfile
from io import BytesIO
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
logger = logging.getLogger(__name__)

JPEG_STRIP_MARKERS = {0xE1, 0xED, 0xFE}
JPEG_STANDALONE_MARKERS = {0x01, 0xD8, 0xD9, *range(0xD0, 0xD8)}
COPY_CHUNK_SIZE = 1024 * 1024
COMPRESSIBLE_EXTENSIONS = {
    ".jpg",
    ".jpeg",
    ".png",
    ".webp",
    ".bmp",
    ".tif",
    ".tiff",
    ".avif",
    ".ico",
}
UNSUPPORTED_COMPRESSION_EXTENSIONS = {".svg", ".gif", ".apng"}


def _copy_remaining(src, dst) -> None:
    while True:
        chunk = src.read(COPY_CHUNK_SIZE)
        if not chunk:
            return
        dst.write(chunk)


def _copy_exact(src, dst, size: int) -> None:
    remaining = max(0, int(size))
    while remaining > 0:
        chunk = src.read(min(COPY_CHUNK_SIZE, remaining))
        if not chunk:
            return
        dst.write(chunk)
        remaining -= len(chunk)


def _skip_exact(src, size: int) -> None:
    remaining = max(0, int(size))
    while remaining > 0:
        chunk = src.read(min(COPY_CHUNK_SIZE, remaining))
        if not chunk:
            return
        remaining -= len(chunk)


def _copy_jpeg_without_metadata(src, dst) -> None:
    header = src.read(2)
    if header != b"\xff\xd8":
        dst.write(header)
        _copy_remaining(src, dst)
        return

    dst.write(header)
    while True:
        prefix = src.read(1)
        if not prefix:
            return
        if prefix != b"\xff":
            dst.write(prefix)
            _copy_remaining(src, dst)
            return

        marker_byte = src.read(1)
        while marker_byte == b"\xff":
            marker_byte = src.read(1)
        if not marker_byte:
            dst.write(prefix)
            return

        marker = marker_byte[0]
        if marker in JPEG_STANDALONE_MARKERS:
            dst.write(b"\xff" + marker_byte)
            if marker == 0xD9:
                _copy_remaining(src, dst)
                return
            continue

        length_bytes = src.read(2)
        if len(length_bytes) < 2:
            dst.write(b"\xff" + marker_byte + length_bytes)
            return
        segment_length = (length_bytes[0] << 8) | length_bytes[1]
        if segment_length < 2:
            dst.write(b"\xff" + marker_byte + length_bytes)
            _copy_remaining(src, dst)
            return

        payload_length = segment_length - 2
        if marker in JPEG_STRIP_MARKERS:
            _skip_exact(src, payload_length)
        else:
            dst.write(b"\xff" + marker_byte + length_bytes)
            _copy_exact(src, dst, payload_length)

        if marker == 0xDA:
            _copy_remaining(src, dst)
            return


def _strip_jpeg_metadata_bytes(data: bytes) -> bytes:
    if not data:
        return data
    src = BytesIO(data)
    dst = BytesIO()
    _copy_jpeg_without_metadata(src, dst)
    return dst.getvalue()


def _copy_file_streaming(input_path: str, output_path: str) -> None:
    with open(input_path, "rb") as src, open(output_path, "wb") as dst:
        _copy_remaining(src, dst)


def _append_warning(existing: str, message: str) -> str:
    existing = str(existing or "").strip()
    if not existing:
        return message
    return f"{existing}；{message}"


def _unsupported_compression_error(input_path: str) -> str:
    ext = Path(str(input_path or "")).suffix.lower()
    if not ext:
        return ""
    if ext in UNSUPPORTED_COMPRESSION_EXTENSIONS:
        return "[UNSUPPORTED_FORMAT] 压缩功能仅支持静态位图输入，SVG 请使用格式转换，GIF/APNG 请使用 GIF 工具"
    if ext not in COMPRESSIBLE_EXTENSIONS:
        return f"[UNSUPPORTED_FORMAT] 压缩功能不支持该输入格式: {ext}"
    return ""


def _strip_jpeg_metadata_file(path: str) -> None:
    directory = os.path.dirname(os.path.abspath(path)) or "."
    fd, temp_path = tempfile.mkstemp(prefix=".imageflow-strip-", suffix=".jpg", dir=directory)
    os.close(fd)
    try:
        with open(path, "rb") as src, open(temp_path, "wb") as dst:
            _copy_jpeg_without_metadata(src, dst)
        os.replace(temp_path, path)
    except Exception:
        try:
            os.remove(temp_path)
        except OSError:
            pass
        raise


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
        img = None
        try:
            strip_metadata = bool(strip_metadata)
            # Validate compression level
            if not CompressionLevel.validate(level):
                logger.warning(f"Invalid compression level: {level}, using MEDIUM")
                level = CompressionLevel.MEDIUM

            unsupported_error = _unsupported_compression_error(input_path)
            if unsupported_error:
                return {"success": False, "error": unsupported_error}

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
            except (TypeError, ValueError):
                target_bytes = 0

            engine = str(engine or "").strip().lower()
            warning = ""
            fallback_used = False

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
                fallback_used = True
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
            img = None

            candidate_size = os.path.getsize(work_output_path)
            if (
                original_size > 0
                and candidate_size > original_size
                and (not strip_metadata or fallback_used)
            ):
                warning = _append_warning(warning, "压缩结果大于原图，已保留原文件内容")
                if same_file:
                    os.remove(work_output_path)
                    tmp_output_path = None
                else:
                    _copy_file_streaming(input_path, work_output_path)

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
            if img is not None:
                try:
                    img.close()
                except Exception:
                    pass
            try:
                if tmp_output_path:
                    os.remove(tmp_output_path)
            except OSError as cleanup_err:
                logger.warning(f"Failed to cleanup temp compressed file {tmp_output_path}: {cleanup_err}")

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

        def save_once(q):
            if img.mode in ("RGBA", "P", "LA"):
                work = img.convert("RGB")
            else:
                work = img
            if level == CompressionLevel.LOSSLESS and use_mozjpeg and not force_pillow and input_path.lower().endswith((".jpg", ".jpeg")):
                # MozJPEG optimize API requires full bytes. Cap size to avoid multi-hundred-MB spikes.
                max_mozjpeg_bytes = int(os.getenv("IMAGEFLOW_MOZJPEG_MAX_BYTES", str(48 * 1024 * 1024)))
                try:
                    input_size = os.path.getsize(input_path)
                except OSError:
                    input_size = 0
                if max_mozjpeg_bytes > 0 and input_size > max_mozjpeg_bytes:
                    if strip_metadata:
                        with open(input_path, "rb") as src, open(output_path, "wb") as dst:
                            _copy_jpeg_without_metadata(src, dst)
                    else:
                        _copy_file_streaming(input_path, output_path)
                    return
                with open(input_path, "rb") as f:
                    jpeg_bytes = f.read()
                optimized_bytes = mozjpeg_lossless_optimization.optimize(jpeg_bytes)
                if strip_metadata:
                    optimized_bytes = _strip_jpeg_metadata_bytes(optimized_bytes)
                with open(output_path, "wb") as f:
                    f.write(optimized_bytes)
                return
            if level == CompressionLevel.LOSSLESS and (force_pillow or not use_mozjpeg):
                if strip_metadata:
                    with open(input_path, "rb") as src, open(output_path, "wb") as dst:
                        _copy_jpeg_without_metadata(src, dst)
                else:
                    _copy_file_streaming(input_path, output_path)
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
                    max_mozjpeg_bytes = int(os.getenv("IMAGEFLOW_MOZJPEG_MAX_BYTES", str(48 * 1024 * 1024)))
                    output_size = os.path.getsize(output_path)
                    if max_mozjpeg_bytes > 0 and output_size > max_mozjpeg_bytes:
                        raise RuntimeError("mozjpeg skipped for large output")
                    with open(output_path, "rb") as f:
                        jpeg_bytes = f.read()
                    optimized_bytes = mozjpeg_lossless_optimization.optimize(jpeg_bytes)
                    with open(output_path, "wb") as f:
                        f.write(optimized_bytes)
                except Exception as e:
                    logger.warning(f"mozjpeg optimize failed: {e}")
            if strip_metadata:
                try:
                    _strip_jpeg_metadata_file(output_path)
                except OSError as strip_err:
                    logger.warning(f"Failed to strip JPEG metadata for {output_path}: {strip_err}")

        if target_bytes > 0 and level == CompressionLevel.LOSSLESS:
            save_once(100)
            try:
                if os.path.getsize(output_path) <= target_bytes:
                    return lossless_warning
            except OSError:
                logger.debug("Failed to stat JPEG output during target size check: %s", output_path)
            return (lossless_warning + "，" if lossless_warning else "") + f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"

        if target_bytes > 0:
            import io as _io

            if img.mode in ("RGBA", "P", "LA"):
                work = img.convert("RGB")
            else:
                work = img

            best_q = None
            best_size = None
            low = 5
            high = int(quality)
            last_size = None
            stable_hits = 0
            for _ in range(12):
                if low > high:
                    break
                q = (low + high) // 2
                buf = _io.BytesIO()
                work.save(buf, format="JPEG", quality=q, optimize=False, progressive=True)
                size = buf.tell()
                buf.close()
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
                        quantized_img = None
                        try:
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
                        finally:
                            if quantized_img is not None:
                                quantized_img.close()
                        return
                    if img.mode == "RGB":
                        rgba_img = None
                        quantized_img = None
                        try:
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
                        finally:
                            if quantized_img is not None:
                                quantized_img.close()
                            if rgba_img is not None:
                                rgba_img.close()
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

            if img.mode in ("RGBA", "LA", "RGB", "L"):
                colors = int(colors_hint)
                colors = max(2, min(256, colors))
                quantize_method = 2 if img.mode in ("RGBA", "LA") else 0
                quantized = img.quantize(colors=colors, method=quantize_method, dither=1)
                try:
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
                finally:
                    quantized.close()
            else:
                save_kwargs = {"format": "PNG", "optimize": True, "compress_level": 9}
                img.save(output_path, **save_kwargs)

        if level == CompressionLevel.LOSSLESS or engine == "oxipng":
            save_lossless()
            if target_bytes > 0:
                try:
                    if os.path.getsize(output_path) <= target_bytes:
                        return ""
                except OSError:
                    logger.debug("Failed to stat PNG output during target size check: %s", output_path)
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
                except OSError:
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
            except OSError:
                logger.debug("Failed to stat WEBP output during target size check: %s", output_path)
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
                except OSError:
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
            except OSError:
                logger.debug("Failed to stat fallback output during target size check: %s", output_path)
            return f"目标大小 {int(target_bytes / 1024)}KB 未达成，已输出最小可得文件"
        return ""


def process(input_data):
    """
    Process function used by the desktop API engine bridge.

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

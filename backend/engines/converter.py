#!/usr/bin/env python3
"""
Image Format Converter Script

This script converts images between different formats using the Pillow library.
It supports quality settings, resizing, and aspect ratio preservation.

Supported input formats: JPG, PNG, GIF, BMP, TIFF, WEBP, ICO, PSD, TGA, PVR, EXR, PNM
Supported output formats: JPG, PNG, WEBP, BMP, PDF, ICO, CUR

Usage:
    python converter.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
import gc
import io
import re
import shutil
import subprocess
import tempfile
import xml.etree.ElementTree as ET
from pathlib import Path
from PIL import Image
import logging
import time

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)
_PROFILE_ENABLED = os.getenv("IMAGEFLOW_PROFILE") == "1"


def decode_svg_text(data: bytes) -> str:
    if not data:
        return ""
    if data.startswith((b"\xff\xfe", b"\xfe\xff")):
        try:
            return data.decode("utf-16")
        except UnicodeDecodeError:
            pass
    for encoding in ("utf-8", "utf-16", "latin-1"):
        try:
            return data.decode(encoding, errors="strict" if encoding != "latin-1" else "replace")
        except UnicodeDecodeError:
            continue
    return data.decode("utf-8", errors="ignore")


def contains_unsafe_svg_xml(text: str) -> bool:
    return bool(re.search(r"<!\s*(DOCTYPE|ENTITY)\b", text or "", re.IGNORECASE))


def extract_svg_root_fragment(text: str):
    match = re.search(r"<svg\b[^>]*>", text or "", re.IGNORECASE | re.DOTALL)
    return match.group(0) if match else ""


def extract_svg_attribute(svg_tag: str, name: str):
    if not svg_tag:
        return ""
    pattern = rf"""\b{re.escape(name)}\s*=\s*(['"])(.*?)\1"""
    match = re.search(pattern, svg_tag, re.IGNORECASE | re.DOTALL)
    return match.group(2).strip() if match else ""


def parse_svg_number(v: str):
    v = (v or "").strip()
    if not v:
        return None
    m = re.match(r"^\s*([0-9]*\.?[0-9]+)", v)
    if not m:
        return None
    return float(m.group(1))


def parse_svg_intrinsic_size_from_attrs(width_raw: str, height_raw: str, view_box: str):
    width = parse_svg_number(width_raw)
    height = parse_svg_number(height_raw)
    if width and height and width > 0 and height > 0:
        return int(round(width)), int(round(height))

    if view_box:
        parts = re.split(r"[,\s]+", view_box.strip())
        if len(parts) == 4:
            try:
                vb_w = float(parts[2])
                vb_h = float(parts[3])
                if vb_w > 0 and vb_h > 0:
                    return int(round(vb_w)), int(round(vb_h))
            except (TypeError, ValueError):
                logger.debug("Invalid viewBox size values in SVG: %s", view_box)
    return None


def parse_svg_intrinsic_size_from_text(text: str):
    svg_tag = extract_svg_root_fragment(text)
    if not svg_tag:
        return None
    return parse_svg_intrinsic_size_from_attrs(
        extract_svg_attribute(svg_tag, "width"),
        extract_svg_attribute(svg_tag, "height"),
        extract_svg_attribute(svg_tag, "viewBox") or extract_svg_attribute(svg_tag, "viewbox"),
    )


def parse_svg_intrinsic_size_from_bytes(data: bytes):
    text = decode_svg_text(data)
    if contains_unsafe_svg_xml(text):
        return parse_svg_intrinsic_size_from_text(text)

    try:
        root = ET.fromstring(data)
    except ET.ParseError:
        return parse_svg_intrinsic_size_from_text(text)

    return parse_svg_intrinsic_size_from_attrs(
        root.attrib.get("width", ""),
        root.attrib.get("height", ""),
        root.attrib.get("viewBox", "") or root.attrib.get("viewbox", ""),
    )


def _profile_log(message: str) -> None:
    if _PROFILE_ENABLED:
        logger.info(message)


def _resolve_path(path_value, base_dirs):
    if not path_value:
        return path_value
    if os.path.isabs(path_value):
        return path_value
    for base in base_dirs:
        if not base:
            continue
        candidate = os.path.join(base, path_value)
        if os.path.exists(candidate):
            return os.path.abspath(candidate)
    return os.path.abspath(path_value)


def _normalize_paths(input_path, output_path):
    output_abs = None
    if output_path:
        output_abs = output_path if os.path.isabs(output_path) else os.path.abspath(output_path)
    base_dirs = []
    if output_abs:
        base_dirs.append(os.path.dirname(output_abs))
    base_dirs.append(os.getcwd())
    base_dirs.append(os.path.dirname(os.path.abspath(__file__)))
    input_abs = _resolve_path(input_path, base_dirs)
    return input_abs, output_abs or output_path


def _format_extensions(format_type: str):
    normalized = str(format_type or "").strip().lower()
    groups = {
        "jpg": {"jpg", "jpeg"},
        "jpeg": {"jpg", "jpeg"},
        "tif": {"tif", "tiff"},
        "tiff": {"tif", "tiff"},
    }
    return groups.get(normalized, {normalized} if normalized else set())


def _can_convert_in_place(input_path: str, output_path: str, format_type: str) -> bool:
    if not input_path or not output_path:
        return True
    if os.path.abspath(input_path) != os.path.abspath(output_path):
        return True
    current_ext = Path(input_path).suffix.lower().lstrip(".")
    if not current_ext:
        return False
    return current_ext in _format_extensions(format_type)


class ImageConverter:
    """Handles image format conversion operations."""
    
    # Supported output formats and their parameters
    OUTPUT_FORMATS = ['jpg', 'jpeg', 'png', 'webp', 'bmp', 'tiff', 'tif', 'ico', 'avif']
    VALID_ICO_SIZES = {16, 32, 48, 64, 128, 256}

    # Quality-aware formats
    QUALITY_FORMATS = ['jpg', 'jpeg', 'webp', 'avif']
    
    def __init__(self):
        """Initialize the converter."""
        logger.info("ImageConverter initialized")

    def _parse_svg_intrinsic_size(self, svg_path: str):
        try:
            data = Path(svg_path).read_bytes()
        except OSError:
            return None
        return parse_svg_intrinsic_size_from_bytes(data)

    def _calculate_svg_render_size(self, svg_path: str, resize_mode: str, scale_percent: int, long_edge: int, width: int, height: int, maintain_ar: bool, format_type: str, ico_sizes):
        base = self._parse_svg_intrinsic_size(svg_path) or (1024, 1024)
        base_w, base_h = base

        mode = str(resize_mode or "").strip().lower()
        target_w, target_h = base_w, base_h

        if mode == "percent" and int(scale_percent or 0) > 0:
            pct = max(1, int(scale_percent))
            target_w = max(1, int(base_w * pct / 100))
            target_h = max(1, int(base_h * pct / 100))
        elif mode == "long_edge" and int(long_edge or 0) > 0:
            le = max(1, int(long_edge))
            scale = le / float(max(base_w, base_h))
            target_w = max(1, int(base_w * scale))
            target_h = max(1, int(base_h * scale))
        elif mode == "fixed" and (int(width or 0) > 0 or int(height or 0) > 0):
            w = int(width or 0)
            h = int(height or 0)
            if maintain_ar:
                if w > 0 and h == 0:
                    scale = w / float(base_w)
                    target_w = w
                    target_h = max(1, int(base_h * scale))
                elif h > 0 and w == 0:
                    scale = h / float(base_h)
                    target_h = h
                    target_w = max(1, int(base_w * scale))
                elif w > 0 and h > 0:
                    scale = min(w / float(base_w), h / float(base_h))
                    target_w = max(1, int(base_w * scale))
                    target_h = max(1, int(base_h * scale))
            else:
                target_w = w if w > 0 else base_w
                target_h = h if h > 0 else base_h
        else:
            if int(width or 0) > 0 or int(height or 0) > 0:
                w = int(width or 0)
                h = int(height or 0)
                if maintain_ar:
                    if w > 0 and h == 0:
                        scale = w / float(base_w)
                        target_w = w
                        target_h = max(1, int(base_h * scale))
                    elif h > 0 and w == 0:
                        scale = h / float(base_h)
                        target_h = h
                        target_w = max(1, int(base_w * scale))
                    elif w > 0 and h > 0:
                        scale = min(w / float(base_w), h / float(base_h))
                        target_w = max(1, int(base_w * scale))
                        target_h = max(1, int(base_h * scale))
                else:
                    target_w = w if w > 0 else base_w
                    target_h = h if h > 0 else base_h

        if format_type == "ico" and ico_sizes and isinstance(ico_sizes, list):
            try:
                max_size = max(int(s) for s in ico_sizes if int(s) > 0)
                target_edge = max(target_w, target_h, max_size)
                target_w = target_edge
                target_h = target_edge
            except (TypeError, ValueError):
                logger.debug("Invalid ico size value in request: %s", ico_sizes)

        return max(1, int(target_w)), max(1, int(target_h))

    def _svg_to_pil(self, svg_path: str, render_width: int, render_height: int):
        try:
            import cairosvg  # type: ignore
            png_data = cairosvg.svg2png(url=svg_path, output_width=render_width, output_height=render_height)
            img = Image.open(io.BytesIO(png_data))
            img.load()
            if render_width > 0 and render_height > 0 and img.size != (render_width, render_height):
                img = img.resize((render_width, render_height), Image.Resampling.LANCZOS)
            return img
        except ImportError:
            logger.debug("cairosvg not available, fallback to svglib/inkscape")
        except Exception as e:
            logger.warning(f"cairosvg SVG render failed: {e}")

        try:
            from svglib.svglib import svg2rlg  # type: ignore
            from reportlab.graphics import renderPM  # type: ignore
            from reportlab.graphics.shapes import Drawing, Group  # type: ignore

            drawing = svg2rlg(svg_path)
            if drawing is None:
                raise RuntimeError("svg2rlg returned None")

            if render_width > 0 and render_height > 0:
                source_w = float(getattr(drawing, "width", render_width) or render_width)
                source_h = float(getattr(drawing, "height", render_height) or render_height)
                sx = render_width / source_w
                sy = render_height / source_h

                wrapped = Drawing(render_width, render_height)
                group = Group(drawing)
                group.scale(sx, sy)
                wrapped.add(group)
                drawing = wrapped

            png_bytes = renderPM.drawToString(drawing, fmt="PNG")
            img = Image.open(io.BytesIO(png_bytes))
            img.load()
            if render_width > 0 and render_height > 0 and img.size != (render_width, render_height):
                img = img.resize((render_width, render_height), Image.Resampling.LANCZOS)
            return img
        except ImportError:
            logger.debug("svglib/reportlab not available, fallback to inkscape")
        except Exception as e:
            logger.warning(f"svglib SVG render failed: {e}")

        inkscape = shutil.which("inkscape")
        if inkscape:
            try:
                with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as tmp:
                    tmp_path = tmp.name
                args = [
                    inkscape,
                    svg_path,
                    "--export-type=png",
                    f"--export-filename={tmp_path}",
                ]
                if render_width > 0:
                    args.append(f"--export-width={render_width}")
                if render_height > 0:
                    args.append(f"--export-height={render_height}")
                subprocess.run(args, check=True, capture_output=True)
                img = Image.open(tmp_path)
                img.load()
                try:
                    os.unlink(tmp_path)
                except OSError as cleanup_err:
                    logger.warning(f"Failed to cleanup temp SVG render file {tmp_path}: {cleanup_err}")
                if render_width > 0 and render_height > 0 and img.size != (render_width, render_height):
                    img = img.resize((render_width, render_height), Image.Resampling.LANCZOS)
                return img
            except Exception as e:
                logger.warning(f"Inkscape SVG render failed: {e}")

        raise RuntimeError(
            "SVG input is not supported by the current Python environment. "
            "Install one of: cairosvg (recommended), svglib+lxml, or inkscape."
        )

    def _normalize_ico_sizes(self, ico_sizes):
        values = []
        if ico_sizes and isinstance(ico_sizes, list):
            for s in ico_sizes:
                try:
                    v = int(s)
                    if v in self.VALID_ICO_SIZES:
                        values.append(v)
                except (TypeError, ValueError):
                    continue
        values = sorted(set(values))
        return values

    def _default_ico_sizes(self, source_edge):
        edge = max(1, int(source_edge or 0))
        allowed = sorted(self.VALID_ICO_SIZES)
        sizes = [size for size in allowed if size <= min(edge, 256)]
        if not sizes:
            return [allowed[0]]
        return sizes

    def _prepare_ico_image(self, img, ico_sizes):
        source_edge = max(img.size)
        sizes = self._normalize_ico_sizes(ico_sizes)
        if not sizes:
            sizes = self._default_ico_sizes(source_edge)
        max_edge = max(sizes) if sizes else source_edge

        if img.mode != 'RGBA':
            img = img.convert('RGBA')

        w, h = img.size
        edge = max(w, h)
        if w != h:
            canvas = Image.new('RGBA', (edge, edge), (0, 0, 0, 0))
            canvas.paste(img, ((edge - w) // 2, (edge - h) // 2))
            img = canvas

        if edge != max_edge:
            img = img.resize((max_edge, max_edge), Image.Resampling.LANCZOS)

        return img, sizes
    
    def convert(self, input_path, output_path, format_type, quality=95,
                width=0, height=0, maintain_ar=True,
                resize_mode='',
                scale_percent=0,
                long_edge=0,
                keep_metadata=False,
                compress_level=6,
                ico_sizes=None):
        """
        Convert an image to a different format.
        
        Args:
            input_path (str): Path to the input image
            output_path (str): Path to save the converted image
            format_type (str): Target format (jpg, png, webp, etc.)
            quality (int): Quality setting (1-100) for jpg/webp
            width (int): Target width (0 to keep original)
            height (int): Target height (0 to keep original)
            maintain_ar (bool): Maintain aspect ratio when resizing
            compress_level (int): ZLIB compression level for PNG (0-9)
            ico_sizes (list): List of sizes for ICO format
        
        Returns:
            dict: Conversion result with success status and metadata
        """
        try:
            # Validate format
            format_type = format_type.lower()
            if format_type not in self.OUTPUT_FORMATS:
                return {
                    'success': False,
                    'error': f'[UNSUPPORTED_FORMAT] Unsupported output format: {format_type}'
                }
            if not _can_convert_in_place(input_path, output_path, format_type):
                return {
                    'success': False,
                    'error': '[BAD_INPUT] 目标格式会改变文件扩展名，不能直接覆盖源文件，请另存为新文件'
                }

            total_start = time.perf_counter() if _PROFILE_ENABLED else 0.0
            open_elapsed = 0.0
            resize_elapsed = 0.0
            save_elapsed = 0.0
            
            # Open input image (special handling for SVG)
            open_start = time.perf_counter() if _PROFILE_ENABLED else 0.0
            if input_path.lower().endswith('.svg'):
                logger.info(f"Detected SVG file: {input_path}")
                try:
                    render_w, render_h = self._calculate_svg_render_size(
                        input_path,
                        resize_mode=resize_mode,
                        scale_percent=scale_percent,
                        long_edge=long_edge,
                        width=width,
                        height=height,
                        maintain_ar=maintain_ar,
                        format_type=format_type,
                        ico_sizes=ico_sizes,
                    )
                    img = self._svg_to_pil(input_path, render_w, render_h)
                    logger.info(f"SVG rasterized: {render_w}x{render_h}")
                    resize_mode = ""
                    scale_percent = 0
                    long_edge = 0
                    width = 0
                    height = 0
                except Exception as e:
                    logger.error(f"Failed to open SVG: {e}")
                    return {
                        'success': False,
                        'error': f'Failed to open SVG: {str(e)}'
                    }
            else:
                logger.info(f"Opening image: {input_path}")
                img = Image.open(input_path)

            if _PROFILE_ENABLED:
                open_elapsed = time.perf_counter() - open_start

            exif_bytes = img.info.get('exif')

            mode = str(resize_mode or '').strip().lower()
            resized = False
            resize_start = time.perf_counter() if _PROFILE_ENABLED else 0.0
            if mode == 'percent' and int(scale_percent or 0) > 0:
                pct = max(1, int(scale_percent))
                new_w = max(1, int(img.size[0] * pct / 100))
                new_h = max(1, int(img.size[1] * pct / 100))
                img = img.resize((new_w, new_h), Image.Resampling.LANCZOS)
                resized = True
            elif mode == 'long_edge' and int(long_edge or 0) > 0:
                le = max(1, int(long_edge))
                w0, h0 = img.size
                if max(w0, h0) != le:
                    scale = le / float(max(w0, h0))
                    new_w = max(1, int(w0 * scale))
                    new_h = max(1, int(h0 * scale))
                    img = img.resize((new_w, new_h), Image.Resampling.LANCZOS)
                    resized = True
            elif mode == 'fixed':
                if width > 0 or height > 0:
                    img = self._resize_image(img, width, height, maintain_ar)
                    resized = True
            else:
                if width > 0 or height > 0:
                    img = self._resize_image(img, width, height, maintain_ar)
                    resized = True

            if _PROFILE_ENABLED and resized:
                resize_elapsed = time.perf_counter() - resize_start
            
            if format_type == 'ico':
                img, ico_sizes = self._prepare_ico_image(img, ico_sizes)

            img = self._prepare_for_output(img, format_type)

            # Prepare save parameters based on format
            save_params = self._get_save_params(format_type, quality, compress_level, ico_sizes)
            if keep_metadata and exif_bytes:
                save_params['exif'] = exif_bytes
            
            # Convert format names for Pillow
            pillow_format = self._convert_format_name(format_type)
            
            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Save the image
            tmp_output_path = None
            save_path = output_path
            try:
                if os.path.abspath(input_path) == os.path.abspath(output_path):
                    final_dir = os.path.dirname(os.path.abspath(output_path)) or "."
                    os.makedirs(final_dir, exist_ok=True)
                    with tempfile.NamedTemporaryFile(
                        suffix=Path(output_path).suffix or ".tmp",
                        delete=False,
                        dir=final_dir,
                    ) as tmp:
                        tmp_output_path = tmp.name
                    save_path = tmp_output_path
                    logger.info(f"Overwrite mode detected; writing to temp: {save_path}")

                logger.info(f"Saving to: {save_path} (format: {pillow_format})")
                save_start = time.perf_counter() if _PROFILE_ENABLED else 0.0
                img.save(save_path, format=pillow_format, **save_params)
                if _PROFILE_ENABLED:
                    save_elapsed = time.perf_counter() - save_start

                if tmp_output_path:
                    os.replace(tmp_output_path, output_path)
                    tmp_output_path = None
            finally:
                try:
                    if tmp_output_path:
                        os.remove(tmp_output_path)
                except OSError as cleanup_err:
                    logger.warning(f"Failed to cleanup temp output file {tmp_output_path}: {cleanup_err}")

            # Explicitly close image to free memory
            img.close()
            del img

            # Force garbage collection for large batch processing
            gc.collect()

            if _PROFILE_ENABLED:
                total_elapsed = time.perf_counter() - total_start
                _profile_log(
                    "[Profile] convert total={:.3f}s open={:.3f}s resize={:.3f}s save={:.3f}s format={}".format(
                        total_elapsed,
                        open_elapsed,
                        resize_elapsed,
                        save_elapsed,
                        format_type,
                    )
                )

            # Return success result
            return {
                'success': True,
                'input_path': input_path,
                'output_path': output_path
            }
            
        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            return {
                'success': False,
                'error': f'[NOT_FOUND] Input file not found: {input_path}'
            }
        except Exception as e:
            logger.error(f"Conversion failed: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _resize_image(self, img, target_width, target_height, maintain_ar):
        """
        Resize an image with optional aspect ratio preservation.
        
        Args:
            img: PIL Image object
            target_width (int): Target width
            target_height (int): Target height
            maintain_ar (bool): Whether to maintain aspect ratio
        
        Returns:
            PIL Image: Resized image
        """
        original_width, original_height = img.size
        
        # Calculate new dimensions
        if maintain_ar:
            if target_width > 0 and target_height == 0:
                # Width specified, calculate height
                ratio = target_width / original_width
                new_height = int(original_height * ratio)
                new_width = target_width
            elif target_height > 0 and target_width == 0:
                # Height specified, calculate width
                ratio = target_height / original_height
                new_width = int(original_width * ratio)
                new_height = target_height
            else:
                # Both specified, use the limiting dimension
                ratio_w = target_width / original_width
                ratio_h = target_height / original_height
                ratio = min(ratio_w, ratio_h)
                new_width = int(original_width * ratio)
                new_height = int(original_height * ratio)
        else:
            # Force stretch (do not maintain aspect ratio)
            new_width = target_width if target_width > 0 else original_width
            new_height = target_height if target_height > 0 else original_height
        
        logger.info(f"Resizing from {original_width}x{original_height} to {new_width}x{new_height}")
        
        # Use high-quality resampling
        return img.resize((new_width, new_height), Image.Resampling.LANCZOS)

    def _prepare_for_output(self, img, format_type):
        if format_type not in ['jpg', 'jpeg', 'pdf']:
            return img
        if img.mode in ('RGBA', 'LA'):
            return self._flatten_alpha(img)
        if img.mode == 'P' and 'transparency' in img.info:
            return self._flatten_alpha(img.convert('RGBA'))
        if img.mode != 'RGB':
            return img.convert('RGB')
        return img

    def _flatten_alpha(self, img, background=(255, 255, 255)):
        if img.mode != 'RGBA':
            img = img.convert('RGBA')
        base = Image.new('RGB', img.size, background)
        base.paste(img, mask=img.split()[3])
        return base
    
    def _get_save_params(self, format_type, quality, compress_level=6, ico_sizes=None):
        """
        Get save parameters based on the output format.
        
        Args:
            format_type (str): Output format
            quality (int): Quality setting
            compress_level (int): PNG compression level
            ico_sizes (list): ICO sizes
        
        Returns:
            dict: Save parameters for PIL
        """
        params = {}
        
        if format_type in self.QUALITY_FORMATS:
            params['quality'] = max(1, min(100, quality))
            # Optimize for smaller file size
            params['optimize'] = True
        
        if format_type in ['jpg', 'jpeg']:
            params['progressive'] = True
        
        if format_type == 'png':
            params['compress_level'] = max(0, min(9, compress_level))
        
        if format_type == 'webp':
            params['method'] = 6  # Best compression
        
        if format_type == 'ico':
            if ico_sizes and isinstance(ico_sizes, list):
                # Convert list of ints/lists to tuples if needed, but Pillow expects list of tuples
                # Input might be [16, 32] -> [(16,16), (32,32)]
                sizes = []
                for s in ico_sizes:
                    if isinstance(s, (int, float)):
                        s = int(s)
                        sizes.append((s, s))
                    elif isinstance(s, (list, tuple)) and len(s) == 2:
                        sizes.append((int(s[0]), int(s[1])))
                if sizes:
                    params['sizes'] = sizes

        return params
    
    def _convert_format_name(self, format_type):
        """
        Convert format name for Pillow.
        
        Args:
            format_type (str): Input format name
        
        Returns:
            str: Pillow-compatible format name
        """
        # Pillow expects specific format names
        format_mapping = {
            'jpg': 'JPEG',
            'jpeg': 'JPEG',
            'tif': 'TIFF',
            'tiff': 'TIFF',
            'cur': 'ICO',
        }
        
        return format_mapping.get(format_type.lower(), format_type.upper())


def is_svg_path(input_path):
    return str(input_path or "").strip().lower().endswith(".svg")


def open_image_with_svg_support(input_path, *, format_type="png", ico_sizes=None):
    if is_svg_path(input_path):
        converter = ImageConverter()
        render_w, render_h = converter._calculate_svg_render_size(
            str(input_path),
            resize_mode="",
            scale_percent=0,
            long_edge=0,
            width=0,
            height=0,
            maintain_ar=True,
            format_type=str(format_type or "png").lower(),
            ico_sizes=ico_sizes,
        )
        img = converter._svg_to_pil(str(input_path), render_w, render_h)
        img.load()
        return img

    img = Image.open(input_path)
    img.load()
    return img


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
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')
        input_path, output_path = _normalize_paths(input_path, output_path)
        format_type = input_data.get('format', 'jpg')
        quality = input_data.get('quality', 95)
        width = input_data.get('width', 0)
        height = input_data.get('height', 0)
        maintain_ar = input_data.get('maintain_ar', True)
        resize_mode = input_data.get('resize_mode', '')
        scale_percent = input_data.get('scale_percent', 0)
        long_edge = input_data.get('long_edge', 0)
        keep_metadata = input_data.get('keep_metadata', False)
        compress_level = input_data.get('compress_level', 6)
        ico_sizes = input_data.get('ico_sizes', None)
        if not ico_sizes:
            ico_sizes = input_data.get('icoSizes', None)

        # Validate required parameters
        if not input_path or not output_path:
            return {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }

        # Create converter and perform conversion
        converter = ImageConverter()
        result = converter.convert(
            input_path=input_path,
            output_path=output_path,
            format_type=format_type,
            quality=quality,
            width=width,
            height=height,
            maintain_ar=maintain_ar,
            resize_mode=resize_mode,
            scale_percent=scale_percent,
            long_edge=long_edge,
            keep_metadata=keep_metadata,
            compress_level=compress_level,
            ico_sizes=ico_sizes
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the converter script."""
    try:
        try:
            sys.stdin.reconfigure(encoding="utf-8", errors="strict")
            sys.stdout.reconfigure(encoding="utf-8", errors="strict")
        except (AttributeError, ValueError, OSError):
            logger.debug("stdin/stdout reconfigure is not supported on this runtime")

        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received conversion request: {input_data.get('input_path')} -> {input_data.get('output_path')}")
        
        # Extract parameters
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')
        input_path, output_path = _normalize_paths(input_path, output_path)
        format_type = input_data.get('format', 'jpg')
        quality = input_data.get('quality', 95)
        width = input_data.get('width', 0)
        height = input_data.get('height', 0)
        maintain_ar = input_data.get('maintain_ar', True)
        resize_mode = input_data.get('resize_mode', '')
        scale_percent = input_data.get('scale_percent', 0)
        long_edge = input_data.get('long_edge', 0)
        keep_metadata = input_data.get('keep_metadata', False)
        compress_level = input_data.get('compress_level', 6)
        ico_sizes = input_data.get('ico_sizes', None)
        if not ico_sizes:
            ico_sizes = input_data.get('icoSizes', None)
        
        # Validate required parameters
        if not input_path or not output_path:
            result = {
                'success': False,
                'error': '[BAD_INPUT] Missing required parameters: input_path or output_path'
            }
        else:
            # Create converter and perform conversion
            converter = ImageConverter()
            result = converter.convert(
                input_path=input_path,
                output_path=output_path,
                format_type=format_type,
                quality=quality,
                width=width,
                height=height,
                maintain_ar=maintain_ar,
                resize_mode=resize_mode,
                scale_percent=scale_percent,
                long_edge=long_edge,
                keep_metadata=keep_metadata,
                compress_level=compress_level,
                ico_sizes=ico_sizes
            )
        
        # Write result to stdout
        logger.info(f"Conversion completed: {result.get('success')}")
        json.dump(result, sys.stdout)
        
    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON input: {e}")
        json.dump({
            'success': False,
            'error': f'[BAD_INPUT] Invalid JSON input: {str(e)}'
        }, sys.stdout)
    except Exception as e:
        logger.error(f"Unexpected error: {e}", exc_info=True)
        json.dump({
            'success': False,
            'error': f'[INTERNAL] {str(e)}'
        }, sys.stdout)


if __name__ == '__main__':
    main()

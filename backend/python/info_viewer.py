#!/usr/bin/env python3
"""
Image Info Viewer Script

This script extracts and displays detailed information about images,
including basic info and metadata (ExifRead + piexif).

Usage:
    python info_viewer.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
import traceback
import logging
import struct
import zlib
from pathlib import Path

import exifread
import piexif

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


class InfoViewer:
    """Handles image information extraction and display."""
    MAX_TEXT_LENGTH = 4000
    HEX_FOLD_THRESHOLD = 512
    HEX_HEAD_BYTES = 128
    HEX_TAIL_BYTES = 32

    def __init__(self):
        """Initialize the info viewer."""
        logger.info("InfoViewer initialized")

    def get_info(self, input_path):
        """
        Get detailed information about an image.

        Args:
            input_path (str): Path to the image file

        Returns:
            dict: Image information including basic info and metadata
        """
        try:
            logger.info(f"Reading image info: {input_path}")

            file_info = self._get_file_info(input_path)

            image_info, extra_meta = self._read_format_info(input_path)
            exifread_meta = self._get_exifread_data(input_path, image_info.get("format"))
            piexif_meta = self._get_piexif_data(input_path, image_info.get("format"))

            image_info = self._fill_image_info_from_exif(image_info, exifread_meta, piexif_meta)

            metadata_groups = {
                "exifread": exifread_meta,
                "piexif": piexif_meta,
                "extra": extra_meta,
            }
            flat_meta = self._flatten_metadata(metadata_groups)

            result = {
                "file_name": file_info["name"],
                "file_size": file_info["size"],
                "modified": file_info["modified"],
                "format": image_info.get("format") or "Unknown",
                "width": image_info.get("width") or 0,
                "height": image_info.get("height") or 0,
                "mode": image_info.get("mode") or "Unknown",
                "bit_depth": image_info.get("bit_depth") or 0,
                "exif": flat_meta,
                "metadata": metadata_groups,
                "success": True,
            }

            logger.info(
                "Successfully read image info: %sx%s %s",
                result["width"],
                result["height"],
                result["format"],
            )

            return result

        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            return {"success": False, "error": f"[NOT_FOUND] File not found: {input_path}"}
        except Exception as e:
            logger.error(f"Failed to get image info: {e}", exc_info=True)
            return {"success": False, "error": f"[INTERNAL] {str(e)}"}

    def _get_file_info(self, file_path):
        """Get basic file information."""
        stat = os.stat(file_path)
        return {
            "name": Path(file_path).name,
            "size": stat.st_size,
            "modified": int(stat.st_mtime),
        }

    def _flatten_metadata(self, groups):
        flat = {}
        for group_name, items in groups.items():
            if not items:
                continue
            for key, value in items.items():
                if key in flat:
                    flat[f"{group_name}:{key}"] = value
                else:
                    flat[key] = value
        return flat

    def _stringify_value(self, value, max_length=None):
        if value is None:
            return ""
        if isinstance(value, bytes):
            text = self._bytes_to_string(value)
        else:
            text = str(value)
        if max_length is None:
            max_length = self.MAX_TEXT_LENGTH
        if max_length and len(text) > max_length:
            return self._fold_text(text, max_length)
        return text

    def _bytes_to_string(self, value):
        if not value:
            return ""
        try:
            decoded = value.decode("utf-8")
            if self._is_probably_text(decoded):
                return decoded
        except UnicodeDecodeError:
            pass
        if len(value) <= self.HEX_FOLD_THRESHOLD:
            return f"hex:{len(value)}:{value.hex()}"
        head = value[: self.HEX_HEAD_BYTES].hex()
        tail = value[-self.HEX_TAIL_BYTES :].hex() if self.HEX_TAIL_BYTES > 0 else ""
        folded = len(value) - self.HEX_HEAD_BYTES - (self.HEX_TAIL_BYTES if self.HEX_TAIL_BYTES > 0 else 0)
        if tail:
            return f"hex:{len(value)}:{head}...<folded {folded} bytes>...{tail}"
        return f"hex:{len(value)}:{head}...<folded {folded} bytes>"

    def _fold_text(self, text, max_length):
        if len(text) <= max_length:
            return text
        head_len = max(0, max_length // 2)
        tail_len = max(0, max_length - head_len)
        middle = len(text) - head_len - tail_len
        if tail_len == 0:
            return f"{text[:head_len]}...<folded {middle} chars>"
        return f"{text[:head_len]}...<folded {middle} chars>...{text[-tail_len:]}"

    def _is_probably_text(self, text):
        if not text:
            return False
        printable = sum(1 for ch in text if ch.isprintable() or ch in "\r\n\t")
        return printable / len(text) > 0.85

    def _get_exifread_data(self, input_path, fmt=None):
        tags_out = {}
        supported = {"JPEG", "JPG", "TIFF", "TIF", "WEBP"}
        if fmt and fmt.upper() not in supported:
            return tags_out
        try:
            with open(input_path, "rb") as f:
                tags = exifread.process_file(f, details=True)
            for tag, value in tags.items():
                raw_values = getattr(value, "values", None)
                if isinstance(raw_values, (bytes, bytearray)):
                    tags_out[tag] = self._stringify_value(bytes(raw_values))
                else:
                    printable = getattr(value, "printable", None)
                    tags_out[tag] = self._stringify_value(printable if printable is not None else raw_values or value)
        except Exception as e:
            logger.warning(f"ExifRead failed: {e}")
        return tags_out

    def _get_piexif_data(self, input_path, fmt=None):
        tags_out = {}
        supported = {"JPEG", "JPG", "TIFF", "TIF", "WEBP"}
        if fmt and fmt.upper() not in supported:
            return tags_out
        try:
            exif_dict = piexif.load(input_path)
        except Exception as e:
            logger.warning(f"piexif load failed: {e}")
            return tags_out

        for ifd_name, ifd in exif_dict.items():
            if ifd_name == "thumbnail":
                if ifd:
                    tags_out["thumbnail_bytes"] = self._stringify_value(ifd)
                continue
            if not isinstance(ifd, dict):
                continue
            tag_map = piexif.TAGS.get(ifd_name, {})
            for tag_id, value in ifd.items():
                tag_info = tag_map.get(tag_id)
                tag_name = tag_info["name"] if tag_info else str(tag_id)
                key = f"{ifd_name}:{tag_name}"
                tags_out[key] = self._stringify_value(value)
        return tags_out

    def _fill_image_info_from_exif(self, image_info, exifread_meta, piexif_meta):
        info = dict(image_info or {})

        if not info.get("width"):
            info["width"] = self._get_dimension_from_exif(exifread_meta, piexif_meta, "width")
        if not info.get("height"):
            info["height"] = self._get_dimension_from_exif(exifread_meta, piexif_meta, "height")

        if not info.get("bit_depth"):
            info["bit_depth"] = self._get_bit_depth_from_exif(exifread_meta, piexif_meta)

        if not info.get("mode") and info.get("bit_depth"):
            info["mode"] = "RGB" if info["bit_depth"] >= 24 else "L"

        return info

    def _get_dimension_from_exif(self, exifread_meta, piexif_meta, kind):
        exifread_keys = {
            "width": ["EXIF ExifImageWidth", "Image ImageWidth", "EXIF PixelXDimension"],
            "height": ["EXIF ExifImageLength", "Image ImageLength", "EXIF PixelYDimension"],
        }
        for key in exifread_keys.get(kind, []):
            val = exifread_meta.get(key)
            parsed = self._parse_int(val)
            if parsed:
                return parsed

        piexif_keys = {
            "width": ["0th:ImageWidth", "Exif:PixelXDimension"],
            "height": ["0th:ImageLength", "Exif:PixelYDimension"],
        }
        for key in piexif_keys.get(kind, []):
            val = piexif_meta.get(key)
            parsed = self._parse_int(val)
            if parsed:
                return parsed
        return 0

    def _get_bit_depth_from_exif(self, exifread_meta, piexif_meta):
        bits = None
        samples = None

        bits = self._parse_int(exifread_meta.get("Image BitsPerSample"))
        samples = self._parse_int(exifread_meta.get("Image SamplesPerPixel"))

        if bits is None:
            bits = self._parse_int(piexif_meta.get("0th:BitsPerSample"))
        if samples is None:
            samples = self._parse_int(piexif_meta.get("0th:SamplesPerPixel"))

        if bits and samples:
            return bits * samples
        if bits:
            return bits
        return 0

    def _parse_int(self, value):
        if value is None:
            return None
        if isinstance(value, (int, float)):
            return int(value)
        if isinstance(value, str):
            value = value.strip()
            if value.isdigit():
                return int(value)
            if "/" in value:
                parts = value.split("/")
                if len(parts) == 2 and parts[0].isdigit() and parts[1].isdigit() and int(parts[1]) != 0:
                    return int(int(parts[0]) / int(parts[1]))
        return None

    def _read_format_info(self, input_path):
        try:
            with open(input_path, "rb") as f:
                signature = f.read(16)
        except OSError:
            return {"format": "Unknown", "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}, {}

        if signature.startswith(b"\x89PNG\r\n\x1a\n"):
            return self._read_png_info(input_path)
        if signature[:3] == b"GIF":
            return self._read_gif_info(input_path)
        if signature[:2] == b"BM":
            return self._read_bmp_info(input_path)
        if signature[:2] == b"\xff\xd8":
            return self._read_jpeg_info(input_path)
        if signature[:4] == b"RIFF" and signature[8:12] == b"WEBP":
            return self._read_webp_info(input_path)
        if signature[:4] in (b"II*\x00", b"MM\x00*"):
            return {"format": "TIFF", "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}, {}
        if signature[:4] == b"\x00\x00\x01\x00":
            return self._read_ico_info(input_path)

        ext = os.path.splitext(input_path)[1].lower().lstrip(".")
        fmt = ext.upper() if ext else "Unknown"
        return {"format": fmt, "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}, {}

    def _read_png_info(self, input_path):
        info = {"format": "PNG", "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}
        extra = {}
        try:
            with open(input_path, "rb") as f:
                f.read(8)
                while True:
                    length_bytes = f.read(4)
                    if len(length_bytes) < 4:
                        break
                    length = struct.unpack(">I", length_bytes)[0]
                    chunk_type = f.read(4)
                    data = f.read(length)
                    f.read(4)

                    if chunk_type == b"IHDR" and len(data) >= 13:
                        width, height, bit_depth, color_type = struct.unpack(">IIBB", data[:10])
                        info["width"] = int(width)
                        info["height"] = int(height)
                        channels_map = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}
                        mode_map = {0: "L", 2: "RGB", 3: "P", 4: "LA", 6: "RGBA"}
                        channels = channels_map.get(color_type, 0)
                        info["bit_depth"] = int(bit_depth) * channels if channels else int(bit_depth)
                        info["mode"] = mode_map.get(color_type, "Unknown")
                    elif chunk_type == b"tEXt":
                        if b"\x00" in data:
                            key, val = data.split(b"\x00", 1)
                            extra[f"PNG:Text:{key.decode('latin-1', errors='replace')}"] = self._stringify_value(
                                val.decode("latin-1", errors="replace")
                            )
                    elif chunk_type == b"zTXt":
                        if b"\x00" in data and len(data) > 2:
                            key, rest = data.split(b"\x00", 1)
                            if len(rest) > 1:
                                comp = rest[1:]
                                try:
                                    text = zlib.decompress(comp)
                                    extra[f"PNG:Text:{key.decode('latin-1', errors='replace')}"] = self._stringify_value(
                                        text.decode("utf-8", errors="replace")
                                    )
                                except (zlib.error, UnicodeDecodeError, ValueError):
                                    logger.debug("Failed to decompress PNG zTXt chunk for key %s", key.decode("latin-1", errors="replace"))
                    elif chunk_type == b"iTXt":
                        try:
                            key_raw, rest = data.split(b"\x00", 1)
                            key = key_raw.decode("latin-1", errors="replace")
                            if len(rest) < 2:
                                continue
                            comp_flag = rest[0]
                            rest = rest[2:]
                            lang, rest = rest.split(b"\x00", 1)
                            trans_key, text = rest.split(b"\x00", 1)
                            if comp_flag == 1:
                                try:
                                    text = zlib.decompress(text)
                                except (zlib.error, UnicodeDecodeError, ValueError):
                                    logger.debug("Failed to decompress PNG iTXt chunk for key %s", key)
                            extra[f"PNG:Text:{key}"] = self._stringify_value(text.decode("utf-8", errors="replace"))
                        except (ValueError, UnicodeDecodeError):
                            logger.debug("Failed to parse PNG iTXt chunk")
                    elif chunk_type == b"pHYs" and len(data) >= 9:
                        ppu_x, ppu_y, unit = struct.unpack(">IIB", data[:9])
                        if unit == 1 and ppu_x > 0 and ppu_y > 0:
                            dpi_x = round(ppu_x * 0.0254, 2)
                            dpi_y = round(ppu_y * 0.0254, 2)
                            extra["PNG:DPI"] = f"{dpi_x}x{dpi_y}"
                    elif chunk_type == b"iCCP":
                        if b"\x00" in data:
                            name, rest = data.split(b"\x00", 1)
                            extra["PNG:ICC:Name"] = name.decode("latin-1", errors="replace")
                            if len(rest) > 1:
                                extra["PNG:ICC:Bytes"] = self._stringify_value(rest[1:])

                    if chunk_type == b"IEND":
                        break
        except Exception as e:
            logger.warning(f"PNG parse failed: {e}")
        return info, extra

    def _read_gif_info(self, input_path):
        info = {"format": "GIF", "width": 0, "height": 0, "mode": "P", "bit_depth": 8}
        extra = {}
        try:
            with open(input_path, "rb") as f:
                header = f.read(13)
                if len(header) >= 10:
                    width, height = struct.unpack("<HH", header[6:10])
                    info["width"] = int(width)
                    info["height"] = int(height)
        except Exception as e:
            logger.warning(f"GIF parse failed: {e}")
        return info, extra

    def _read_bmp_info(self, input_path):
        info = {"format": "BMP", "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}
        extra = {}
        try:
            with open(input_path, "rb") as f:
                f.read(14)
                header = f.read(40)
                if len(header) >= 16:
                    width, height, planes, bpp = struct.unpack("<iiHH", header[:12])
                    info["width"] = int(abs(width))
                    info["height"] = int(abs(height))
                    info["bit_depth"] = int(bpp)
                    if bpp == 1:
                        info["mode"] = "1"
                    elif bpp in (4, 8):
                        info["mode"] = "P"
                    elif bpp == 24:
                        info["mode"] = "RGB"
                    elif bpp == 32:
                        info["mode"] = "RGBA"
        except Exception as e:
            logger.warning(f"BMP parse failed: {e}")
        return info, extra

    def _read_jpeg_info(self, input_path):
        info = {"format": "JPEG", "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}
        extra = {}
        sof_markers = {0xC0, 0xC1, 0xC2, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCA, 0xCB, 0xCD, 0xCE, 0xCF}
        try:
            with open(input_path, "rb") as f:
                if f.read(2) != b"\xff\xd8":
                    return info, extra
                while True:
                    byte = f.read(1)
                    if not byte:
                        break
                    if byte != b"\xff":
                        continue
                    marker = f.read(1)
                    if not marker:
                        break
                    while marker == b"\xff":
                        marker = f.read(1)
                    if not marker:
                        break
                    marker_id = marker[0]
                    if marker_id == 0xD9 or marker_id == 0xDA:
                        break
                    length_bytes = f.read(2)
                    if len(length_bytes) < 2:
                        break
                    seg_length = struct.unpack(">H", length_bytes)[0]
                    if seg_length < 2:
                        break
                    data = f.read(seg_length - 2)

                    if marker_id in sof_markers and len(data) >= 6:
                        precision = data[0]
                        height = struct.unpack(">H", data[1:3])[0]
                        width = struct.unpack(">H", data[3:5])[0]
                        components = data[5]
                        info["width"] = int(width)
                        info["height"] = int(height)
                        info["bit_depth"] = int(precision) * int(components)
                        if components == 1:
                            info["mode"] = "L"
                        elif components == 3:
                            info["mode"] = "RGB"
                        elif components == 4:
                            info["mode"] = "CMYK"
                    elif marker_id == 0xFE:
                        extra["JPEG:Comment"] = self._stringify_value(data)
                    elif marker_id == 0xE1 and data.startswith(b"http://ns.adobe.com/xap/1.0/\x00"):
                        xmp = data[len(b"http://ns.adobe.com/xap/1.0/\x00"):]
                        extra["JPEG:XMP"] = self._stringify_value(xmp)
        except Exception as e:
            logger.warning(f"JPEG parse failed: {e}")
        return info, extra

    def _read_webp_info(self, input_path):
        info = {"format": "WEBP", "width": 0, "height": 0, "mode": "RGB", "bit_depth": 24}
        extra = {}
        try:
            with open(input_path, "rb") as f:
                header = f.read(12)
                if len(header) < 12 or header[:4] != b"RIFF" or header[8:12] != b"WEBP":
                    return info, extra
                while True:
                    chunk_header = f.read(8)
                    if len(chunk_header) < 8:
                        break
                    chunk_type = chunk_header[:4]
                    chunk_size = struct.unpack("<I", chunk_header[4:])[0]
                    data = f.read(chunk_size)
                    if chunk_size % 2 == 1:
                        f.read(1)

                    if chunk_type == b"VP8X" and len(data) >= 10:
                        width = 1 + data[4] + (data[5] << 8) + (data[6] << 16)
                        height = 1 + data[7] + (data[8] << 8) + (data[9] << 16)
                        info["width"] = int(width)
                        info["height"] = int(height)
                        if data[0] & 0x10:
                            info["mode"] = "RGBA"
                            info["bit_depth"] = 32
                    elif chunk_type == b"VP8L" and len(data) >= 5:
                        if data[0] == 0x2F:
                            bits = int.from_bytes(data[1:5], "little")
                            width = (bits & 0x3FFF) + 1
                            height = ((bits >> 14) & 0x3FFF) + 1
                            info["width"] = int(width)
                            info["height"] = int(height)
                    elif chunk_type == b"VP8 " and len(data) >= 10:
                        if data[3:6] == b"\x9d\x01\x2a":
                            width = struct.unpack("<H", data[6:8])[0] & 0x3FFF
                            height = struct.unpack("<H", data[8:10])[0] & 0x3FFF
                            info["width"] = int(width)
                            info["height"] = int(height)
                    elif chunk_type == b"XMP ":
                        extra["WEBP:XMP"] = self._stringify_value(data)
        except Exception as e:
            logger.warning(f"WEBP parse failed: {e}")
        return info, extra

    def _read_ico_info(self, input_path):
        info = {"format": "ICO", "width": 0, "height": 0, "mode": "RGBA", "bit_depth": 32}
        extra = {}
        try:
            with open(input_path, "rb") as f:
                header = f.read(6)
                if len(header) < 6:
                    return info, extra
                count = struct.unpack("<H", header[4:6])[0]
                max_w = 0
                max_h = 0
                for _ in range(count):
                    entry = f.read(16)
                    if len(entry) < 16:
                        break
                    w = entry[0] if entry[0] != 0 else 256
                    h = entry[1] if entry[1] != 0 else 256
                    max_w = max(max_w, w)
                    max_h = max(max_h, h)
                info["width"] = int(max_w)
                info["height"] = int(max_h)
        except Exception as e:
            logger.warning(f"ICO parse failed: {e}")
        return info, extra

    def export_info(self, image_info, output_path, format="json"):
        """
        Export image information to a file.

        Args:
            image_info (dict): Image information to export
            output_path (str): Output file path
            format (str): Output format (json or txt)

        Returns:
            dict: Export result
        """
        try:
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)

            if format == "json":
                with open(output_path, "w", encoding="utf-8") as f:
                    json.dump(image_info, f, indent=2, ensure_ascii=False)
            elif format == "txt":
                with open(output_path, "w", encoding="utf-8") as f:
                    f.write("Image Information\n")
                    f.write("=" * 50 + "\n\n")

                    f.write("Basic Information:\n")
                    f.write(f"  File Name: {image_info.get('file_name')}\n")
                    f.write(f"  File Size: {image_info.get('file_size')} bytes\n")
                    f.write(f"  Format: {image_info.get('format')}\n")
                    f.write(f"  Dimensions: {image_info.get('width')}x{image_info.get('height')}\n")
                    f.write(f"  Color Mode: {image_info.get('mode')}\n")
                    f.write(f"  Bit Depth: {image_info.get('bit_depth')}\n\n")

                    metadata = image_info.get("metadata", {})
                    for group in ("exifread", "piexif", "extra"):
                        group_data = metadata.get(group) or {}
                        if not group_data:
                            continue
                        f.write(f"{group.upper()} Metadata:\n")
                        for key, value in group_data.items():
                            f.write(f"  {key}: {value}\n")
                        f.write("\n")

            logger.info(f"Image info exported to {output_path}")
            return {"success": True, "output_path": output_path}

        except Exception as e:
            logger.error(f"Failed to export image info: {e}", exc_info=True)
            return {"success": False, "error": str(e)}

    def edit_exif(self, input_path, output_path, exif_data, overwrite=False):
        """
        Edit EXIF metadata for an image.

        Args:
            input_path (str): Input image path
            output_path (str): Output image path
            exif_data (dict): EXIF data to update (partial updates)

        Returns:
            dict: Edit result
        """
        try:
            exif_dict = piexif.load(input_path)
        except Exception as e:
            return {"success": False, "error": f"Failed to load EXIF: {e}"}

        if not isinstance(exif_data, dict):
            return {"success": False, "error": "Invalid exif_data"}

        for key, value in exif_data.items():
            if not isinstance(key, str) or ":" not in key:
                continue
            ifd_name, tag_name = key.split(":", 1)
            ifd_name = ifd_name.strip()
            tag_name = tag_name.strip()
            if ifd_name.lower() == "thumbnail":
                continue
            tag_map = piexif.TAGS.get(ifd_name)
            if not tag_map:
                continue
            tag_id = None
            tag_info = None
            for tid, info in tag_map.items():
                if info.get("name") == tag_name:
                    tag_id = tid
                    tag_info = info
                    break
            if tag_id is None:
                continue

            if value is None:
                try:
                    exif_dict[ifd_name].pop(tag_id, None)
                except (AttributeError, KeyError, TypeError):
                    logger.debug("Failed to remove EXIF tag %s:%s", ifd_name, tag_name)
                continue

            converted = self._coerce_exif_value(value, tag_info)
            exif_dict[ifd_name][tag_id] = converted

        try:
            exif_bytes = piexif.dump(exif_dict)
            if overwrite or not output_path:
                output_path = input_path
            piexif.insert(exif_bytes, input_path, output_path)
            return {"success": True, "output_path": output_path}
        except Exception as e:
            logger.error(f"Failed to edit EXIF: {e}", exc_info=True)
            return {"success": False, "error": str(e)}

    def _coerce_exif_value(self, value, tag_info):
        if value is None:
            return None

        if isinstance(value, str):
            text = value.strip()
            if text.lower().startswith("hex:"):
                try:
                    return bytes.fromhex(text[4:])
                except ValueError:
                    return value
            if "/" in text:
                parts = text.split("/")
                if len(parts) == 2 and parts[0].strip("-").isdigit() and parts[1].strip("-").isdigit():
                    den = int(parts[1])
                    if den != 0:
                        return (int(parts[0]), den)
            if text.strip("-").isdigit():
                return int(text)
            if tag_info and tag_info.get("type") in (piexif.TYPES.Byte, piexif.TYPES.Undefined):
                return text.encode("utf-8")
            return text

        if isinstance(value, (int, float)):
            return int(value) if float(value).is_integer() else value

        if isinstance(value, list):
            if len(value) == 2 and all(isinstance(v, (int, float)) for v in value):
                return (int(value[0]), int(value[1]))
            return tuple(self._coerce_exif_value(v, tag_info) for v in value)

        if isinstance(value, tuple):
            return tuple(self._coerce_exif_value(v, tag_info) for v in value)

        return value


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
        action = input_data.get("action")
        input_path_fallback = input_data.get("input_path")
        if action is None and input_path_fallback is not None:
            action = "get_info"

        if action == "get_info":
            input_path = input_data.get("input_path")
            if not input_path:
                return {
                    "success": False,
                    "error": "[BAD_INPUT] Missing required parameter: input_path",
                }
            viewer = InfoViewer()
            result = viewer.get_info(input_path)
            if isinstance(result, dict):
                result["input_path"] = input_path

        elif action == "export":
            image_info = input_data.get("image_info")
            output_path = input_data.get("output_path")
            format_type = input_data.get("format", "json")

            if not image_info or not output_path:
                return {
                    "success": False,
                    "error": "[BAD_INPUT] Missing required parameters: image_info or output_path",
                }

            viewer = InfoViewer()
            result = viewer.export_info(image_info, output_path, format_type)

        elif action == "edit_exif":
            input_path = input_data.get("input_path")
            output_path = input_data.get("output_path")
            exif_data = input_data.get("exif_data", {})
            overwrite = input_data.get("overwrite", False)

            if not input_path or not output_path:
                return {
                    "success": False,
                    "error": "[BAD_INPUT] Missing required parameters: input_path or output_path",
                }

            viewer = InfoViewer()
            result = viewer.edit_exif(input_path, output_path, exif_data, overwrite=overwrite)

        else:
            return {"success": False, "error": f"[INVALID_ACTION] Unknown action: {action}"}

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {"success": False, "error": f"[INTERNAL] {str(e)}", "traceback": traceback.format_exc()[-4000:]}


def main():
    """Main entry point for the info viewer script."""
    try:
        input_data = json.load(sys.stdin)

        action = input_data.get("action")
        input_path = input_data.get("input_path")

        if action is None and input_path is not None:
            action = "get_info"

        if action == "get_info":
            input_path = input_data.get("input_path")
            if not input_path:
                result = {
                    "success": False,
                    "error": "Missing required parameter: input_path",
                }
            else:
                viewer = InfoViewer()
                result = viewer.get_info(input_path)
                result["input_path"] = input_path

        elif action == "export":
            image_info = input_data.get("image_info")
            output_path = input_data.get("output_path")
            format_type = input_data.get("format", "json")

            if not image_info or not output_path:
                result = {
                    "success": False,
                    "error": "Missing required parameters: image_info or output_path",
                }
            else:
                viewer = InfoViewer()
                result = viewer.export_info(image_info, output_path, format_type)

        elif action == "edit_exif":
            input_path = input_data.get("input_path")
            output_path = input_data.get("output_path")
            exif_data = input_data.get("exif_data", {})
            overwrite = input_data.get("overwrite", False)

            if not input_path or not output_path:
                result = {
                    "success": False,
                    "error": "Missing required parameters: input_path or output_path",
                }
            else:
                viewer = InfoViewer()
                result = viewer.edit_exif(input_path, output_path, exif_data, overwrite=overwrite)

        else:
            result = {"success": False, "error": f"Unknown action: {action}"}

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

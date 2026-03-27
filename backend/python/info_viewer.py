#!/usr/bin/env python3
"""
Image Info Viewer Script

Extracts real, verifiable image information from file headers, container
structures, standard metadata blocks, and supported EXIF readers.
"""

import json
import logging
import os
import re
import struct
import sys
import traceback
import zlib
from pathlib import Path
from xml.etree import ElementTree as ET

import exifread
import piexif
from PIL import Image, UnidentifiedImageError


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
    SVG_SCAN_BYTES = 2 * 1024 * 1024
    HEIF_SCAN_BYTES = 8 * 1024 * 1024

    BASIC_LABELS = {
        "path": "路径",
        "file_name": "文件名",
        "extension": "扩展名",
        "format": "格式",
        "mime_type": "MIME 类型",
        "mode": "色彩模式",
        "width": "宽度",
        "height": "高度",
        "bit_depth": "位深",
        "file_size": "文件大小",
        "modified": "修改时间",
        "orientation": "方向",
        "has_alpha": "Alpha 通道",
        "is_animated": "动画",
        "frame_count": "帧数",
        "duration_ms": "时长(ms)",
        "loop_count": "循环次数",
        "dpi_x": "水平 DPI",
        "dpi_y": "垂直 DPI",
    }

    MIME_BY_FORMAT = {
        "JPEG": "image/jpeg",
        "PNG": "image/png",
        "GIF": "image/gif",
        "BMP": "image/bmp",
        "TIFF": "image/tiff",
        "WEBP": "image/webp",
        "ICO": "image/x-icon",
        "SVG": "image/svg+xml",
        "HEIC": "image/heic",
        "HEIF": "image/heif",
    }

    MODE_BIT_DEPTHS = {
        "1": 1,
        "L": 8,
        "P": 8,
        "RGB": 24,
        "RGBA": 32,
        "CMYK": 32,
        "LA": 16,
        "PA": 16,
        "I;16": 16,
        "I;16B": 16,
        "I;16L": 16,
        "I;16S": 16,
        "I;16BS": 16,
        "I;16LS": 16,
        "I": 32,
        "F": 32,
    }

    HEIF_BRANDS = {
        "heic",
        "heix",
        "heim",
        "heis",
        "hevc",
        "hevx",
        "hevm",
        "hevs",
        "mif1",
        "msf1",
        "avif",
        "avis",
    }

    def __init__(self):
        logger.info("InfoViewer initialized")

    def get_info(self, input_path):
        try:
            logger.info("Reading image info: %s", input_path)

            file_info = self._get_file_info(input_path)
            image_info, extra_meta, format_details, warnings = self._read_format_info(
                input_path
            )
            exifread_meta = self._get_exifread_data(
                input_path, image_info.get("format")
            )
            piexif_meta = self._get_piexif_data(input_path, image_info.get("format"))
            image_info = self._fill_image_info_from_exif(
                image_info, exifread_meta, piexif_meta
            )

            metadata_groups = {
                "exifread": exifread_meta,
                "piexif": piexif_meta,
                "extra": extra_meta,
            }
            flat_meta = self._flatten_metadata(metadata_groups)
            basic = self._build_basic_info(input_path, file_info, image_info)
            fields = self._build_fields(
                basic, format_details, metadata_groups, piexif_meta
            )

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
                "basic": basic,
                "format_details": format_details,
                "fields": fields,
                "warnings": warnings,
                "success": True,
            }

            logger.info(
                "Successfully read image info: %sx%s %s",
                result["width"],
                result["height"],
                result["format"],
            )
            return result
        except FileNotFoundError:
            logger.error("File not found: %s", input_path)
            return {"success": False, "error": f"[NOT_FOUND] File not found: {input_path}"}
        except Exception as exc:
            logger.error("Failed to get image info: %s", exc, exc_info=True)
            return {"success": False, "error": f"[INTERNAL] {str(exc)}"}

    def _get_file_info(self, file_path):
        stat = os.stat(file_path)
        return {
            "name": Path(file_path).name,
            "size": stat.st_size,
            "modified": int(stat.st_mtime),
        }

    def _build_basic_info(self, input_path, file_info, image_info):
        return {
            "path": input_path,
            "file_name": file_info["name"],
            "extension": Path(input_path).suffix.lower(),
            "format": image_info.get("format") or "Unknown",
            "mime_type": image_info.get("mime_type")
            or self.MIME_BY_FORMAT.get((image_info.get("format") or "").upper(), ""),
            "mode": image_info.get("mode") or "Unknown",
            "width": int(image_info.get("width") or 0),
            "height": int(image_info.get("height") or 0),
            "bit_depth": int(image_info.get("bit_depth") or 0),
            "file_size": int(file_info["size"]),
            "modified": int(file_info["modified"]),
            "orientation": self._stringify_value(image_info.get("orientation")),
            "has_alpha": bool(image_info.get("has_alpha")),
            "is_animated": bool(image_info.get("is_animated")),
            "frame_count": int(image_info.get("frame_count") or 0),
            "duration_ms": int(image_info.get("duration_ms") or 0),
            "loop_count": int(image_info.get("loop_count") or 0),
            "dpi_x": float(image_info.get("dpi_x") or 0.0),
            "dpi_y": float(image_info.get("dpi_y") or 0.0),
        }

    def _build_fields(self, basic, format_details, metadata_groups, piexif_meta):
        fields = []

        for key in (
            "format",
            "mime_type",
            "width",
            "height",
            "bit_depth",
            "mode",
            "file_size",
            "dpi_x",
            "dpi_y",
            "orientation",
            "has_alpha",
            "is_animated",
            "frame_count",
            "duration_ms",
            "loop_count",
            "modified",
            "path",
        ):
            value = basic.get(key)
            if value in ("", None, 0, 0.0, False) and key not in (
                "has_alpha",
                "is_animated",
            ):
                continue
            self._append_field(
                fields,
                key=f"basic.{key}",
                label=self.BASIC_LABELS.get(key, key),
                value=self._stringify_value(value),
                group="basic",
                source="container",
                editable=False,
            )

        for key in sorted(format_details.keys()):
            self._append_field(
                fields,
                key=key,
                label=key,
                value=self._stringify_value(format_details[key]),
                group=key.split(".", 1)[0],
                source="container",
                editable=False,
            )

        editable_keys = set(piexif_meta.keys())
        for source, items in metadata_groups.items():
            for key in sorted(items.keys()):
                group = self._metadata_group_for_field(source, key)
                normalized_key = self._normalize_metadata_field_key(source, key)
                field_source = source
                if source == "extra":
                    field_source = group or "container"
                self._append_field(
                    fields,
                    key=normalized_key,
                    label=key,
                    value=self._stringify_value(items[key]),
                    group=group,
                    source=field_source,
                    editable=source == "piexif"
                    and key in editable_keys
                    and "thumbnail" not in key.lower(),
                )
        return fields

    def _append_field(self, fields, key, label, value, group, source, editable):
        if value in ("", "undefined", "null"):
            return
        fields.append(
            {
                "key": key,
                "label": label,
                "value": value,
                "group": group,
                "source": source,
                "editable": bool(editable),
            }
        )

    def _normalize_metadata_field_key(self, source, key):
        normalized = key.strip().replace(" ", ".").replace(":", ".").lower()
        if source == "extra":
            return normalized
        return f"{source}.{normalized}"

    def _metadata_group_for_field(self, source, key):
        if source in ("exifread", "piexif"):
            prefix = key.split(":", 1)[0].split(" ", 1)[0].lower()
            if prefix == "gps":
                return "gps"
            if prefix == "interop":
                return "interop"
            if prefix in ("0th", "ifd0", "image"):
                return "ifd0"
            if prefix in ("1st", "ifd1", "thumbnail"):
                return "thumbnail"
            if prefix == "exif":
                return "exif"
            return "metadata"
        upper = key.upper()
        if upper.startswith("PNG:TEXT:"):
            return "png_text"
        if upper.startswith("PNG:ICC"):
            return "icc"
        if upper.startswith("JPEG:XMP") or upper.startswith("WEBP:XMP"):
            return "xmp"
        if upper.startswith("GIF:"):
            return "gif"
        if upper.startswith("SVG:"):
            return "svg"
        if upper.startswith("WEBP:"):
            return "webp"
        if upper.startswith("PNG:"):
            return "png"
        if upper.startswith("JPEG:"):
            return "jpeg"
        if upper.startswith("TIFF:"):
            return "tiff"
        if upper.startswith("HEIC:") or upper.startswith("HEIF:"):
            return "heif"
        return "extra"

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
        max_length = self.MAX_TEXT_LENGTH if max_length is None else max_length
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
        tail = (
            value[-self.HEX_TAIL_BYTES :].hex() if self.HEX_TAIL_BYTES > 0 else ""
        )
        folded = len(value) - self.HEX_HEAD_BYTES - (
            self.HEX_TAIL_BYTES if self.HEX_TAIL_BYTES > 0 else 0
        )
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
                    tags_out[tag] = self._stringify_value(
                        printable if printable is not None else raw_values or value
                    )
        except Exception as exc:
            logger.warning("ExifRead failed: %s", exc)
        return tags_out

    def _get_piexif_data(self, input_path, fmt=None):
        tags_out = {}
        supported = {"JPEG", "JPG", "TIFF", "TIF", "WEBP"}
        if fmt and fmt.upper() not in supported:
            return tags_out
        try:
            exif_dict = piexif.load(input_path)
        except Exception as exc:
            logger.warning("piexif load failed: %s", exc)
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
                tags_out[f"{ifd_name}:{tag_name}"] = self._stringify_value(value)
        return tags_out

    def _fill_image_info_from_exif(self, image_info, exifread_meta, piexif_meta):
        info = dict(image_info or {})
        if not info.get("width"):
            info["width"] = self._get_dimension_from_exif(
                exifread_meta, piexif_meta, "width"
            )
        if not info.get("height"):
            info["height"] = self._get_dimension_from_exif(
                exifread_meta, piexif_meta, "height"
            )
        if not info.get("bit_depth"):
            info["bit_depth"] = self._get_bit_depth_from_exif(
                exifread_meta, piexif_meta
            )
        if not info.get("mode") and info.get("bit_depth"):
            info["mode"] = "RGB" if info["bit_depth"] >= 24 else "L"
        if not info.get("orientation"):
            info["orientation"] = (
                piexif_meta.get("0th:Orientation")
                or exifread_meta.get("Image Orientation")
                or ""
            )
        return info

    def _get_dimension_from_exif(self, exifread_meta, piexif_meta, kind):
        exifread_keys = {
            "width": [
                "EXIF ExifImageWidth",
                "Image ImageWidth",
                "EXIF PixelXDimension",
            ],
            "height": [
                "EXIF ExifImageLength",
                "Image ImageLength",
                "EXIF PixelYDimension",
            ],
        }
        for key in exifread_keys.get(kind, []):
            parsed = self._parse_int(exifread_meta.get(key))
            if parsed:
                return parsed
        piexif_keys = {
            "width": ["0th:ImageWidth", "Exif:PixelXDimension"],
            "height": ["0th:ImageLength", "Exif:PixelYDimension"],
        }
        for key in piexif_keys.get(kind, []):
            parsed = self._parse_int(piexif_meta.get(key))
            if parsed:
                return parsed
        return 0

    def _get_bit_depth_from_exif(self, exifread_meta, piexif_meta):
        bits = self._parse_int(exifread_meta.get("Image BitsPerSample"))
        samples = self._parse_int(exifread_meta.get("Image SamplesPerPixel"))
        if bits is None:
            bits = self._parse_int(piexif_meta.get("0th:BitsPerSample"))
        if samples is None:
            samples = self._parse_int(piexif_meta.get("0th:SamplesPerPixel"))
        if bits and samples:
            return bits * samples
        return bits or 0

    def _parse_int(self, value):
        if value is None:
            return None
        if isinstance(value, bool):
            return int(value)
        if isinstance(value, (int, float)):
            return int(value)
        if isinstance(value, str):
            text = value.strip()
            if text.isdigit():
                return int(text)
            if "/" in text:
                parts = text.split("/")
                if (
                    len(parts) == 2
                    and parts[0].strip("-").isdigit()
                    and parts[1].strip("-").isdigit()
                    and int(parts[1]) != 0
                ):
                    return int(int(parts[0]) / int(parts[1]))
        return None

    def _parse_float(self, value):
        if value is None:
            return 0.0
        if isinstance(value, (int, float)):
            return float(value)
        if isinstance(value, str):
            text = value.strip()
            if "/" in text:
                parts = text.split("/")
                if (
                    len(parts) == 2
                    and parts[0].strip("-").isdigit()
                    and parts[1].strip("-").isdigit()
                    and int(parts[1]) != 0
                ):
                    return float(int(parts[0]) / int(parts[1]))
            try:
                return float(text)
            except ValueError:
                return 0.0
        return 0.0

    def _read_format_info(self, input_path):
        warnings = []
        container_info = {
            "format": "Unknown",
            "width": 0,
            "height": 0,
            "mode": "Unknown",
            "bit_depth": 0,
        }
        extra_meta = {}
        format_details = {}

        try:
            with open(input_path, "rb") as f:
                signature = f.read(512)
        except OSError as exc:
            return container_info, extra_meta, format_details, [
                {"code": "READ_ERROR", "message": str(exc)}
            ]

        head = signature.lstrip()
        ext = Path(input_path).suffix.lower()
        parser = None

        if signature.startswith(b"\x89PNG\r\n\x1a\n"):
            parser = self._read_png_info
        elif signature[:3] == b"GIF":
            parser = self._read_gif_info
        elif signature[:2] == b"BM":
            parser = self._read_bmp_info
        elif signature[:2] == b"\xff\xd8":
            parser = self._read_jpeg_info
        elif signature[:4] == b"RIFF" and signature[8:12] == b"WEBP":
            parser = self._read_webp_info
        elif signature[:4] in (b"II*\x00", b"MM\x00*", b"II+\x00", b"MM\x00+"):
            parser = lambda path: self._read_with_pillow(path, "TIFF")
        elif signature[:4] == b"\x00\x00\x01\x00":
            parser = self._read_ico_info
        elif self._looks_like_heif(signature):
            parser = self._read_heif_info
        elif ext == ".svg" or head.startswith(b"<svg") or head.startswith(b"<?xml"):
            parser = self._read_svg_info

        if parser is not None:
            try:
                container_info, extra_meta, format_details, warnings = parser(input_path)
            except Exception as exc:
                logger.warning("Format parser failed for %s: %s", input_path, exc)
                warnings.append(
                    {"code": "FORMAT_PARSE_FAILED", "message": str(exc)}
                )

        pillow_info, pillow_extra, pillow_details, pillow_warnings = self._read_with_pillow(
            input_path, container_info.get("format")
        )

        merged_info = dict(pillow_info)
        for key, value in container_info.items():
            if key in ("has_alpha", "is_animated"):
                merged_info[key] = bool(merged_info.get(key)) or bool(value)
            elif key in (
                "frame_count",
                "duration_ms",
                "loop_count",
                "width",
                "height",
                "bit_depth",
            ):
                if value:
                    merged_info[key] = value
            elif key in ("dpi_x", "dpi_y"):
                if value:
                    merged_info[key] = value
            elif value not in (None, "", "Unknown"):
                merged_info[key] = value

        if "has_alpha" not in merged_info:
            merged_info["has_alpha"] = "A" in str(merged_info.get("mode") or "")
        if not merged_info.get("mime_type"):
            merged_info["mime_type"] = self.MIME_BY_FORMAT.get(
                (merged_info.get("format") or "").upper(), ""
            )
        if not merged_info.get("bit_depth") and merged_info.get("mode"):
            merged_info["bit_depth"] = self.MODE_BIT_DEPTHS.get(
                merged_info["mode"], 0
            )

        combined_extra = dict(extra_meta)
        for key, value in pillow_extra.items():
            combined_extra.setdefault(key, value)

        combined_details = dict(pillow_details)
        for key, value in format_details.items():
            combined_details[key] = value

        warnings.extend(pillow_warnings)
        return (
            merged_info,
            combined_extra,
            combined_details,
            self._dedupe_warnings(warnings),
        )

    def _read_with_pillow(self, input_path, fallback_format=None):
        info = {
            "format": fallback_format or "Unknown",
            "width": 0,
            "height": 0,
            "mode": "Unknown",
            "bit_depth": 0,
            "has_alpha": False,
            "is_animated": False,
            "frame_count": 0,
            "duration_ms": 0,
            "loop_count": 0,
            "dpi_x": 0.0,
            "dpi_y": 0.0,
            "mime_type": self.MIME_BY_FORMAT.get((fallback_format or "").upper(), ""),
        }
        extra = {}
        details = {}
        warnings = []

        try:
            with Image.open(input_path) as img:
                fmt = img.format or fallback_format or "Unknown"
                info["format"] = fmt
                info["mime_type"] = Image.MIME.get(fmt, info["mime_type"])
                info["width"], info["height"] = img.size
                info["mode"] = img.mode or info["mode"]
                info["bit_depth"] = self.MODE_BIT_DEPTHS.get(img.mode, 0)
                info["has_alpha"] = "A" in img.mode or "transparency" in img.info
                frame_count = int(getattr(img, "n_frames", 1) or 1)
                duration_ms = self._parse_int(img.info.get("duration")) or 0
                loop_count = self._parse_int(img.info.get("loop")) or 0
                info["frame_count"] = frame_count
                info["duration_ms"] = duration_ms
                info["loop_count"] = loop_count
                info["is_animated"] = (
                    bool(getattr(img, "is_animated", False))
                    or frame_count > 1
                    or duration_ms > 0
                    or loop_count > 0
                )

                dpi = img.info.get("dpi")
                if isinstance(dpi, tuple) and len(dpi) >= 2:
                    info["dpi_x"] = round(self._safe_float(dpi[0]), 2)
                    info["dpi_y"] = round(self._safe_float(dpi[1]), 2)

                if fmt == "PNG":
                    color_type = self._png_color_type_from_mode(img.mode)
                    if color_type is not None:
                        details["png.color_type"] = self._png_color_type_name(color_type)
                if fmt == "TIFF":
                    details["tiff.frames"] = str(frame_count)
                if fmt == "GIF":
                    details["gif.palette_mode"] = img.mode

                details["pillow.mode"] = img.mode
                details["pillow.format"] = fmt

                for key, value in (img.info or {}).items():
                    if key in ("exif", "icc_profile"):
                        extra_key = f"{fmt}:{key.upper()}"
                    else:
                        extra_key = f"{fmt}:{key}"

                    if key == "dpi" and isinstance(value, tuple) and len(value) >= 2:
                        details[f"{fmt.lower()}.dpi_x"] = str(
                            round(self._safe_float(value[0]), 2)
                        )
                        details[f"{fmt.lower()}.dpi_y"] = str(
                            round(self._safe_float(value[1]), 2)
                        )
                        continue

                    if isinstance(value, dict):
                        for sub_key, sub_value in value.items():
                            extra[f"{extra_key}:{sub_key}"] = self._stringify_value(
                                sub_value
                            )
                        continue
                    extra[extra_key] = self._stringify_value(value)
        except UnidentifiedImageError as exc:
            warnings.append({"code": "PILLOW_UNSUPPORTED", "message": str(exc)})
        except Exception as exc:
            warnings.append({"code": "PILLOW_READ_FAILED", "message": str(exc)})

        return info, extra, details, warnings

    def _read_png_info(self, input_path):
        info = {
            "format": "PNG",
            "width": 0,
            "height": 0,
            "mode": "Unknown",
            "bit_depth": 0,
            "has_alpha": False,
        }
        extra = {}
        details = {}
        warnings = []
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
                        width, height, bit_depth, color_type, compression, filter_method, interlace = struct.unpack(
                            ">IIBBBBB", data[:13]
                        )
                        info["width"] = int(width)
                        info["height"] = int(height)
                        mode_map = {0: "L", 2: "RGB", 3: "P", 4: "LA", 6: "RGBA"}
                        channels_map = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}
                        info["mode"] = mode_map.get(color_type, "Unknown")
                        channels = channels_map.get(color_type, 0)
                        info["bit_depth"] = (
                            int(bit_depth) * channels if channels else int(bit_depth)
                        )
                        info["has_alpha"] = color_type in (4, 6)
                        details["png.color_type"] = self._png_color_type_name(
                            color_type
                        )
                        details["png.bit_depth_per_channel"] = str(bit_depth)
                        details["png.compression_method"] = str(compression)
                        details["png.filter_method"] = str(filter_method)
                        details["png.interlace_method"] = str(interlace)
                    elif chunk_type == b"tEXt":
                        if b"\x00" in data:
                            key, val = data.split(b"\x00", 1)
                            extra[
                                f"PNG:Text:{key.decode('latin-1', errors='replace')}"
                            ] = self._stringify_value(
                                val.decode("latin-1", errors="replace")
                            )
                    elif chunk_type == b"zTXt":
                        if b"\x00" in data and len(data) > 2:
                            key, rest = data.split(b"\x00", 1)
                            if len(rest) > 1:
                                try:
                                    text = zlib.decompress(rest[1:])
                                    extra[
                                        f"PNG:Text:{key.decode('latin-1', errors='replace')}"
                                    ] = self._stringify_value(
                                        text.decode("utf-8", errors="replace")
                                    )
                                except zlib.error as exc:
                                    warnings.append(
                                        {
                                            "code": "PNG_ZTXT_DECOMPRESS_FAILED",
                                            "message": str(exc),
                                        }
                                    )
                    elif chunk_type == b"iTXt":
                        try:
                            key_raw, rest = data.split(b"\x00", 1)
                            key = key_raw.decode("latin-1", errors="replace")
                            if len(rest) < 2:
                                continue
                            comp_flag = rest[0]
                            rest = rest[2:]
                            _, rest = rest.split(b"\x00", 1)
                            _, text = rest.split(b"\x00", 1)
                            if comp_flag == 1:
                                text = zlib.decompress(text)
                            extra[f"PNG:Text:{key}"] = self._stringify_value(
                                text.decode("utf-8", errors="replace")
                            )
                        except (ValueError, zlib.error) as exc:
                            warnings.append(
                                {"code": "PNG_ITXT_PARSE_FAILED", "message": str(exc)}
                            )
                    elif chunk_type == b"pHYs" and len(data) >= 9:
                        ppu_x, ppu_y, unit = struct.unpack(">IIB", data[:9])
                        details["png.pixels_per_unit_x"] = str(ppu_x)
                        details["png.pixels_per_unit_y"] = str(ppu_y)
                        details["png.phys_unit"] = "meter" if unit == 1 else "unknown"
                        if unit == 1 and ppu_x > 0 and ppu_y > 0:
                            dpi_x = round(ppu_x * 0.0254, 2)
                            dpi_y = round(ppu_y * 0.0254, 2)
                            info["dpi_x"] = dpi_x
                            info["dpi_y"] = dpi_y
                            extra["PNG:DPI"] = f"{dpi_x}x{dpi_y}"
                    elif chunk_type == b"gAMA" and len(data) == 4:
                        details["png.gamma"] = str(
                            round(struct.unpack(">I", data)[0] / 100000.0, 5)
                        )
                    elif chunk_type == b"sRGB" and len(data) == 1:
                        details["png.rendering_intent"] = str(data[0])
                    elif chunk_type == b"iCCP":
                        if b"\x00" in data:
                            name, rest = data.split(b"\x00", 1)
                            extra["PNG:ICC:Name"] = name.decode(
                                "latin-1", errors="replace"
                            )
                            if len(rest) > 1:
                                try:
                                    profile = zlib.decompress(rest[1:])
                                    extra["PNG:ICC:Profile"] = self._stringify_value(
                                        profile
                                    )
                                except zlib.error as exc:
                                    warnings.append(
                                        {
                                            "code": "PNG_ICC_DECOMPRESS_FAILED",
                                            "message": str(exc),
                                        }
                                    )

                    if chunk_type == b"IEND":
                        break
        except Exception as exc:
            warnings.append({"code": "PNG_PARSE_FAILED", "message": str(exc)})

        return info, extra, details, warnings

    def _read_gif_info(self, input_path):
        info = {
            "format": "GIF",
            "width": 0,
            "height": 0,
            "mode": "P",
            "bit_depth": 8,
            "is_animated": False,
            "frame_count": 0,
            "duration_ms": 0,
            "loop_count": 0,
        }
        extra = {}
        details = {}
        warnings = []
        frame_count = 0
        total_duration_cs = 0

        try:
            with open(input_path, "rb") as f:
                header = f.read(13)
                if len(header) < 13:
                    return info, extra, details, [
                        {"code": "GIF_PARSE_FAILED", "message": "invalid gif header"}
                    ]

                width, height = struct.unpack("<HH", header[6:10])
                packed = header[10]
                global_color_table = bool(packed & 0x80)
                color_resolution = ((packed >> 4) & 0x07) + 1
                global_table_size = (
                    2 ** ((packed & 0x07) + 1) if global_color_table else 0
                )

                info["width"] = int(width)
                info["height"] = int(height)
                details["gif.color_resolution_bits"] = str(color_resolution)
                details["gif.global_color_table"] = str(global_color_table).lower()
                details["gif.global_palette_size"] = str(global_table_size)

                if global_color_table:
                    f.read(3 * global_table_size)

                while True:
                    introducer = f.read(1)
                    if not introducer:
                        break
                    marker = introducer[0]
                    if marker == 0x3B:
                        break
                    if marker == 0x2C:
                        frame_count += 1
                        descriptor = f.read(9)
                        if len(descriptor) < 9:
                            break
                        packed_field = descriptor[8]
                        if packed_field & 0x80:
                            local_table_size = 2 ** ((packed_field & 0x07) + 1)
                            f.read(3 * local_table_size)
                        f.read(1)
                        while True:
                            sub_size = f.read(1)
                            if not sub_size:
                                break
                            size = sub_size[0]
                            if size == 0:
                                break
                            f.read(size)
                    elif marker == 0x21:
                        label = f.read(1)
                        if not label:
                            break
                        label = label[0]
                        if label == 0xF9:
                            block_size = f.read(1)
                            payload = f.read(4)
                            f.read(1)
                            if block_size and block_size[0] == 4 and len(payload) == 4:
                                delay_cs = struct.unpack("<H", payload[1:3])[0]
                                total_duration_cs += delay_cs
                        elif label == 0xFE:
                            comment_bytes = b""
                            while True:
                                sub_size = f.read(1)
                                if not sub_size:
                                    break
                                size = sub_size[0]
                                if size == 0:
                                    break
                                comment_bytes += f.read(size)
                            if comment_bytes:
                                extra["GIF:Comment"] = self._stringify_value(comment_bytes)
                        elif label == 0xFF:
                            block_size = f.read(1)
                            app_id = f.read(block_size[0]) if block_size else b""
                            app_name = app_id[:8].decode("latin-1", errors="replace")
                            app_auth = app_id[8:].decode("latin-1", errors="replace")
                            details["gif.application"] = app_name.strip()
                            if app_auth.strip():
                                details["gif.application_auth"] = app_auth.strip()
                            data_blocks = []
                            while True:
                                sub_size = f.read(1)
                                if not sub_size:
                                    break
                                size = sub_size[0]
                                if size == 0:
                                    break
                                data_blocks.append(f.read(size))
                            if (
                                app_name.startswith("NETSCAPE")
                                and data_blocks
                                and len(data_blocks[0]) >= 3
                                and data_blocks[0][0] == 1
                            ):
                                info["loop_count"] = struct.unpack(
                                    "<H", data_blocks[0][1:3]
                                )[0]
                                details["gif.loop_count"] = str(info["loop_count"])
                        else:
                            while True:
                                sub_size = f.read(1)
                                if not sub_size:
                                    break
                                size = sub_size[0]
                                if size == 0:
                                    break
                                f.read(size)
                    else:
                        warnings.append(
                            {
                                "code": "GIF_UNKNOWN_BLOCK",
                                "message": f"unknown block marker 0x{marker:02x}",
                            }
                        )
                        break
        except Exception as exc:
            warnings.append({"code": "GIF_PARSE_FAILED", "message": str(exc)})

        info["frame_count"] = frame_count or 1
        info["duration_ms"] = total_duration_cs * 10
        info["is_animated"] = (
            info["frame_count"] > 1
            or info["duration_ms"] > 0
            or info["loop_count"] > 0
        )
        details.setdefault("gif.loop_count", str(info["loop_count"]))
        details["gif.frame_count"] = str(info["frame_count"])
        if info["duration_ms"]:
            details["gif.duration_ms"] = str(info["duration_ms"])

        return info, extra, details, warnings

    def _read_bmp_info(self, input_path):
        info = {"format": "BMP", "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}
        extra = {}
        details = {}
        warnings = []
        try:
            with open(input_path, "rb") as f:
                f.read(14)
                dib_size_bytes = f.read(4)
                if len(dib_size_bytes) < 4:
                    return info, extra, details, [
                        {"code": "BMP_PARSE_FAILED", "message": "invalid bmp header"}
                    ]
                dib_size = struct.unpack("<I", dib_size_bytes)[0]
                header = f.read(max(36, dib_size - 4))
                if dib_size >= 40 and len(header) >= 16:
                    width, height, planes, bpp = struct.unpack("<iiHH", header[:12])
                    compression = struct.unpack("<I", header[12:16])[0]
                    info["width"] = int(abs(width))
                    info["height"] = int(abs(height))
                    info["bit_depth"] = int(bpp)
                    details["bmp.planes"] = str(planes)
                    details["bmp.compression"] = str(compression)
                    if bpp == 1:
                        info["mode"] = "1"
                    elif bpp in (4, 8):
                        info["mode"] = "P"
                    elif bpp == 16:
                        info["mode"] = "RGB"
                    elif bpp == 24:
                        info["mode"] = "RGB"
                    elif bpp == 32:
                        info["mode"] = "RGBA"
                        info["has_alpha"] = True
        except Exception as exc:
            warnings.append({"code": "BMP_PARSE_FAILED", "message": str(exc)})
        return info, extra, details, warnings

    def _read_jpeg_info(self, input_path):
        info = {"format": "JPEG", "width": 0, "height": 0, "mode": "Unknown", "bit_depth": 0}
        extra = {}
        details = {}
        warnings = []
        sof_markers = {
            0xC0: "baseline_dct",
            0xC1: "extended_sequential_dct",
            0xC2: "progressive_dct",
            0xC3: "lossless_sequential",
            0xC5: "differential_sequential_dct",
            0xC6: "differential_progressive_dct",
            0xC7: "differential_lossless",
            0xC9: "extended_sequential_arithmetic",
            0xCA: "progressive_arithmetic",
            0xCB: "lossless_arithmetic",
            0xCD: "differential_sequential_arithmetic",
            0xCE: "differential_progressive_arithmetic",
            0xCF: "differential_lossless_arithmetic",
        }
        density_unit_map = {0: "none", 1: "dpi", 2: "dpcm"}
        icc_parts = {}
        try:
            with open(input_path, "rb") as f:
                if f.read(2) != b"\xff\xd8":
                    return info, extra, details, [
                        {"code": "JPEG_PARSE_FAILED", "message": "missing soi"}
                    ]
                while True:
                    byte = f.read(1)
                    if not byte:
                        break
                    if byte != b"\xff":
                        continue
                    marker = f.read(1)
                    while marker == b"\xff":
                        marker = f.read(1)
                    if not marker:
                        break
                    marker_id = marker[0]
                    if marker_id in (0xD9, 0xDA):
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
                        details["jpeg.precision_bits"] = str(precision)
                        details["jpeg.components"] = str(components)
                        details["jpeg.sof_marker"] = f"0x{marker_id:02x}"
                        details["jpeg.encoding_process"] = sof_markers[marker_id]
                        details["jpeg.progressive"] = str(
                            marker_id in (0xC2, 0xC6, 0xCA, 0xCE)
                        ).lower()
                        if components == 1:
                            info["mode"] = "L"
                        elif components == 3:
                            info["mode"] = "RGB"
                        elif components == 4:
                            info["mode"] = "CMYK"
                    elif marker_id == 0xE0 and data.startswith(b"JFIF\x00") and len(data) >= 14:
                        density_unit = data[7]
                        density_x = struct.unpack(">H", data[8:10])[0]
                        density_y = struct.unpack(">H", data[10:12])[0]
                        details["jpeg.app0"] = "JFIF"
                        details["jpeg.density_unit"] = density_unit_map.get(
                            density_unit, str(density_unit)
                        )
                        details["jpeg.density_x"] = str(density_x)
                        details["jpeg.density_y"] = str(density_y)
                        if density_unit == 1:
                            info["dpi_x"] = float(density_x)
                            info["dpi_y"] = float(density_y)
                        elif density_unit == 2:
                            info["dpi_x"] = round(density_x * 2.54, 2)
                            info["dpi_y"] = round(density_y * 2.54, 2)
                    elif marker_id == 0xFE:
                        extra["JPEG:Comment"] = self._stringify_value(data)
                    elif marker_id == 0xE1 and data.startswith(
                        b"http://ns.adobe.com/xap/1.0/\x00"
                    ):
                        xmp = data[len(b"http://ns.adobe.com/xap/1.0/\x00") :]
                        extra["JPEG:XMP"] = self._stringify_value(xmp)
                    elif marker_id == 0xE2 and data.startswith(
                        b"ICC_PROFILE\x00"
                    ) and len(data) >= 14:
                        seq_no = data[12]
                        icc_parts[seq_no] = data[14:]
                if icc_parts:
                    extra["JPEG:ICC:Profile"] = self._stringify_value(
                        b"".join(icc_parts[idx] for idx in sorted(icc_parts))
                    )
        except Exception as exc:
            warnings.append({"code": "JPEG_PARSE_FAILED", "message": str(exc)})
        return info, extra, details, warnings

    def _read_webp_info(self, input_path):
        info = {
            "format": "WEBP",
            "width": 0,
            "height": 0,
            "mode": "RGB",
            "bit_depth": 24,
            "has_alpha": False,
            "is_animated": False,
        }
        extra = {}
        details = {}
        warnings = []
        try:
            with open(input_path, "rb") as f:
                header = f.read(12)
                if len(header) < 12 or header[:4] != b"RIFF" or header[8:12] != b"WEBP":
                    return info, extra, details, [
                        {
                            "code": "WEBP_PARSE_FAILED",
                            "message": "invalid riff/webp header",
                        }
                    ]
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
                        flags = data[0]
                        width = 1 + data[4] + (data[5] << 8) + (data[6] << 16)
                        height = 1 + data[7] + (data[8] << 8) + (data[9] << 16)
                        info["width"] = int(width)
                        info["height"] = int(height)
                        info["has_alpha"] = bool(flags & 0x10)
                        info["is_animated"] = bool(flags & 0x02)
                        if info["has_alpha"]:
                            info["mode"] = "RGBA"
                            info["bit_depth"] = 32
                        details["webp.has_icc"] = str(bool(flags & 0x20)).lower()
                        details["webp.has_alpha"] = str(info["has_alpha"]).lower()
                        details["webp.has_exif"] = str(bool(flags & 0x08)).lower()
                        details["webp.has_xmp"] = str(bool(flags & 0x04)).lower()
                        details["webp.is_animated"] = str(
                            info["is_animated"]
                        ).lower()
                    elif chunk_type == b"VP8L" and len(data) >= 5 and data[0] == 0x2F:
                        bits = int.from_bytes(data[1:5], "little")
                        width = (bits & 0x3FFF) + 1
                        height = ((bits >> 14) & 0x3FFF) + 1
                        info["width"] = int(width)
                        info["height"] = int(height)
                        info["mode"] = "RGBA"
                        info["bit_depth"] = 32
                        info["has_alpha"] = True
                        details["webp.encoding"] = "lossless"
                    elif (
                        chunk_type == b"VP8 "
                        and len(data) >= 10
                        and data[3:6] == b"\x9d\x01\x2a"
                    ):
                        width = struct.unpack("<H", data[6:8])[0] & 0x3FFF
                        height = struct.unpack("<H", data[8:10])[0] & 0x3FFF
                        info["width"] = int(width)
                        info["height"] = int(height)
                        details["webp.encoding"] = "lossy"
                    elif chunk_type == b"EXIF":
                        extra["WEBP:EXIF"] = self._stringify_value(data)
                    elif chunk_type == b"XMP ":
                        extra["WEBP:XMP"] = self._stringify_value(data)
                    elif chunk_type == b"ANIM" and len(data) >= 6:
                        info["is_animated"] = True
                        details["webp.loop_count"] = str(
                            struct.unpack("<H", data[4:6])[0]
                        )
        except Exception as exc:
            warnings.append({"code": "WEBP_PARSE_FAILED", "message": str(exc)})
        return info, extra, details, warnings

    def _read_ico_info(self, input_path):
        info = {
            "format": "ICO",
            "width": 0,
            "height": 0,
            "mode": "RGBA",
            "bit_depth": 32,
            "has_alpha": True,
        }
        extra = {}
        details = {}
        warnings = []
        sizes = []
        try:
            with open(input_path, "rb") as f:
                header = f.read(6)
                if len(header) < 6:
                    return info, extra, details, [
                        {"code": "ICO_PARSE_FAILED", "message": "invalid ico header"}
                    ]
                count = struct.unpack("<H", header[4:6])[0]
                bit_depths = []
                for _ in range(count):
                    entry = f.read(16)
                    if len(entry) < 16:
                        break
                    w = entry[0] if entry[0] != 0 else 256
                    h = entry[1] if entry[1] != 0 else 256
                    bit_depth = struct.unpack("<H", entry[6:8])[0]
                    sizes.append(f"{w}x{h}")
                    if bit_depth:
                        bit_depths.append(bit_depth)
                    info["width"] = max(info["width"], w)
                    info["height"] = max(info["height"], h)
                if bit_depths:
                    info["bit_depth"] = max(bit_depths)
                if sizes:
                    details["ico.sizes"] = ", ".join(sizes)
                    details["ico.image_count"] = str(len(sizes))
        except Exception as exc:
            warnings.append({"code": "ICO_PARSE_FAILED", "message": str(exc)})
        return info, extra, details, warnings

    def _read_svg_info(self, input_path):
        info = {
            "format": "SVG",
            "width": 0,
            "height": 0,
            "mode": "Vector",
            "bit_depth": 0,
            "mime_type": "image/svg+xml",
        }
        extra = {}
        details = {}
        warnings = []
        try:
            text, truncated = self._read_text_limited(input_path, self.SVG_SCAN_BYTES)
            if truncated:
                warnings.append(
                    {
                        "code": "SVG_SCAN_TRUNCATED",
                        "message": f"SVG larger than {self.SVG_SCAN_BYTES} bytes; metadata scan truncated",
                    }
                )

            if self._contains_unsafe_svg_xml(text):
                warnings.append(
                    {
                        "code": "SVG_UNSAFE_XML",
                        "message": "SVG contains DOCTYPE or ENTITY declarations; skipped XML parser and used limited text scan",
                    }
                )
                self._fill_svg_info_from_text(text, info, extra, details)
                return info, extra, details, warnings

            root = ET.fromstring(text)
            self._fill_svg_info_from_root(root, info, extra, details)
        except ET.ParseError as exc:
            warnings.append({"code": "SVG_PARSE_FAILED", "message": str(exc)})
            try:
                text, _ = self._read_text_limited(input_path, self.SVG_SCAN_BYTES)
                self._fill_svg_info_from_text(text, info, extra, details)
            except Exception:
                pass
        except Exception as exc:
            warnings.append({"code": "SVG_READ_FAILED", "message": str(exc)})
        return info, extra, details, warnings

    def _read_heif_info(self, input_path):
        info = {
            "format": "HEIF",
            "width": 0,
            "height": 0,
            "mode": "Unknown",
            "bit_depth": 0,
        }
        extra = {}
        details = {}
        warnings = []
        try:
            with open(input_path, "rb") as f:
                data = f.read(self.HEIF_SCAN_BYTES + 1)
            truncated = len(data) > self.HEIF_SCAN_BYTES
            if truncated:
                data = data[: self.HEIF_SCAN_BYTES]
                warnings.append(
                    {
                        "code": "HEIF_SCAN_TRUNCATED",
                        "message": f"HEIF metadata scan truncated to first {self.HEIF_SCAN_BYTES} bytes",
                    }
                )

            brand = self._extract_ftyp_brand(data)
            if brand:
                info["format"] = self._normalize_heif_display_format(brand)
                details[f"{info['format'].lower()}.major_brand"] = brand
            width, height = self._extract_ispe_dimensions(data)
            if width:
                info["width"] = int(width)
            if height:
                info["height"] = int(height)
            if width and height:
                details[f"{info['format'].lower()}.primary_size"] = f"{width}x{height}"
        except Exception as exc:
            warnings.append({"code": "HEIF_PARSE_FAILED", "message": str(exc)})
        return info, extra, details, warnings

    def _looks_like_heif(self, signature):
        if len(signature) < 12 or signature[4:8] != b"ftyp":
            return False
        brand = signature[8:12].decode("latin-1", errors="ignore").lower()
        return brand in self.HEIF_BRANDS

    def _extract_ftyp_brand(self, data):
        if len(data) < 16 or data[4:8] != b"ftyp":
            return ""
        return data[8:12].decode("latin-1", errors="ignore").lower()

    def _normalize_heif_display_format(self, brand):
        normalized = (brand or "").strip().lower()
        if normalized in {"avif", "avis"}:
            return "AVIF"
        if normalized.startswith("hei") or normalized.startswith("hev"):
            return "HEIC"
        if normalized in {"mif1", "msf1"}:
            return "HEIF"
        return normalized.upper() if normalized else "HEIF"

    def _read_text_limited(self, input_path, max_bytes):
        with open(input_path, "rb") as f:
            data = f.read(max_bytes + 1)
        truncated = len(data) > max_bytes
        if truncated:
            data = data[:max_bytes]
        return self._decode_text_bytes(data), truncated

    def _decode_text_bytes(self, data):
        if data.startswith(b"\xff\xfe") or data.startswith(b"\xfe\xff"):
            return data.decode("utf-16", errors="replace")
        if data.startswith(b"\xef\xbb\xbf"):
            return data.decode("utf-8-sig", errors="replace")
        try:
            return data.decode("utf-8")
        except UnicodeDecodeError:
            try:
                return data.decode("utf-16")
            except UnicodeDecodeError:
                return data.decode("latin-1", errors="replace")

    def _contains_unsafe_svg_xml(self, text):
        return bool(re.search(r"<!\s*(DOCTYPE|ENTITY)\b", text, re.IGNORECASE))

    def _extract_svg_root_fragment(self, text):
        match = re.search(r"<svg\b[^>]*>", text, re.IGNORECASE | re.DOTALL)
        return match.group(0) if match else ""

    def _extract_svg_attribute(self, svg_tag, name):
        if not svg_tag:
            return ""
        pattern = rf"""\b{name}\s*=\s*(['"])(.*?)\1"""
        match = re.search(pattern, svg_tag, re.IGNORECASE | re.DOTALL)
        return match.group(2).strip() if match else ""

    def _extract_svg_text_tag(self, text, tag_name):
        pattern = rf"<{tag_name}\b[^>]*>(.*?)</{tag_name}>"
        match = re.search(pattern, text, re.IGNORECASE | re.DOTALL)
        if not match:
            return ""
        value = re.sub(r"<[^>]+>", "", match.group(1))
        return value.strip()

    def _apply_svg_dimensions(self, width_raw, height_raw, view_box, info, details):
        if view_box:
            details["svg.view_box"] = view_box

        width = self._parse_svg_length(width_raw)
        height = self._parse_svg_length(height_raw)

        if (not width or not height) and view_box:
            parts = view_box.replace(",", " ").split()
            if len(parts) == 4:
                width = width or self._safe_float(parts[2])
                height = height or self._safe_float(parts[3])

        if width:
            info["width"] = int(round(width))
        if height:
            info["height"] = int(round(height))
        if width_raw:
            details["svg.width_attr"] = width_raw
        if height_raw:
            details["svg.height_attr"] = height_raw

    def _fill_svg_info_from_root(self, root, info, extra, details):
        width_raw = root.attrib.get("width")
        height_raw = root.attrib.get("height")
        view_box = root.attrib.get("viewBox") or root.attrib.get("viewbox")
        self._apply_svg_dimensions(width_raw, height_raw, view_box, info, details)

        title = root.find(".//{http://www.w3.org/2000/svg}title")
        desc = root.find(".//{http://www.w3.org/2000/svg}desc")
        if title is not None and title.text:
            extra["SVG:Title"] = self._stringify_value(title.text.strip())
        if desc is not None and desc.text:
            extra["SVG:Description"] = self._stringify_value(desc.text.strip())

        details["svg.root_tag"] = root.tag.rsplit("}", 1)[-1]

    def _fill_svg_info_from_text(self, text, info, extra, details):
        svg_tag = self._extract_svg_root_fragment(text)
        width_raw = self._extract_svg_attribute(svg_tag, "width")
        height_raw = self._extract_svg_attribute(svg_tag, "height")
        view_box = self._extract_svg_attribute(svg_tag, "viewBox")
        self._apply_svg_dimensions(width_raw, height_raw, view_box, info, details)

        title = self._extract_svg_text_tag(text, "title")
        desc = self._extract_svg_text_tag(text, "desc")
        if title:
            extra["SVG:Title"] = self._stringify_value(title)
        if desc:
            extra["SVG:Description"] = self._stringify_value(desc)
        if svg_tag:
            details["svg.root_tag"] = "svg"

    def _extract_ispe_dimensions(self, data):
        def walk(offset, end):
            while offset + 8 <= end:
                size = struct.unpack(">I", data[offset : offset + 4])[0]
                box_type = data[offset + 4 : offset + 8]
                header = 8
                if size == 1:
                    if offset + 16 > end:
                        return 0, 0
                    size = struct.unpack(">Q", data[offset + 8 : offset + 16])[0]
                    header = 16
                elif size == 0:
                    size = end - offset
                if size < header or offset + size > end:
                    return 0, 0

                body_start = offset + header
                body_end = offset + size

                if box_type == b"ispe" and body_end - body_start >= 12:
                    width = struct.unpack(">I", data[body_start + 4 : body_start + 8])[0]
                    height = struct.unpack(">I", data[body_start + 8 : body_start + 12])[0]
                    return width, height

                if box_type in {
                    b"meta",
                    b"moov",
                    b"trak",
                    b"mdia",
                    b"minf",
                    b"stbl",
                    b"iprp",
                    b"ipco",
                }:
                    nested_start = (
                        body_start + 4
                        if box_type == b"meta" and body_end - body_start >= 4
                        else body_start
                    )
                    width, height = walk(nested_start, body_end)
                    if width and height:
                        return width, height

                offset += size
            return 0, 0

        return walk(0, len(data))

    def _parse_svg_length(self, value):
        if not value:
            return 0.0
        text = str(value).strip().lower()
        if text.endswith("%"):
            return 0.0
        unit_scales = {
            "px": 1.0,
            "pt": 96.0 / 72.0,
            "pc": 16.0,
            "cm": 96.0 / 2.54,
            "mm": 96.0 / 25.4,
            "in": 96.0,
        }
        for unit, scale in unit_scales.items():
            if text.endswith(unit):
                return self._safe_float(text[: -len(unit)]) * scale
        return self._safe_float(text)

    def _safe_float(self, value):
        try:
            return float(str(value).strip())
        except (TypeError, ValueError):
            return 0.0

    def _png_color_type_name(self, color_type):
        names = {
            0: "grayscale",
            2: "truecolor",
            3: "indexed",
            4: "grayscale_alpha",
            6: "rgba",
        }
        return names.get(color_type, str(color_type))

    def _png_color_type_from_mode(self, mode):
        mode_map = {
            "1": 0,
            "L": 0,
            "LA": 4,
            "P": 3,
            "RGB": 2,
            "RGBA": 6,
        }
        return mode_map.get(mode)

    def _dedupe_warnings(self, warnings):
        deduped = []
        seen = set()
        for warning in warnings or []:
            if isinstance(warning, str):
                warning = {"code": "WARNING", "message": warning}
            code = self._stringify_value(warning.get("code"))
            message = self._stringify_value(warning.get("message"))
            if not code and not message:
                continue
            key = (code, message)
            if key in seen:
                continue
            seen.add(key)
            deduped.append({"code": code, "message": message})
        return deduped

    def export_info(self, image_info, output_path, format="json"):
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
                    f.write(
                        f"  Dimensions: {image_info.get('width')}x{image_info.get('height')}\n"
                    )
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
            else:
                return {"success": False, "error": f"Unsupported export format: {format}"}

            logger.info("Image info exported to %s", output_path)
            return {"success": True, "output_path": output_path}
        except Exception as exc:
            logger.error("Failed to export image info: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}

    def edit_exif(self, input_path, output_path, exif_data, overwrite=False):
        try:
            exif_dict = piexif.load(input_path)
        except Exception as exc:
            return {"success": False, "error": f"Failed to load EXIF: {exc}"}

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
            return {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
            }
        except Exception as exc:
            logger.error("Failed to edit EXIF: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}

    def _coerce_exif_value(self, value, tag_info):
        if value is None:
            return None

        if isinstance(value, str):
            text = value.strip()
            if text.lower().startswith("hex:"):
                payload = text[4:]
                if ":" in payload:
                    _, _, maybe_hex = payload.partition(":")
                    payload = maybe_hex
                try:
                    return bytes.fromhex(payload)
                except ValueError:
                    return value
            if "/" in text:
                parts = text.split("/")
                if (
                    len(parts) == 2
                    and parts[0].strip("-").isdigit()
                    and parts[1].strip("-").isdigit()
                ):
                    den = int(parts[1])
                    if den != 0:
                        return (int(parts[0]), den)
            if text.strip("-").isdigit():
                return int(text)
            if tag_info and tag_info.get("type") in (
                piexif.TYPES.Byte,
                piexif.TYPES.Undefined,
            ):
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
            return result

        if action == "export":
            image_info = input_data.get("image_info")
            output_path = input_data.get("output_path")
            format_type = input_data.get("format", "json")
            if not image_info or not output_path:
                return {
                    "success": False,
                    "error": "[BAD_INPUT] Missing required parameters: image_info or output_path",
                }
            viewer = InfoViewer()
            return viewer.export_info(image_info, output_path, format_type)

        if action == "edit_exif":
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
            return viewer.edit_exif(
                input_path, output_path, exif_data, overwrite=overwrite
            )

        return {
            "success": False,
            "error": f"[INVALID_ACTION] Unknown action: {action}",
        }
    except Exception as exc:
        logger.error("Process function error: %s", exc, exc_info=True)
        return {
            "success": False,
            "error": f"[INTERNAL] {str(exc)}",
            "traceback": traceback.format_exc()[-4000:],
        }


def main():
    try:
        input_data = json.load(sys.stdin)
        result = process(input_data)
        json.dump(result, sys.stdout)
    except json.JSONDecodeError as exc:
        logger.error("Invalid JSON input: %s", exc)
        json.dump(
            {"success": False, "error": f"Invalid JSON input: {str(exc)}"},
            sys.stdout,
        )
    except Exception as exc:
        logger.error("Unexpected error: %s", exc, exc_info=True)
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()

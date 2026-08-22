"""
Shared SVG operation utilities extracted from converter.py and info_viewer.py.

Provides safe SVG parsing, dimension extraction, and render-size clamping
used across multiple engines.
"""

import logging
import os
import re
from pathlib import Path

logger = logging.getLogger(__name__)

# ── Constants ──────────────────────────────────────────────────────────────

SVG_DIMENSION_SCAN_BYTES = 256 * 1024
MAX_SVG_SOURCE_BYTES = 8 * 1024 * 1024
MAX_SVG_EDGE = 8192
MAX_SVG_PIXELS = 16_000_000

_SVG_UNSAFE_PATTERN = re.compile(
    r"(?is)"
    r"(<!DOCTYPE\b|<!ENTITY\b|"
    r"\bxlink:href\s*=\s*['\"]\s*(?:file:|https?:|//)|"
    r"\bhref\s*=\s*['\"]\s*(?:file:|https?:|//)|"
    r"\bsrc\s*=\s*['\"]\s*(?:file:|https?:|//)|"
    r"url\s*\(\s*['\"]?\s*(?:file:|https?:|//)|"
    r"<foreignObject\b|"
    r"<script\b|"
    r"@import\b)"
)

# ── Public helpers ─────────────────────────────────────────────────────────


def decode_svg_text(data: bytes) -> str:
    """Decode raw SVG bytes to text, trying various encodings."""
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


def extract_svg_root_fragment(text: str) -> str:
    """Return the opening <svg …> tag as a string, or ''."""
    match = re.search(r"<svg\b[^>]*>", text or "", re.IGNORECASE | re.DOTALL)
    return match.group(0) if match else ""


def extract_svg_attribute(svg_tag: str, name: str) -> str:
    """Extract a single SVG attribute value by name from a <svg …> fragment."""
    if not svg_tag:
        return ""
    pattern = rf"""\b{re.escape(name)}\s*=\s*(['"])(.*?)\1"""
    match = re.search(pattern, svg_tag, re.IGNORECASE | re.DOTALL)
    return match.group(2).strip() if match else ""


def parse_svg_number(v: str) -> float | None:
    """Extract the first numeric value from a string. Returns None on failure."""
    v = (v or "").strip()
    if not v:
        return None
    m = re.match(r"^\s*([0-9]*\.?[0-9]+)", v)
    if not m:
        return None
    return float(m.group(1))


def parse_svg_intrinsic_size_from_attrs(
    width_raw: str, height_raw: str, view_box: str
) -> tuple[int, int] | None:
    """Derive (width, height) from SVG width/height/viewBox attributes."""
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


def parse_svg_intrinsic_size_from_text(text: str) -> tuple[int, int] | None:
    svg_tag = extract_svg_root_fragment(text)
    if not svg_tag:
        return None
    return parse_svg_intrinsic_size_from_attrs(
        extract_svg_attribute(svg_tag, "width"),
        extract_svg_attribute(svg_tag, "height"),
        extract_svg_attribute(svg_tag, "viewBox")
        or extract_svg_attribute(svg_tag, "viewbox"),
    )


def parse_svg_intrinsic_size_from_bytes(data: bytes) -> tuple[int, int] | None:
    text = decode_svg_text(data)
    return parse_svg_intrinsic_size_from_text(text)


def clamp_svg_render_size(width: int, height: int) -> tuple[int, int]:
    """Clamp dimensions to MAX_SVG_EDGE / MAX_SVG_PIXELS while preserving ratio."""
    w = max(1, int(width or 1))
    h = max(1, int(height or 1))
    edge_scale = min(1.0, MAX_SVG_EDGE / float(max(w, h)))
    pixel_scale = min(1.0, (MAX_SVG_PIXELS / float(w * h)) ** 0.5)
    scale = min(edge_scale, pixel_scale)
    if scale < 1.0:
        w = max(1, int(w * scale))
        h = max(1, int(h * scale))
    if w * h > MAX_SVG_PIXELS:
        scale = (MAX_SVG_PIXELS / float(w * h)) ** 0.5
        w = max(1, int(w * scale))
        h = max(1, int(h * scale))
    return w, h


def assert_svg_render_safe(svg_path: str) -> None:
    """Raise RuntimeError if SVG contains disallowed constructs."""
    try:
        with open(svg_path, "rb") as handle:
            data = handle.read(MAX_SVG_SOURCE_BYTES + 1)
    except OSError as exc:
        raise RuntimeError(f"Failed to read SVG: {exc}") from exc
    if len(data) > MAX_SVG_SOURCE_BYTES:
        raise RuntimeError(
            f"SVG file size exceeds maximum safe limit ({MAX_SVG_SOURCE_BYTES // (1024 * 1024)}MB)"
        )
    text = decode_svg_text(data)
    if _SVG_UNSAFE_PATTERN.search(text):
        raise RuntimeError(
            "SVG contains disallowed constructs (DOCTYPE/ENTITY/external "
            "references). Remove external links and entity declarations "
            "before converting."
        )


def is_svg_path(input_path: str | os.PathLike) -> bool:
    """Check whether *input_path* looks like an SVG file by extension."""
    return Path(str(input_path)).suffix.lower() == ".svg"

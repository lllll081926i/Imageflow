#!/usr/bin/env python3
"""
Image Processor Script — merged from adjuster + filter.

Applies colour/transform adjustments, crop, and preset/advanced filters
to images using the Pillow library.
"""

import json
import logging
import os
import sys
from pathlib import Path

from PIL import Image, ImageChops, ImageDraw, ImageEnhance, ImageFilter, ImageOps

from backend.engines.converter import open_image_with_svg_support

logger = logging.getLogger(__name__)


# ── Preset / filter constants ───────────────────────────────────────────────

BASIC_FILTERS = frozenset({
    "grayscale", "sepia", "cool", "warm", "high_contrast", "soft",
})

ADVANCED_FILTERS = frozenset({
    "blur_gaussian", "blur_motion", "sharpen", "noise", "vignette",
    "color_offset",
})

PRESET_FILTERS = [
    "vivid", "bw", "retro", "cool", "warm", "film", "cyber",
    "fresh", "japan", "lomo", "hdr", "fade", "frosted", "cinema", "polaroid",
    "sunset", "ocean", "forest", "purple", "amber", "nordic", "oldphoto", "noir",
    "highkey", "lowkey", "haze", "neon", "matte", "ice", "coffee", "caramel",
    "teal_orange", "silver", "crisp", "low_contrast",
]

# ── ImageProcessor class ────────────────────────────────────────────────────


class ImageProcessor:
    """Applies colour adjustments, transforms, crops, and filters."""

    def __init__(self):
        logger.info("ImageProcessor initialized")

    # ── Public entry point ──────────────────────────────────────────────

    def process(
        self,
        input_path,
        output_path,
        # adjust parameters
        rotate=0,
        flip_h=False,
        flip_v=False,
        brightness=0,
        contrast=0,
        saturation=0,
        hue=0,
        exposure=0,
        vibrance=0,
        sharpness=0,
        crop_ratio="",
        crop_mode="",
        # filter parameters
        filter_name="",
        intensity=1.0,
        blur_radius=2.0,
        sharpen_factor=2.0,
        noise_level=0.1,
        vignette_strength=0.5,
        color_offset_x=5,
        color_offset_y=5,
        grain=0.0,
        vignette=0.0,
    ):
        """Apply adjustments and/or a filter to an image.

        Returns dict with success/error contract.
        """
        img = None
        alpha_channel = None
        try:
            logger.info("Opening image: %s", input_path)
            img = open_image_with_svg_support(
                input_path, format_type=Path(output_path).suffix.lstrip(".") or "png"
            )
            img_format = self._resolve_output_format(output_path, img.format or "PNG")

            # Preserve alpha for filter processing
            needs_alpha = filter_name and filter_name not in ("", "none", "original", "raw")
            if needs_alpha and img.mode == "RGBA":
                alpha_channel = img.split()[3]
                prev = img
                img = img.convert("RGB")
                prev.close()
            elif img.mode != "RGB":
                prev = img
                img = img.convert("RGB")
                prev.close()

            # ── Apply adjustments ──────────────────────────────────────
            prev = img

            img = self._apply_rotation(img, rotate)
            if img is not prev:
                prev.close()
            prev = img

            img = self._apply_flip(img, flip_h, flip_v)
            if img is not prev:
                prev.close()
            prev = img

            img = self._apply_crop_ratio(img, crop_ratio, crop_mode)
            if img is not prev:
                prev.close()

            brightness = self._merge_exposure(brightness, exposure)
            prev = img
            img = self._apply_brightness(img, brightness)
            if img is not prev:
                prev.close()
            prev = img
            img = self._apply_contrast(img, contrast)
            if img is not prev:
                prev.close()
            prev = img
            img = self._apply_saturation(img, saturation)
            if img is not prev:
                prev.close()
            prev = img
            img = self._apply_vibrance(img, vibrance)
            if img is not prev:
                prev.close()
            prev = img
            img = self._apply_hue(img, hue)
            if img is not prev:
                prev.close()
            prev = img
            img = self._apply_sharpness(img, sharpness)
            if img is not prev:
                prev.close()

            # ── Apply filter ───────────────────────────────────────────
            filter_name = str(filter_name or "").strip().lower()
            has_filter = filter_name not in ("", "none", "original", "raw")

            if has_filter:
                if filter_name in PRESET_FILTERS:
                    img = self._replace_image(img, self._apply_preset_filter(img, filter_name, intensity))
                elif filter_name in BASIC_FILTERS:
                    img = self._replace_image(img, self._apply_basic_filter(img, filter_name, intensity))
                elif filter_name in ADVANCED_FILTERS:
                    img = self._replace_image(
                        img,
                        self._apply_advanced_filter(
                            img, filter_name, intensity,
                            blur_radius, sharpen_factor, noise_level,
                            vignette_strength, color_offset_x, color_offset_y,
                        ),
                    )
                else:
                    return {"success": False, "error": f"Unknown filter: {filter_name}"}

            # Grain / vignette apply even without a named filter
            try:
                grain_level = max(0.0, min(1.0, float(grain)))
            except (TypeError, ValueError):
                grain_level = 0.0
            if grain_level > 0:
                img = self._replace_image(img, self._add_noise(img, grain_level))

            try:
                vignette_level = max(0.0, min(1.0, float(vignette)))
            except (TypeError, ValueError):
                vignette_level = 0.0
            if vignette_level > 0:
                img = self._replace_image(img, self._add_vignette(img, vignette_level))

            # Reattach alpha
            if alpha_channel is not None:
                img = self._replace_image(img, img.convert("RGBA"))
                img.putalpha(alpha_channel)
                alpha_channel.close()
                alpha_channel = None

            # ── Save ───────────────────────────────────────────────────
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)

            logger.info("Saving processed image: %s", output_path)
            self._save_image(img, output_path, img_format)
            img.close()
            img = None

            file_size = os.path.getsize(output_path)
            return {
                "success": True,
                "input_path": input_path,
                "output_path": output_path,
                "file_size": file_size,
            }

        except FileNotFoundError:
            logger.error("File not found: %s", input_path)
            return {"success": False, "error": f"File not found: {input_path}"}
        except Exception as exc:
            logger.error("Processing failed: %s", exc, exc_info=True)
            return {"success": False, "error": str(exc)}
        finally:
            if alpha_channel is not None:
                alpha_channel.close()
            if img is not None:
                img.close()

    # ── Format / save helpers ───────────────────────────────────────────

    @staticmethod
    def _resolve_output_format(output_path, fallback_format):
        ext = Path(output_path).suffix.lower()
        fmt_map = {
            ".jpg": "JPEG",
            ".jpeg": "JPEG",
            ".png": "PNG",
            ".webp": "WEBP",
            ".bmp": "BMP",
            ".tif": "TIFF",
            ".tiff": "TIFF",
            ".gif": "GIF",
            ".ico": "ICO",
            ".avif": "AVIF",
        }
        return fmt_map.get(ext, fallback_format or "PNG")

    @staticmethod
    def _save_image(img, output_path, img_format):
        save_img = img
        if img_format == "JPEG" and img.mode not in ("RGB", "L"):
            save_img = img.convert("RGB")
        save_img.save(output_path, format=img_format)
        if save_img is not img:
            save_img.close()

    @staticmethod
    def _replace_image(current, replacement):
        if replacement is not current:
            try:
                current.close()
            except Exception:
                pass
        return replacement

    # ── Adjustment methods ──────────────────────────────────────────────

    @staticmethod
    def _apply_rotation(img, angle):
        if angle == 0:
            return img
        logger.debug("Applying rotation: %s degrees", angle)
        return img.rotate(angle, expand=True, resample=Image.Resampling.BICUBIC)

    @staticmethod
    def _apply_flip(img, flip_h, flip_v):
        if not flip_h and not flip_v:
            return img
        if flip_h:
            logger.debug("Applying horizontal flip")
            img = ImageOps.mirror(img)
        if flip_v:
            logger.debug("Applying vertical flip")
            img = ImageOps.flip(img)
        return img

    @staticmethod
    def _apply_brightness(img, adjustment):
        if adjustment == 0:
            return img
        factor = 1.0 + (adjustment / 100.0)
        factor = max(0.0, min(2.0, factor))
        return ImageEnhance.Brightness(img).enhance(factor)

    @staticmethod
    def _apply_contrast(img, adjustment):
        if adjustment == 0:
            return img
        factor = 1.0 + (adjustment / 100.0)
        factor = max(0.0, min(2.0, factor))
        return ImageEnhance.Contrast(img).enhance(factor)

    @staticmethod
    def _apply_saturation(img, adjustment):
        if adjustment == 0:
            return img
        factor = 1.0 + (adjustment / 100.0)
        factor = max(0.0, min(2.0, factor))
        if img.mode in ("L", "LA", "P"):
            img = img.convert("RGB")
        return ImageEnhance.Color(img).enhance(factor)

    @staticmethod
    def _apply_vibrance(img, adjustment):
        """Boost low-saturation colours more than already-saturated ones."""
        if adjustment == 0:
            return img
        try:
            factor = max(-100.0, min(100.0, float(adjustment))) / 100.0
        except Exception:
            return img
        if img.mode not in ("RGB", "RGBA"):
            img = img.convert("RGB")
        alpha = None
        if img.mode == "RGBA":
            alpha = img.split()[3]
            rgb = img.convert("RGB")
        else:
            rgb = img
        hsv = rgb.convert("HSV")
        h, s, v = hsv.split()
        if factor >= 0:
            lut = [min(255, int(x + (255 - x) * factor)) for x in range(256)]
        else:
            lut = [max(0, int(x * (1 + factor))) for x in range(256)]
        s_mod = s.point(lut)
        out = Image.merge("HSV", (h, s_mod, v)).convert("RGB")
        s_mod.close()
        for ch in (h, s, v):
            ch.close()
        hsv.close()
        if rgb is not img:
            rgb.close()
        if alpha is not None:
            out = out.convert("RGBA")
            out.putalpha(alpha)
        return out

    @staticmethod
    def _apply_sharpness(img, adjustment):
        if adjustment == 0:
            return img
        try:
            factor = 1.0 + (float(adjustment) / 100.0)
        except Exception:
            return img
        factor = max(0.0, min(3.0, factor))
        return ImageEnhance.Sharpness(img).enhance(factor)

    @staticmethod
    def _apply_hue(img, adjustment):
        if adjustment == 0:
            return img
        try:
            shift_degrees = float(adjustment)
        except Exception:
            return img
        if shift_degrees % 360 == 0:
            return img
        if img.mode not in ("RGB", "RGBA"):
            img = img.convert("RGB")
        alpha = None
        if img.mode == "RGBA":
            alpha = img.split()[3]
            rgb = img.convert("RGB")
        else:
            rgb = img
        hsv = rgb.convert("HSV")
        h, s, v = hsv.split()
        shift = int(round((shift_degrees % 360) * 255 / 360))
        lut = [(i + shift) % 256 for i in range(256)]
        h_mod = h.point(lut)
        out = Image.merge("HSV", (h_mod, s, v)).convert("RGB")
        h_mod.close()
        for ch in (h, s, v):
            ch.close()
        hsv.close()
        if rgb is not img:
            rgb.close()
        if alpha is not None:
            out = out.convert("RGBA")
            out.putalpha(alpha)
        return out

    @staticmethod
    def _merge_exposure(brightness, exposure):
        try:
            base = float(brightness)
        except Exception:
            base = 0.0
        try:
            exp = float(exposure)
        except Exception:
            exp = 0.0
        return max(-100.0, min(100.0, base + exp))

    @staticmethod
    def _apply_crop_ratio(img, crop_ratio, crop_mode):
        ratio_text = str(crop_ratio or "").strip().lower()
        if ratio_text in ("", "free", "original", "none", "原图", "自由"):
            return img
        if ":" not in ratio_text:
            return img
        try:
            parts = ratio_text.split(":")
            rw = float(parts[0])
            rh = float(parts[1])
            if rw <= 0 or rh <= 0:
                return img
            target_ratio = rw / rh
        except Exception:
            return img

        width, height = img.size
        if width <= 0 or height <= 0:
            return img
        current_ratio = width / float(height)
        if abs(current_ratio - target_ratio) < 1e-6:
            return img

        mode = str(crop_mode or "").strip().lower()
        focus_x, focus_y = 0.5, 0.5
        if mode.startswith("focus:") or mode.startswith("pos:"):
            try:
                data = mode.split(":", 1)[1]
                parts = [p for p in data.replace(" ", "").split(",") if p]
                if len(parts) == 2:
                    focus_x = max(0.0, min(1.0, float(parts[0])))
                    focus_y = max(0.0, min(1.0, float(parts[1])))
            except Exception:
                pass

        if current_ratio > target_ratio:
            new_width = int(height * target_ratio)
            center_x = focus_x * width
            left = int(max(0, min(width - new_width, center_x - new_width / 2)))
            box = (left, 0, left + new_width, height)
        else:
            new_height = int(width / target_ratio)
            center_y = focus_y * height
            top = int(max(0, min(height - new_height, center_y - new_height / 2)))
            box = (0, top, width, top + new_height)

        return img.crop(box)

    # ── Filter methods ──────────────────────────────────────────────────

    def _apply_basic_filter(self, img, filter_name, intensity):
        logger.debug("Applying basic filter: %s (intensity: %s)", filter_name, intensity)
        if filter_name == "grayscale":
            gray_img = img.convert("L")
            try:
                if intensity < 1.0:
                    img_rgb = img.convert("RGB")
                    gray_rgb = gray_img.convert("RGB")
                    try:
                        img = Image.blend(img_rgb, gray_rgb, intensity)
                    finally:
                        img_rgb.close()
                        gray_rgb.close()
                else:
                    img = gray_img.convert("RGB")
            finally:
                gray_img.close()
        elif filter_name == "sepia":
            gray_img = ImageOps.grayscale(img)
            try:
                sepia_img = ImageOps.colorize(gray_img, "#704214", "#F5E7C8")
            finally:
                gray_img.close()
            if intensity < 1.0:
                try:
                    img = Image.blend(img, sepia_img, intensity)
                finally:
                    sepia_img.close()
            else:
                img = sepia_img
        elif filter_name == "cool":
            gray_img = img.convert("L")
            try:
                cool_img = ImageOps.colorize(gray_img, (0, 0, 50), (0, 100, 255))
            finally:
                gray_img.close()
            if intensity < 1.0:
                try:
                    img = Image.blend(img, cool_img, intensity)
                finally:
                    cool_img.close()
            else:
                img = cool_img
        elif filter_name == "warm":
            gray_img = img.convert("L")
            try:
                warm_img = ImageOps.colorize(gray_img, (50, 0, 0), (255, 150, 50))
            finally:
                gray_img.close()
            if intensity < 1.0:
                try:
                    img = Image.blend(img, warm_img, intensity)
                finally:
                    warm_img.close()
            else:
                img = warm_img
        elif filter_name == "high_contrast":
            factor = 1.0 + intensity
            img = ImageEnhance.Contrast(img).enhance(factor)
        elif filter_name == "soft":
            blurred = img.filter(ImageFilter.GaussianBlur(radius=2))
            try:
                blend = intensity * 0.5 if intensity < 1.0 else 0.5
                img = Image.blend(img, blurred, blend)
            finally:
                blurred.close()
        return img

    def _apply_preset_filter(self, img, preset, intensity):
        """Apply a preset filter by combining simple adjustments."""
        try:
            t = max(0.0, min(1.0, float(intensity)))
        except (TypeError, ValueError):
            t = 1.0

        def ec(image, factor):
            return ImageEnhance.Color(image).enhance(factor)

        def econtrast(image, factor):
            return ImageEnhance.Contrast(image).enhance(factor)

        def ebrightness(image, factor):
            return ImageEnhance.Brightness(image).enhance(factor)

        def esharpness(image, factor):
            return ImageEnhance.Sharpness(image).enhance(factor)

        def colorize_blend(image, shadow, highlight, amount):
            try:
                mix = max(0.0, min(1.0, float(amount)))
            except (TypeError, ValueError):
                mix = 0.0
            if mix <= 0:
                return image
            gray = ImageOps.grayscale(image)
            tone = ImageOps.colorize(gray, shadow, highlight)
            return Image.blend(image, tone, mix)

        base = img
        if preset == "vivid":
            out = ec(base, 1 + 0.8 * t)
            out = econtrast(out, 1 + 0.25 * t)
            return out
        if preset == "bw":
            gray = ImageOps.grayscale(base).convert("RGB")
            return Image.blend(base, gray, t)
        if preset == "retro":
            sepia = self._apply_basic_filter(base, "sepia", 1.0)
            sepia = econtrast(sepia, 1 + 0.15 * t)
            return Image.blend(base, sepia, t)
        if preset == "cool":
            cool = self._apply_basic_filter(base, "cool", 1.0)
            return Image.blend(base, cool, t)
        if preset == "warm":
            warm = self._apply_basic_filter(base, "warm", 1.0)
            return Image.blend(base, warm, t)
        if preset == "film":
            out = self._apply_basic_filter(base, "sepia", 1.0)
            out = econtrast(out, 1 + 0.2 * t)
            out = ebrightness(out, 1 + 0.05 * t)
            return Image.blend(base, out, t)
        if preset == "cyber":
            out = ec(base, 1 + 0.5 * t)
            out = econtrast(out, 1 + 0.2 * t)
            out = self._add_color_offset(out, int(6 * t), int(3 * t))
            return out
        if preset == "fresh":
            out = ebrightness(base, 1 + 0.08 * t)
            out = ec(out, 1 + 0.25 * t)
            return out
        if preset == "japan":
            out = ebrightness(base, 1 + 0.12 * t)
            out = econtrast(out, 1 - 0.12 * t)
            out = ec(out, 1 + 0.1 * t)
            return out
        if preset == "lomo":
            out = econtrast(base, 1 + 0.3 * t)
            out = ec(out, 1 + 0.35 * t)
            return out
        if preset == "hdr":
            out = econtrast(base, 1 + 0.45 * t)
            out = ec(out, 1 + 0.2 * t)
            out = esharpness(out, 1 + 0.3 * t)
            return out
        if preset == "fade":
            out = ec(base, 1 - 0.35 * t)
            out = ebrightness(out, 1 + 0.08 * t)
            out = econtrast(out, 1 - 0.1 * t)
            return out
        if preset == "frosted":
            blur = base.filter(ImageFilter.GaussianBlur(radius=2.0 * t))
            return Image.blend(base, blur, 0.6 * t)
        if preset == "cinema":
            out = econtrast(base, 1 + 0.25 * t)
            out = ec(out, 1 - 0.1 * t)
            return out
        if preset == "polaroid":
            out = self._apply_basic_filter(base, "sepia", 1.0)
            out = ebrightness(out, 1 + 0.08 * t)
            out = econtrast(out, 1 + 0.1 * t)
            return Image.blend(base, out, t)
        if preset == "sunset":
            out = colorize_blend(base, "#3B1D2A", "#F6A35B", 0.6 * t)
            out = ec(out, 1 + 0.25 * t)
            out = econtrast(out, 1 + 0.1 * t)
            out = ebrightness(out, 1 + 0.03 * t)
            return out
        if preset == "ocean":
            out = colorize_blend(base, "#0B1D3A", "#5BC0FF", 0.55 * t)
            out = ec(out, 1 + 0.15 * t)
            out = econtrast(out, 1 + 0.1 * t)
            return out
        if preset == "forest":
            out = colorize_blend(base, "#0C2A1D", "#5FBF7A", 0.5 * t)
            out = ec(out, 1 + 0.1 * t)
            out = econtrast(out, 1 + 0.05 * t)
            return out
        if preset == "purple":
            out = colorize_blend(base, "#1C0F2A", "#C79BFF", 0.55 * t)
            out = ec(out, 1 + 0.15 * t)
            return out
        if preset == "amber":
            out = colorize_blend(base, "#3A240F", "#F4C27A", 0.6 * t)
            out = econtrast(out, 1 + 0.08 * t)
            return out
        if preset == "nordic":
            out = ebrightness(base, 1 + 0.08 * t)
            out = econtrast(out, 1 - 0.08 * t)
            out = ec(out, 1 - 0.08 * t)
            return out
        if preset == "oldphoto":
            out = self._apply_basic_filter(base, "sepia", 1.0)
            out = econtrast(out, 1 - 0.1 * t)
            out = ebrightness(out, 1 + 0.04 * t)
            out = ec(out, 1 - 0.2 * t)
            return Image.blend(base, out, t)
        if preset == "noir":
            gray = ImageOps.grayscale(base).convert("RGB")
            out = econtrast(gray, 1 + 0.35 * t)
            return Image.blend(base, out, t)
        if preset == "highkey":
            out = ebrightness(base, 1 + 0.2 * t)
            out = econtrast(out, 1 - 0.1 * t)
            out = ec(out, 1 - 0.05 * t)
            return out
        if preset == "lowkey":
            out = ebrightness(base, 1 - 0.2 * t)
            out = econtrast(out, 1 + 0.2 * t)
            return out
        if preset == "haze":
            out = econtrast(base, 1 - 0.15 * t)
            out = ebrightness(out, 1 + 0.04 * t)
            blur = base.filter(ImageFilter.GaussianBlur(radius=1.2 * t))
            return Image.blend(out, blur, 0.25 * t)
        if preset == "neon":
            out = ec(base, 1 + 0.6 * t)
            out = econtrast(out, 1 + 0.15 * t)
            out = self._add_color_offset(out, int(4 * t), int(2 * t))
            return out
        if preset == "matte":
            out = econtrast(base, 1 - 0.15 * t)
            out = ec(out, 1 - 0.1 * t)
            out = ebrightness(out, 1 + 0.03 * t)
            return out
        if preset == "ice":
            cool = self._apply_basic_filter(base, "cool", 1.0)
            out = Image.blend(base, cool, 0.6 * t)
            out = ebrightness(out, 1 + 0.1 * t)
            out = ec(out, 1 - 0.15 * t)
            return out
        if preset == "coffee":
            out = colorize_blend(base, "#2B190F", "#B77B44", 0.55 * t)
            out = econtrast(out, 1 + 0.05 * t)
            out = ec(out, 1 - 0.05 * t)
            return out
        if preset == "caramel":
            out = colorize_blend(base, "#3B210F", "#F1B469", 0.55 * t)
            out = ec(out, 1 + 0.2 * t)
            out = econtrast(out, 1 + 0.1 * t)
            return out
        if preset == "teal_orange":
            out = colorize_blend(base, "#0F3B49", "#F4A261", 0.5 * t)
            out = econtrast(out, 1 + 0.1 * t)
            out = ec(out, 1 + 0.15 * t)
            return out
        if preset == "silver":
            gray = ImageOps.grayscale(base).convert("RGB")
            out = Image.blend(base, gray, 0.4 * t)
            out = econtrast(out, 1 + 0.15 * t)
            return out
        if preset == "crisp":
            out = econtrast(base, 1 + 0.25 * t)
            out = esharpness(out, 1 + 0.3 * t)
            return out
        if preset == "low_contrast":
            out = econtrast(base, 1 - 0.2 * t)
            out = ec(out, 1 - 0.05 * t)
            return out
        return base

    def _apply_advanced_filter(
        self, img, filter_name, intensity,
        blur_radius, sharpen_factor, noise_level,
        vignette_strength, color_offset_x, color_offset_y,
    ):
        logger.debug("Applying advanced filter: %s (intensity: %s)", filter_name, intensity)
        if filter_name == "blur_gaussian":
            radius = blur_radius * intensity
            img = img.filter(ImageFilter.GaussianBlur(radius=radius))
        elif filter_name == "blur_motion":
            radius = int(blur_radius * intensity)
            if radius > 0:
                blurred = img.filter(ImageFilter.BoxBlur(radius))
                softer = blurred.filter(ImageFilter.BoxBlur(radius))
                try:
                    img = Image.blend(softer, blurred, 0.5)
                finally:
                    softer.close()
                    blurred.close()
        elif filter_name == "sharpen":
            factor = 1.0 + (sharpen_factor - 1.0) * intensity
            img = ImageEnhance.Sharpness(img).enhance(factor)
        elif filter_name == "noise":
            img = self._add_noise(img, noise_level * intensity)
        elif filter_name == "vignette":
            img = self._add_vignette(img, vignette_strength * intensity)
        elif filter_name == "color_offset":
            offset_x = int(color_offset_x * intensity)
            offset_y = int(color_offset_y * intensity)
            img = self._add_color_offset(img, offset_x, offset_y)
        return img

    # ── Effect helpers ──────────────────────────────────────────────────

    @staticmethod
    def _add_noise(img, noise_level):
        try:
            level = max(0.0, min(1.0, float(noise_level)))
        except (TypeError, ValueError):
            return img
        if level <= 0:
            return img
        try:
            base = img.convert("RGB") if img.mode != "RGB" else img
            sigma = max(1.0, 72.0 * level)
            noise = Image.effect_noise(base.size, sigma)
            noise = ImageOps.autocontrast(noise)
            noise_rgb = Image.merge("RGB", (noise, noise, noise))
            noisy = ImageChops.add(base, noise_rgb, scale=1.0, offset=-128)
            result = Image.blend(base, noisy, level)
            noisy.close()
            noise_rgb.close()
            noise.close()
            if base is not img:
                base.close()
            return result
        except Exception:
            logger.warning("Fast noise path failed; refusing slow per-pixel fallback")
            raise RuntimeError("Noise filter is unavailable in this environment")

    @staticmethod
    def _add_noise_slow(img, noise_level):
        raise RuntimeError("Slow noise fallback is disabled")

    @staticmethod
    def _add_vignette(img, strength):
        try:
            level = max(0.0, min(1.0, float(strength)))
        except (TypeError, ValueError):
            return img
        if level <= 0:
            return img
        try:
            width, height = img.size
            gradient = Image.radial_gradient("L")
            center = gradient.getpixel((gradient.width // 2, gradient.height // 2))
            edge = gradient.getpixel((0, 0))
            if center > edge:
                gradient = ImageOps.invert(gradient)
            mask = gradient.resize((width, height), Image.Resampling.LANCZOS)
            mask = mask.point(lambda x: int(x * level))
            dark = ImageEnhance.Brightness(img).enhance(1 - 0.7 * level)
            result = Image.composite(dark, img, mask)
            dark.close()
            mask.close()
            gradient.close()
            return result
        except Exception:
            logger.warning("Fast vignette path failed; refusing slow ellipse fallback")
            raise RuntimeError("Vignette filter is unavailable in this environment")

    @staticmethod
    def _add_vignette_slow(img, strength):
        raise RuntimeError("Slow vignette fallback is disabled")

    @staticmethod
    def _add_color_offset(img, offset_x, offset_y):
        """RGB colour offset (chromatic aberration effect)."""
        if offset_x == 0 and offset_y == 0:
            return img
        r, g, b = img.split()
        shifted_r = None
        shifted_b = None
        try:
            shifted_r = ImageChops.offset(r, offset_x, offset_y)
            shifted_b = ImageChops.offset(b, -offset_x, -offset_y)
            return Image.merge("RGB", (shifted_r, g, shifted_b))
        finally:
            for ch in (shifted_r, shifted_b, r, g, b):
                if ch is not None:
                    try:
                        ch.close()
                    except Exception:
                        pass


# ── Bridge entry point ──────────────────────────────────────────────────────


def process(input_data):
    """Process function used by the desktop API engine bridge."""
    try:
        # Adjust params
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        rotate = input_data.get("rotate", 0)
        flip_h = input_data.get("flip_h", False)
        flip_v = input_data.get("flip_v", False)
        brightness = input_data.get("brightness", 0)
        contrast = input_data.get("contrast", 0)
        saturation = input_data.get("saturation", 0)
        hue = input_data.get("hue", 0)
        exposure = input_data.get("exposure", 0)
        vibrance = input_data.get("vibrance", 0)
        sharpness = input_data.get("sharpness", 0)
        crop_ratio = input_data.get("crop_ratio", "")
        crop_mode = input_data.get("crop_mode", "")

        # Filter params
        filter_name = (
            input_data.get("filter")
            or input_data.get("filter_type")
            or input_data.get("preset")
            or ""
        )
        intensity = input_data.get("intensity", 1.0)
        blur_radius = input_data.get("blur_radius", 2.0)
        sharpen_factor = input_data.get("sharpen_factor", 2.0)
        noise_level = input_data.get("noise_level", 0.1)
        vignette_strength = input_data.get("vignette_strength", 0.5)
        color_offset_x = input_data.get("color_offset_x", 5)
        color_offset_y = input_data.get("color_offset_y", 5)
        grain = input_data.get("grain", 0.0)
        vignette = input_data.get("vignette", 0.0)

        if not input_path or not output_path:
            return {
                "success": False,
                "error": "Missing required parameters: input_path or output_path",
            }

        proc = ImageProcessor()
        result = proc.process(
            input_path=input_path,
            output_path=output_path,
            rotate=rotate,
            flip_h=flip_h,
            flip_v=flip_v,
            brightness=brightness,
            contrast=contrast,
            saturation=saturation,
            hue=hue,
            exposure=exposure,
            vibrance=vibrance,
            sharpness=sharpness,
            crop_ratio=crop_ratio,
            crop_mode=crop_mode,
            filter_name=filter_name,
            intensity=intensity,
            blur_radius=blur_radius,
            sharpen_factor=sharpen_factor,
            noise_level=noise_level,
            vignette_strength=vignette_strength,
            color_offset_x=color_offset_x,
            color_offset_y=color_offset_y,
            grain=grain,
            vignette=vignette,
        )
        return result
    except Exception as exc:
        logger.error("Process function error: %s", exc, exc_info=True)
        return {"success": False, "error": str(exc)}


def main():
    """Main entry point for standalone use (stdin JSON)."""
    try:
        try:
            sys.stdin.reconfigure(encoding="utf-8", errors="strict")
            sys.stdout.reconfigure(encoding="utf-8", errors="strict")
        except (AttributeError, ValueError, OSError):
            pass
        input_data = json.load(sys.stdin)
        result = process(input_data)
        json.dump(result, sys.stdout)
    except json.JSONDecodeError as exc:
        logger.error("Invalid JSON input: %s", exc)
        json.dump({"success": False, "error": f"Invalid JSON input: {str(exc)}"}, sys.stdout)
    except Exception as exc:
        logger.error("Unexpected error: %s", exc, exc_info=True)
        json.dump({"success": False, "error": str(exc)}, sys.stdout)


if __name__ == "__main__":
    main()
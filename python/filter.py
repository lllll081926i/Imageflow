#!/usr/bin/env python3
"""
Image Filter Script

This script applies various filters to images using the Pillow library.
It includes basic and advanced filters with adjustable intensity.

Usage:
    python filter.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
import gc
from pathlib import Path
from PIL import Image, ImageFilter, ImageEnhance, ImageOps, ImageDraw, ImageChops
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class ImageFilterApplier:
    """Handles filter application to images."""
    
    # Basic filters
    BASIC_FILTERS = [
        'grayscale', 'sepia', 'cool', 'warm', 'high_contrast', 'soft'
    ]
    
    # Advanced filters
    ADVANCED_FILTERS = [
        'blur_gaussian', 'blur_motion', 'sharpen', 'noise', 'vignette', 'color_offset'
    ]

    PRESET_FILTERS = [
        'vivid', 'bw', 'retro', 'cool', 'warm', 'film', 'cyber',
        'fresh', 'japan', 'lomo', 'hdr', 'fade', 'frosted', 'cinema', 'polaroid'
    ]
    
    def __init__(self):
        """Initialize the filter applier."""
        logger.info("ImageFilterApplier initialized")
    
    def apply(self, input_path, output_path, filter_name, intensity=1.0,
              blur_radius=2.0, sharpen_factor=2.0, noise_level=0.1,
              vignette_strength=0.5, color_offset_x=5, color_offset_y=5,
              grain=0.0, vignette=0.0):
        """
        Apply a filter to an image.
        
        Args:
            input_path (str): Path to the input image
            output_path (str): Path to save the filtered image
            filter_name (str): Name of the filter to apply
            intensity (float): Filter intensity (0.0-1.0)
            blur_radius (float): Radius for blur filters
            sharpen_factor (float): Factor for sharpen filter
            noise_level (float): Noise level (0.0-1.0)
            vignette_strength (float): Vignette strength (0.0-1.0)
            color_offset_x (int): Color shift X in pixels
            color_offset_y (int): Color shift Y in pixels
        
        Returns:
            dict: Filter application result
        """
        try:
            # Open input image
            logger.info(f"Opening image: {input_path}")
            img = Image.open(input_path)
            
            # Store original format
            img_format = img.format or 'PNG'
            
            # Convert to RGB for processing if necessary
            if img.mode != 'RGB':
                img = img.convert('RGB')
            
            filter_name = str(filter_name or '').strip().lower()

            # Apply filter based on type
            if filter_name in ("", "none", "original", "raw"):
                pass
            elif filter_name in self.PRESET_FILTERS:
                img = self._apply_preset_filter(img, filter_name, intensity)
            elif filter_name in self.BASIC_FILTERS:
                img = self._apply_basic_filter(img, filter_name, intensity)
            elif filter_name in self.ADVANCED_FILTERS:
                img = self._apply_advanced_filter(
                    img, filter_name, intensity, blur_radius, sharpen_factor,
                    noise_level, vignette_strength, color_offset_x, color_offset_y
                )
            else:
                return {
                    'success': False,
                    'error': f'Unknown filter: {filter_name}'
                }

            # Apply extra grain/vignette after base filter
            try:
                grain_level = max(0.0, min(1.0, float(grain)))
            except (TypeError, ValueError):
                grain_level = 0.0
            if grain_level > 0:
                img = self._add_noise(img, grain_level)

            try:
                vignette_level = max(0.0, min(1.0, float(vignette)))
            except (TypeError, ValueError):
                vignette_level = 0.0
            if vignette_level > 0:
                img = self._add_vignette(img, vignette_level)
            
            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Save the filtered image
            logger.info(f"Saving filtered image: {output_path} (filter: {filter_name})")
            img.save(output_path, format=img_format)

            # Explicitly close image to free memory
            img.close()
            del img

            # Get file size
            file_size = os.path.getsize(output_path)

            # Force garbage collection
            gc.collect()

            return {
                'success': True,
                'input_path': input_path,
                'output_path': output_path,
                'file_size': file_size
            }
            
        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            return {
                'success': False,
                'error': f'File not found: {input_path}'
            }
        except Exception as e:
            logger.error(f"Filter application failed: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _apply_basic_filter(self, img, filter_name, intensity):
        """Apply a basic filter to an image."""
        logger.debug(f"Applying basic filter: {filter_name} (intensity: {intensity})")
        
        if filter_name == 'grayscale':
            # Convert to grayscale
            gray_img = img.convert('L')
            # Blend with original based on intensity
            if intensity < 1.0:
                img_rgb = img.convert('RGB')
                gray_rgb = gray_img.convert('RGB')
                img = Image.blend(img_rgb, gray_rgb, intensity)
            else:
                img = gray_img.convert('RGB')
        
        elif filter_name == 'sepia':
            # Sepia tone effect
            sepia_img = img.copy()
            pixels = sepia_img.load()
            width, height = sepia_img.size
            
            for y in range(height):
                for x in range(width):
                    r, g, b = pixels[x, y]
                    # Sepia transformation
                    tr = int(0.393 * r + 0.769 * g + 0.189 * b)
                    tg = int(0.349 * r + 0.686 * g + 0.168 * b)
                    tb = int(0.272 * r + 0.534 * g + 0.131 * b)
                    pixels[x, y] = (
                        min(255, tr),
                        min(255, tg),
                        min(255, tb)
                    )
            
            # Blend with original based on intensity
            if intensity < 1.0:
                img = Image.blend(img, sepia_img, intensity)
            else:
                img = sepia_img
        
        elif filter_name == 'cool':
            # Cool blue tone
            cool_img = ImageOps.colorize(img.convert('L'), (0, 0, 50), (0, 100, 255))
            if intensity < 1.0:
                img = Image.blend(img, cool_img, intensity)
            else:
                img = cool_img
        
        elif filter_name == 'warm':
            # Warm orange tone
            warm_img = ImageOps.colorize(img.convert('L'), (50, 0, 0), (255, 150, 50))
            if intensity < 1.0:
                img = Image.blend(img, warm_img, intensity)
            else:
                img = warm_img
        
        elif filter_name == 'high_contrast':
            # Increase contrast
            enhancer = ImageEnhance.Contrast(img)
            factor = 1.0 + intensity
            img = enhancer.enhance(factor)
        
        elif filter_name == 'soft':
            # Soft focus effect
            blurred = img.filter(ImageFilter.GaussianBlur(radius=2))
            if intensity < 1.0:
                img = Image.blend(img, blurred, intensity * 0.5)
            else:
                img = Image.blend(img, blurred, 0.5)
        
        return img

    def _apply_preset_filter(self, img, preset, intensity):
        """Apply a preset filter by combining simple adjustments."""
        try:
            t = max(0.0, min(1.0, float(intensity)))
        except (TypeError, ValueError):
            t = 1.0

        def enhance_color(image, factor):
            enhancer = ImageEnhance.Color(image)
            return enhancer.enhance(factor)

        def enhance_contrast(image, factor):
            enhancer = ImageEnhance.Contrast(image)
            return enhancer.enhance(factor)

        def enhance_brightness(image, factor):
            enhancer = ImageEnhance.Brightness(image)
            return enhancer.enhance(factor)

        def enhance_sharpness(image, factor):
            enhancer = ImageEnhance.Sharpness(image)
            return enhancer.enhance(factor)

        base = img
        if preset == 'vivid':
            out = enhance_color(base, 1 + 0.8 * t)
            out = enhance_contrast(out, 1 + 0.25 * t)
            return out
        if preset == 'bw':
            gray = ImageOps.grayscale(base).convert('RGB')
            return Image.blend(base, gray, t)
        if preset == 'retro':
            sepia = self._apply_basic_filter(base, 'sepia', 1.0)
            sepia = enhance_contrast(sepia, 1 + 0.15 * t)
            return Image.blend(base, sepia, t)
        if preset == 'cool':
            cool = self._apply_basic_filter(base, 'cool', 1.0)
            return Image.blend(base, cool, t)
        if preset == 'warm':
            warm = self._apply_basic_filter(base, 'warm', 1.0)
            return Image.blend(base, warm, t)
        if preset == 'film':
            out = self._apply_basic_filter(base, 'sepia', 1.0)
            out = enhance_contrast(out, 1 + 0.2 * t)
            out = enhance_brightness(out, 1 + 0.05 * t)
            return Image.blend(base, out, t)
        if preset == 'cyber':
            out = enhance_color(base, 1 + 0.5 * t)
            out = enhance_contrast(out, 1 + 0.2 * t)
            out = self._add_color_offset(out, int(6 * t), int(3 * t))
            return out
        if preset == 'fresh':
            out = enhance_brightness(base, 1 + 0.08 * t)
            out = enhance_color(out, 1 + 0.25 * t)
            return out
        if preset == 'japan':
            out = enhance_brightness(base, 1 + 0.12 * t)
            out = enhance_contrast(out, 1 - 0.12 * t)
            out = enhance_color(out, 1 + 0.1 * t)
            return out
        if preset == 'lomo':
            out = enhance_contrast(base, 1 + 0.3 * t)
            out = enhance_color(out, 1 + 0.35 * t)
            return out
        if preset == 'hdr':
            out = enhance_contrast(base, 1 + 0.45 * t)
            out = enhance_color(out, 1 + 0.2 * t)
            out = enhance_sharpness(out, 1 + 0.3 * t)
            return out
        if preset == 'fade':
            out = enhance_color(base, 1 - 0.35 * t)
            out = enhance_brightness(out, 1 + 0.08 * t)
            out = enhance_contrast(out, 1 - 0.1 * t)
            return out
        if preset == 'frosted':
            blur = base.filter(ImageFilter.GaussianBlur(radius=2.0 * t))
            return Image.blend(base, blur, 0.6 * t)
        if preset == 'cinema':
            out = enhance_contrast(base, 1 + 0.25 * t)
            out = enhance_color(out, 1 - 0.1 * t)
            return out
        if preset == 'polaroid':
            out = self._apply_basic_filter(base, 'sepia', 1.0)
            out = enhance_brightness(out, 1 + 0.08 * t)
            out = enhance_contrast(out, 1 + 0.1 * t)
            return Image.blend(base, out, t)
        return base
    
    def _apply_advanced_filter(self, img, filter_name, intensity,
                            blur_radius, sharpen_factor, noise_level,
                            vignette_strength, color_offset_x, color_offset_y):
        """Apply an advanced filter to an image."""
        logger.debug(f"Applying advanced filter: {filter_name} (intensity: {intensity})")
        
        if filter_name == 'blur_gaussian':
            # Gaussian blur
            radius = blur_radius * intensity
            img = img.filter(ImageFilter.GaussianBlur(radius=radius))
        
        elif filter_name == 'blur_motion':
            # Motion blur (simulated)
            radius = int(blur_radius * intensity)
            if radius > 0:
                # Apply directional blur
                img = img.filter(ImageFilter.BoxBlur(radius))
                # Blend to soften
                img = Image.blend(img.filter(ImageFilter.BoxBlur(radius)), img, 0.5)
        
        elif filter_name == 'sharpen':
            # Sharpen
            factor = 1.0 + (sharpen_factor - 1.0) * intensity
            enhancer = ImageEnhance.Sharpness(img)
            img = enhancer.enhance(factor)
        
        elif filter_name == 'noise':
            # Add noise/grain
            img = self._add_noise(img, noise_level * intensity)
        
        elif filter_name == 'vignette':
            # Vignette effect
            img = self._add_vignette(img, vignette_strength * intensity)
        
        elif filter_name == 'color_offset':
            # RGB color shift
            offset_x = int(color_offset_x * intensity)
            offset_y = int(color_offset_y * intensity)
            img = self._add_color_offset(img, offset_x, offset_y)
        
        return img
    
    def _add_noise(self, img, noise_level):
        """Add random noise to an image."""
        try:
            level = max(0.0, min(1.0, float(noise_level)))
        except (TypeError, ValueError):
            return img
        if level <= 0:
            return img

        try:
            base = img.convert('RGB') if img.mode != 'RGB' else img
            sigma = max(1.0, 72.0 * level)
            noise = Image.effect_noise(base.size, sigma)
            noise = ImageOps.autocontrast(noise)
            noise_rgb = Image.merge('RGB', (noise, noise, noise))
            noisy = ImageChops.add(base, noise_rgb, scale=1.0, offset=-128)
            return Image.blend(base, noisy, level)
        except Exception:
            return self._add_noise_slow(img, level)

    def _add_noise_slow(self, img, noise_level):
        import random

        pixels = img.load()
        width, height = img.size

        for y in range(height):
            for x in range(width):
                r, g, b = pixels[x, y]
                noise = int((random.random() - 0.5) * 256 * noise_level)
                pixels[x, y] = (
                    max(0, min(255, r + noise)),
                    max(0, min(255, g + noise)),
                    max(0, min(255, b + noise))
                )

        return img
    
    def _add_vignette(self, img, strength):
        """Add a vignette effect to an image."""
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
            return Image.composite(dark, img, mask)
        except Exception:
            return self._add_vignette_slow(img, level)

    def _add_vignette_slow(self, img, strength):
        width, height = img.size

        overlay = Image.new('L', (width, height), 0)
        draw = ImageDraw.Draw(overlay)

        center_x, center_y = width // 2, height // 2
        max_radius = int((width ** 2 + height ** 2) ** 0.5 / 2)

        for radius in range(max_radius, 0, -1):
            alpha = int(255 * (1 - (radius / max_radius) ** 2) * strength)
            draw.ellipse([
                center_x - radius, center_y - radius,
                center_x + radius, center_y + radius
            ], fill=alpha)

        img_with_vignette = img.copy()
        img_with_vignette.putalpha(overlay)

        vignette_rgb = img_with_vignette.convert('RGB')
        return Image.blend(img, vignette_rgb, strength)
    
    def _add_color_offset(self, img, offset_x, offset_y):
        """Add RGB color offset (chromatic aberration effect)."""
        if offset_x == 0 and offset_y == 0:
            return img
        
        r, g, b = img.split()
        
        # Shift channels
        if offset_x > 0 or offset_y > 0:
            r = ImageChops.offset(r, offset_x, offset_y)
            b = ImageChops.offset(b, -offset_x, -offset_y)
        elif offset_x < 0 or offset_y < 0:
            r = ImageChops.offset(r, offset_x, offset_y)
            b = ImageChops.offset(b, -offset_x, -offset_y)
        
        # Merge back
        return Image.merge('RGB', (r, g, b))


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
        filter_name = input_data.get('filter') or input_data.get('filter_type') or input_data.get('preset')
        intensity = input_data.get('intensity', 1.0)
        blur_radius = input_data.get('blur_radius', 2.0)
        sharpen_factor = input_data.get('sharpen_factor', 2.0)
        noise_level = input_data.get('noise_level', 0.1)
        vignette_strength = input_data.get('vignette_strength', 0.5)
        color_offset_x = input_data.get('color_offset_x', 5)
        color_offset_y = input_data.get('color_offset_y', 5)
        grain = input_data.get('grain', 0.0)
        vignette = input_data.get('vignette', 0.0)

        # Validate required parameters
        if not input_path or not output_path:
            return {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }
        elif not filter_name:
            return {
                'success': False,
                'error': 'Missing required parameter: filter'
            }

        # Create filter applier and apply filter
        applier = ImageFilterApplier()
        result = applier.apply(
            input_path=input_path,
            output_path=output_path,
            filter_name=filter_name,
            intensity=intensity,
            blur_radius=blur_radius,
            sharpen_factor=sharpen_factor,
            noise_level=noise_level,
            vignette_strength=vignette_strength,
            color_offset_x=color_offset_x,
            color_offset_y=color_offset_y,
            grain=grain,
            vignette=vignette
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the filter script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received filter request: {input_data.get('filter')}")
        
        # Extract parameters
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')
        filter_name = input_data.get('filter') or input_data.get('filter_type') or input_data.get('preset')
        intensity = input_data.get('intensity', 1.0)
        blur_radius = input_data.get('blur_radius', 2.0)
        sharpen_factor = input_data.get('sharpen_factor', 2.0)
        noise_level = input_data.get('noise_level', 0.1)
        vignette_strength = input_data.get('vignette_strength', 0.5)
        color_offset_x = input_data.get('color_offset_x', 5)
        color_offset_y = input_data.get('color_offset_y', 5)
        grain = input_data.get('grain', 0.0)
        vignette = input_data.get('vignette', 0.0)
        
        # Validate required parameters
        if not input_path or not output_path:
            result = {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }
        elif not filter_name:
            result = {
                'success': False,
                'error': 'Missing required parameter: filter'
            }
        else:
            # Create filter applier and apply filter
            applier = ImageFilterApplier()
            result = applier.apply(
                input_path=input_path,
                output_path=output_path,
                filter_name=filter_name,
                intensity=intensity,
                blur_radius=blur_radius,
                sharpen_factor=sharpen_factor,
                noise_level=noise_level,
                vignette_strength=vignette_strength,
                color_offset_x=color_offset_x,
                color_offset_y=color_offset_y,
                grain=grain,
                vignette=vignette
            )
        
        # Write result to stdout
        logger.info(f"Filter application completed: {result.get('success')}")
        json.dump(result, sys.stdout)
        
    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON input: {e}")
        json.dump({
            'success': False,
            'error': f'Invalid JSON input: {str(e)}'
        }, sys.stdout)
    except Exception as e:
        logger.error(f"Unexpected error: {e}", exc_info=True)
        json.dump({
            'success': False,
            'error': str(e)
        }, sys.stdout)


if __name__ == '__main__':
    main()

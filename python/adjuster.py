#!/usr/bin/env python3
"""
Image Adjuster Script

This script applies various adjustments to images including rotation, flipping,
brightness, contrast, saturation, and hue adjustments.

Usage:
    python adjuster.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import gc
import sys
import json
import os
from pathlib import Path
from PIL import Image, ImageEnhance, ImageOps
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class ImageAdjuster:
    """Handles image adjustment operations."""
    
    def __init__(self):
        """Initialize the image adjuster."""
        logger.info("ImageAdjuster initialized")
    
    def adjust(self, input_path, output_path, rotate=0, flip_h=False, flip_v=False,
               brightness=0, contrast=0, saturation=0, hue=0,
               exposure=0, vibrance=0, sharpness=0, crop_ratio="", crop_mode=""):
        """
        Apply adjustments to an image.
        
        Args:
            input_path (str): Path to the input image
            output_path (str): Path to save the adjusted image
            rotate (float): Rotation angle in degrees
            flip_h (bool): Flip horizontally
            flip_v (bool): Flip vertically
            brightness (int): Brightness adjustment (-100 to +100)
            contrast (int): Contrast adjustment (-100 to +100)
            saturation (int): Saturation adjustment (-100 to +100)
            hue (int): Hue adjustment (-180 to +180)
        
        Returns:
            dict: Adjustment result
        """
        try:
            # Open input image
            logger.info(f"Opening image: {input_path}")
            img = Image.open(input_path)
            
            # Store original format
            img_format = img.format or 'PNG'
            
            # Apply adjustments in order
            img = self._apply_rotation(img, rotate)
            img = self._apply_flip(img, flip_h, flip_v)
            img = self._apply_crop_ratio(img, crop_ratio, crop_mode)
            brightness = self._merge_exposure(brightness, exposure)
            img = self._apply_brightness(img, brightness)
            img = self._apply_contrast(img, contrast)
            img = self._apply_saturation(img, saturation)
            img = self._apply_vibrance(img, vibrance)
            img = self._apply_hue(img, hue)
            img = self._apply_sharpness(img, sharpness)
            
            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Save the adjusted image
            logger.info(f"Saving adjusted image: {output_path}")
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
            logger.error(f"Image adjustment failed: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _apply_rotation(self, img, angle):
        """
        Apply rotation to an image.
        
        Args:
            img: PIL Image object
            angle (float): Rotation angle in degrees
        
        Returns:
            PIL Image: Rotated image
        """
        if angle == 0:
            return img
        
        logger.debug(f"Applying rotation: {angle} degrees")
        
        # Use expand=True to prevent cropping
        return img.rotate(angle, expand=True, resample=Image.Resampling.BICUBIC)
    
    def _apply_flip(self, img, flip_h, flip_v):
        """
        Apply flipping to an image.
        
        Args:
            img: PIL Image object
            flip_h (bool): Flip horizontally
            flip_v (bool): Flip vertically
        
        Returns:
            PIL Image: Flipped image
        """
        if not flip_h and not flip_v:
            return img
        
        if flip_h:
            logger.debug("Applying horizontal flip")
            img = ImageOps.mirror(img)
        
        if flip_v:
            logger.debug("Applying vertical flip")
            img = ImageOps.flip(img)
        
        return img
    
    def _apply_brightness(self, img, adjustment):
        """
        Apply brightness adjustment to an image.
        
        Args:
            img: PIL Image object
            adjustment (int): Brightness adjustment (-100 to +100)
        
        Returns:
            PIL Image: Brightness-adjusted image
        """
        if adjustment == 0:
            return img
        
        logger.debug(f"Applying brightness adjustment: {adjustment}")
        
        # Convert -100 to +100 range to 0.0 to 2.0 factor
        # 0 = no change, -100 = 0.0 (black), +100 = 2.0 (double brightness)
        factor = 1.0 + (adjustment / 100.0)
        factor = max(0.0, min(2.0, factor))  # Clamp to valid range
        
        enhancer = ImageEnhance.Brightness(img)
        return enhancer.enhance(factor)
    
    def _apply_contrast(self, img, adjustment):
        """
        Apply contrast adjustment to an image.
        
        Args:
            img: PIL Image object
            adjustment (int): Contrast adjustment (-100 to +100)
        
        Returns:
            PIL Image: Contrast-adjusted image
        """
        if adjustment == 0:
            return img
        
        logger.debug(f"Applying contrast adjustment: {adjustment}")
        
        # Convert -100 to +100 range to 0.0 to 2.0 factor
        factor = 1.0 + (adjustment / 100.0)
        factor = max(0.0, min(2.0, factor))
        
        enhancer = ImageEnhance.Contrast(img)
        return enhancer.enhance(factor)
    
    def _apply_saturation(self, img, adjustment):
        """
        Apply saturation adjustment to an image.
        
        Args:
            img: PIL Image object
            adjustment (int): Saturation adjustment (-100 to +100)
        
        Returns:
            PIL Image: Saturation-adjusted image
        """
        if adjustment == 0:
            return img
        
        logger.debug(f"Applying saturation adjustment: {adjustment}")
        
        # Convert -100 to +100 range to 0.0 to 2.0 factor
        factor = 1.0 + (adjustment / 100.0)
        factor = max(0.0, min(2.0, factor))
        
        # ImageEnhance.Color only works on RGB or RGBA images
        if img.mode in ('L', 'LA', 'P'):
            img = img.convert('RGB')
        
        enhancer = ImageEnhance.Color(img)
        return enhancer.enhance(factor)

    def _apply_vibrance(self, img, adjustment):
        """
        Apply vibrance adjustment (boost low-saturation colors more).
        """
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
        s = s.point(lut)
        out = Image.merge("HSV", (h, s, v)).convert("RGB")

        if alpha is not None:
            out = out.convert("RGBA")
            out.putalpha(alpha)
        return out

    def _apply_sharpness(self, img, adjustment):
        """
        Apply sharpness adjustment (-100 to 100).
        """
        if adjustment == 0:
            return img

        try:
            factor = 1.0 + (float(adjustment) / 100.0)
        except Exception:
            return img

        factor = max(0.0, min(3.0, factor))
        enhancer = ImageEnhance.Sharpness(img)
        return enhancer.enhance(factor)

    def _merge_exposure(self, brightness, exposure):
        try:
            base = float(brightness)
        except Exception:
            base = 0.0
        try:
            exp = float(exposure)
        except Exception:
            exp = 0.0
        merged = max(-100.0, min(100.0, base + exp))
        return merged

    def _apply_crop_ratio(self, img, crop_ratio, crop_mode):
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

        if current_ratio > target_ratio:
            new_width = int(height * target_ratio)
            left = max(0, (width - new_width) // 2)
            box = (left, 0, left + new_width, height)
        else:
            new_height = int(width / target_ratio)
            top = max(0, (height - new_height) // 2)
            box = (0, top, width, top + new_height)

        mode = str(crop_mode or "").strip().lower()
        if mode not in ("", "center", "centre"):
            return img
        return img.crop(box)
    
    def _apply_hue(self, img, adjustment):
        """
        Apply hue adjustment to an image.
        
        Args:
            img: PIL Image object
            adjustment (int): Hue adjustment (-180 to +180)
        
        Returns:
            PIL Image: Hue-adjusted image
        """
        if adjustment == 0:
            return img
        
        logger.debug(f"Applying hue adjustment: {adjustment}")
        
        # Convert to RGB mode for hue adjustment
        if img.mode not in ('RGB', 'RGBA'):
            img = img.convert('RGB')
        
        # Split into channels
        r, g, b = img.split()
        
        # Convert to HSL
        # This is a simplified hue shift implementation
        # For production use, consider using colorsys module or specialized libraries
        
        # Apply hue rotation by shifting RGB values
        # This is a simplified approach - a full HSL conversion would be more accurate
        shift = adjustment
        
        # Convert image to a mutable format for pixel manipulation
        pixels = img.load()
        width, height = img.size
        
        for y in range(height):
            for x in range(width):
                r_val, g_val, b_val = pixels[x, y][:3]
                
                # Simple RGB hue rotation
                # Rotate the color wheel
                if shift > 0:
                    # Shift towards next colors
                    r_val, g_val, b_val = self._rotate_hue(r_val, g_val, b_val, shift)
                elif shift < 0:
                    # Shift towards previous colors
                    r_val, g_val, b_val = self._rotate_hue(g_val, b_val, r_val, -shift)
                
                pixels[x, y] = (r_val, g_val, b_val) + pixels[x, y][3:]
        
        return img
    
    def _rotate_hue(self, r, g, b, angle):
        """
        Simple hue rotation for RGB values.
        
        Args:
            r, g, b: RGB values
            angle (int): Rotation angle in degrees
        
        Returns:
            tuple: Adjusted RGB values
        """
        # Normalize angle to 0-360
        angle = angle % 360
        
        # Simplified hue rotation
        # In a full implementation, this would use HSL/HSV color space
        if angle < 120:
            # Rotate R -> G
            factor = angle / 120.0
            r = int(r * (1 - factor) + g * factor)
        elif angle < 240:
            # Rotate G -> B
            factor = (angle - 120) / 120.0
            g = int(g * (1 - factor) + b * factor)
        else:
            # Rotate B -> R
            factor = (angle - 240) / 120.0
            b = int(b * (1 - factor) + r * factor)
        
        return (r, g, b)


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
        rotate = input_data.get('rotate', 0)
        flip_h = input_data.get('flip_h', False)
        flip_v = input_data.get('flip_v', False)
        brightness = input_data.get('brightness', 0)
        contrast = input_data.get('contrast', 0)
        saturation = input_data.get('saturation', 0)
        hue = input_data.get('hue', 0)
        exposure = input_data.get('exposure', 0)
        vibrance = input_data.get('vibrance', 0)
        sharpness = input_data.get('sharpness', 0)
        crop_ratio = input_data.get('crop_ratio', '')
        crop_mode = input_data.get('crop_mode', '')

        # Validate required parameters
        if not input_path or not output_path:
            return {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }

        # Create adjuster and perform adjustment
        adjuster = ImageAdjuster()
        result = adjuster.adjust(
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
            crop_mode=crop_mode
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the adjuster script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received adjustment request: {input_data.get('input_path')}")
        
        # Extract parameters
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')
        rotate = input_data.get('rotate', 0)
        flip_h = input_data.get('flip_h', False)
        flip_v = input_data.get('flip_v', False)
        brightness = input_data.get('brightness', 0)
        contrast = input_data.get('contrast', 0)
        saturation = input_data.get('saturation', 0)
        hue = input_data.get('hue', 0)
        exposure = input_data.get('exposure', 0)
        vibrance = input_data.get('vibrance', 0)
        sharpness = input_data.get('sharpness', 0)
        crop_ratio = input_data.get('crop_ratio', '')
        crop_mode = input_data.get('crop_mode', '')
        
        # Validate required parameters
        if not input_path or not output_path:
            result = {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }
        else:
            # Create adjuster and perform adjustment
            adjuster = ImageAdjuster()
            result = adjuster.adjust(
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
                crop_mode=crop_mode
            )
        
        # Write result to stdout
        logger.info(f"Image adjustment completed: {result.get('success')}")
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

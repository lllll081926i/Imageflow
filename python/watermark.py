#!/usr/bin/env python3
"""
Watermark Script

This script applies text or image watermarks to images using the Pillow library.
It supports flexible positioning, styling, and batch processing.

Usage:
    python watermark.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont, ImageEnhance
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class WatermarkApplier:
    """Handles watermark application to images."""
    
    # Supported watermark positions
    POSITIONS = [
        'top-left', 'top-center', 'top-right',
        'center-left', 'center', 'center-right',
        'bottom-left', 'bottom-center', 'bottom-right'
    ]
    
    def __init__(self):
        """Initialize the watermark applier."""
        logger.info("WatermarkApplier initialized")
    
    def apply(self, watermark_type, input_path, output_path,
              text='', font='arial', font_size=36, font_color='#FFFFFF',
              watermark_path='', watermark_scale=0.2,
              opacity=1.0, position='center', rotation=0,
              offset_x=0, offset_y=0):
        """
        Apply a watermark to an image.
        
        Args:
            watermark_type (str): Type of watermark ('text' or 'image')
            input_path (str): Path to the input image
            output_path (str): Path to save the watermarked image
            text (str): Text content for text watermark
            font (str): Font family name
            font_size (int): Font size in pixels
            font_color (str): Font color in hex format
            watermark_path (str): Path to watermark image
            watermark_scale (float): Scale factor for watermark image
            opacity (float): Opacity (0.0-1.0)
            position (str): Position of the watermark
            rotation (float): Rotation angle in degrees
            offset_x (int): Horizontal offset in pixels
            offset_y (int): Vertical offset in pixels
        
        Returns:
            dict: Watermark application result
        """
        try:
            # Open input image
            logger.info(f"Opening image: {input_path}")
            with Image.open(input_path) as base_img:
                original_mode = base_img.mode
                img = base_img.convert('RGBA') if base_img.mode != 'RGBA' else base_img.copy()
            
            # Create transparent overlay
            overlay = Image.new('RGBA', img.size, (0, 0, 0, 0))
            
            try:
                watermark_scale = float(watermark_scale)
            except (TypeError, ValueError):
                watermark_scale = 0.2
            if watermark_scale <= 0:
                watermark_scale = 0.2

            # Apply watermark based on type
            if watermark_type == 'text':
                self._apply_text_watermark(overlay, img.size, text, font,
                                         font_size, font_color, opacity,
                                         position, rotation, offset_x, offset_y)
            elif watermark_type == 'image':
                self._apply_image_watermark(overlay, img.size, watermark_path,
                                          watermark_scale, opacity, position,
                                          rotation, offset_x, offset_y)
            else:
                return {
                    'success': False,
                    'error': f'Invalid watermark type: {watermark_type}'
                }
            
            # Composite watermark onto original image
            watermarked = Image.alpha_composite(img, overlay)
            
            # Convert back to original mode if necessary
            if original_mode != 'RGBA':
                watermarked = watermarked.convert(original_mode)
            
            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Save the watermarked image
            logger.info(f"Saving watermarked image: {output_path}")
            watermarked.save(output_path)
            
            # Get file size
            file_size = os.path.getsize(output_path)
            
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
            logger.error(f"Watermark application failed: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _apply_text_watermark(self, overlay, img_size, text, font,
                             font_size, font_color, opacity, position,
                             rotation, offset_x, offset_y):
        """Apply a text watermark to the overlay."""
        try:
            # Get font
            try:
                pil_font = ImageFont.truetype(font, font_size)
            except:
                # Fall back to default font
                logger.warning(f"Font '{font}' not found, using default font")
                pil_font = ImageFont.load_default()
            
            # Create drawing context
            draw = ImageDraw.Draw(overlay)
            
            # Get text bounding box
            bbox = draw.textbbox((0, 0), text, font=pil_font)
            text_width = bbox[2] - bbox[0]
            text_height = bbox[3] - bbox[1]
            
            # Calculate position
            x, y = self._calculate_position(
                img_size, (text_width, text_height), position, offset_x, offset_y
            )
            
            # Create text image with alpha channel
            text_img = Image.new('RGBA', (text_width, text_height), (0, 0, 0, 0))
            text_draw = ImageDraw.Draw(text_img)
            
            # Parse color and apply opacity
            color = self._parse_color(font_color, opacity)
            
            # Draw text
            text_draw.text((0, 0), text, font=pil_font, fill=color)
            
            # Rotate if necessary
            if rotation != 0:
                text_img = text_img.rotate(rotation, expand=True, resample=Image.Resampling.BICUBIC)
            
            # Paste text onto overlay
            overlay.paste(text_img, (x, y), text_img)
            
            logger.debug(f"Applied text watermark at ({x}, {y})")
            
        except Exception as e:
            logger.error(f"Failed to apply text watermark: {e}")
            raise
    
    def _apply_image_watermark(self, overlay, img_size, watermark_path,
                              watermark_scale, opacity, position,
                              rotation, offset_x, offset_y):
        """Apply an image watermark to the overlay."""
        # Open watermark image
        with Image.open(watermark_path) as base_wm:
            watermark = base_wm.convert('RGBA') if base_wm.mode != 'RGBA' else base_wm.copy()
        
        # Scale watermark
        orig_width, orig_height = watermark.size
        scale = max(0.01, float(watermark_scale))
        new_width = max(1, int(img_size[0] * scale))
        new_height = max(1, int(orig_height * new_width / max(1, orig_width)))
        watermark = watermark.resize((new_width, new_height), Image.Resampling.LANCZOS)
        
        # Apply opacity
        if opacity < 1.0:
            alpha = watermark.split()[3]
            alpha = ImageEnhance.Brightness(alpha).enhance(opacity)
            watermark.putalpha(alpha)
        
        # Rotate if necessary
        if rotation != 0:
            watermark = watermark.rotate(rotation, expand=True, resample=Image.Resampling.BICUBIC)
        
        # Calculate position
        wm_width, wm_height = watermark.size
        x, y = self._calculate_position(
            img_size, (wm_width, wm_height), position, offset_x, offset_y
        )
        
        # Paste watermark onto overlay
        overlay.paste(watermark, (x, y), watermark)
        
        logger.debug(f"Applied image watermark at ({x}, {y})")
    
    def _calculate_position(self, img_size, wm_size, position, offset_x, offset_y):
        """Calculate watermark position based on position string."""
        img_width, img_height = img_size
        wm_width, wm_height = wm_size
        
        # Calculate base position
        if position == 'top-left':
            x = offset_x
            y = offset_y
        elif position == 'top-center':
            x = (img_width - wm_width) // 2 + offset_x
            y = offset_y
        elif position == 'top-right':
            x = img_width - wm_width - offset_x
            y = offset_y
        elif position == 'center-left':
            x = offset_x
            y = (img_height - wm_height) // 2 + offset_y
        elif position == 'center':
            x = (img_width - wm_width) // 2 + offset_x
            y = (img_height - wm_height) // 2 + offset_y
        elif position == 'center-right':
            x = img_width - wm_width - offset_x
            y = (img_height - wm_height) // 2 + offset_y
        elif position == 'bottom-left':
            x = offset_x
            y = img_height - wm_height - offset_y
        elif position == 'bottom-center':
            x = (img_width - wm_width) // 2 + offset_x
            y = img_height - wm_height - offset_y
        elif position == 'bottom-right':
            x = img_width - wm_width - offset_x
            y = img_height - wm_height - offset_y
        else:
            # Default to center
            x = (img_width - wm_width) // 2 + offset_x
            y = (img_height - wm_height) // 2 + offset_y
        
        return x, y
    
    def _parse_color(self, color_str, opacity):
        """Parse color string to RGBA tuple."""
        color_text = str(color_str or "").strip().lstrip("#")
        if len(color_text) == 3:
            color_text = "".join(ch * 2 for ch in color_text)
        if len(color_text) != 6 or any(ch not in "0123456789abcdefABCDEF" for ch in color_text):
            logger.warning("Invalid font color '%s', using white", color_str)
            color_text = "FFFFFF"

        # Parse hex color
        r = int(color_text[0:2], 16)
        g = int(color_text[2:4], 16)
        b = int(color_text[4:6], 16)
        a = int(opacity * 255)
        
        return (r, g, b, a)


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
        watermark_type = input_data.get('type')
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')

        # Text watermark parameters
        text = input_data.get('text', '')
        font = input_data.get('font', 'arial')
        font_size = input_data.get('font_size', 36)
        font_color = input_data.get('font_color', '#FFFFFF')

        # Image watermark parameters
        watermark_path = input_data.get('watermark_path', '')
        watermark_scale = input_data.get('watermark_scale', 0.2)

        # Common parameters
        opacity = input_data.get('opacity', 1.0)
        position = input_data.get('position', 'center')
        rotation = input_data.get('rotation', 0)
        offset_x = input_data.get('offset_x', 0)
        offset_y = input_data.get('offset_y', 0)

        # Validate required parameters
        if not input_path or not output_path:
            return {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }

        # Create watermark applier and apply watermark
        applier = WatermarkApplier()
        result = applier.apply(
            watermark_type=watermark_type,
            input_path=input_path,
            output_path=output_path,
            text=text,
            font=font,
            font_size=font_size,
            font_color=font_color,
            watermark_path=watermark_path,
            watermark_scale=watermark_scale,
            opacity=opacity,
            position=position,
            rotation=rotation,
            offset_x=offset_x,
            offset_y=offset_y
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the watermark script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received watermark request: {input_data.get('type')} watermark")
        
        # Extract parameters
        watermark_type = input_data.get('type')
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')
        
        # Text watermark parameters
        text = input_data.get('text', '')
        font = input_data.get('font', 'arial')
        font_size = input_data.get('font_size', 36)
        font_color = input_data.get('font_color', '#FFFFFF')
        
        # Image watermark parameters
        watermark_path = input_data.get('watermark_path', '')
        watermark_scale = input_data.get('watermark_scale', 0.2)
        
        # Common parameters
        opacity = input_data.get('opacity', 1.0)
        position = input_data.get('position', 'center')
        rotation = input_data.get('rotation', 0)
        offset_x = input_data.get('offset_x', 0)
        offset_y = input_data.get('offset_y', 0)
        
        # Validate required parameters
        if not input_path or not output_path:
            result = {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }
        else:
            # Create watermark applier and apply watermark
            applier = WatermarkApplier()
            result = applier.apply(
                watermark_type=watermark_type,
                input_path=input_path,
                output_path=output_path,
                text=text,
                font=font,
                font_size=font_size,
                font_color=font_color,
                watermark_path=watermark_path,
                watermark_scale=watermark_scale,
                opacity=opacity,
                position=position,
                rotation=rotation,
                offset_x=offset_x,
                offset_y=offset_y
            )
        
        # Write result to stdout
        logger.info(f"Watermark application completed: {result.get('success')}")
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

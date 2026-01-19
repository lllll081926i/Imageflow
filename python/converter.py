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
from pathlib import Path
from PIL import Image
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class ImageConverter:
    """Handles image format conversion operations."""
    
    # Supported output formats and their parameters
    OUTPUT_FORMATS = ['jpg', 'jpeg', 'png', 'webp', 'bmp', 'tiff', 'tif', 'ico', 'avif']

    # Quality-aware formats
    QUALITY_FORMATS = ['jpg', 'jpeg', 'webp', 'avif']
    
    def __init__(self):
        """Initialize the converter."""
        logger.info("ImageConverter initialized")
    
    def convert(self, input_path, output_path, format_type, quality=95,
                width=0, height=0, maintain_ar=True,
                resize_mode='',
                scale_percent=0,
                long_edge=0,
                keep_metadata=False,
                color_space='',
                dpi=0):
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
        
        Returns:
            dict: Conversion result with success status and metadata
        """
        try:
            # Validate format
            format_type = format_type.lower()
            if format_type not in self.OUTPUT_FORMATS:
                return {
                    'success': False,
                    'error': f'Unsupported output format: {format_type}'
                }
            
            # Handle SVG files (convert to PNG first)
            if input_path.lower().endswith('.svg'):
                logger.info(f"Detected SVG file: {input_path}")
                try:
                    import cairosvg  # type: ignore
                    import io
                    # Convert SVG to PNG in memory
                    png_data = cairosvg.svg2png(url=input_path)
                    img = Image.open(io.BytesIO(png_data))
                    logger.info("SVG converted to PNG successfully")
                except ImportError:
                    logger.warning("cairosvg not installed, trying Pillow directly")
                    img = Image.open(input_path)
                except Exception as e:
                    logger.error(f"Failed to convert SVG: {e}")
                    return {
                        'success': False,
                        'error': f'Failed to convert SVG: {str(e)}. Please install cairosvg: pip install cairosvg'
                    }
            else:
                # Open input image
                logger.info(f"Opening image: {input_path}")
                img = Image.open(input_path)

            exif_bytes = img.info.get('exif')
            icc_profile = img.info.get('icc_profile')
            
            # Convert RGBA to RGB for formats that don't support transparency
            if format_type in ['jpg', 'jpeg', 'pdf'] and img.mode == 'RGBA':
                logger.info("Converting RGBA to RGB")
                background = Image.new('RGB', img.size, (255, 255, 255))
                background.paste(img, mask=img.split()[3])  # Use alpha channel as mask
                img = background
            
            if color_space:
                cs = str(color_space).strip().lower()
                if cs in ['srgb', 's-rgb']:
                    try:
                        from PIL import ImageCms  # type: ignore
                        srgb = ImageCms.createProfile("sRGB")
                        if icc_profile:
                            src = ImageCms.ImageCmsProfile(io.BytesIO(icc_profile))
                            img = ImageCms.profileToProfile(img, src, srgb, outputMode='RGB')
                        else:
                            img = img.convert('RGB')
                    except Exception:
                        img = img.convert('RGB')
                elif cs == 'cmyk':
                    try:
                        img = img.convert('CMYK')
                    except Exception:
                        pass

            mode = str(resize_mode or '').strip().lower()
            if mode == 'percent' and int(scale_percent or 0) > 0:
                pct = max(1, int(scale_percent))
                new_w = max(1, int(img.size[0] * pct / 100))
                new_h = max(1, int(img.size[1] * pct / 100))
                img = img.resize((new_w, new_h), Image.Resampling.LANCZOS)
            elif mode == 'long_edge' and int(long_edge or 0) > 0:
                le = max(1, int(long_edge))
                w0, h0 = img.size
                if max(w0, h0) != le:
                    scale = le / float(max(w0, h0))
                    new_w = max(1, int(w0 * scale))
                    new_h = max(1, int(h0 * scale))
                    img = img.resize((new_w, new_h), Image.Resampling.LANCZOS)
            elif mode == 'fixed':
                if width > 0 or height > 0:
                    img = self._resize_image(img, width, height, maintain_ar)
            else:
                if width > 0 or height > 0:
                    img = self._resize_image(img, width, height, maintain_ar)
            
            # Prepare save parameters based on format
            save_params = self._get_save_params(format_type, quality)
            if int(dpi or 0) > 0:
                d = max(1, int(dpi))
                save_params['dpi'] = (d, d)
            if keep_metadata:
                if exif_bytes:
                    save_params['exif'] = exif_bytes
                if icc_profile:
                    save_params['icc_profile'] = icc_profile
            
            # Convert format names for Pillow
            pillow_format = self._convert_format_name(format_type)
            
            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Save the image
            logger.info(f"Saving to: {output_path} (format: {pillow_format})")
            img.save(output_path, format=pillow_format, **save_params)

            # Explicitly close image to free memory
            img.close()
            del img

            # Force garbage collection for large batch processing
            gc.collect()

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
                'error': f'Input file not found: {input_path}'
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
            new_width = target_width if target_width > 0 else original_width
            new_height = target_height if target_height > 0 else original_height
        
        logger.info(f"Resizing from {original_width}x{original_height} to {new_width}x{new_height}")
        
        # Use high-quality resampling
        return img.resize((new_width, new_height), Image.Resampling.LANCZOS)
    
    def _get_save_params(self, format_type, quality):
        """
        Get save parameters based on the output format.
        
        Args:
            format_type (str): Output format
            quality (int): Quality setting
        
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
            params['compress_level'] = 9
        
        if format_type == 'webp':
            params['method'] = 6  # Best compression
        
        if format_type == 'ico':
            params['sizes'] = [(16, 16), (32, 32), (64, 64), (128, 128)]
        
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
        format_type = input_data.get('format', 'jpg')
        quality = input_data.get('quality', 95)
        width = input_data.get('width', 0)
        height = input_data.get('height', 0)
        maintain_ar = input_data.get('maintain_ar', True)
        resize_mode = input_data.get('resize_mode', '')
        scale_percent = input_data.get('scale_percent', 0)
        long_edge = input_data.get('long_edge', 0)
        keep_metadata = input_data.get('keep_metadata', False)
        color_space = input_data.get('color_space', '')
        dpi = input_data.get('dpi', 0)

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
            color_space=color_space,
            dpi=dpi
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
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received conversion request: {input_data.get('input_path')} -> {input_data.get('output_path')}")
        
        # Extract parameters
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')
        format_type = input_data.get('format', 'jpg')
        quality = input_data.get('quality', 95)
        width = input_data.get('width', 0)
        height = input_data.get('height', 0)
        maintain_ar = input_data.get('maintain_ar', True)
        resize_mode = input_data.get('resize_mode', '')
        scale_percent = input_data.get('scale_percent', 0)
        long_edge = input_data.get('long_edge', 0)
        keep_metadata = input_data.get('keep_metadata', False)
        color_space = input_data.get('color_space', '')
        dpi = input_data.get('dpi', 0)
        
        # Validate required parameters
        if not input_path or not output_path:
            result = {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
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
                color_space=color_space,
                dpi=dpi
            )
        
        # Write result to stdout
        logger.info(f"Conversion completed: {result.get('success')}")
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

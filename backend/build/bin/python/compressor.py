#!/usr/bin/env python3
"""
Image Compressor Script

This script compresses images using the Pillow library.
It supports lossy, lossless, and smart compression modes.

Usage:
    python compressor.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
import gc
from pathlib import Path
from PIL import Image
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class ImageCompressor:
    """Handles image compression operations."""
    
    # Compression modes
    MODE_LOSSY = 'lossy'
    MODE_LOSSLESS = 'lossless'
    MODE_SMART = 'smart'
    
    def __init__(self):
        """Initialize the compressor."""
        logger.info("ImageCompressor initialized")
    
    def compress(self, input_path, output_path, mode='smart', quality=75):
        """
        Compress an image.
        
        Args:
            input_path (str): Path to the input image
            output_path (str): Path to save the compressed image
            mode (str): Compression mode (lossy, lossless, smart)
            quality (int): Quality setting (1-100) for lossy/smart modes
        
        Returns:
            dict: Compression result with success status and metadata
        """
        try:
            # Open input image
            logger.info(f"Opening image: {input_path}")
            img = Image.open(input_path)
            
            # Get original file size
            original_size = os.path.getsize(input_path)
            logger.info(f"Original size: {original_size} bytes")
            
            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Determine compression parameters based on mode and format
            format_type = img.format or 'PNG'
            save_params = self._get_compression_params(img, mode, quality, format_type)
            
            # Save the compressed image
            logger.info(f"Compressing to: {output_path} (mode: {mode})")
            img.save(output_path, format=format_type, **save_params)

            # Explicitly close image to free memory
            img.close()
            del img

            # Get compressed file size
            compressed_size = os.path.getsize(output_path)
            compression_rate = (1 - compressed_size / original_size) * 100 if original_size > 0 else 0

            logger.info(f"Compressed size: {compressed_size} bytes (saved {compression_rate:.2f}%)")

            # Force garbage collection
            gc.collect()

            # Return success result
            return {
                'success': True,
                'input_path': input_path,
                'output_path': output_path,
                'original_size': original_size,
                'compressed_size': compressed_size,
                'compression_rate': round(compression_rate, 2)
            }
            
        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            return {
                'success': False,
                'error': f'Input file not found: {input_path}'
            }
        except Exception as e:
            logger.error(f"Compression failed: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _get_compression_params(self, img, mode, quality, format_type):
        """
        Get compression parameters based on mode and image format.
        
        Args:
            img: PIL Image object
            mode (str): Compression mode
            quality (int): Quality setting
            format_type (str): Image format
        
        Returns:
            dict: Save parameters for PIL
        """
        params = {}
        
        if mode == self.MODE_LOSSY:
            # Lossy compression
            params.update(self._get_lossy_params(format_type, quality))
        
        elif mode == self.MODE_LOSSLESS:
            # Lossless compression
            params.update(self._get_lossless_params(format_type))
        
        elif mode == self.MODE_SMART:
            # Smart compression: automatically choose best approach
            params.update(self._get_smart_params(img, format_type, quality))
        
        return params
    
    def _get_lossy_params(self, format_type, quality):
        """Get parameters for lossy compression."""
        params = {}
        
        # Clamp quality to valid range
        quality = max(1, min(100, quality))
        
        if format_type in ['JPEG', 'JPG']:
            params['quality'] = quality
            params['optimize'] = True
            params['progressive'] = True
        elif format_type == 'WEBP':
            params['quality'] = quality
            params['method'] = 6  # Best compression
        elif format_type == 'PNG':
            # For PNG, lossy means reducing color depth
            params['optimize'] = True
            params['compress_level'] = 9
        
        return params
    
    def _get_lossless_params(self, format_type):
        """Get parameters for lossless compression."""
        params = {}
        
        if format_type in ['JPEG', 'JPG']:
            # JPEG is inherently lossy, use maximum quality
            params['quality'] = 100
            params['optimize'] = True
        elif format_type == 'WEBP':
            # Lossless WEBP
            params['lossless'] = True
            params['method'] = 6
        elif format_type == 'PNG':
            # Maximum PNG compression
            params['optimize'] = True
            params['compress_level'] = 9
        
        return params
    
    def _get_smart_params(self, img, format_type, quality):
        """
        Get parameters for smart compression.
        Automatically chooses the best approach based on image characteristics.
        """
        params = {}
        
        # Analyze image characteristics
        width, height = img.size
        has_transparency = img.mode in ['RGBA', 'LA', 'P']
        num_colors = self._count_colors(img) if img.mode == 'P' else None
        
        logger.debug(f"Image analysis: {width}x{height}, mode={img.mode}, transparency={has_transparency}")
        
        if format_type == 'JPEG' or (format_type == 'PNG' and not has_transparency and width * height > 4000000):
            # For large images without transparency, use JPEG with adaptive quality
            if format_type == 'PNG':
                # Convert to JPEG for better compression
                rgb_img = Image.new('RGB', img.size, (255, 255, 255))
                rgb_img.paste(img, mask=img.split()[-1] if has_transparency else None)
                return self._get_lossy_params('JPEG', min(quality + 10, 100))
            
            # Adjust quality based on image complexity
            adjusted_quality = self._adjust_quality_for_image(img, quality)
            params.update(self._get_lossy_params(format_type, adjusted_quality))
        
        elif format_type == 'PNG':
            if num_colors and num_colors <= 256:
                # Use optimal palette for limited color images
                params['optimize'] = True
                params['compress_level'] = 9
            else:
                # Standard PNG compression
                params['optimize'] = True
                params['compress_level'] = 9
        
        elif format_type == 'WEBP':
            # Use WEBP's smart compression
            params['quality'] = quality
            params['method'] = 6
            if has_transparency:
                params['alpha_quality'] = 100
        
        return params
    
    def _count_colors(self, img):
        """Count the number of unique colors in an image."""
        try:
            return len(img.getcolors())
        except (MemoryError, TypeError):
            return None
    
    def _adjust_quality_for_image(self, img, base_quality):
        """
        Adjust quality based on image characteristics.
        
        Args:
            img: PIL Image object
            base_quality (int): Base quality setting
        
        Returns:
            int: Adjusted quality
        """
        # Simple heuristic: reduce quality for large areas of uniform color
        try:
            # Sample a few pixels to estimate image complexity
            pixels = list(img.resize((100, 100)).getdata())
            unique_colors = len(set(pixels))
            
            if unique_colors < 1000:
                # Low complexity image, can reduce quality
                return max(base_quality - 10, 60)
            elif unique_colors > 50000:
                # High complexity image, increase quality
                return min(base_quality + 5, 100)
            else:
                return base_quality
        except Exception:
            return base_quality


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
        mode = input_data.get('mode', 'smart')
        quality = input_data.get('quality', 75)

        # Validate required parameters
        if not input_path or not output_path:
            return {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }

        # Create compressor and perform compression
        compressor = ImageCompressor()
        result = compressor.compress(
            input_path=input_path,
            output_path=output_path,
            mode=mode,
            quality=quality
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the compressor script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received compression request: {input_data.get('input_path')} -> {input_data.get('output_path')}")
        
        # Extract parameters
        input_path = input_data.get('input_path')
        output_path = input_data.get('output_path')
        mode = input_data.get('mode', 'smart')
        quality = input_data.get('quality', 75)
        
        # Validate required parameters
        if not input_path or not output_path:
            result = {
                'success': False,
                'error': 'Missing required parameters: input_path or output_path'
            }
        else:
            # Create compressor and perform compression
            compressor = ImageCompressor()
            result = compressor.compress(
                input_path=input_path,
                output_path=output_path,
                mode=mode,
                quality=quality
            )
        
        # Write result to stdout
        logger.info(f"Compression completed: {result.get('success')}")
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

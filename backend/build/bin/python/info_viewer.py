#!/usr/bin/env python3
"""
Image Info Viewer Script

This script extracts and displays detailed information about images,
including basic info, EXIF metadata, and histogram data.

Usage:
    python info_viewer.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
from pathlib import Path
from PIL import Image
from PIL.ExifTags import TAGS
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class InfoViewer:
    """Handles image information extraction and display."""
    
    def __init__(self):
        """Initialize the info viewer."""
        logger.info("InfoViewer initialized")
    
    def get_info(self, input_path):
        """
        Get detailed information about an image.
        
        Args:
            input_path (str): Path to the image file
        
        Returns:
            dict: Image information including basic info, EXIF data, and histogram
        """
        try:
            # Open image
            logger.info(f"Reading image info: {input_path}")
            img = Image.open(input_path)
            
            # Get file info
            file_info = self._get_file_info(input_path)
            
            # Get basic image info
            basic_info = self._get_basic_info(img, input_path)
            
            # Get EXIF data
            exif_data = self._get_exif_data(img)
            
            # Get histogram
            histogram_data = self._get_histogram(img)
            
            # Combine all information
            result = {
                'file_name': file_info['name'],
                'file_size': file_info['size'],
                'format': basic_info['format'],
                'width': basic_info['width'],
                'height': basic_info['height'],
                'mode': basic_info['mode'],
                'bit_depth': basic_info['bit_depth'],
                'exif': exif_data,
                'histogram': histogram_data,
                'success': True
            }
            
            logger.info(f"Successfully read image info: {basic_info['width']}x{basic_info['height']} {basic_info['format']}")
            
            return result
            
        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            return {
                'success': False,
                'error': f'File not found: {input_path}'
            }
        except Exception as e:
            logger.error(f"Failed to get image info: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _get_file_info(self, file_path):
        """Get basic file information."""
        stat = os.stat(file_path)
        return {
            'name': Path(file_path).name,
            'size': stat.st_size,
            'modified': stat.st_mtime
        }
    
    def _get_basic_info(self, img, file_path):
        """Get basic image information."""
        return {
            'format': img.format or 'Unknown',
            'width': img.width,
            'height': img.height,
            'mode': img.mode,
            'bit_depth': self._get_bit_depth(img)
        }
    
    def _get_bit_depth(self, img):
        """Calculate the bit depth of an image."""
        mode_to_depth = {
            '1': 1,      # 1-bit pixels, black and white
            'L': 8,      # 8-bit pixels, grayscale
            'P': 8,      # 8-bit pixels, mapped to any other mode using a color palette
            'RGB': 24,   # 8x3-bit pixels, true color
            'RGBA': 32,  # 8x4-bit pixels, true color with transparency mask
            'CMYK': 32,  # 8x4-bit pixels, color separation
            'YCbCr': 24, # 8x3-bit pixels, color video format
            'LAB': 24,   # 8x3-bit pixels, the L*a*b color space
            'HSV': 24,   # 8x3-bit pixels, Hue, Saturation, Value color space
            'I': 32,     # 32-bit signed integer pixels
            'F': 32,     # 32-bit floating point pixels
        }
        return mode_to_depth.get(img.mode, 0)
    
    def _get_exif_data(self, img):
        """
        Extract EXIF metadata from an image.
        
        Args:
            img: PIL Image object
        
        Returns:
            dict: EXIF data with human-readable tag names
        """
        exif_data = {}
        
        try:
            # Get EXIF data
            exif = img._getexif()
            
            if exif is None:
                return exif_data
            
            # Convert tag IDs to human-readable names
            for tag_id, value in exif.items():
                tag_name = TAGS.get(tag_id, tag_id)
                
                # Convert bytes to string if necessary
                if isinstance(value, bytes):
                    try:
                        value = value.decode('utf-8', errors='replace')
                    except:
                        value = str(value)
                
                # Skip very long values (like thumbnail data)
                if tag_name != 'MakerNote' and tag_name != 'UserComment':
                    # Limit value length for readability
                    if isinstance(value, str) and len(value) > 200:
                        value = value[:200] + '...'
                    exif_data[tag_name] = value
            
        except AttributeError:
            # Image doesn't have EXIF data
            pass
        except Exception as e:
            logger.warning(f"Failed to read EXIF data: {e}")
        
        return exif_data
    
    def _get_histogram(self, img):
        """
        Calculate histogram data for an image.
        
        Args:
            img: PIL Image object
        
        Returns:
            dict: RGB and luminance histograms
        """
        try:
            # Convert to RGB if necessary
            if img.mode != 'RGB':
                img = img.convert('RGB')
            
            # Calculate histogram
            hist = img.histogram()
            
            # Split into RGB channels
            red_hist = hist[0:256]
            green_hist = hist[256:512]
            blue_hist = hist[512:768]
            
            # Calculate luminance histogram
            luminance_hist = [0] * 256
            for r, g, b in zip(red_hist, green_hist, blue_hist):
                idx = int((r * 0.299 + g * 0.587 + b * 0.114) / 255)
                luminance_hist[idx] += 1
            
            # Normalize histograms to 0-100 range for easier transmission
            max_val = max(max(red_hist), max(green_hist), max(blue_hist), max(luminance_hist))
            if max_val > 0:
                red_hist = [int(v * 100 / max_val) for v in red_hist]
                green_hist = [int(v * 100 / max_val) for v in green_hist]
                blue_hist = [int(v * 100 / max_val) for v in blue_hist]
                luminance_hist = [int(v * 100 / max_val) for v in luminance_hist]
            
            return {
                'red': red_hist,
                'green': green_hist,
                'blue': blue_hist,
                'luminance': luminance_hist
            }
            
        except Exception as e:
            logger.warning(f"Failed to calculate histogram: {e}")
            return {
                'red': [0] * 256,
                'green': [0] * 256,
                'blue': [0] * 256,
                'luminance': [0] * 256
            }
    
    def export_info(self, image_info, output_path, format='json'):
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
            # Create output directory
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Export based on format
            if format == 'json':
                with open(output_path, 'w', encoding='utf-8') as f:
                    json.dump(image_info, f, indent=2, ensure_ascii=False)
            elif format == 'txt':
                with open(output_path, 'w', encoding='utf-8') as f:
                    f.write("Image Information\n")
                    f.write("=" * 50 + "\n\n")
                    
                    # Basic info
                    f.write("Basic Information:\n")
                    f.write(f"  File Name: {image_info.get('file_name')}\n")
                    f.write(f"  File Size: {image_info.get('file_size')} bytes\n")
                    f.write(f"  Format: {image_info.get('format')}\n")
                    f.write(f"  Dimensions: {image_info.get('width')}x{image_info.get('height')}\n")
                    f.write(f"  Color Mode: {image_info.get('mode')}\n")
                    f.write(f"  Bit Depth: {image_info.get('bit_depth')}\n\n")
                    
                    # EXIF data
                    exif = image_info.get('exif', {})
                    if exif:
                        f.write("EXIF Metadata:\n")
                        for key, value in exif.items():
                            f.write(f"  {key}: {value}\n")
                        f.write("\n")
                    
                    # Histogram note
                    f.write("Histogram data is available in JSON format only.\n")
            
            logger.info(f"Image info exported to {output_path}")
            
            return {
                'success': True,
                'output_path': output_path
            }
            
        except Exception as e:
            logger.error(f"Failed to export image info: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def edit_exif(self, input_path, output_path, exif_data):
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
            # Open image
            img = Image.open(input_path)
            
            # Get existing EXIF data
            existing_exif = img._getexif() or {}
            
            # Update EXIF data
            # Note: This is a simplified implementation
            # Full EXIF editing requires more sophisticated libraries like piexif
            logger.warning("Full EXIF editing requires piexif library. This is a placeholder implementation.")
            
            # Save image with EXIF
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            img.save(output_path)
            
            return {
                'success': True,
                'output_path': output_path
            }
            
        except Exception as e:
            logger.error(f"Failed to edit EXIF: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }


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
        action = input_data.get('action')

        if action == 'get_info':
            input_path = input_data.get('input_path')

            if not input_path:
                return {
                    'success': False,
                    'error': 'Missing required parameter: input_path'
                }

            # Get image info
            viewer = InfoViewer()
            result = viewer.get_info(input_path)

        elif action == 'export':
            image_info = input_data.get('image_info')
            output_path = input_data.get('output_path')
            format_type = input_data.get('format', 'json')

            if not image_info or not output_path:
                return {
                    'success': False,
                    'error': 'Missing required parameters: image_info or output_path'
                }

            # Export image info
            viewer = InfoViewer()
            result = viewer.export_info(image_info, output_path, format_type)

        elif action == 'edit_exif':
            input_path = input_data.get('input_path')
            output_path = input_data.get('output_path')
            exif_data = input_data.get('exif_data', {})

            if not input_path or not output_path:
                return {
                    'success': False,
                    'error': 'Missing required parameters: input_path or output_path'
                }

            # Edit EXIF data
            viewer = InfoViewer()
            result = viewer.edit_exif(input_path, output_path, exif_data)

        else:
            return {
                'success': False,
                'error': f'Unknown action: {action}'
            }

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the info viewer script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        
        # Extract parameters
        action = input_data.get('action')
        
        if action == 'get_info':
            input_path = input_data.get('input_path')
            
            if not input_path:
                result = {
                    'success': False,
                    'error': 'Missing required parameter: input_path'
                }
            else:
                # Get image info
                viewer = InfoViewer()
                result = viewer.get_info(input_path)
        
        elif action == 'export':
            image_info = input_data.get('image_info')
            output_path = input_data.get('output_path')
            format_type = input_data.get('format', 'json')
            
            if not image_info or not output_path:
                result = {
                    'success': False,
                    'error': 'Missing required parameters: image_info or output_path'
                }
            else:
                # Export image info
                viewer = InfoViewer()
                result = viewer.export_info(image_info, output_path, format_type)
        
        elif action == 'edit_exif':
            input_path = input_data.get('input_path')
            output_path = input_data.get('output_path')
            exif_data = input_data.get('exif_data', {})
            
            if not input_path or not output_path:
                result = {
                    'success': False,
                    'error': 'Missing required parameters: input_path or output_path'
                }
            else:
                # Edit EXIF data
                viewer = InfoViewer()
                result = viewer.edit_exif(input_path, output_path, exif_data)
        
        else:
            result = {
                'success': False,
                'error': f'Unknown action: {action}'
            }
        
        # Write result to stdout
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

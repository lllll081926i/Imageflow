#!/usr/bin/env python3
"""
GIF Splitter Script

This script splits animated GIFs into individual frames using the Pillow library.
It supports frame range selection and multiple output formats.

Usage:
    python gif_splitter.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
from pathlib import Path
from PIL import Image
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class GIFSplitter:
    """Handles GIF splitting operations."""
    
    # Supported output formats
    OUTPUT_FORMATS = ['png', 'jpg', 'bmp']
    
    def __init__(self):
        """Initialize the GIF splitter."""
        logger.info("GIFSplitter initialized")
    
    def split(self, input_path, output_dir, output_format='png', frame_range='all'):
        """
        Split a GIF animation into individual frames.
        
        Args:
            input_path (str): Path to the input GIF file
            output_dir (str): Directory to save the extracted frames
            output_format (str): Output format (png, jpg, bmp)
            frame_range (str): Frame range specification:
                - 'all': Export all frames
                - 'start-end': Export frames from start to end (e.g., '0-10')
                - 'start:step': Export frames with step (e.g., '0:3' for every 3rd frame)
        
        Returns:
            dict: Splitting result with success status and metadata
        """
        try:
            # Validate output format
            output_format = output_format.lower()
            if output_format not in self.OUTPUT_FORMATS:
                return {
                    'success': False,
                    'error': f'Unsupported output format: {output_format}'
                }
            
            # Open GIF image
            logger.info(f"Opening GIF: {input_path}")
            gif = Image.open(input_path)
            
            # Get total frame count
            frame_count = self._get_frame_count(gif)
            logger.info(f"GIF contains {frame_count} frames")
            
            # Parse frame range
            frame_indices = self._parse_frame_range(frame_range, frame_count)
            logger.info(f"Exporting {len(frame_indices)} frames: {frame_indices}")
            
            # Create output directory
            os.makedirs(output_dir, exist_ok=True)
            
            # Extract frames
            frame_files = []
            base_name = Path(input_path).stem
            
            for i, frame_idx in enumerate(frame_indices):
                # Seek to the frame
                gif.seek(frame_idx)
                
                # Copy the frame to ensure we don't lose it when seeking
                frame = gif.copy()
                
                # Construct output filename
                output_filename = f"{base_name}_frame_{frame_idx:04d}.{output_format}"
                output_path = os.path.join(output_dir, output_filename)
                
                # Handle transparency for JPEG output
                if output_format == 'jpg' and frame.mode in ('RGBA', 'LA', 'P'):
                    background = Image.new('RGB', frame.size, (255, 255, 255))
                    if frame.mode == 'P':
                        frame = frame.convert('RGBA')
                    background.paste(frame, mask=frame.split()[-1] if len(frame.split()) > 1 else None)
                    frame = background
                
                # Save the frame
                frame.save(output_path, format=output_format.upper())
                frame_files.append(output_path)
                
                logger.debug(f"Saved frame {frame_idx} to {output_path}")
            
            logger.info(f"Successfully exported {len(frame_files)} frames to {output_dir}")
            
            return {
                'success': True,
                'input_path': input_path,
                'output_dir': output_dir,
                'frame_count': frame_count,
                'export_count': len(frame_files),
                'frame_files': frame_files
            }
            
        except FileNotFoundError as e:
            logger.error(f"File not found: {e}")
            return {
                'success': False,
                'error': f'Input file not found: {input_path}'
            }
        except Exception as e:
            logger.error(f"GIF splitting failed: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _get_frame_count(self, gif):
        """
        Get the total number of frames in a GIF.
        
        Args:
            gif: PIL Image object (GIF)
        
        Returns:
            int: Number of frames
        """
        frame_count = 0
        try:
            while True:
                gif.seek(frame_count)
                frame_count += 1
        except EOFError:
            pass
        
        return frame_count
    
    def _parse_frame_range(self, frame_range, total_frames):
        """
        Parse frame range specification and return list of frame indices.
        
        Args:
            frame_range (str): Frame range specification
            total_frames (int): Total number of frames in the GIF
        
        Returns:
            list: List of frame indices to export
        """
        if frame_range == 'all':
            return list(range(total_frames))
        
        try:
            if '-' in frame_range:
                # Range specification: start-end
                start, end = map(int, frame_range.split('-'))
                return list(range(start, min(end + 1, total_frames)))
            
            elif ':' in frame_range:
                # Interval specification: start:step
                parts = frame_range.split(':')
                if len(parts) == 2:
                    start, step = map(int, parts)
                    return list(range(start, total_frames, step))
            
            # Single frame
            return [int(frame_range)]
            
        except (ValueError, IndexError) as e:
            logger.warning(f"Invalid frame range '{frame_range}': {e}. Using all frames.")
            return list(range(total_frames))


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
        # Check for special action to get frame count only
        action = input_data.get('action')
        if action == 'get_frame_count':
            input_path = input_data.get('input_path')
            try:
                gif = Image.open(input_path)
                frame_count = 0
                while True:
                    gif.seek(frame_count)
                    frame_count += 1
            except EOFError:
                pass

            return {
                'frame_count': frame_count
            }

        # Extract parameters for normal splitting
        input_path = input_data.get('input_path')
        output_dir = input_data.get('output_dir')
        output_format = input_data.get('output_format', 'png')
        frame_range = input_data.get('frame_range', 'all')

        # Validate required parameters
        if not input_path or not output_dir:
            return {
                'success': False,
                'error': 'Missing required parameters: input_path or output_dir'
            }

        # Create splitter and perform splitting
        splitter = GIFSplitter()
        result = splitter.split(
            input_path=input_path,
            output_dir=output_dir,
            output_format=output_format,
            frame_range=frame_range
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the GIF splitter script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received GIF split request: {input_data.get('input_path')}")
        
        # Check for special action to get frame count only
        action = input_data.get('action')
        if action == 'get_frame_count':
            input_path = input_data.get('input_path')
            try:
                gif = Image.open(input_path)
                frame_count = 0
                while True:
                    gif.seek(frame_count)
                    frame_count += 1
            except EOFError:
                pass
            
            result = {
                'frame_count': frame_count
            }
            json.dump(result, sys.stdout)
            return
        
        # Extract parameters
        input_path = input_data.get('input_path')
        output_dir = input_data.get('output_dir')
        output_format = input_data.get('output_format', 'png')
        frame_range = input_data.get('frame_range', 'all')
        
        # Validate required parameters
        if not input_path or not output_dir:
            result = {
                'success': False,
                'error': 'Missing required parameters: input_path or output_dir'
            }
        else:
            # Create splitter and perform splitting
            splitter = GIFSplitter()
            result = splitter.split(
                input_path=input_path,
                output_dir=output_dir,
                output_format=output_format,
                frame_range=frame_range
            )
        
        # Write result to stdout
        logger.info(f"GIF splitting completed: {result.get('success')}")
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

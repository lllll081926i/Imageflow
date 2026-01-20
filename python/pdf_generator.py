#!/usr/bin/env python3
"""
PDF Generator Script

This script generates PDF documents from multiple images using the reportlab library.
It supports various layouts, page sizes, and metadata settings.

Usage:
    python pdf_generator.py
    (Input is provided via JSON on stdin)
    (Output is provided via JSON on stdout)
"""

import sys
import json
import os
import io
import math
from pathlib import Path
from PIL import Image
from reportlab.lib.pagesizes import A4, A3, letter, legal
from reportlab.lib.units import inch, mm
from reportlab.platypus import SimpleDocTemplate, PageBreak, Image as RLImage
from reportlab.pdfgen import canvas
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class PDFGenerator:
    """Handles PDF generation from images."""
    
    # Supported page sizes
    PAGE_SIZES = {
        'A4': A4,
        'A3': A3,
        'Letter': letter,
        'Legal': legal,
    }
    
    # Default margin (1 inch)
    DEFAULT_MARGIN = 72  # points
    
    def __init__(self):
        """Initialize the PDF generator."""
        logger.info("PDFGenerator initialized")
    
    def generate(self, images, output_path, layout='single',
                 custom_rows=2, custom_cols=2, page_size='A4',
                 margin=72, title='', author='', portrait=True,
                 compression_level=0):
        """
        Generate a PDF from multiple images.
        
        Args:
            images (list): List of image file paths
            output_path (str): Path to save the PDF
            layout (str): Layout mode (single, 2x2, 3x3, custom)
            custom_rows (int): Number of rows for custom layout
            custom_cols (int): Number of columns for custom layout
            page_size (str): Page size (A4, A3, Letter, Legal)
            margin (float): Page margin in points
            title (str): PDF metadata title
            author (str): PDF metadata author
            portrait (bool): True for portrait, False for landscape
            compression_level (int): 0=none, 1-3 JPEG quality levels
        
        Returns:
            dict: Generation result with success status and metadata
        """
        try:
            # Validate images
            valid_images = self._validate_images(images)
            if not valid_images:
                return {
                    'success': False,
                    'error': 'No valid images found'
                }
            
            image_count = len(valid_images)
            page_count = self._estimate_page_count(image_count, layout, custom_rows, custom_cols)
            logger.info(f"Generating PDF with {image_count} images")
            
            # Get page size
            if page_size in self.PAGE_SIZES:
                pagesize = self.PAGE_SIZES[page_size]
            else:
                # Default to A4
                pagesize = A4
            
            # Swap dimensions for landscape
            if not portrait:
                pagesize = (pagesize[1], pagesize[0])
            
            # Create output directory if it doesn't exist
            output_dir = os.path.dirname(output_path)
            if output_dir:
                os.makedirs(output_dir, exist_ok=True)
            
            # Create PDF document
            doc = SimpleDocTemplate(
                output_path,
                pagesize=pagesize,
                leftMargin=margin,
                rightMargin=margin,
                topMargin=margin,
                bottomMargin=margin
            )
            
            # Set metadata
            doc.title = title
            doc.author = author
            
            # Build content based on layout
            story = self._build_content(
                valid_images,
                layout,
                custom_rows,
                custom_cols,
                pagesize,
                margin,
                compression_level,
            )
            
            # Generate PDF
            doc.build(story)
            
            # Get file size
            file_size = os.path.getsize(output_path)
            
            logger.info(f"PDF generated: {output_path} ({file_size} bytes)")
            
            return {
                'success': True,
                'output_path': output_path,
                'file_size': file_size,
                'image_count': image_count,
                'page_count': page_count,
            }
            
        except Exception as e:
            logger.error(f"PDF generation failed: {e}", exc_info=True)
            return {
                'success': False,
                'error': str(e)
            }
    
    def _validate_images(self, images):
        """Validate that all image files exist and can be opened."""
        valid_images = []
        
        for img_path in images:
            if os.path.exists(img_path):
                try:
                    # Try to open the image to validate it
                    with Image.open(img_path) as img:
                        img.verify()
                    valid_images.append(img_path)
                except Exception as e:
                    logger.warning(f"Cannot open image {img_path}: {e}")
            else:
                logger.warning(f"Image not found: {img_path}")
        
        return valid_images
    
    def _build_content(self, images, layout, custom_rows, custom_cols, pagesize, margin, compression_level=0):
        """
        Build the content list for the PDF based on the layout.
        
        Args:
            images (list): List of valid image paths
            layout (str): Layout mode
            custom_rows (int): Number of rows for custom layout
            custom_cols (int): Number of columns for custom layout
            pagesize (tuple): Page size (width, height)
            margin (float): Page margin in points
        
        Returns:
            list: List of reportlab flowables
        """
        story = []
        
        if layout == 'single':
            # One image per page
            for img_path in images:
                img = self._create_image_flowable(
                    img_path,
                    pagesize,
                    margin,
                    scale=0.95,
                    compression_level=compression_level,
                )
                story.append(img)
                story.append(PageBreak())
        
        elif layout == '2x2':
            # 2x2 grid layout
            story.extend(self._create_grid_layout(images, 2, 2, pagesize, margin, compression_level))
        
        elif layout == '3x3':
            # 3x3 grid layout
            story.extend(self._create_grid_layout(images, 3, 3, pagesize, margin, compression_level))
        
        elif layout == 'custom':
            # Custom grid layout
            story.extend(self._create_grid_layout(images, custom_rows, custom_cols, pagesize, margin, compression_level))
        
        else:
            # Default to single image per page
            for img_path in images:
                img = self._create_image_flowable(
                    img_path,
                    pagesize,
                    margin,
                    scale=1.0,
                    compression_level=compression_level,
                )
                story.append(img)
                story.append(PageBreak())
        
        # Remove trailing PageBreak
        if story and isinstance(story[-1], PageBreak):
            story.pop()
        
        return story
    
    def _create_grid_layout(self, images, rows, cols, pagesize, margin, compression_level=0):
        """Create a grid layout for images."""
        from reportlab.platypus import Table, TableStyle
        from reportlab.lib import colors
        
        story = []
        page_width = pagesize[0] - 2 * margin
        page_height = pagesize[1] - 2 * margin
        
        cell_width = page_width / cols
        cell_height = page_height / rows
        
        # Process images in batches
        for i in range(0, len(images), rows * cols):
            batch = images[i:i + rows * cols]
            
            # Create table
            table_data = []
            for row in range(rows):
                row_data = []
                for col in range(cols):
                    idx = row * cols + col
                    if idx < len(batch):
                        img = self._create_image_flowable(
                            batch[idx],
                            (cell_width, cell_height),
                            margin=0,
                            scale=0.9,
                            compression_level=compression_level,
                        )
                        row_data.append(img)
                    else:
                        row_data.append('')
                table_data.append(row_data)
            
            if table_data:
                table = Table(
                    table_data,
                    colWidths=[cell_width] * cols,
                    rowHeights=[cell_height] * rows
                )
                table.setStyle(TableStyle([
                    ('ALIGN', (0, 0), (-1, -1), 'CENTER'),
                    ('VALIGN', (0, 0), (-1, -1), 'MIDDLE'),
                    ('GRID', (0, 0), (-1, -1), 1, colors.lightgrey),
                ]))
                story.append(table)
                story.append(PageBreak())
        
        # Remove trailing PageBreak
        if story and isinstance(story[-1], PageBreak):
            story.pop()
        
        return story
    
    def _create_image_flowable(self, img_path, available_size, margin=0, scale=0.95, compression_level=0):
        """
        Create a reportlab Image flowable from a file path.

        Args:
            img_path (str): Path to the image file
            available_size (tuple): Available size (width, height)
            margin (float): Margin to apply
            scale (float): Scale factor (0-1)

        Returns:
            RLImage: Reportlab Image flowable
        """
        # Load image with PIL to get dimensions
        with Image.open(img_path) as pil_img:
            img_width, img_height = pil_img.size

            # Calculate available space (subtract margins first, then apply scale)
            avail_width = (available_size[0] - 2 * margin) * scale
            avail_height = (available_size[1] - 2 * margin) * scale

            # Calculate scale to fit in available space
            width_ratio = avail_width / img_width
            height_ratio = avail_height / img_height
            scale_factor = min(width_ratio, height_ratio, 1.0)

            # Calculate final dimensions
            final_width = img_width * scale_factor
            final_height = img_height * scale_factor

            # Create reportlab image
            if compression_level > 0:
                buffer = self._encode_image_for_pdf(pil_img, compression_level)
                img = RLImage(buffer, width=final_width, height=final_height)
                img._image_buffer = buffer
            else:
                img = RLImage(img_path, width=final_width, height=final_height)

        return img

    def _estimate_page_count(self, image_count, layout, rows, cols):
        if image_count <= 0:
            return 0
        if layout == 'single':
            return image_count
        if layout == '2x2':
            per_page = 4
        elif layout == '3x3':
            per_page = 9
        elif layout == 'custom':
            per_page = max(1, rows) * max(1, cols)
        else:
            return image_count
        return int(math.ceil(image_count / per_page))

    def _encode_image_for_pdf(self, pil_img, compression_level):
        quality_map = {1: 85, 2: 70, 3: 50}
        quality = quality_map.get(int(compression_level), 70)

        pil_img = self._prepare_jpeg_image(pil_img)

        buffer = io.BytesIO()
        pil_img.save(
            buffer,
            format="JPEG",
            quality=quality,
            optimize=True,
            progressive=True,
        )
        buffer.seek(0)
        return buffer

    def _prepare_jpeg_image(self, pil_img):
        if pil_img.mode in ("RGBA", "LA") or (pil_img.mode == "P" and "transparency" in pil_img.info):
            rgba = pil_img.convert("RGBA")
            background = Image.new("RGB", rgba.size, (255, 255, 255))
            background.paste(rgba, mask=rgba.split()[-1])
            return background
        if pil_img.mode != "RGB":
            return pil_img.convert("RGB")
        return pil_img


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
        images = input_data.get('images', [])
        output_path = input_data.get('output_path', 'output.pdf')
        layout = input_data.get('layout', 'single')
        custom_rows = input_data.get('custom_rows', 2)
        custom_cols = input_data.get('custom_cols', 2)
        page_size = input_data.get('page_size', 'A4')
        margin = input_data.get('margin', 72)
        title = input_data.get('title', '')
        author = input_data.get('author', '')
        portrait = input_data.get('portrait', True)
        compression_level = input_data.get('compression_level', 0)

        # Validate required parameters
        if not images or not output_path:
            return {
                'success': False,
                'error': 'Missing required parameters: images or output_path'
            }

        # Create generator and perform PDF generation
        generator = PDFGenerator()
        result = generator.generate(
            images=images,
            output_path=output_path,
            layout=layout,
            custom_rows=custom_rows,
            custom_cols=custom_cols,
            page_size=page_size,
            margin=margin,
            title=title,
            author=author,
            portrait=portrait,
            compression_level=compression_level
        )

        return result

    except Exception as e:
        logger.error(f"Process function error: {e}", exc_info=True)
        return {
            'success': False,
            'error': str(e)
        }


def main():
    """Main entry point for the PDF generator script."""
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
        logger.info(f"Received PDF generation request: {len(input_data.get('images', []))} images")
        
        # Extract parameters
        images = input_data.get('images', [])
        output_path = input_data.get('output_path', 'output.pdf')
        layout = input_data.get('layout', 'single')
        custom_rows = input_data.get('custom_rows', 2)
        custom_cols = input_data.get('custom_cols', 2)
        page_size = input_data.get('page_size', 'A4')
        margin = input_data.get('margin', 72)
        title = input_data.get('title', '')
        author = input_data.get('author', '')
        portrait = input_data.get('portrait', True)
        compression_level = input_data.get('compression_level', 0)
        
        # Validate required parameters
        if not images or not output_path:
            result = {
                'success': False,
                'error': 'Missing required parameters: images or output_path'
            }
        else:
            # Create generator and perform PDF generation
            generator = PDFGenerator()
            result = generator.generate(
                images=images,
                output_path=output_path,
                layout=layout,
                custom_rows=custom_rows,
                custom_cols=custom_cols,
                page_size=page_size,
                margin=margin,
                title=title,
                author=author,
                portrait=portrait,
                compression_level=compression_level
            )
        
        # Write result to stdout
        logger.info(f"PDF generation completed: {result.get('success')}")
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

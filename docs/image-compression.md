# Image Compression Feature Documentation

## Overview

The Image Compression feature provides advanced image optimization using specialized compression libraries. It supports multiple compression levels with automatic algorithm selection based on image format and quality requirements.

## Supported Compression Libraries

The implementation uses three high-quality compression libraries:

1. **MoZJPEG** - For JPEG compression with quality control and lossless optimization
2. **PNGQuant (imagequant)** - For lossy PNG compression with palette optimization
3. **OxiPNG** - For lossless PNG optimization and metadata stripping

## Supported Image Formats

- **JPEG/JPG** - Optimized using MoZJPEG for both lossy and lossless compression
- **PNG** - Uses PNGQuant for lossy compression or OxiPNG for lossless optimization
- **WEBP** - Uses Pillow's WEBP encoder with quality control

## Compression Levels

The system provides 5 compression levels, mapped to integer values 1-5:

| Level | Name | Quality | Description |
|-------|------|---------|-------------|
| 1 | Lossless | 100% | No quality loss, uses lossless algorithms |
| 2 | Light | 90% | Minimal quality loss, ~10% reduction |
| 3 | Medium | 75% | Balanced quality/compression, ~25% reduction |
| 4 | Heavy | 60% | Significant compression, ~40% reduction |
| 5 | Extreme | 40% | Maximum compression, ~60% reduction |

### Algorithm Selection by Level and Format

#### JPEG Images
- **Level 1 (Lossless)**: Uses MoZJPEG lossless optimization on existing JPEGs, or saves at 100% quality for new images
- **Levels 2-5**: Uses MoZJPEG with progressive JPEG encoding and optimized Huffman tables

#### PNG Images
- **Level 1 (Lossless)**: Uses OxiPNG for lossless optimization (level 4, safe metadata stripping)
- **Levels 2-5**: Uses imagequant for lossy compression with adaptive quality ranges:
  - Level 2: 80-100% quality range
  - Level 3: 65-80% quality range
  - Level 4: 40-65% quality range
  - Level 5: 20-40% quality range

#### WEBP Images
- **Level 1 (Lossless)**: Uses lossless WEBP encoding
- **Levels 2-5**: Uses lossy WEBP with quality percentages as above

## API Usage

### Backend (Go) API

The compression is exposed through a Go backend using Wails framework.

#### Request/Response Structures

**CompressRequest**:
- `input_path` (string): Path to input image file
- `output_path` (string): Path where compressed image will be saved
- `level` (int): Compression level (1-5)

**CompressResult**:
- `success` (bool): true if compression succeeded
- `input_path` (string): Original input path
- `output_path` (string): Output path where file was saved
- `original_size` (int64): Original file size in bytes
- `compressed_size` (int64): Compressed file size in bytes
- `compression_rate` (float64): Percentage saved (0-100)
- `compression_level` (int): Level used (1-5)
- `error` (string): Error message if success=false

#### Single and Batch Operations

The API supports both single image compression and batch processing of multiple images concurrently.

### Frontend (TypeScript/React) API

The frontend calls the compression API through Wails bindings, supporting both individual and batch compression operations.

### Python Script Interface

The backend communicates with a Python script via JSON stdin/stdout interface.

#### Input/Output JSON Format

Input expects a JSON object with `input_path`, `output_path`, and `level` fields. Output returns a JSON object with compression results or error information.

## Error Handling

### Common Error Scenarios

1. **File Not Found**: Input file does not exist
2. **Unsupported Format**: Image format not supported
3. **Permission Denied**: Cannot write to output path
4. **Invalid Compression Level**: Automatically clamped to valid range (1-5)

### Error Recovery

- Invalid levels are automatically corrected
- File I/O errors are caught and reported
- Library failures fall back to basic Pillow compression

## Performance Considerations

### Compression Speed vs Quality Trade-offs

| Level | Typical Speed | Quality Loss | Use Case |
|-------|---------------|--------------|----------|
| 1 (Lossless) | Slowest | None | Archival, medical images |
| 2 (Light) | Fast | Minimal | Web images, slight optimization |
| 3 (Medium) | Medium | Moderate | General web use, social media |
| 4 (Heavy) | Fast | High | Thumbnails, mobile apps |
| 5 (Extreme) | Fastest | Very High | Icons, very small previews |

### Memory Usage

- Images are loaded entirely into memory
- Large images (>100MB) may cause memory issues
- Automatic garbage collection after each compression

### Library Availability

The system gracefully degrades if libraries are not available, falling back to Pillow for basic compression.

## Dependencies

### Python Requirements

```txt
# Core image processing
Pillow>=10.0.0

# Advanced compression libraries
mozjpeg-lossless-optimization>=1.1.0  # JPEG lossless optimization
imagequant>=1.0.0                     # PNG lossy compression
pyoxipng>=1.0.0                       # PNG lossless optimization

# Other utilities
loguru>=0.7.0                         # Enhanced logging
typing-extensions>=4.0.0              # Type hints
```

## Testing

### Unit Tests

The compression functionality includes comprehensive tests covering all compression levels, supported formats, error conditions, and library availability fallbacks.

### Integration Tests

Tests verify end-to-end compression workflows and API integration.

## Limitations

1. **Large Images**: Very large images (>500MB) may cause memory issues
2. **Format Conversion**: Only optimizes within the same format (JPEG→JPEG, PNG→PNG)
3. **Metadata**: EXIF data is preserved but not optimized
4. **Animation**: GIF/WEBP animations are treated as single frames
5. **Color Profiles**: ICC profiles are maintained but may be stripped in lossless PNG optimization

## Future Enhancements

Potential improvements include progressive loading support, custom quality ranges, batch processing optimizations, GPU acceleration, and advanced metadata handling.

## Troubleshooting

### Common Issues

1. **Library Not Found**: Ensure all Python dependencies are installed
2. **Permission Errors**: Check write permissions on output directory
3. **Memory Errors**: Reduce image size or use lower quality levels
4. **Unsupported Format**: Check that input format is JPEG, PNG, or WEBP

### Debug Mode

Enable detailed logging to troubleshoot issues.

### Performance Monitoring

Monitor compression ratios and processing times for optimization.
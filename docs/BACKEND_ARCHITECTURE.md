# ImageFlow Backend Architecture

## Overview

ImageFlow backend is built using **Go** as the main application layer and **Python (Pillow)** as the image processing engine. This hybrid architecture leverages Go's excellent concurrency and performance for task scheduling, while Python's rich ecosystem (Pillow, reportlab) handles actual image processing.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Wails v3 Framework                     │
│                  (Go <-> Frontend Bridge)                   │
└─────────────────────────────────────────────────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────┐
│                    Go Application Layer                      │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              App (main.go, app.go)                   │  │
│  │  - Exposes methods to frontend via Wails             │  │
│  │  - Manages service lifecycle                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                           ↓                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                  Service Layer                        │  │
│  │  - ConverterService    (Format conversion)           │  │
│  │  - CompressorService   (Image compression)            │  │
│  │  - PDFGeneratorService (PDF generation)               │  │
│  │  - GIFSplitterService  (GIF splitting)              │  │
│  │  - InfoViewerService  (Image metadata)              │  │
│  │  - WatermarkService   (Watermark application)        │  │
│  │  - AdjusterService    (Image adjustments)            │  │
│  │  - FilterService      (Filter application)            │  │
│  └──────────────────────────────────────────────────────┘  │
│                           ↓                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                  Utility Layer                        │  │
│  │  - PythonExecutor (Go-Python communication)         │  │
│  │  - Logger (Logging infrastructure)                  │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                               ↓
                    Standard I/O (Pipes)
                               ↓
┌─────────────────────────────────────────────────────────────┐
│                 Python Processing Engine                    │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              CLI-based Scripts                        │  │
│  │  - converter.py      (Format conversion)            │  │
│  │  - compressor.py     (Image compression)            │  │
│  │  - pdf_generator.py (PDF generation)                │  │
│  │  - gif_splitter.py  (GIF splitting)                │  │
│  │  - info_viewer.py   (Image metadata)                │  │
│  │  - watermark.py     (Watermark application)          │  │
│  │  - adjuster.py      (Image adjustments)            │  │
│  │  - filter.py        (Filter application)            │  │
│  └──────────────────────────────────────────────────────┘  │
│                           ↓                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Python Libraries                        │  │
│  │  - Pillow      (Core image processing)               │  │
│  │  - reportlab   (PDF generation)                     │  │
│  │  - piexif     (EXIF manipulation)                  │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### 1. Go as Orchestrator, Python as Worker

**Why?**
- Go provides excellent concurrency (goroutines, channels) for batch processing
- Go's type system and compiled nature ensure reliability
- Python's Pillow library has decades of optimization (C/C++ cores)
- Avoids reinventing the wheel - Pillow handles all edge cases

**Trade-offs:**
- ✅ Fast development (use existing Python libraries)
- ✅ Proven reliability (Pillow is battle-tested)
- ✅ High concurrency (Go handles parallel tasks)
- ⚠️ Process overhead (Go → Python communication)
- ⚠️ Dependency on Python installation

### 2. JSON over Standard I/O

**Why?**
- Language-agnostic communication
- Easy debugging (can test Python scripts independently)
- No need for complex IPC mechanisms
- Minimal overhead for typical image processing workloads

**Data Flow:**
```
Go → JSON → stdin → Python script → stdout → JSON → Go
```

### 3. Service-Based Architecture

**Why?**
- Clear separation of concerns
- Easy to test individual services
- Simple to add new features
- Consistent error handling patterns

## Directory Structure

```
backend/
├── main.go              # Application entry point
├── app.go               # Main application struct and Wails bindings
├── go.mod               # Go module dependencies
├── models/
│   └── types.go         # Data structures for all operations
├── services/
│   ├── converter.go      # Format conversion service
│   ├── compressor.go     # Image compression service
│   ├── pdf_generator.go  # PDF generation service
│   ├── gif_splitter.go  # GIF splitting service
│   ├── info_viewer.go   # Image information service
│   ├── watermark.go      # Watermark service
│   ├── adjuster.go      # Image adjustment service
│   └── filter.go        # Filter service
└── utils/
    ├── python_executor.go # Python script execution
    └── logger.go        # Logging utilities

python/
├── converter.py         # Format conversion script
├── compressor.py        # Image compression script
├── pdf_generator.py     # PDF generation script
├── gif_splitter.py      # GIF splitting script
├── info_viewer.py       # Image information script
├── watermark.py         # Watermark script
├── adjuster.py         # Image adjustment script
├── filter.py           # Filter script
└── requirements.txt     # Python dependencies
```

## Communication Protocol

### Request Format (Go → Python)

All Python scripts accept JSON input via `stdin`:

```json
{
  "input_path": "/path/to/image.jpg",
  "output_path": "/path/to/output.jpg",
  "format": "png",
  "quality": 95,
  ...
}
```

### Response Format (Python → Go)

All Python scripts return JSON output via `stdout`:

```json
{
  "success": true,
  "input_path": "/path/to/image.jpg",
  "output_path": "/path/to/output.jpg",
  "file_size": 102400,
  ...
}
```

### Error Handling

Errors are returned with a descriptive message:

```json
{
  "success": false,
  "error": "File not found: /path/to/image.jpg"
}
```

## Concurrency Model

### Batch Processing

All services implement batch processing with goroutines:

```go
// Process images concurrently
for i, req := range requests {
    go func(idx int, r models.ConvertRequest) {
        result, err := s.Convert(r)
        resultChan <- result
    }(i, req)
}

// Collect results
for i := 0; i < len(requests); i++ {
    results[i] = <-resultChan
}
```

### Progress Updates

Batch operations send progress updates via channels:

```go
progressChan := make(chan *models.ProgressUpdate)
go func() {
    for progress := range progressChan {
        app.EmitEvent("convert_progress", progress)
    }
}()
```

## Performance Considerations

### Optimizations

1. **Concurrent Processing**: Goroutines handle parallel image operations
2. **Efficient JSON**: Compact JSON structure minimizes serialization overhead
3. **Python Optimization**: Pillow uses optimized C/C++ code
4. **Streaming**: Large files are handled efficiently by Pillow

### Bottlenecks

1. **Process Creation**: Each Python script execution creates a new process
   - **Mitigation**: Batch operations reuse processes when possible
   
2. **JSON Serialization**: Large images with extensive metadata
   - **Mitigation**: Compact histograms (normalized 0-100 range)

3. **Memory Usage**: Processing multiple large images
   - **Mitigation**: Limit concurrent operations, use streaming

## Security Considerations

### Input Validation

1. **File Path Validation**: Ensure paths are within allowed directories
2. **Format Validation**: Only process supported formats
3. **Parameter Clamping**: Validate all numeric parameters (e.g., quality 1-100)

### Python Execution

1. **Sandboxing**: Scripts run with restricted permissions
2. **Input Sanitization**: All inputs are parsed via JSON (not shell injection)
3. **Error Handling**: Graceful degradation on script failures

## Extending the Backend

### Adding a New Service

1. **Define Data Models** in `models/types.go`
2. **Create Service** in `services/new_feature.go`
3. **Implement Python Script** in `python/new_feature.py`
4. **Wire Up** in `app.go` (expose methods)
5. **Add Dependencies** if needed (Python: `requirements.txt`, Go: `go.mod`)

### Example: Adding a Resize Service

1. **Model** (`models/types.go`):
```go
type ResizeRequest struct {
    InputPath  string `json:"input_path"`
    OutputPath string `json:"output_path"`
    Width      int    `json:"width"`
    Height     int    `json:"height"`
    MaintainAR  bool   `json:"maintain_ar"`
}
```

2. **Service** (`services/resizer.go`):
```go
type ResizerService struct {
    executor *utils.PythonExecutor
    logger   *utils.Logger
}

func (s *ResizerService) Resize(req models.ResizeRequest) (models.ResizeResult, error) {
    // Implementation
}
```

3. **Python Script** (`python/resizer.py`):
```python
def resize(input_path, output_path, width, height, maintain_ar):
    img = Image.open(input_path)
    # Resize logic
    img.save(output_path)
```

4. **App Binding** (`app.go`):
```go
func (a *App) Resize(req models.ResizeRequest) (models.ResizeResult, error) {
    return a.resizerService.Resize(req)
}
```

## Testing

### Unit Tests

Each service should have unit tests for:
- Input validation
- Error handling
- Data transformation

### Integration Tests

Test the complete flow:
- Go → Python communication
- File I/O operations
- Batch processing

### Manual Testing

Use Python scripts directly:

```bash
# Test converter
echo '{"input_path": "test.jpg", "output_path": "test.png", "format": "png"}' | \
  python3 python/converter.py
```

## Logging

### Log Levels

- **DEBUG**: Detailed information for debugging
- **INFO**: General informational messages
- **WARN**: Warning messages
- **ERROR**: Error messages
- **FATAL**: Critical errors causing application termination

### Log Files

Logs are stored in the `logs/` directory with timestamps:
- `imageflow_20260113_153045.log`

### Log Format

```
[2026-01-13 15:30:45.123] [INFO] Starting image conversion: test.jpg -> test.png
[2026-01-13 15:30:45.456] [DEBUG] Resizing from 1920x1080 to 1280x720
[2026-01-13 15:30:45.789] [INFO] Conversion completed successfully
```

## Deployment

### Requirements

- Go 1.21+
- Python 3.11+
- Required Python packages (see `python/requirements.txt`)

### Building

```bash
# Build Go backend
cd backend
go build -o imageflow-backend

# Install Python dependencies
cd python
pip install -r requirements.txt
```

### Running

```bash
# Run backend (desktop mode)
./imageflow-backend

# Run backend (development server mode)
./imageflow-backend -server
```

## Troubleshooting

### Common Issues

1. **Python not found**
   - Ensure Python 3.11+ is in PATH
   - Or bundle Python with the application

2. **Script execution failed**
   - Check logs for detailed error messages
   - Verify Python dependencies are installed
   - Test script manually via command line

3. **Memory errors**
   - Reduce batch size
   - Process images sequentially instead of concurrently
   - Check for memory leaks in Python scripts

4. **Slow performance**
   - Verify Pillow is using optimized backends (libjpeg, etc.)
   - Reduce image sizes if processing very large images
   - Check system resources (CPU, memory, disk I/O)

## Future Improvements

### Completed (v1.1.0)

1. ✅ **Worker Pool**: Implemented controlled concurrency with max 10 workers
2. ✅ **Timeout Control**: Added 60-second timeout for Python script execution
3. ✅ **Input Validation**: Comprehensive validation for all inputs
4. ✅ **Python Version Check**: Automatic verification of Python 3.11+ requirement
5. ✅ **Non-blocking Progress**: Progress updates no longer block processing
6. ✅ **Graceful Shutdown**: Proper resource cleanup on application exit

See [IMPROVEMENTS.md](IMPROVEMENTS.md) for detailed information.

### Planned

1. **Caching**: Cache frequently processed images
2. **GPU Acceleration**: Use CUDA/OpenCL for supported operations
3. **Process Pool**: Reuse Python processes for batch operations (30-50% performance gain)
4. **Streaming API**: Support real-time streaming for large files
5. **Compression Pipeline**: Apply multiple operations in a single pass

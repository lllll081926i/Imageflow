package models

// ConvertRequest represents a request to convert an image format
type ConvertRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
	Quality    int    `json:"quality"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	MaintainAR bool   `json:"maintain_ar"`
}

// ConvertResult represents the result of an image conversion
type ConvertResult struct {
	Success    bool   `json:"success"`
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Error      string `json:"error,omitempty"`
}

// CompressRequest represents a request to compress an image
type CompressRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Mode       string `json:"mode"` // lossy, lossless, smart
	Quality    int    `json:"quality"`
}

// CompressResult represents the result of image compression
type CompressResult struct {
	Success         bool    `json:"success"`
	InputPath       string  `json:"input_path"`
	OutputPath      string  `json:"output_path"`
	OriginalSize    int64   `json:"original_size"`
	CompressedSize  int64   `json:"compressed_size"`
	CompressionRate float64 `json:"compression_rate"`
	Error           string  `json:"error,omitempty"`
}

// PDFRequest represents a request to generate a PDF from images
type PDFRequest struct {
	ImagePaths []string `json:"image_paths"`
	OutputPath string   `json:"output_path"`
	PageSize   string   `json:"page_size"`   // A4, Letter, etc.
	Layout     string   `json:"layout"`      // portrait, landscape
	Margin     int      `json:"margin"`      // in points
	Title      string   `json:"title"`
	Author     string   `json:"author"`
}

// PDFResult represents the result of PDF generation
type PDFResult struct {
	Success    bool   `json:"success"`
	OutputPath string `json:"output_path"`
	PageCount  int    `json:"page_count"`
	FileSize   int64  `json:"file_size"`
	Error      string `json:"error,omitempty"`
}

// GIFSplitRequest represents a request to split a GIF into frames
type GIFSplitRequest struct {
	InputPath  string `json:"input_path"`
	OutputDir  string `json:"output_dir"`
	StartFrame int    `json:"start_frame"`
	EndFrame   int    `json:"end_frame"`
	Format     string `json:"format"` // png, jpg, etc.
}

// GIFSplitResult represents the result of GIF splitting
type GIFSplitResult struct {
	Success    bool     `json:"success"`
	InputPath  string   `json:"input_path"`
	OutputDir  string   `json:"output_dir"`
	FrameCount int      `json:"frame_count"`
	FramePaths []string `json:"frame_paths"`
	Error      string   `json:"error,omitempty"`
}

// InfoRequest represents a request to get image information
type InfoRequest struct {
	InputPath string `json:"input_path"`
}

// InfoResult represents image information
type InfoResult struct {
	Success    bool              `json:"success"`
	InputPath  string            `json:"input_path"`
	Format     string            `json:"format"`
	Mode       string            `json:"mode"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	FileSize   int64             `json:"file_size"`
	EXIF       map[string]string `json:"exif,omitempty"`
	Histogram  map[string][]int  `json:"histogram,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// WatermarkRequest represents a request to add a watermark
type WatermarkRequest struct {
	InputPath    string  `json:"input_path"`
	OutputPath   string  `json:"output_path"`
	WatermarkType string `json:"watermark_type"` // text, image
	Text         string  `json:"text,omitempty"`
	ImagePath    string  `json:"image_path,omitempty"`
	Position     string  `json:"position"`     // center, top-left, top-right, etc.
	Opacity      float64 `json:"opacity"`      // 0.0 to 1.0
	Scale        float64 `json:"scale"`        // for image watermarks
	FontSize     int     `json:"font_size"`    // for text watermarks
	FontColor    string  `json:"font_color"`   // for text watermarks
	Rotation     int     `json:"rotation"`     // rotation angle
}

// WatermarkResult represents the result of watermark application
type WatermarkResult struct {
	Success    bool   `json:"success"`
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Error      string `json:"error,omitempty"`
}

// AdjustRequest represents a request to adjust image properties
type AdjustRequest struct {
	InputPath  string  `json:"input_path"`
	OutputPath string  `json:"output_path"`
	Rotate     int     `json:"rotate"`      // rotation angle
	FlipH      bool    `json:"flip_h"`      // flip horizontal
	FlipV      bool    `json:"flip_v"`      // flip vertical
	Brightness float64 `json:"brightness"`  // -1.0 to 1.0
	Contrast   float64 `json:"contrast"`    // -1.0 to 1.0
	Saturation float64 `json:"saturation"`  // -1.0 to 1.0
	Hue        float64 `json:"hue"`         // -180 to 180
}

// AdjustResult represents the result of image adjustment
type AdjustResult struct {
	Success    bool   `json:"success"`
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Error      string `json:"error,omitempty"`
}

// FilterRequest represents a request to apply a filter
type FilterRequest struct {
	InputPath  string  `json:"input_path"`
	OutputPath string  `json:"output_path"`
	FilterType string  `json:"filter_type"` // blur, sharpen, grayscale, sepia, etc.
	Intensity  float64 `json:"intensity"`   // 0.0 to 1.0
}

// FilterResult represents the result of filter application
type FilterResult struct {
	Success    bool   `json:"success"`
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Error      string `json:"error,omitempty"`
}

// ProgressUpdate represents a progress update for batch operations
type ProgressUpdate struct {
	Current    int     `json:"current"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
	Message    string  `json:"message"`
}

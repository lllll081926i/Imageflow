package engine

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// ResizeRequest is the resizer tool payload.
type ResizeRequest struct {
	InputPath    string  `json:"input_path"`
	OutputPath   string  `json:"output_path"`
	Mode         string  `json:"mode"`
	Value        float64 `json:"value"`
	OutputFormat string  `json:"output_format"`
	Quality      int     `json:"quality"`
}

// Resize scales one image by percent, long edge or short edge.
func Resize(req ResizeRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(filepath.Ext(req.OutputPath), "."))
	}
	if format == "" {
		format = "png"
	}
	target := ensureExtension(req.OutputPath, format)

	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	img := opened.Image

	value := req.Value
	if value <= 0 {
		value = 100
	}
	spec := ResizeSpec{Mode: strings.ToLower(req.Mode)}
	switch strings.ToLower(req.Mode) {
	case "percent", "":
		spec.ScalePercent = value
	case "long_edge", "longedge", "short_edge", "shortedge", "width", "height":
		if math.IsNaN(value) || math.IsInf(value, 0) || value > output.MaxOutputSide {
			return Fail("OUTPUT_TOO_LARGE", "缩放尺寸超出安全限制")
		}
		spec.LongEdge, spec.Width, spec.Height = int(value), int(value), int(value)
	default:
		return Fail("BAD_INPUT", "未知的缩放模式: "+req.Mode)
	}
	resized, err := applyResize(img, spec)
	if err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	if err := output.CheckImage(resized); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	if err := codec.Save(resized, target, codec.SaveOptions{Quality: quality, MetadataSource: req.InputPath}); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Width = resized.Bounds().Dx()
	out.Height = resized.Bounds().Dy()
	return withSizes(out, req.InputPath, target)
}

// GridRequest cuts one image into a square grid.
type GridRequest struct {
	InputPath    string `json:"input_path"`
	OutputDir    string `json:"output_dir"`
	GridSize     int    `json:"grid_size"`
	OutputFormat string `json:"output_format"`
	Quality      int    `json:"quality"`
	BaseName     string `json:"base_name"`
}

// CutGrid splits one image into gridSize x gridSize cells.
func CutGrid(req GridRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputDir) == "" {
		return Fail("BAD_INPUT", "缺少输入路径或输出目录")
	}
	size := req.GridSize
	if size < 1 {
		size = 3
	}
	if size > 20 {
		return Fail("BAD_INPUT", "宫格数量超出范围")
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = "png"
	}
	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	img := opened.Image
	return cutCells(img, req.OutputDir, req.BaseName, size, size, format, req.Quality, "grid", req.InputPath)
}

// SplitRequest cuts one image into an arbitrary rows x cols grid.
type SplitRequest struct {
	InputPath    string `json:"input_path"`
	OutputDir    string `json:"output_dir"`
	Rows         int    `json:"rows"`
	Cols         int    `json:"cols"`
	Mode         string `json:"mode"`
	OutputFormat string `json:"output_format"`
	Quality      int    `json:"quality"`
	BaseName     string `json:"base_name"`
}

// Split cuts one image into rows x cols pieces.
func Split(req SplitRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputDir) == "" {
		return Fail("BAD_INPUT", "缺少输入路径或输出目录")
	}
	rows, cols := req.Rows, req.Cols
	if rows < 1 {
		rows = 2
	}
	if cols < 1 {
		cols = 2
	}
	if rows > 50 || cols > 50 || rows*cols > 2500 {
		return Fail("BAD_INPUT", "行列数量超出范围")
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = "png"
	}
	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	img := opened.Image
	if strings.EqualFold(req.Mode, "square") {
		img = centerSquare(img)
	}
	return cutCells(img, req.OutputDir, req.BaseName, cols, rows, format, req.Quality, "split", req.InputPath)
}

func centerSquare(img image.Image) image.Image {
	b := img.Bounds()
	side := minInt(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	return imaging.Crop(img, image.Rect(x0, y0, x0+side, y0+side))
}

func cutCells(img image.Image, dir, baseName string, cols, rows int, format string, quality int, kind string, source ...string) Result {
	metadataSource := ""
	if len(source) > 0 {
		metadataSource = source[0]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	b := img.Bounds()
	cellW := b.Dx() / cols
	cellH := b.Dy() / rows
	if cellW < 1 || cellH < 1 {
		return Fail("BAD_INPUT", "图像尺寸小于切分数量")
	}
	if baseName == "" {
		baseName = strings.TrimSuffix(filepath.Base(dir), filepath.Ext(dir))
	}
	if baseName == "" || baseName == "." {
		baseName = kind
	}
	qualityValue := quality
	if qualityValue <= 0 {
		qualityValue = 95
	}
	var written []string
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			rect := image.Rect(
				b.Min.X+c*cellW,
				b.Min.Y+r*cellH,
				b.Min.X+(c+1)*cellW,
				b.Min.Y+(r+1)*cellH,
			)
			cell := imaging.Crop(img, rect)
			name := fmt.Sprintf("%s_%02d_%02d.%s", baseName, r+1, c+1, canonicalExt(format))
			target := filepath.Join(dir, name)
			target, err := uniquePath(target)
			if err != nil {
				return Fail("WRITE_FAILED", err.Error())
			}
			if err := codec.Save(cell, target, codec.SaveOptions{Quality: qualityValue, MetadataSource: metadataSource}); err != nil {
				return Fail("WRITE_FAILED", err.Error())
			}
			written = append(written, target)
		}
	}
	out := OK()
	out.OutputDir = dir
	out.Outputs = written
	out.Count = len(written)
	return out
}

// uniquePath appends _1, _2 … until the path does not exist.
func uniquePath(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return path, nil
	}
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", stem, i, ext)
		if _, err := os.Stat(candidate); err != nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("无法为 %s 找到可用文件名", path)
}

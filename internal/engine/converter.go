package engine

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// ConvertRequest mirrors the converter payload of the Python desktop API.
type ConvertRequest struct {
	InputPath    string  `json:"input_path"`
	OutputPath   string  `json:"output_path"`
	Format       string  `json:"format"`
	Quality      int     `json:"quality"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	MaintainAR   bool    `json:"maintain_ar"`
	ResizeMode   string  `json:"resize_mode"`
	ScalePercent float64 `json:"scale_percent"`
	LongEdge     int     `json:"long_edge"`
	KeepMetadata bool    `json:"keep_metadata"`
	IcoSizes     []int   `json:"ico_sizes"`
}

// Convert reads one image, applies the requested geometry and writes it out.
func Convert(req ConvertRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	format := normalizeFormat(req.Format)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(filepath.Ext(req.OutputPath), "."))
	}
	if format == "" {
		return Fail("BAD_INPUT", "未知的输出格式")
	}
	target := ensureExtension(req.OutputPath, format)

	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	img := opened.Image

	img, err = applyResize(img, ResizeSpec{
		Mode:         req.ResizeMode,
		Width:        req.Width,
		Height:       req.Height,
		MaintainAR:   req.MaintainAR,
		ScalePercent: req.ScalePercent,
		LongEdge:     req.LongEdge,
	})
	if err != nil {
		if errors.Is(err, output.ErrSizeExceeded) {
			return Fail("OUTPUT_TOO_LARGE", err.Error())
		}
		return Fail("RESIZE_FAILED", err.Error())
	}
	if err := output.CheckImage(img); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}

	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	if err := codec.Save(img, target, codec.SaveOptions{
		Quality:        quality,
		IcoSizes:       req.IcoSizes,
		KeepMetadata:   req.KeepMetadata,
		MetadataSource: req.InputPath,
	}); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Width = img.Bounds().Dx()
	out.Height = img.Bounds().Dy()
	return withSizes(out, req.InputPath, target)
}

// ResizeSpec is the geometry the converter, resizer and stitcher share.
type ResizeSpec struct {
	Mode         string
	Width        int
	Height       int
	MaintainAR   bool
	ScalePercent float64
	LongEdge     int
}

// applyResize returns a resized copy, or the original when no geometry applies.
func applyResize(img image.Image, spec ResizeSpec) (image.Image, error) {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	nw, nh := w, h
	box := func(maintain bool) {
		nw, nh = float64(spec.Width), float64(spec.Height)
		if !maintain {
			nw, nh = math.Max(1, nw), math.Max(1, nh)
			return
		}
		scale := math.Inf(1)
		if nw > 0 {
			scale = nw / w
		}
		if nh > 0 {
			scale = math.Min(scale, nh/h)
		}
		nw, nh = math.Max(1, math.Floor(w*scale)), math.Max(1, math.Floor(h*scale))
	}
	if math.IsNaN(spec.ScalePercent) || math.IsInf(spec.ScalePercent, 0) {
		return nil, fmt.Errorf("%w: 缩放比例无效", output.ErrSizeExceeded)
	}
	switch strings.ToLower(spec.Mode) {
	case "percent", "":
		if spec.ScalePercent > 0 && spec.ScalePercent != 100 {
			nw, nh = math.Max(1, math.Floor(w*spec.ScalePercent/100)), math.Max(1, math.Floor(h*spec.ScalePercent/100))
		} else if spec.Width > 0 || spec.Height > 0 {
			box(spec.MaintainAR)
		}
	case "width":
		if spec.Width > 0 {
			nw, nh = float64(spec.Width), math.Max(1, math.Floor(h*float64(spec.Width)/w+.5))
		}
	case "height":
		if spec.Height > 0 {
			nw, nh = math.Max(1, math.Floor(w*float64(spec.Height)/h+.5)), float64(spec.Height)
		}
	case "long_edge", "longedge":
		if spec.LongEdge > 0 {
			scale := float64(spec.LongEdge) / math.Max(w, h)
			nw, nh = math.Max(1, math.Floor(w*scale+.5)), math.Max(1, math.Floor(h*scale+.5))
		}
	case "short_edge", "shortedge":
		if spec.LongEdge > 0 {
			scale := float64(spec.LongEdge) / math.Min(w, h)
			nw, nh = math.Max(1, math.Floor(w*scale+.5)), math.Max(1, math.Floor(h*scale+.5))
		}
	case "fit":
		if spec.Width > 0 || spec.Height > 0 {
			box(true)
		}
	case "exact":
		if spec.Width > 0 && spec.Height > 0 {
			nw, nh = float64(spec.Width), float64(spec.Height)
		}
	}
	// 浮点阶段先检查，避免整数溢出及图像库提前分配超大画布。
	if nw > output.MaxOutputSide || nh > output.MaxOutputSide || nw*nh > output.MaxOutputPixels {
		return nil, fmt.Errorf("%w: %.0fx%.0f", output.ErrSizeExceeded, nw, nh)
	}
	if err := output.CheckSize(int(nw), int(nh)); err != nil {
		return nil, err
	}
	if int(nw) == b.Dx() && int(nh) == b.Dy() {
		return img, nil
	}
	return codec.ResizePixels(img, int(nw), int(nh)), nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ensureExtension forces the output path to carry the target format suffix.
//
// A source SVG named logo.svg already ends in .svg, which is not the encoder's
// suffix, so the path became logo_watermark.svg.png. The stem is trimmed before
// the suffix is appended, which keeps one extension on every path.
func ensureExtension(path, format string) string {
	want := "." + canonicalExt(format)
	have := strings.ToLower(filepath.Ext(path))
	if have == want {
		return path
	}
	if have == "" {
		return path + want
	}
	return strings.TrimSuffix(path, filepath.Ext(path)) + want
}

func canonicalExt(format string) string {
	switch format {
	case "jpeg":
		return "jpg"
	case "tif":
		return "tiff"
	}
	return format
}

func normalizeFormat(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "jpg", "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	case "avif":
		return "avif"
	case "tiff", "tif":
		return "tiff"
	case "bmp":
		return "bmp"
	case "ico":
		return "ico"
	}
	return ""
}

func withSizes(r Result, input, outputPath string) Result {
	if st, err := os.Stat(input); err == nil {
		r.InputSize = st.Size()
	}
	if st, err := os.Stat(outputPath); err == nil {
		r.OutputSize = st.Size()
	}
	return r
}

// QualityFormats names the formats whose encoder honours a quality setting.
var QualityFormats = map[string]bool{"jpg": true, "jpeg": true, "webp": true, "avif": true}

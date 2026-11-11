package engine

// preset.go generates one copy of an image per target size.
//
// The workflow it serves is "I need this photo at 1024×512, 256×128 and 64×16
// for the same page": one source, several boxes, each copy scaled to fit its box
// without distortion. A box's aspect ratio is a constraint on the canvas, not on
// the picture, so the result is letterboxed onto the requested background when
// the two ratios differ — that is what makes a 3:2 photo usable in a 8:1 banner
// without cropping the subject.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// PresetSize is one target box.
type PresetSize struct {
	Width  int
	Height int
}

// String is the label the UI and the file names use: "1024x512".
func (p PresetSize) String() string {
	return fmt.Sprintf("%dx%d", p.Width, p.Height)
}

// PresetRequest asks for one copy per size.
type PresetRequest struct {
	InputPath    string       `json:"input_path"`
	OutputDir    string       `json:"output_dir"`
	BaseName     string       `json:"base_name"`
	Sizes        []PresetSize `json:"sizes"`
	Mode         string       `json:"mode"` // contain | cover | stretch
	Background   string       `json:"background"`
	OutputFormat string       `json:"output_format"`
	Quality      int          `json:"quality"`
}

// ParsePresetSizes reads a list of boxes written as "1024x512, 256x128, 64x16".
//
// The separators people actually type are accepted: x, X, ×, * and , or ;. A
// bare number is read as a square. Duplicates are dropped, and the order the
// user wrote is kept, because that is the order the files appear in.
func ParsePresetSizes(text string) []PresetSize {
	out := make([]PresetSize, 0, 4)
	seen := map[PresetSize]bool{}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == ' '
	})
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		lower := strings.ToLower(field)
		lower = strings.ReplaceAll(lower, "×", "x")
		lower = strings.ReplaceAll(lower, "*", "x")
		parts := strings.Split(lower, "x")
		var size PresetSize
		if len(parts) == 1 {
			n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil || n <= 0 {
				continue
			}
			size = PresetSize{Width: n, Height: n}
		} else if len(parts) == 2 {
			w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
			h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
			if errW != nil || errH != nil || w <= 0 || h <= 0 {
				continue
			}
			size = PresetSize{Width: w, Height: h}
		} else {
			continue
		}
		if size.Width > output.MaxOutputSide || size.Height > output.MaxOutputSide {
			continue
		}
		if seen[size] {
			continue
		}
		seen[size] = true
		out = append(out, size)
	}
	return out
}

// ResizeToPresets writes one copy of the input per requested size.
func ResizeToPresets(req PresetRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputDir) == "" {
		return Fail("BAD_INPUT", "缺少输入路径或输出目录")
	}
	sizes := req.Sizes
	if len(sizes) == 0 {
		return Fail("PRESET_NO_SIZES", "没有可用的目标尺寸")
	}
	if len(sizes) > 12 {
		return Fail("PRESET_TOO_MANY", "一次最多生成 12 个尺寸")
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = "png"
	}
	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	base := strings.TrimSpace(req.BaseName)
	if base == "" {
		base = strings.TrimSuffix(filepath.Base(req.InputPath), filepath.Ext(req.InputPath))
	}
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "contain"
	}
	bg := hexToColor(req.Background, 255)
	if strings.TrimSpace(req.Background) == "" {
		bg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}

	src := opened.Image
	var written []string
	var lastW, lastH int
	for _, size := range sizes {
		if err := output.CheckSize(size.Width, size.Height); err != nil {
			return Fail("OUTPUT_TOO_LARGE", size.String()+": "+err.Error())
		}
		var canvas image.Image
		switch mode {
		case "stretch":
			canvas = imaging.Resize(src, size.Width, size.Height, imaging.Lanczos)
		case "cover":
			canvas = imaging.Fill(src, size.Width, size.Height, imaging.Center, imaging.Lanczos)
		default: // contain
			canvas = fitOnCanvas(src, size, bg)
		}
		name := fmt.Sprintf("%s_%s.%s", base, size.String(), canonicalExt(format))
		target := filepath.Join(req.OutputDir, name)
		target, err = uniquePath(target)
		if err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		if err := codec.Save(canvas, target, codec.SaveOptions{Quality: quality, MetadataSource: req.InputPath}); err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		written = append(written, target)
		lastW, lastH = size.Width, size.Height
	}
	out := OK()
	out.OutputDir = req.OutputDir
	out.Outputs = written
	out.Count = len(written)
	out.Width = lastW
	out.Height = lastH
	return out
}

// fitOnCanvas scales src to sit inside size, then centres it on a canvas of that
// exact size filled with bg. A source whose ratio already matches fills the box
// with no margin.
func fitOnCanvas(src image.Image, size PresetSize, bg color.NRGBA) image.Image {
	canvas := image.NewNRGBA(image.Rect(0, 0, size.Width, size.Height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	fitted := imaging.Fit(src, size.Width, size.Height, imaging.Lanczos)
	b := fitted.Bounds()
	dx := (size.Width - b.Dx()) / 2
	dy := (size.Height - b.Dy()) / 2
	draw.Draw(canvas, image.Rect(dx, dy, dx+b.Dx(), dy+b.Dy()), fitted, b.Min, draw.Over)
	return canvas
}

package engine

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// WatermarkRequest is the watermark tool payload.
type WatermarkRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Text       string `json:"text"`
	FontName   string `json:"font_name"`
	SizeMode   string `json:"size_mode"`
	FontSize   int    `json:"font_size"`
	Color      string `json:"color"`
	Opacity    int    `json:"opacity"`
	Rotation   int    `json:"rotation"`
	Position   string `json:"position"`
	Margin     int    `json:"margin"`
	Tiled      bool   `json:"tiled"`
	TileGapX   int    `json:"tile_gap_x"`
	TileGapY   int    `json:"tile_gap_y"`
	StrokeWide int    `json:"stroke_width"`
	StrokeColr string `json:"stroke_color"`
	Shadow     bool   `json:"shadow"`
	Quality    int    `json:"quality"`
	OutputForm string `json:"output_format"`
}

var watermarkPositions = map[string]bool{
	"tl": true, "tc": true, "tr": true,
	"cl": true, "c": true, "cr": true,
	"bl": true, "bc": true, "br": true,
}

// Watermark stamps a text watermark onto one image.
func Watermark(req WatermarkRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	if strings.TrimSpace(req.Text) == "" {
		return Fail("WM_NO_TEXT", "水印内容为空")
	}
	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	layer, result := renderWatermarkImage(opened.Image, req)
	if !result.Success {
		return result
	}
	format := normalizeFormat(req.OutputForm)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(filepath.Ext(req.OutputPath), "."))
	}
	if format == "" {
		format = "png"
	}
	target := ensureExtension(req.OutputPath, format)
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	if err := codec.Save(layer, target, codec.SaveOptions{Quality: quality, MetadataSource: req.InputPath}); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Width = layer.Bounds().Dx()
	out.Height = layer.Bounds().Dy()
	return withSizes(out, req.InputPath, target)
}

func RenderWatermarkImage(base image.Image, req WatermarkRequest) (image.Image, error) {
	img, result := renderWatermarkImage(base, req)
	if !result.Success {
		return nil, errors.New(result.Error)
	}
	return img, nil
}

func renderWatermarkImage(base image.Image, req WatermarkRequest) (image.Image, Result) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, Fail("WM_NO_TEXT", "水印内容为空")
	}
	position := strings.ToLower(strings.TrimSpace(req.Position))
	if !watermarkPositions[position] {
		position = "br"
	}

	if base == nil {
		return nil, Fail("BAD_INPUT", "缺少水印底图")
	}
	b := base.Bounds()
	if err := output.CheckImage(base); err != nil {
		return nil, Fail("OUTPUT_TOO_LARGE", err.Error())
	}

	size := req.FontSize
	if size <= 0 {
		size = 32
	}
	if strings.EqualFold(req.SizeMode, "percent") {
		size = maxInt(8, int(float64(b.Dx())*float64(req.FontSize)/100))
	}
	size = clampInt(size, 6, 512)

	opacity := req.Opacity
	if opacity <= 0 {
		opacity = 60
	}
	opacity = clampInt(opacity, 1, 100)
	alpha := uint8(float64(opacity) / 100 * 255)

	style := TextOptions{
		Family: req.FontName,
		Size:   size,
		Color:  hexToColor(req.Color, alpha),
		Stroke: clampInt(req.StrokeWide, 0, 20),
		StrokeCol: color.NRGBA{
			R: hexToColor(req.StrokeColr, 255).R,
			G: hexToColor(req.StrokeColr, 255).G,
			B: hexToColor(req.StrokeColr, 255).B,
			A: uint8(math.Min(255, float64(alpha)+60)),
		},
		Shadow:    req.Shadow,
		ShadowCol: color.NRGBA{R: 0, G: 0, B: 0, A: uint8(math.Min(160, float64(alpha)))},
	}

	layer := image.NewNRGBA(b)
	draw.Draw(layer, b, base, b.Min, draw.Src)

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	face, ferr := FontFace(req.FontName, size, 72, false)
	if ferr != nil {
		return nil, Fail("FONT_FAILED", ferr.Error())
	}
	lineHeight := MeasureHeight(face) * 1.15
	blockW := float32(0)
	for _, line := range lines {
		if w := MeasureText(line, face); w > blockW {
			blockW = w
		}
	}
	blockH := lineHeight * float32(len(lines))

	// A negative margin means "not set": the panel's slider offers 0 as flush to the
	// edge, so treating 0 as unset silently replaced the user's choice with half the
	// font size.
	margin := req.Margin
	if margin < 0 {
		margin = size / 2
	}

	// A single very long watermark line makes a wide stamp; reject it before the
	// allocation rather than letting the runtime try to allocate a huge image.
	if err := output.CheckSize(int(blockW)+style.Stroke*4+8, int(blockH)+style.Stroke*4+8); err != nil {
		return nil, Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	stamp := image.NewNRGBA(image.Rect(0, 0, int(blockW)+style.Stroke*4+8, int(blockH)+style.Stroke*4+8))
	for i, line := range lines {
		x := float32(style.Stroke*2 + 4)
		y := float32(style.Stroke*2+4) + float32(i)*lineHeight + float32(size)
		if _, err := DrawText(stamp, line, x, y, style); err != nil {
			return nil, Fail("FONT_FAILED", err.Error())
		}
	}

	rotation := float64(clampInt(req.Rotation, -180, 180))
	if rotation != 0 {
		stamp = imaging.Rotate(stamp, rotation, color.NRGBA{})
	}

	if req.Tiled {
		gapX := req.TileGapX
		if gapX <= 0 {
			gapX = int(stamp.Bounds().Dx()) + size
		}
		gapY := req.TileGapY
		if gapY <= 0 {
			gapY = int(stamp.Bounds().Dy()) + size
		}
		stepX := maxInt(16, gapX)
		stepY := maxInt(16, gapY)
		for y := -stamp.Bounds().Dy(); y < b.Dy()+stamp.Bounds().Dy(); y += stepY {
			offset := 0
			if ((y / stepY) % 2) != 0 {
				offset = stepX / 2
			}
			for x := -stamp.Bounds().Dx() + offset; x < b.Dx()+stamp.Bounds().Dx(); x += stepX {
				draw.Draw(layer, image.Rect(x, y, x+stamp.Bounds().Dx(), y+stamp.Bounds().Dy()), stamp, image.Point{}, draw.Over)
			}
		}
	} else {
		x, y := anchorFor(position, b, stamp.Bounds(), margin)
		draw.Draw(layer, image.Rect(x, y, x+stamp.Bounds().Dx(), y+stamp.Bounds().Dy()), stamp, image.Point{}, draw.Over)
	}

	return layer, OK()
}

// anchorFor places a stamp box inside the canvas for one of nine positions.
func anchorFor(position string, canvas image.Rectangle, stamp image.Rectangle, margin int) (int, int) {
	sw, sh := stamp.Dx(), stamp.Dy()
	cw, ch := canvas.Dx(), canvas.Dy()
	var x, y int
	switch position {
	case "tl":
		x, y = margin, margin
	case "tc":
		x, y = (cw-sw)/2, margin
	case "tr":
		x, y = cw-sw-margin, margin
	case "cl":
		x, y = margin, (ch-sh)/2
	case "c":
		x, y = (cw-sw)/2, (ch-sh)/2
	case "cr":
		x, y = cw-sw-margin, (ch-sh)/2
	case "bl":
		x, y = margin, ch-sh-margin
	case "bc":
		x, y = (cw-sw)/2, ch-sh-margin
	default: // br
		x, y = cw-sw-margin, ch-sh-margin
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x + canvas.Min.X, y + canvas.Min.Y
}

// PaletteRequest extracts a colour palette from one image.
type PaletteRequest struct {
	InputPath    string `json:"input_path"`
	OutputPath   string `json:"output_path"`
	SwatchHeight int    `json:"swatch_height"`
	IncludeText  bool   `json:"include_hex_text"`
	OutputFormat string `json:"output_format"`
	Colors       int    `json:"colors"`
}

// PaletteResult carries the extracted colours back to the UI.
type PaletteResult struct {
	ThemeColor string   `json:"theme_color"`
	Colors     []string `json:"colors"`
}

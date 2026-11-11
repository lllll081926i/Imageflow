package engine

// cropbox.go crops to a ratio without a drag rectangle.
//
// A batch tool cannot ask the user to draw a box on forty photos, so this one
// takes a ratio, an anchor and a size: it cuts the largest (or requested) box of
// that ratio at the chosen corner or centre, then optionally scales the result to
// exact pixels and rounds its corners. The ratio ladder is the one every product
// team asks for: 1:1, 4:3, 3:2, 16:9, 21:9 and their portrait turns.

import (
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// CropRatio is one aspect ratio written as "16:9".
type CropRatio struct {
	W int
	H int
}

// CropRatios is the ladder the panel offers, in the order it shows them.
var CropRatios = []CropRatio{
	{W: 1, H: 1}, {W: 4, H: 3}, {W: 3, H: 4}, {W: 3, H: 2},
	{W: 2, H: 3}, {W: 16, H: 9}, {W: 9, H: 16}, {W: 21, H: 9}, {W: 4, H: 5},
}

// CropAnchors names the nine places the box can sit inside the source.
var CropAnchors = []string{"tl", "tc", "tr", "cl", "c", "cr", "bl", "bc", "br"}

// String is the label the panel and the file names use: "16x9".
func (r CropRatio) String() string {
	return strconv.Itoa(r.W) + "x" + strconv.Itoa(r.H)
}

// Label is the human form: "16:9".
func (r CropRatio) Label() string {
	return strconv.Itoa(r.W) + ":" + strconv.Itoa(r.H)
}

// CropRequest is the crop tool payload.
type CropRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	RatioW     int    `json:"ratio_w"`
	RatioH     int    `json:"ratio_h"`
	Anchor     string `json:"anchor"`
	// ScaledWidth and ScaledHeight, when both are positive, resize the crop to
	// exactly that box; zero keeps the crop's own pixels.
	ScaledWidth  int `json:"scaled_width"`
	ScaledHeight int `json:"scaled_height"`
	// Radius rounds the result's corners, in percent of its short side.
	Radius       int    `json:"radius"`
	Background   string `json:"background"`
	OutputFormat string `json:"output_format"`
	Quality      int    `json:"quality"`
}

// CropBox cuts one image to a ratio and optionally scales it.
func CropBox(req CropRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	if req.RatioW <= 0 || req.RatioH <= 0 {
		return Fail("CROP_BAD_RATIO", "裁剪比例无效")
	}
	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	cropped := cropToRatio(opened.Image, req.RatioW, req.RatioH, req.Anchor)
	if req.ScaledWidth > 0 && req.ScaledHeight > 0 {
		if err := output.CheckSize(req.ScaledWidth, req.ScaledHeight); err != nil {
			return Fail("OUTPUT_TOO_LARGE", err.Error())
		}
		cropped = imaging.Resize(cropped, req.ScaledWidth, req.ScaledHeight, imaging.Lanczos)
	}
	if err := output.CheckImage(cropped); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	if radius := clampInt(req.Radius, 0, 50); radius > 0 {
		cropped = roundCorners(cropped, radius, req.Background)
	}

	format := normalizeFormat(req.OutputFormat)
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
	if err := codec.Save(cropped, target, codec.SaveOptions{Quality: quality, MetadataSource: req.InputPath}); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Width = cropped.Bounds().Dx()
	out.Height = cropped.Bounds().Dy()
	return withSizes(out, req.InputPath, target)
}

// cropToRatio returns the largest box of the given ratio that fits the source,
// placed at anchor.
func cropToRatio(src image.Image, rw, rh int, anchor string) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	boxW, boxH := w, h
	if w*rh > h*rw {
		// Source is wider than the ratio: the height is the limit.
		boxW = maxInt(1, h*rw/rh)
	} else {
		boxH = maxInt(1, w*rh/rw)
	}
	x0 := b.Min.X + (w-boxW)/2
	y0 := b.Min.Y + (h-boxH)/2
	switch strings.ToLower(strings.TrimSpace(anchor)) {
	case "tl":
		x0, y0 = b.Min.X, b.Min.Y
	case "tc":
		y0 = b.Min.Y
	case "tr":
		x0, y0 = b.Min.X+w-boxW, b.Min.Y
	case "cl":
		x0 = b.Min.X
	case "cr":
		x0 = b.Min.X + w - boxW
	case "bl":
		x0, y0 = b.Min.X, b.Min.Y+h-boxH
	case "bc":
		y0 = b.Min.Y + h - boxH
	case "br":
		x0, y0 = b.Min.X+w-boxW, b.Min.Y+h-boxH
	}
	return imaging.Crop(src, image.Rect(x0, y0, x0+boxW, y0+boxH))
}

// roundCorners masks the result to a rounded rectangle. A transparent
// background keeps the corners clear; a colour fills them.
//
// The mask is measured against the canvas that is actually written. The previous
// version tested a square of the short side and then mirrored it, so on a wide
// canvas the middle of the top edge (x beyond the square, y inside the corner
// band) was treated as a corner and cleared: a 400×225 crop masked the whole top
// and bottom edges instead of just the four corners.
func roundCorners(img image.Image, radiusPercent int, background string) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(canvas, canvas.Bounds(), img, b.Min, draw.Src)
	r := float64(minInt(w, h)) * float64(radiusPercent) / 100
	if r <= 0 {
		return canvas
	}
	bg := hexToColor(background, 255)
	transparent := strings.TrimSpace(background) == ""
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !outsideCorners(x, y, w, h, r) {
				continue
			}
			if transparent {
				canvas.SetNRGBA(x, y, color.NRGBA{})
			} else {
				canvas.SetNRGBA(x, y, bg)
			}
		}
	}
	return canvas
}

// outsideCorners reports whether a pixel lies beyond the rounded rectangle that
// fills a w×h canvas with corner radius r. All four corners are tested with the
// real canvas size, which is what makes a long edge stay filled.
func outsideCorners(x, y, w, h int, r float64) bool {
	cx, cy := float64(x)+0.5, float64(y)+0.5
	lx, ly := float64(w)-r, float64(h)-r
	var ox, oy float64
	switch {
	case cx < r && cy < r:
		ox, oy = cx-r, cy-r
	case cx > lx && cy < r:
		ox, oy = cx-lx, cy-r
	case cx < r && cy > ly:
		ox, oy = cx-r, cy-ly
	case cx > lx && cy > ly:
		ox, oy = cx-lx, cy-ly
	default:
		return false
	}
	return ox*ox+oy*oy > r*r
}

// ParseCropRatio reads "16:9", "16x9" or "16/9".
func ParseCropRatio(text string) (CropRatio, bool) {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ':' || r == 'x' || r == 'X' || r == '×' || r == '/' || r == '*'
	})
	if len(fields) != 2 {
		return CropRatio{}, false
	}
	w, errW := strconv.Atoi(strings.TrimSpace(fields[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(fields[1]))
	if errW != nil || errH != nil || w <= 0 || h <= 0 || w > 999 || h > 999 {
		return CropRatio{}, false
	}
	return CropRatio{W: w, H: h}, true
}

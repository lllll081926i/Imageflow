package engine

// edit.go is the basic-editing engine behind the 基础编辑 page.
//
// The page is deliberately small: rotation, flip, canvas, border, corners, shadow,
// opacity and the two basic tone tools. Professional colour lives in the 专业调色
// page and annotation lives in 图片标注, which is what keeps this one simple enough
// to stay predictable.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// EditRequest is one basic-edit render.
type EditRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`

	// Rotation is a right angle: 0, 90, 180 or 270.
	Rotation int  `json:"rotation"`
	FlipH    bool `json:"flip_h"`
	FlipV    bool `json:"flip_v"`
	// Angle is an arbitrary rotation in degrees. AngleMode decides what happens to
	// the corners it exposes.
	Angle     float64 `json:"angle"`
	AngleMode string  `json:"angle_mode"`

	// CanvasW and CanvasH resize the canvas. Zero keeps the current size on that
	// axis. Anchor places the picture inside the new canvas.
	CanvasW int    `json:"canvas_w"`
	CanvasH int    `json:"canvas_h"`
	Anchor  string `json:"anchor"`
	// Background fills the added canvas and any transparent corner. Empty keeps
	// transparency, which only a format with alpha can carry.
	Background string `json:"background"`

	Border      int    `json:"border"`
	BorderColor string `json:"border_color"`
	// Radius rounds the corners, in percent of the short side.
	Radius int `json:"radius"`
	// Shadow spreads a soft shadow under the picture.
	Shadow bool `json:"shadow"`
	// Opacity is 0..100.
	Opacity int `json:"opacity"`

	Blur      int     `json:"blur"`
	Sharpen   float64 `json:"sharpen"`
	Grayscale bool    `json:"grayscale"`
	Invert    bool    `json:"invert"`

	OutputFormat string `json:"output_format"`
	Quality      int    `json:"quality"`
}

// BasicEdit applies the request to one image.
func BasicEdit(req EditRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	opened, err := codec.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	img, err := RenderEdit(opened.Image, req)
	if err != nil {
		return Fail("EDIT_FAILED", err.Error())
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(extOfPath(req.OutputPath), "."))
	}
	if format == "" {
		format = "png"
	}
	target := ensureExtension(req.OutputPath, format)
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	opts := codec.SaveOptions{Quality: quality, MetadataSource: req.InputPath}
	if format == "jpg" || format == "jpeg" {
		// A JPEG cannot carry the transparent corners a rounded edit produces, so
		// the requested background is what they get.
		if strings.TrimSpace(req.Background) != "" {
			opts.SRGB = true
		}
	}
	if err := codec.Save(img, target, opts); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	res := OK()
	res.OutputPath = target
	res.Width = img.Bounds().Dx()
	res.Height = img.Bounds().Dy()
	return withSizes(res, req.InputPath, target)
}

// RenderEdit applies the request and returns the picture, for the preview.
func RenderEdit(src image.Image, req EditRequest) (image.Image, error) {
	if src == nil {
		return nil, fmt.Errorf("缺少输入图像")
	}
	img := src
	// Right-angle rotation and flip first: they are lossless and everything after
	// works on the already-oriented picture.
	switch ((req.Rotation % 360) + 360) % 360 {
	case 90:
		img = imaging.Rotate90(img)
	case 180:
		img = imaging.Rotate180(img)
	case 270:
		img = imaging.Rotate270(img)
	}
	if req.FlipH {
		img = imaging.FlipH(img)
	}
	if req.FlipV {
		img = imaging.FlipV(img)
	}
	if req.Angle != 0 {
		angle := math.Mod(req.Angle, 360)
		if strings.EqualFold(req.AngleMode, "crop") {
			img = rotateAndCrop(img, angle)
		} else {
			img = imaging.Rotate(img, angle, color.NRGBA{})
		}
	}

	// Canvas resize, with the picture placed at the anchor.
	b := img.Bounds()
	cw, ch := b.Dx(), b.Dy()
	if req.CanvasW > 0 {
		cw = req.CanvasW
	}
	if req.CanvasH > 0 {
		ch = req.CanvasH
	}
	if cw != b.Dx() || ch != b.Dy() {
		if err := output.CheckSize(cw, ch); err != nil {
			return nil, err
		}
		canvas := image.NewNRGBA(image.Rect(0, 0, cw, ch))
		bg := markColor(req.Background)
		if strings.TrimSpace(req.Background) == "" {
			bg = color.NRGBA{}
		}
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		ox, oy := anchorOffset(req.Anchor, cw, ch, b.Dx(), b.Dy())
		draw.Draw(canvas, image.Rect(ox, oy, ox+b.Dx(), oy+b.Dy()), img, b.Min, draw.Over)
		img = canvas
	}

	if border := clampInt(req.Border, 0, 400); border > 0 {
		img = addBorder(img, border, markColor(req.BorderColor))
	}
	if radius := clampInt(req.Radius, 0, 50); radius > 0 {
		img = roundCorners(img, radius, req.Background)
	}
	if req.Shadow {
		img = addShadow(img, maxInt(6, img.Bounds().Dy()/40))
	}
	if opacity := clampInt(req.Opacity, 0, 100); opacity < 100 {
		img = applyOpacity(img, float64(opacity)/100)
	}
	if blur := clampInt(req.Blur, 0, 64); blur > 0 {
		img = imaging.Blur(img, float64(blur))
	}
	if req.Sharpen > 0 {
		img = ApplyFilter(img, FilterRequest{Kind: "sharpen", Strength: 100, Amount: clampF(req.Sharpen, 0, 100)})
	}
	if req.Grayscale {
		img = ApplyFilter(img, FilterRequest{Kind: "grayscale", Strength: 100})
	}
	if req.Invert {
		img = ApplyFilter(img, FilterRequest{Kind: "invert", Strength: 100})
	}
	if err := output.CheckImage(img); err != nil {
		return nil, err
	}
	return img, nil
}

// anchorOffset places a w×h picture inside a cw×ch canvas.
func anchorOffset(anchor string, cw, ch, w, h int) (int, int) {
	x, y := (cw-w)/2, (ch-h)/2
	switch strings.ToLower(strings.TrimSpace(anchor)) {
	case "tl":
		x, y = 0, 0
	case "tc":
		y = 0
	case "tr":
		x, y = cw-w, 0
	case "cl":
		x = 0
	case "cr":
		x = cw - w
	case "bl":
		x, y = 0, ch-h
	case "bc":
		y = ch - h
	case "br":
		x, y = cw-w, ch-h
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// rotateAndCrop rotates and then removes the transparent corners the rotation
// exposed, which is the "crop away the empty area" option.
func rotateAndCrop(src image.Image, angle float64) image.Image {
	rotated := imaging.Rotate(src, angle, color.NRGBA{})
	nrgba := toNRGBA(rotated)
	b := nrgba.Bounds()
	minX, minY := b.Dx(), b.Dy()
	maxX, maxY := 0, 0
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if nrgba.NRGBAAt(x, y).A != 0 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if minX >= maxX || minY >= maxY {
		return rotated
	}
	return imaging.Crop(rotated, image.Rect(minX, minY, maxX+1, maxY+1))
}

// addBorder grows the canvas by the border width and fills the ring.
func addBorder(src image.Image, width int, col color.NRGBA) image.Image {
	b := src.Bounds()
	w, h := b.Dx()+width*2, b.Dy()+width*2
	if err := output.CheckSize(w, h); err != nil {
		return src
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), &image.Uniform{col}, image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(width, width, width+b.Dx(), width+b.Dy()), src, b.Min, draw.Over)
	return out
}

// addShadow pads the picture with room for a blurred silhouette underneath it.
func addShadow(src image.Image, spread int) image.Image {
	b := src.Bounds()
	w, h := b.Dx()+spread*2, b.Dy()+spread*2
	if err := output.CheckSize(w, h); err != nil {
		return src
	}
	mask := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(mask, mask.Bounds(), src, b.Min, draw.Src)
	shadow := boxBlur(mask, maxInt(2, spread/2))
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Offset down and right, the way a light from the top-left throws it.
			sx, sy := x-spread-spread/3, y-spread-spread/3
			if sx < 0 || sy < 0 || sx >= b.Dx() || sy >= b.Dy() {
				continue
			}
			a := shadow.NRGBAAt(sx, sy).A
			if a == 0 {
				continue
			}
			out.SetNRGBA(x, y, color.NRGBA{A: uint8(float64(a) * 0.35)})
		}
	}
	draw.Draw(out, image.Rect(spread, spread, spread+b.Dx(), spread+b.Dy()), src, b.Min, draw.Over)
	return out
}

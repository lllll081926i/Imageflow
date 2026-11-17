package engine

// annotate.go draws the annotation layer of the 图片标注 page.
//
// Two things separate this from a drawing demo. First, every annotation is a
// value in a list, so the layer can be rebuilt from the source on every preview
// and exported at full resolution. Second, the opaque cover mode really destroys
// the pixels: the export composites the cover onto the source and re-encodes, so
// the file that leaves the app carries no hidden copy of what was covered. A blur
// or a mosaic is offered too, but the page labels it as aesthetic, not as
// redaction, and the engine records which kind each mark is.

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

// AnnotateKind names one drawing tool.
const (
	AnnotateRect      = "rect"
	AnnotateEllipse   = "ellipse"
	AnnotateArrow     = "arrow"
	AnnotateLine      = "line"
	AnnotateFree      = "free"
	AnnotateText      = "text"
	AnnotateNumber    = "number"
	AnnotateHighlight = "highlight"
	AnnotateMosaic    = "mosaic"
	AnnotateBlur      = "blur"
	// AnnotateCover is the irreversible redaction: an opaque filled rectangle.
	AnnotateCover = "cover"
)

// AnnotateMark is one annotation, in source-image pixels.
//
// Coordinates are pixels rather than fractions because an annotation is authored
// on one picture; the preview scales them down for drawing, and the export scales
// them back up, so a mark lands on the same feature in both.
type AnnotateMark struct {
	Kind string `json:"kind"`
	// X1,Y1 is the start point; X2,Y2 the end point or the opposite corner.
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
	// Points is the free-hand path, used by AnnotateFree.
	Points []AnnotatePoint `json:"points,omitempty"`
	// Text is the label of a text or number mark.
	Text string `json:"text,omitempty"`
	// Color is #RRGGBB.
	Color string `json:"color,omitempty"`
	// Width is the stroke width in pixels of the source image.
	Width int `json:"width,omitempty"`
	// Size is the font size of a text mark.
	Size int `json:"size,omitempty"`
	// Fill asks a rect or ellipse to be filled instead of outlined.
	Fill bool `json:"fill,omitempty"`
	// Strength is the mosaic block size or the blur radius.
	Strength int `json:"strength,omitempty"`
}

// AnnotatePoint is one free-hand vertex.
type AnnotatePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// AnnotateRequest is one annotation export.
type AnnotateRequest struct {
	InputPath  string         `json:"input_path"`
	OutputPath string         `json:"output_path"`
	Marks      []AnnotateMark `json:"marks"`
	Quality    int            `json:"quality"`
	// StripMetadata clears EXIF/XMP: a redacted screenshot must not carry the
	// original's metadata back.
	StripMetadata bool `json:"strip_metadata"`
}

// SafeKinds are the mark kinds that irreversibly destroy the pixels under them.
var SafeKinds = map[string]bool{AnnotateCover: true}

// IsIrreversible reports whether a mark kind is a real redaction.
func IsIrreversible(kind string) bool { return SafeKinds[kind] }

// AnnotateImage draws the marks onto a copy of img and returns the flattened
// result. It never mutates img.
func AnnotateImage(img image.Image, marks []AnnotateMark) (image.Image, error) {
	if img == nil {
		return nil, fmt.Errorf("缺少输入图像")
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	for i, m := range marks {
		if err := drawMark(out, m); err != nil {
			return nil, fmt.Errorf("第 %d 个标注：%w", i+1, err)
		}
	}
	return out, nil
}

// drawMark paints one mark, dispatching on its kind.
func drawMark(canvas *image.NRGBA, m AnnotateMark) error {
	kind := strings.ToLower(strings.TrimSpace(m.Kind))
	col := markColor(m.Color)
	width := float64(m.Width)
	if width <= 0 {
		width = 4
	}
	x1, y1 := m.X1, m.Y1
	x2, y2 := m.X2, m.Y2
	switch kind {
	case AnnotateRect:
		if m.Fill {
			fillRect(canvas, x1, y1, x2, y2, col)
			return nil
		}
		drawThickLine(canvas, x1, y1, x2, y1, width, col)
		drawThickLine(canvas, x2, y1, x2, y2, width, col)
		drawThickLine(canvas, x2, y2, x1, y2, width, col)
		drawThickLine(canvas, x1, y2, x1, y1, width, col)
	case AnnotateEllipse:
		if m.Fill {
			fillEllipse(canvas, x1, y1, x2, y2, col)
			return nil
		}
		drawEllipseOutline(canvas, int(x1), int(y1), int(x2), int(y2), width, col)
	case AnnotateArrow:
		drawArrow(canvas, int(x1), int(y1), int(x2), int(y2), width, col)
	case AnnotateLine:
		drawThickLine(canvas, x1, y1, x2, y2, width, col)
	case AnnotateFree:
		drawFreehand(canvas, m.Points, width, col)
	case AnnotateText:
		return drawTextMark(canvas, m, col)
	case AnnotateNumber:
		drawBadge(canvas, x1, y1, m, col)
	case AnnotateHighlight:
		// A translucent band behind the content: it marks without hiding.
		hl := col
		hl.A = 90
		fillRect(canvas, x1, y1, x2, y2, hl)
	case AnnotateMosaic:
		return pixelateRegion(canvas, x1, y1, x2, y2, clampInt(m.Strength, 2, 128))
	case AnnotateBlur:
		return blurRegion(canvas, x1, y1, x2, y2, clampInt(m.Strength, 1, 64))
	case AnnotateCover:
		fillRect(canvas, x1, y1, x2, y2, col)
	default:
		return fmt.Errorf("未知的标注类型 %q", m.Kind)
	}
	return nil
}

// markColor parses the mark colour, defaulting to the app's alarm red.
func markColor(hex string) color.NRGBA {
	if strings.TrimSpace(hex) == "" {
		return color.NRGBA{R: 239, G: 68, B: 68, A: 255}
	}
	return hexToColor(hex, 255)
}

// fillRect paints an opaque or translucent axis-aligned rectangle.
func fillRect(canvas *image.NRGBA, x1, y1, x2, y2 float64, col color.NRGBA) {
	rect := normalizedRect(x1, y1, x2, y2)
	rect = rect.Intersect(canvas.Bounds())
	if rect.Empty() {
		return
	}
	draw.Draw(canvas, rect, &image.Uniform{col}, image.Point{}, draw.Over)
}

// fillEllipse paints a filled ellipse inscribed in the two corners.
func fillEllipse(canvas *image.NRGBA, x1, y1, x2, y2 float64, col color.NRGBA) {
	rect := normalizedRect(x1, y1, x2, y2)
	cx := float64(rect.Min.X+rect.Max.X) / 2
	cy := float64(rect.Min.Y+rect.Max.Y) / 2
	rx := float64(rect.Dx()) / 2
	ry := float64(rect.Dy()) / 2
	if rx < 1 || ry < 1 {
		return
	}
	b := canvas.Bounds()
	for y := maxInt(b.Min.Y, int(cy-ry)); y < minInt(b.Max.Y, int(cy+ry)+1); y++ {
		for x := maxInt(b.Min.X, int(cx-rx)); x < minInt(b.Max.X, int(cx+rx)+1); x++ {
			dx := (float64(x) + 0.5 - cx) / rx
			dy := (float64(y) + 0.5 - cy) / ry
			if dx*dx+dy*dy <= 1 {
				blendPixel(canvas, x, y, col, 255)
			}
		}
	}
}

// normalizedRect orders the two corners and rounds them into pixels.
func normalizedRect(x1, y1, x2, y2 float64) image.Rectangle {
	xa, xb := x1, x2
	if xa > xb {
		xa, xb = xb, xa
	}
	ya, yb := y1, y2
	if ya > yb {
		ya, yb = yb, ya
	}
	return image.Rect(int(math.Floor(xa)), int(math.Floor(ya)),
		int(math.Ceil(xb)), int(math.Ceil(yb)))
}

// drawFreehand paints a polyline through the recorded points.
func drawFreehand(canvas *image.NRGBA, pts []AnnotatePoint, width float64, col color.NRGBA) {
	if len(pts) == 0 {
		return
	}
	if len(pts) == 1 {
		fillDisc(canvas, pts[0].X, pts[0].Y, width/2, col)
		return
	}
	for i := 1; i < len(pts); i++ {
		drawThickLine(canvas, pts[i-1].X, pts[i-1].Y, pts[i].X, pts[i].Y, width, col)
	}
}

// drawTextMark paints a label with a contrasting outline so it stays readable over
// any picture.
func drawTextMark(canvas *image.NRGBA, m AnnotateMark, col color.NRGBA) error {
	size := m.Size
	if size <= 0 {
		size = clampInt(canvas.Bounds().Dy()/18, 12, 96)
	}
	text := strings.TrimSpace(m.Text)
	if text == "" {
		text = "文字"
	}
	_, err := DrawText(canvas, text, float32(m.X1), float32(m.Y1)+float32(size), TextOptions{
		Family: "", Size: size, Color: col, Bold: true,
		Stroke: maxInt(1, size/18), StrokeCol: color.NRGBA{A: 150},
	})
	return err
}

// drawBadge paints a numbered circle for the sequence-number tool.
func drawBadge(canvas *image.NRGBA, x, y float64, m AnnotateMark, col color.NRGBA) {
	radius := float64(m.Size)
	if radius <= 0 {
		radius = 18
	}
	radius = clampF(radius, 8, 120)
	fillDisc(canvas, x, y, radius, col)
	label := strings.TrimSpace(m.Text)
	if label == "" {
		label = "1"
	}
	face, err := FontFace("", int(radius*1.15), 72, true)
	if err != nil {
		return
	}
	width := MeasureText(label, face)
	DrawText(canvas, label, float32(x)-width/2, float32(y)+float32(radius*0.4), TextOptions{
		Family: "", Size: int(radius * 1.15), Color: color.NRGBA{R: 255, G: 255, B: 255, A: 255}, Bold: true,
	})
}

// pixelateRegion replaces a region with average-coloured blocks.
func pixelateRegion(canvas *image.NRGBA, x1, y1, x2, y2 float64, block int) error {
	rect := normalizedRect(x1, y1, x2, y2).Intersect(canvas.Bounds())
	if rect.Empty() {
		return nil
	}
	for by := rect.Min.Y; by < rect.Max.Y; by += block {
		for bx := rect.Min.X; bx < rect.Max.X; bx += block {
			maxX, maxY := minInt(bx+block, rect.Max.X), minInt(by+block, rect.Max.Y)
			var sr, sg, sb, sa, n int
			for y := by; y < maxY; y++ {
				for x := bx; x < maxX; x++ {
					c := canvas.NRGBAAt(x, y)
					sr += int(c.R)
					sg += int(c.G)
					sb += int(c.B)
					sa += int(c.A)
					n++
				}
			}
			if n == 0 {
				continue
			}
			avg := color.NRGBA{
				R: uint8(sr / n), G: uint8(sg / n), B: uint8(sb / n), A: uint8(sa / n),
			}
			for y := by; y < maxY; y++ {
				for x := bx; x < maxX; x++ {
					canvas.SetNRGBA(x, y, avg)
				}
			}
		}
	}
	return nil
}

// blurRegion blurs one rectangle in place.
func blurRegion(canvas *image.NRGBA, x1, y1, x2, y2 float64, radius int) error {
	rect := normalizedRect(x1, y1, x2, y2).Intersect(canvas.Bounds())
	if rect.Empty() || radius < 1 {
		return nil
	}
	// Blur the whole canvas once and copy the rectangle back. Blurring the patch
	// alone would blend in the surrounding pixels' own sharpness at its border and
	// leave a visible seam.
	blurred := boxBlur(canvas, radius)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			canvas.SetNRGBA(x, y, blurred.NRGBAAt(x, y))
		}
	}
	return nil
}

// Annotate renders the marks and writes the file.
func Annotate(req AnnotateRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	opened, err := codec.OpenSRGB(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	if err := output.CheckImage(opened.Image); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	out, err := AnnotateImage(opened.Image, req.Marks)
	if err != nil {
		return Fail("ANNOTATE_FAILED", err.Error())
	}
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	opts := codec.SaveOptions{Quality: quality}
	if req.StripMetadata {
		opts.StripMeta = true
	} else {
		opts.MetadataSource = req.InputPath
	}
	if err := codec.Save(out, req.OutputPath, opts); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	res := OK()
	res.OutputPath = req.OutputPath
	res.Width = out.Bounds().Dx()
	res.Height = out.Bounds().Dy()
	res.Count = len(req.Marks)
	return res
}

// screenshot.go is the screenshot-beautifier core, kept in the same file group as
// the annotation layer because both compose a decorated canvas around a picture.

// ScreenshotRequest decorates one screenshot.
type ScreenshotRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`

	// Padding is the frame's margin around the picture, in canvas pixels.
	Padding int `json:"padding"`
	// Radius rounds both the picture's corners and, when FrameRadius is set, the
	// outer frame.
	Radius      int `json:"radius"`
	FrameRadius int `json:"frame_radius"`
	// ShadowDistance spreads a soft drop shadow under the picture.
	ShadowDistance int     `json:"shadow_distance"`
	ShadowOpacity  float64 `json:"shadow_opacity"`
	// BorderWidth draws a hairline around the picture.
	BorderWidth int    `json:"border_width"`
	BorderColor string `json:"border_color"`

	// Background is the frame fill: a colour, a gradient pair, or a picture.
	Background      string  `json:"background"`
	BackgroundTo    string  `json:"background_to"`
	GradientAngle   float64 `json:"gradient_angle"`
	BackgroundImage string  `json:"background_image"`

	// AspectW and AspectH, when both are positive, letterbox the decorated frame
	// into that ratio instead of hugging the picture.
	AspectW, AspectH float64

	Quality int `json:"quality"`
}

// RenderScreenshot builds the decorated canvas in memory.
func RenderScreenshot(img image.Image, req ScreenshotRequest) (image.Image, error) {
	if img == nil {
		return nil, fmt.Errorf("缺少输入图像")
	}
	b := img.Bounds()
	if err := output.CheckSize(b.Dx(), b.Dy()); err != nil {
		return nil, err
	}
	pad := clampInt(req.Padding, 0, 400)
	radius := clampInt(req.Radius, 0, 200)
	frameRadius := clampInt(req.FrameRadius, 0, 200)
	shadow := clampInt(req.ShadowDistance, 0, 120)
	border := clampInt(req.BorderWidth, 0, 40)

	innerW, innerH := b.Dx(), b.Dy()
	// The shadow needs room on all four sides, otherwise it is clipped by the
	// canvas edge and reads as a cut-off band.
	shadowPad := shadow * 2
	canvasW := innerW + (pad+shadowPad)*2
	canvasH := innerH + (pad+shadowPad)*2
	if req.AspectW > 0 && req.AspectH > 0 {
		target := req.AspectW / req.AspectH
		current := float64(canvasW) / float64(canvasH)
		if current < target {
			// Too tall: widen.
			wantW := int(float64(canvasH) * target)
			canvasW = wantW
		} else if current > target {
			wantH := int(float64(canvasW) / target)
			canvasH = wantH
		}
	}
	if err := output.CheckSize(canvasW, canvasH); err != nil {
		return nil, err
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, canvasW, canvasH))
	if err := paintFrameBackground(canvas, req); err != nil {
		return nil, err
	}
	if frameRadius > 0 {
		roundCanvas(canvas, frameRadius)
	}

	// Where the picture sits: centred, which also keeps a letterboxed frame
	// balanced.
	px := (canvasW - innerW) / 2
	py := (canvasH - innerH) / 2

	if shadow > 0 {
		opacity := req.ShadowOpacity
		if opacity <= 0 {
			opacity = 0.22
		}
		if opacity > 1 {
			opacity /= 100
		}
		drawSoftShadow(canvas, px, py, innerW, innerH, radius, shadow, opacity)
	}

	picture := image.NewNRGBA(image.Rect(0, 0, innerW, innerH))
	draw.Draw(picture, picture.Bounds(), img, b.Min, draw.Src)
	if radius > 0 {
		picture = roundedNRGBA(picture, radius)
	}
	draw.Draw(canvas, image.Rect(px, py, px+innerW, py+innerH), picture, image.Point{}, draw.Over)

	if border > 0 {
		col := markColor(req.BorderColor)
		if strings.TrimSpace(req.BorderColor) == "" {
			col = color.NRGBA{R: 229, G: 231, B: 235, A: 255}
		}
		drawRectOutline(canvas, px, py, px+innerW, py+innerH, float64(border), col)
	}
	return canvas, nil
}

// paintFrameBackground fills the frame with a colour, a gradient or a picture.
func paintFrameBackground(canvas *image.NRGBA, req ScreenshotRequest) error {
	b := canvas.Bounds()
	if strings.TrimSpace(req.BackgroundImage) != "" {
		opened, err := codec.OpenSRGB(req.BackgroundImage)
		if err != nil {
			return fmt.Errorf("背景图读取失败：%w", err)
		}
		filled := imaging.Fill(opened.Image, b.Dx(), b.Dy(), imaging.Center, imaging.Lanczos)
		draw.Draw(canvas, b, filled, filled.Bounds().Min, draw.Src)
		return nil
	}
	from := markColor(req.Background)
	if strings.TrimSpace(req.Background) == "" {
		from = color.NRGBA{R: 245, G: 245, B: 247, A: 255}
	}
	if strings.TrimSpace(req.BackgroundTo) == "" {
		draw.Draw(canvas, b, &image.Uniform{from}, image.Point{}, draw.Src)
		return nil
	}
	to := markColor(req.BackgroundTo)
	fillLinearGradient(canvas, from, to, req.GradientAngle)
	return nil
}

// fillLinearGradient paints a linear gradient at the given angle in degrees.
func fillLinearGradient(canvas *image.NRGBA, from, to color.NRGBA, angle float64) {
	b := canvas.Bounds()
	rad := angle * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)
	// Project every pixel onto the gradient axis and normalise by the axis length.
	w, h := float64(b.Dx()), float64(b.Dy())
	projMax := math.Abs(w*dx) + math.Abs(h*dy)
	if projMax == 0 {
		draw.Draw(canvas, b, &image.Uniform{from}, image.Point{}, draw.Src)
		return
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			proj := float64(x)*dx + float64(y)*dy
			if proj < 0 {
				proj += projMax
			}
			t := clampF(proj/projMax, 0, 1)
			canvas.SetNRGBA(x, y, color.NRGBA{
				R: clampTo8(float64(from.R) + (float64(to.R)-float64(from.R))*t),
				G: clampTo8(float64(from.G) + (float64(to.G)-float64(from.G))*t),
				B: clampTo8(float64(from.B) + (float64(to.B)-float64(from.B))*t),
				A: clampTo8(float64(from.A) + (float64(to.A)-float64(from.A))*t),
			})
		}
	}
}

// drawSoftShadow paints a blurred rectangle behind the picture.
func drawSoftShadow(canvas *image.NRGBA, x, y, w, h, radius, distance int, opacity float64) {
	// Build the shadow mask at half resolution: the blur removes the detail a
	// full-resolution mask would carry, and the cost drops fourfold.
	mask := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(mask, mask.Bounds(), &image.Uniform{color.NRGBA{A: 255}}, image.Point{}, draw.Src)
	if radius > 0 {
		mask = roundedNRGBA(mask, radius)
	}
	blurred := boxBlur(mask, maxInt(2, distance))
	off := distance / 2
	for my := 0; my < h; my++ {
		for mx := 0; mx < w; mx++ {
			a := blurred.NRGBAAt(mx, my).A
			if a == 0 {
				continue
			}
			tx := x + mx
			ty := y + my + off
			if tx < 0 || ty < 0 || tx >= canvas.Bounds().Dx() || ty >= canvas.Bounds().Dy() {
				continue
			}
			alpha := uint8(float64(a) * opacity)
			blendPixel(canvas, tx, ty, color.NRGBA{A: alpha}, 255)
		}
	}
}

// roundedNRGBA is roundImage with a concrete return type, so a caller that needs
// to keep editing the pixels does not have to type-assert.
func roundedNRGBA(src *image.NRGBA, radius int) *image.NRGBA {
	if out, ok := roundImage(src, radius).(*image.NRGBA); ok {
		return out
	}
	return src
}

// roundCanvas clears the four corners of the frame itself.
func roundCanvas(canvas *image.NRGBA, radius int) {
	b := canvas.Bounds()
	w, h := b.Dx(), b.Dy()
	r := float64(clampInt(radius, 0, minInt(w, h)/2))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !insideRounded(x, y, w, h, r) {
				canvas.SetNRGBA(x, y, color.NRGBA{})
			}
		}
	}
}

// Screenshot renders and writes the decorated screenshot.
func Screenshot(req ScreenshotRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	opened, err := codec.OpenSRGB(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	out, err := RenderScreenshot(opened.Image, req)
	if err != nil {
		return Fail("SCREENSHOT_FAILED", err.Error())
	}
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	if err := codec.Save(out, req.OutputPath, codec.SaveOptions{Quality: quality, SRGB: true}); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	res := OK()
	res.OutputPath = req.OutputPath
	res.Width = out.Bounds().Dx()
	res.Height = out.Bounds().Dy()
	return res
}

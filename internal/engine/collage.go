package engine

// collage.go renders a collage: several pictures laid out on one canvas by a
// template, then decorated with labels, a tiled watermark and a background.
//
// The layout itself lives in internal/collage, which the panel also reads, so
// the preview and the exported file are the same geometry.

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/collage"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// CollageFit is how a picture fills its cell.
const (
	CollageCover   = "cover"
	CollageContain = "contain"
	CollageStretch = "stretch"
)

// CollageMark is one annotation drawn over the finished collage.
//
// Coordinates are fractions of the canvas, so a mark keeps its place when the
// long edge changes: an arrow drawn over the preview still points at the same
// faces after a 4K export.
type CollageMark struct {
	// Kind is text, arrow, rect or ellipse.
	Kind string `json:"kind"`
	// Text is the label of a text mark.
	Text string `json:"text,omitempty"`
	// X1,Y1 to X2,Y2 is the line, the box or the ellipse's bounds in canvas
	// fractions. A text mark is anchored at X1,Y1.
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
	// Color is the stroke colour as #RRGGBB.
	Color string `json:"color,omitempty"`
	// Width is the stroke width in pixels of a 1600 px canvas, scaled with it.
	Width int `json:"width,omitempty"`
}

// CollageCell is one picture placed in a layout slot.
type CollageCell struct {
	Path string `json:"path"`
	// Scale multiplies the cover fit: 1 keeps the default crop, 2 doubles it and
	// shows half the picture. Only used with cover/stretch.
	Scale float64 `json:"scale,omitempty"`
	// OffsetX and OffsetY move the picture inside its cell, as fractions of the
	// cell's size. 0,0 centres it; the useful range is -1..1.
	OffsetX float64 `json:"offset_x,omitempty"`
	OffsetY float64 `json:"offset_y,omitempty"`
	// FlipH mirrors the picture left to right, FlipV top to bottom. Both are
	// applied before the cell is filled, so a mirrored picture still crops and
	// pans the same way.
	FlipH bool `json:"flip_h,omitempty"`
	FlipV bool `json:"flip_v,omitempty"`
	// Fit overrides the request's fit for this one cell.
	Fit string `json:"fit,omitempty"`
}

// CollageWatermark is the tiled label drawn over the whole collage.
type CollageWatermark struct {
	Enabled bool    `json:"enabled"`
	Text    string  `json:"text"`
	Size    int     `json:"size"`
	Opacity float64 `json:"opacity"`
	Angle   float64 `json:"angle"`
	// Density is 0 sparse, 1 medium, 2 dense.
	Density int `json:"density"`
}

// CollageRequest is the collage tool payload.
type CollageRequest struct {
	// Cells holds one entry per slot, in template order. A slot without a path
	// stays empty, which is how a short batch still produces a full template.
	Cells []CollageCell `json:"cells"`
	// Rects is the template geometry in canvas fractions. Empty falls back to a
	// single full-cell canvas, so a caller cannot produce an empty picture.
	Rects []collage.Rect `json:"rects"`

	OutputPath string `json:"output_path"`
	// AspectW and AspectH describe the canvas ratio, e.g. 16 and 9.
	AspectW float64 `json:"aspect_w"`
	AspectH float64 `json:"aspect_h"`
	// LongEdge is the longer side of the canvas in pixels.
	LongEdge int `json:"long_edge"`

	// Margins are the canvas padding, in pixels of the output canvas.
	MarginTop    int `json:"margin_top"`
	MarginRight  int `json:"margin_right"`
	MarginBottom int `json:"margin_bottom"`
	MarginLeft   int `json:"margin_left"`
	// Gap separates neighbouring cells.
	Gap int `json:"gap"`
	// Radius rounds each cell, in pixels.
	Radius int `json:"radius"`

	// Background is the canvas colour, #RRGGBB or #RRGGBBAA.
	Background string `json:"background"`
	// BackgroundPath fills the canvas with a picture instead of the colour.
	// BackgroundDim darkens it, 0..100.
	BackgroundPath string  `json:"background_path"`
	BackgroundDim  float64 `json:"background_dim"`

	Fit     string        `json:"fit"`
	Quality int           `json:"quality"`
	Format  string        `json:"format"`
	Marks   []CollageMark `json:"marks,omitempty"`
	// Watermark is the tiled label.
	Watermark CollageWatermark `json:"watermark"`
}

// Collage renders the collage and writes it to OutputPath.
func Collage(req CollageRequest) Result {
	return CollageContext(context.Background(), req)
}

// CollageContext renders the collage, stopping between cells and before the
// write when the batch is cancelled.
func CollageContext(ctx context.Context, req CollageRequest) Result {
	img, res := RenderCollage(ctx, req)
	if !res.Success {
		return res
	}
	target := strings.TrimSpace(req.OutputPath)
	if target == "" {
		return Fail("BAD_INPUT", "缺少输出路径")
	}
	format := normalizeFormat(req.Format)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(filepath.Ext(target), "."))
	}
	if format == "" {
		format = "png"
	}
	target = ensureExtension(target, format)
	quality := req.Quality
	if quality <= 0 {
		quality = 92
	}
	if err := codec.SaveContext(ctx, img, target, codec.SaveOptions{Quality: quality, SRGB: true}); err != nil {
		if ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		return Fail("WRITE_FAILED", err.Error())
	}
	res.OutputPath = target
	if st, err := os.Stat(target); err == nil {
		res.OutputSize = st.Size()
	}
	return res
}

// RenderCollage composes the canvas in memory. The panel calls it for the
// interactive preview, so it must not touch the filesystem beyond reading the
// inputs.
func RenderCollage(ctx context.Context, req CollageRequest) (image.Image, Result) {
	if ctx.Err() != nil {
		return nil, Fail("CANCELLED", "已取消")
	}
	rects := req.Rects
	if len(rects) == 0 {
		rects = []collage.Rect{{X: 0, Y: 0, W: 1, H: 1}}
	}
	if len(rects) > 64 {
		return nil, Fail("BAD_INPUT", "布局块数量超出范围")
	}
	// An empty collage would write a blank picture: every slot is background.
	filled := 0
	for _, cell := range req.Cells {
		if strings.TrimSpace(cell.Path) != "" {
			filled++
		}
	}
	if filled == 0 {
		return nil, Fail("COLLAGE_NO_INPUT", "没有可拼合的图片")
	}

	aspectW, aspectH := req.AspectW, req.AspectH
	if aspectW <= 0 || aspectH <= 0 {
		aspectW, aspectH = 1, 1
	}
	longEdge := req.LongEdge
	if longEdge <= 0 {
		longEdge = 1600
	}
	cw, ch := collage.CanvasSize(aspectW, aspectH, longEdge)
	if err := output.CheckSize(cw, ch); err != nil {
		return nil, Fail("OUTPUT_TOO_LARGE", err.Error())
	}

	margins := collage.Margins{
		Top:    clampInt(req.MarginTop, 0, output.MaxOutputSide),
		Right:  clampInt(req.MarginRight, 0, output.MaxOutputSide),
		Bottom: clampInt(req.MarginBottom, 0, output.MaxOutputSide),
		Left:   clampInt(req.MarginLeft, 0, output.MaxOutputSide),
	}
	// Margins that do not leave room for a picture are clipped rather than
	// allowed to produce a degenerate canvas.
	margins = clampMargins(margins, cw, ch)

	gap := clampInt(req.Gap, 0, 400)
	radius := clampInt(req.Radius, 0, 200)
	px := collage.Layout(rects, cw, ch, margins, gap)

	canvas := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	if err := paintBackground(canvas, req.Background, req.BackgroundPath, req.BackgroundDim); err != nil {
		return nil, Fail("BACKGROUND_FAILED", err.Error())
	}

	fit := strings.ToLower(strings.TrimSpace(req.Fit))
	if fit != CollageContain && fit != CollageStretch {
		fit = CollageCover
	}

	for i, cell := range px {
		if ctx.Err() != nil {
			return nil, Fail("CANCELLED", "已取消")
		}
		var slot CollageCell
		if i < len(req.Cells) {
			slot = req.Cells[i]
		}
		path := strings.TrimSpace(slot.Path)
		if path == "" {
			continue
		}
		opened, err := codec.OpenSRGB(path)
		if err != nil {
			return nil, Fail("READ_FAILED", fmt.Sprintf("第 %d 格：%v", i+1, err))
		}
		cellFit := fit
		if f := strings.ToLower(strings.TrimSpace(slot.Fit)); f == CollageCover || f == CollageContain || f == CollageStretch {
			cellFit = f
		}
		placed := placePicture(opened.Image, cell.W, cell.H, cellFit, slot)
		if radius > 0 {
			placed = roundImage(placed, radius)
		}
		draw.Draw(canvas, image.Rect(cell.X, cell.Y, cell.Right(), cell.Bottom()),
			placed, image.Point{}, draw.Over)
	}

	if err := paintMarks(canvas, req.Marks, 1600); err != nil {
		return nil, Fail("MARKS_FAILED", err.Error())
	}
	if req.Watermark.Enabled {
		if err := paintWatermark(canvas, req.Watermark); err != nil {
			return nil, Fail("WATERMARK_FAILED", err.Error())
		}
	}

	res := OK()
	res.Width = cw
	res.Height = ch
	res.Count = len(req.Cells)
	res.Message = fmt.Sprintf("%d 格", len(px))
	return canvas, res
}

// clampMargins keeps at least a quarter of each axis for the pictures.
func clampMargins(m collage.Margins, w, h int) collage.Margins {
	maxX := w / 2
	maxY := h / 2
	if m.Left+m.Right >= w {
		if m.Left > maxX/2 {
			m.Left = maxX / 2
		}
		m.Right = w - m.Left - 1
		if m.Right < 0 {
			m.Right = 0
		}
	}
	if m.Top+m.Bottom >= h {
		if m.Top > maxY/2 {
			m.Top = maxY / 2
		}
		m.Bottom = h - m.Top - 1
		if m.Bottom < 0 {
			m.Bottom = 0
		}
	}
	return m
}

// placePicture scales one picture into its cell.
func placePicture(src image.Image, w, h int, fit string, slot CollageCell) image.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	// A mirror is applied to the source before any fitting: flipping after the
	// crop would move the visible part of the picture instead of mirroring it.
	if slot.FlipH || slot.FlipV {
		b := src.Bounds()
		flipped := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(flipped, flipped.Bounds(), src, b.Min, draw.Src)
		if slot.FlipH {
			flipped = imaging.FlipH(flipped)
		}
		if slot.FlipV {
			flipped = imaging.FlipV(flipped)
		}
		src = flipped
	}
	scale := slot.Scale
	if scale <= 0 {
		scale = 1
	}
	if scale > 8 {
		scale = 8
	}
	offX := clampFloat(slot.OffsetX, -1, 1)
	offY := clampFloat(slot.OffsetY, -1, 1)
	switch fit {
	case CollageContain:
		// Contain keeps the whole picture; the cell is filled with transparency
		// and the background shows through the letterbox.
		inner := imaging.Fit(src, w, h, imaging.Lanczos)
		canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
		dx := (w - inner.Bounds().Dx()) / 2
		dy := (h - inner.Bounds().Dy()) / 2
		draw.Draw(canvas, image.Rect(dx, dy, dx+inner.Bounds().Dx(), dy+inner.Bounds().Dy()),
			inner, inner.Bounds().Min, draw.Over)
		return canvas
	}
	// Cover: fill the cell and crop the excess, then pan by the offset.
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	ratioCell := float64(w) / float64(h)
	ratioSrc := float64(sw) / float64(sh)
	var cropW, cropH int
	if fit == CollageStretch {
		cropW, cropH = sw, sh
	} else if ratioSrc > ratioCell {
		cropH = sh
		cropW = int(float64(sh) * ratioCell)
	} else {
		cropW = sw
		cropH = int(float64(sw) / ratioCell)
	}
	cropW = int(float64(cropW) / scale)
	cropH = int(float64(cropH) / scale)
	cropW = clampInt(cropW, 1, sw)
	cropH = clampInt(cropH, 1, sh)
	// The offset is a fraction of the room the crop can travel.
	freeX := sw - cropW
	freeY := sh - cropH
	x0 := (freeX / 2) + int(offX*float64(freeX)/2)
	y0 := (freeY / 2) + int(offY*float64(freeY)/2)
	x0 = clampInt(x0, 0, freeX)
	y0 = clampInt(y0, 0, freeY)
	cropped := imaging.Crop(src, image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x0+cropW, b.Min.Y+y0+cropH))
	return imaging.Resize(cropped, w, h, imaging.Lanczos)
}

// roundImage masks the four corners of a cell.
func roundImage(src image.Image, radius int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	r := float64(clampInt(radius, 0, minInt(w, h)/2))
	if r <= 0 {
		return src
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !insideRounded(x, y, w, h, r) {
				out.SetNRGBA(x, y, color.NRGBA{})
			}
		}
	}
	return out
}

// insideRounded reports whether a pixel lies inside a rounded rectangle.
func insideRounded(x, y, w, h int, r float64) bool {
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
		return true
	}
	return ox*ox+oy*oy <= r*r
}

// paintBackground fills the canvas with a colour or a picture.
func paintBackground(canvas *image.NRGBA, hex, path string, dim float64) error {
	base := hexToColor(hex, 255)
	if strings.TrimSpace(hex) == "" {
		base = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{base}, image.Point{}, draw.Src)
	if strings.TrimSpace(path) == "" {
		return nil
	}
	opened, err := codec.OpenSRGB(path)
	if err != nil {
		return fmt.Errorf("背景图读取失败：%w", err)
	}
	b := canvas.Bounds()
	// Cover the canvas, then darken if asked: a background picture must never
	// letterbox, or the colour behind it would show as bars.
	filled := imaging.Fill(opened.Image, b.Dx(), b.Dy(), imaging.Center, imaging.Lanczos)
	mask := clampFloat(dim, 0, 100) / 100
	if mask > 0 {
		ink := image.NewUniform(color.NRGBA{R: 0, G: 0, B: 0, A: uint8(mask * 255)})
		draw.Draw(canvas, b, filled, b.Min, draw.Src)
		draw.Draw(canvas, b, ink, image.Point{}, draw.Over)
		return nil
	}
	draw.Draw(canvas, b, filled, b.Min, draw.Src)
	return nil
}

// paintMarks draws the annotations over the finished collage.
//
// refWidth is the canvas width the widths were authored against: a 3 px line on
// a 1600 px canvas stays 3 px relative to the picture at 4000 px.
func paintMarks(canvas *image.NRGBA, marks []CollageMark, refWidth int) error {
	if len(marks) == 0 {
		return nil
	}
	b := canvas.Bounds()
	k := float64(b.Dx()) / float64(refWidth)
	if k <= 0 {
		k = 1
	}
	for i, m := range marks {
		kind := strings.ToLower(strings.TrimSpace(m.Kind))
		col := hexToColor(m.Color, 255)
		if strings.TrimSpace(m.Color) == "" {
			col = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
		}
		width := float64(m.Width)
		if width <= 0 {
			width = 4
		}
		width *= k
		x1 := int(float64(b.Dx()) * m.X1)
		y1 := int(float64(b.Dy()) * m.Y1)
		x2 := int(float64(b.Dx()) * m.X2)
		y2 := int(float64(b.Dy()) * m.Y2)
		switch kind {
		case "text":
			text := strings.TrimSpace(m.Text)
			if text == "" {
				text = "文字"
			}
			size := int(float64(48) * k)
			if m.Width > 0 {
				size = int(float64(m.Width) * 6 * k)
			}
			size = clampInt(size, 8, 400)
			opts := TextOptions{Family: "", Size: size, Color: col, Bold: true,
				Stroke: maxInt(1, int(2*k)), StrokeCol: color.NRGBA{R: 0, G: 0, B: 0, A: 120}}
			// The mark's Y is the text's top; DrawText wants the baseline.
			if _, err := DrawText(canvas, text, float32(x1), float32(y1)+float32(size), opts); err != nil {
				return fmt.Errorf("第 %d 个标注：%w", i+1, err)
			}
		case "arrow":
			drawArrow(canvas, x1, y1, x2, y2, width, col)
		case "rect", "box":
			drawRectOutline(canvas, x1, y1, x2, y2, width, col)
		case "ellipse", "circle":
			drawEllipseOutline(canvas, x1, y1, x2, y2, width, col)
		default:
			// An unknown kind draws nothing: a typo must not abort an export the
			// user is waiting for.
			continue
		}
	}
	return nil
}

// drawArrow paints a line with a solid triangular head.
func drawArrow(canvas *image.NRGBA, x1, y1, x2, y2 int, width float64, col color.NRGBA) {
	drawThickLine(canvas, float64(x1), float64(y1), float64(x2), float64(y2), width, col)
	angle := math.Atan2(float64(y2-y1), float64(x2-x1))
	head := math.Max(width*4, 12)
	l := angle + math.Pi*0.85
	r := angle - math.Pi*0.85
	ax := float64(x2) + head*math.Cos(l)
	ay := float64(y2) + head*math.Sin(l)
	bx := float64(x2) + head*math.Cos(r)
	by := float64(y2) + head*math.Sin(r)
	fillTriangle(canvas, float64(x2), float64(y2), ax, ay, bx, by, col)
}

// drawThickLine paints a line as a run of discs, which keeps the ends round and
// the width even without a line rasteriser.
func drawThickLine(canvas *image.NRGBA, x1, y1, x2, y2, width float64, col color.NRGBA) {
	steps := int(math.Max(math.Abs(x2-x1), math.Abs(y2-y1)))
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		fillDisc(canvas, x1+(x2-x1)*t, y1+(y2-y1)*t, width/2, col)
	}
}

// fillDisc paints a filled circle of radius r at cx,cy.
func fillDisc(canvas *image.NRGBA, cx, cy, r float64, col color.NRGBA) {
	x0, y0 := int(cx-r)-1, int(cy-r)-1
	x1, y1 := int(cx+r)+1, int(cy+r)+1
	b := canvas.Bounds()
	for y := y0; y <= y1; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := x0; x <= x1; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d > r {
				continue
			}
			coverage := uint8(255)
			if d > r-1 {
				coverage = uint8(255 * (r - d))
			}
			blendPixel(canvas, x, y, col, coverage)
		}
	}
}

// drawRectOutline paints an axis-aligned outline of a rectangle.
func drawRectOutline(canvas *image.NRGBA, x1, y1, x2, y2 int, width float64, col color.NRGBA) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	drawThickLine(canvas, float64(x1), float64(y1), float64(x2), float64(y1), width, col)
	drawThickLine(canvas, float64(x2), float64(y1), float64(x2), float64(y2), width, col)
	drawThickLine(canvas, float64(x2), float64(y2), float64(x1), float64(y2), width, col)
	drawThickLine(canvas, float64(x1), float64(y2), float64(x1), float64(y1), width, col)
}

// drawEllipseOutline paints an ellipse inscribed in the two corners.
func drawEllipseOutline(canvas *image.NRGBA, x1, y1, x2, y2 int, width float64, col color.NRGBA) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	cx := float64(x1+x2) / 2
	cy := float64(y1+y2) / 2
	rx := math.Abs(float64(x2-x1)) / 2
	ry := math.Abs(float64(y2-y1)) / 2
	if rx < 1 || ry < 1 {
		return
	}
	steps := int(math.Max(rx, ry) * 6)
	if steps < 32 {
		steps = 32
	}
	prevX := cx + rx
	prevY := cy
	for i := 1; i <= steps; i++ {
		a := 2 * math.Pi * float64(i) / float64(steps)
		nx := cx + rx*math.Cos(a)
		ny := cy + ry*math.Sin(a)
		drawThickLine(canvas, prevX, prevY, nx, ny, width, col)
		prevX, prevY = nx, ny
	}
}

// fillTriangle fills a triangle with a scanline sweep. The sampler steps two
// rows at a time on a large canvas: the annotation layer is a marker, and half
// the rows is invisible against the picture while the cost drops by half.
func fillTriangle(canvas *image.NRGBA, ax, ay, bx, by, cx, cy float64, col color.NRGBA) {
	minY := int(math.Floor(math.Min(ay, math.Min(by, cy))))
	maxY := int(math.Ceil(math.Max(ay, math.Max(by, cy))))
	b := canvas.Bounds()
	line := 2
	if b.Dy() < 1200 {
		line = 1
	}
	for y := minY; y <= maxY; y += line {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		sy := float64(y) + 0.5
		var xs []float64
		edges := [3][4]float64{{ax, ay, bx, by}, {bx, by, cx, cy}, {cx, cy, ax, ay}}
		for _, e := range edges {
			x1, y1, x2, y2 := e[0], e[1], e[2], e[3]
			if (y1 <= sy && y2 > sy) || (y2 <= sy && y1 > sy) {
				t := (sy - y1) / (y2 - y1)
				xs = append(xs, x1+t*(x2-x1))
			}
		}
		if len(xs) < 2 {
			continue
		}
		lo, hi := xs[0], xs[0]
		for _, v := range xs[1:] {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		for step := 0; step < line; step++ {
			py := y + step
			if py >= b.Max.Y {
				break
			}
			for x := int(math.Round(lo)); x <= int(math.Round(hi)); x++ {
				if x < b.Min.X || x >= b.Max.X {
					continue
				}
				blendPixel(canvas, x, py, col, 255)
			}
		}
	}
}

// blendPixel mixes one colour into the canvas with an extra coverage.
func blendPixel(canvas *image.NRGBA, x, y int, col color.NRGBA, coverage uint8) {
	a := uint32(col.A) * uint32(coverage) / 255
	if a == 0 {
		return
	}
	dst := canvas.NRGBAAt(x, y)
	if dst.A == 0 {
		canvas.SetNRGBA(x, y, color.NRGBA{R: col.R, G: col.G, B: col.B, A: uint8(a)})
		return
	}
	// Straight-alpha source-over, matching how the rest of the engine composes.
	sa := float64(a) / 255
	da := float64(dst.A) / 255
	outA := sa + da*(1-sa)
	if outA <= 0 {
		return
	}
	mix := func(s, d uint8) uint8 {
		v := (float64(s)*sa + float64(d)*da*(1-sa)) / outA
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint8(v + 0.5)
	}
	canvas.SetNRGBA(x, y, color.NRGBA{
		R: mix(col.R, dst.R), G: mix(col.G, dst.G), B: mix(col.B, dst.B),
		A: uint8(outA*255 + 0.5),
	})
}

// paintWatermark tiles a text label across the whole collage.
func paintWatermark(canvas *image.NRGBA, wm CollageWatermark) error {
	text := strings.TrimSpace(wm.Text)
	if text == "" {
		text = "ImageFlow"
	}
	size := wm.Size
	if size <= 0 {
		size = 28
	}
	// The size is authored against a 1600 px canvas so the label keeps its
	// relative weight at every export size.
	b := canvas.Bounds()
	scale := float64(b.Dx()) / 1600
	if scale <= 0 {
		scale = 1
	}
	size = clampInt(int(float64(size)*scale), 8, 400)
	opacity := wm.Opacity
	if opacity <= 0 {
		opacity = 0.25
	}
	opacity = clampFloat(opacity, 0.02, 1)
	alpha := uint8(opacity * 255)

	face, err := FontFace("", size, 72, false)
	if err != nil {
		return err
	}
	col := color.NRGBA{R: 255, G: 255, B: 255, A: alpha}
	tw := int(MeasureText(text, face))
	th := size + size/3
	if tw < 1 {
		tw = 1
	}
	stampW, stampH := tw+size*2, th+size*2
	stamp := image.NewNRGBA(image.Rect(0, 0, stampW, stampH))
	if _, err := DrawText(stamp, text, float32(size), float32(size+th*3/4), TextOptions{
		Family: "", Size: size, Color: col,
	}); err != nil {
		return err
	}
	angle := wm.Angle
	if angle == 0 {
		angle = -30
	}
	rotated := imaging.Rotate(stamp, angle, color.NRGBA{})
	stepX := stampW
	stepY := stampH
	switch wm.Density {
	case 0: // sparse
		stepX, stepY = stampW*2, stampH*3
	case 2: // dense
		stepX, stepY = stampW*7/10, stampH*2/3
	default:
		stepX, stepY = stampW*4/3, stampH*3/2
	}
	if stepX < 40 {
		stepX = 40
	}
	if stepY < 40 {
		stepY = 40
	}
	rb := rotated.Bounds()
	row := 0
	for y := -rb.Dy(); y < b.Dy()+rb.Dy(); y += stepY {
		offset := 0
		if row%2 == 1 {
			offset = stepX / 2
		}
		for x := -rb.Dx() + offset; x < b.Dx()+rb.Dx(); x += stepX {
			draw.Draw(canvas, image.Rect(x, y, x+rb.Dx(), y+rb.Dy()), rotated, rb.Min, draw.Over)
		}
		row++
	}
	return nil
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

package engine

// canvas.go renders the free-layout canvas behind the 图片布局 page.
//
// A layout is a canvas plus an ordered list of layers. The page edits the layer
// list; the engine turns that list into pixels, and the page's preview calls the
// same function, so what the canvas shows and what the file contains come from one
// implementation.
//
// The layer set is deliberately small — image, text, shape — because the PRD puts
// free layout in the Creator family and explicitly refuses a Photoshop-class layer
// editor. Every layer carries the same geometry fields, so the page's selection,
// dragging and property panel stay uniform.

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

// CanvasLayerKind names one layer type.
const (
	LayerImage = "image"
	LayerText  = "text"
	LayerShape = "shape"
)

// Shape kinds.
const (
	ShapeRect    = "rect"
	ShapeEllipse = "ellipse"
	ShapeLine    = "line"
)

// CanvasLayer is one object on the layout canvas.
//
// X and Y are the layer's top-left corner in canvas pixels; W and H its size. A
// rotation turns the layer around its own centre, which is what a drag handle
// implies. Locked layers are skipped by the page's pointer handling, not by the
// renderer: a locked layer still draws.
type CanvasLayer struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`

	// X and Y are the layer's top-left corner in canvas pixels; W and H its size. A
	// rotation turns the layer around its own centre, which is what a drag handle
	// implies. Locked layers are skipped by the page's pointer handling, not by the
	// renderer: a locked layer still draws.
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	W        float64 `json:"w"`
	H        float64 `json:"h"`
	Rotation float64 `json:"rotation,omitempty"`
	Opacity  float64 `json:"opacity,omitempty"`
	// Radius rounds an image layer's corners or a rectangle shape, in pixels.
	Radius int `json:"radius,omitempty"`

	Locked  bool `json:"locked,omitempty"`
	Visible bool `json:"visible"`

	// Image layers.
	Path    string  `json:"path,omitempty"`
	Fit     string  `json:"fit,omitempty"`
	OffsetX float64 `json:"offset_x,omitempty"`
	OffsetY float64 `json:"offset_y,omitempty"`
	FlipH   bool    `json:"flip_h,omitempty"`
	FlipV   bool    `json:"flip_v,omitempty"`

	// Text layers.
	Text       string  `json:"text,omitempty"`
	Font       string  `json:"font,omitempty"`
	FontSize   int     `json:"font_size,omitempty"`
	Bold       bool    `json:"bold,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	LineGap    float64 `json:"line_gap,omitempty"`
	AutoHeight bool    `json:"auto_height,omitempty"`

	// Shape layers.
	Shape       string `json:"shape,omitempty"`
	Fill        string `json:"fill,omitempty"`
	Stroke      string `json:"stroke,omitempty"`
	StrokeWidth int    `json:"stroke_width,omitempty"`
}

// CanvasRequest is one layout render.
type CanvasRequest struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Background string `json:"background"`
	// BackgroundImage is drawn behind the layers with the same cover rule the
	// collage uses.
	BackgroundImage string        `json:"background_image"`
	BackgroundDim   float64       `json:"background_dim,omitempty"`
	Layers          []CanvasLayer `json:"layers"`

	OutputPath string `json:"output_path,omitempty"`
	Quality    int    `json:"quality,omitempty"`
}

// RenderCanvas composes the layout into an image.
func RenderCanvas(req CanvasRequest) (image.Image, error) {
	w := req.Width
	if w <= 0 {
		w = 1200
	}
	h := req.Height
	if h <= 0 {
		h = 1200
	}
	if err := output.CheckSize(w, h); err != nil {
		return nil, err
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
	if strings.TrimSpace(req.BackgroundImage) != "" {
		opened, err := codec.OpenSRGB(req.BackgroundImage)
		if err == nil {
			filled := imaging.Fill(opened.Image, w, h, imaging.Center, imaging.Lanczos)
			draw.Draw(canvas, canvas.Bounds(), filled, filled.Bounds().Min, draw.Src)
			dim := clampF(req.BackgroundDim, 0, 100) / 100
			if dim > 0 {
				draw.Draw(canvas, canvas.Bounds(),
					&image.Uniform{color.NRGBA{A: uint8(dim * 255)}}, image.Point{}, draw.Over)
			}
		} else {
			return nil, fmt.Errorf("背景图读取失败：%w", err)
		}
	} else {
		bg := markColor(req.Background)
		if strings.TrimSpace(req.Background) == "" {
			bg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	}
	for i, layer := range req.Layers {
		if err := drawCanvasLayer(canvas, layer); err != nil {
			return nil, fmt.Errorf("第 %d 层：%w", i+1, err)
		}
	}
	return canvas, nil
}

// drawCanvasLayer renders one layer into its own box, then composites it with the
// layer's rotation and opacity.
func drawCanvasLayer(canvas *image.NRGBA, layer CanvasLayer) error {
	if !layer.Visible && layer.Kind != "" {
		// The page always writes Visible, so an explicit false hides the layer.
		return nil
	}
	lw := int(math.Round(layer.W))
	lh := int(math.Round(layer.H))
	if lw < 1 || lh < 1 {
		return nil
	}
	if err := output.CheckSize(lw, lh); err != nil {
		return err
	}
	var patch image.Image
	switch strings.ToLower(strings.TrimSpace(layer.Kind)) {
	case LayerImage, "":
		opened, err := codec.OpenSRGB(layer.Path)
		if err != nil {
			return fmt.Errorf("图片读取失败：%w", err)
		}
		patch = placePicture(opened.Image, lw, lh, normalizedFit(layer.Fit), CollageCell{
			OffsetX: layer.OffsetX, OffsetY: layer.OffsetY,
			FlipH: layer.FlipH, FlipV: layer.FlipV,
		})
	case LayerText:
		var err error
		patch, err = renderTextLayer(layer, lw, lh)
		if err != nil {
			return err
		}
	case LayerShape:
		patch = renderShapeLayer(layer, lw, lh)
	default:
		return fmt.Errorf("未知的图层类型 %q", layer.Kind)
	}
	if layer.Radius > 0 && strings.EqualFold(layer.Kind, LayerImage) {
		patch = roundImage(patch, layer.Radius)
	}
	if layer.Rotation != 0 {
		patch = imaging.Rotate(patch, layer.Rotation, color.NRGBA{})
	}
	opacity := layer.Opacity
	if opacity <= 0 {
		opacity = 1
	}
	if opacity > 1 {
		opacity /= 100
	}
	if opacity < 1 {
		patch = applyOpacity(patch, opacity)
	}
	pb := patch.Bounds()
	// A rotation grows the patch, so the centre is what stays put.
	cx := layer.X + layer.W/2
	cy := layer.Y + layer.H/2
	x := int(math.Round(cx - float64(pb.Dx())/2))
	y := int(math.Round(cy - float64(pb.Dy())/2))
	draw.Draw(canvas, image.Rect(x, y, x+pb.Dx(), y+pb.Dy()), patch, pb.Min, draw.Over)
	return nil
}

// normalizedFit resolves the layer's fit mode.
func normalizedFit(fit string) string {
	switch strings.ToLower(strings.TrimSpace(fit)) {
	case CollageContain:
		return CollageContain
	case CollageStretch:
		return CollageStretch
	}
	return CollageCover
}

// renderTextLayer wraps and paints the layer's text inside its box.
func renderTextLayer(layer CanvasLayer, w, h int) (image.Image, error) {
	size := layer.FontSize
	if size <= 0 {
		size = 32
	}
	lineGap := layer.LineGap
	if lineGap <= 0 {
		lineGap = 1.3
	}
	face, err := FontFace(layer.Font, size, 72, layer.Bold)
	if err != nil {
		return nil, err
	}
	wrapped := WrapText(layer.Text, face, float32(w))
	lineHeight := float32(float64(size) * lineGap)
	if layer.AutoHeight {
		need := int(math.Ceil(float64(lineHeight) * float64(len(wrapped))))
		if need > h {
			h = need
		}
	}
	patch := image.NewNRGBA(image.Rect(0, 0, w, h))
	col := markColor(layer.Color)
	if strings.TrimSpace(layer.Color) == "" {
		col = color.NRGBA{R: 17, G: 24, B: 39, A: 255}
	}
	for i, line := range wrapped {
		lineWidth := MeasureText(line, face)
		var x float32
		switch strings.ToLower(strings.TrimSpace(layer.Align)) {
		case "center", "c":
			x = (float32(w) - lineWidth) / 2
		case "right", "r":
			x = float32(w) - lineWidth
		default:
			x = 0
		}
		baseline := float32(i)*lineHeight + float32(size)
		if _, err := DrawText(patch, line, x, baseline, TextOptions{
			Family: layer.Font, Size: size, Color: col, Bold: layer.Bold,
		}); err != nil {
			return nil, err
		}
	}
	return patch, nil
}

// renderShapeLayer paints a rectangle, ellipse or line inside the layer box.
func renderShapeLayer(layer CanvasLayer, w, h int) image.Image {
	patch := image.NewNRGBA(image.Rect(0, 0, w, h))
	fill := markColor(layer.Fill)
	if strings.TrimSpace(layer.Fill) == "" {
		fill = color.NRGBA{R: 0, G: 122, B: 255, A: 255}
	}
	stroke := markColor(layer.Stroke)
	strokeWidth := float64(layer.StrokeWidth)
	if strokeWidth <= 0 {
		strokeWidth = 0
	}
	switch strings.ToLower(strings.TrimSpace(layer.Shape)) {
	case ShapeEllipse:
		fillEllipse(patch, 0, 0, float64(w), float64(h), fill)
		if strokeWidth > 0 {
			drawEllipseOutline(patch, 0, 0, w, h, strokeWidth, stroke)
		}
	case ShapeLine:
		col := fill
		if strokeWidth > 0 {
			col = stroke
		}
		if strokeWidth <= 0 {
			strokeWidth = 4
		}
		drawThickLine(patch, 0, float64(h)/2, float64(w), float64(h)/2, strokeWidth, col)
	default:
		fillRect(patch, 0, 0, float64(w), float64(h), fill)
		if strokeWidth > 0 {
			drawRectOutline(patch, 0, 0, w, h, strokeWidth, stroke)
		}
	}
	return patch
}

// applyOpacity multiplies the alpha channel, leaving the straight colour values
// alone so a translucent layer does not darken as it fades.
func applyOpacity(src image.Image, k float64) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	k = clampF(k, 0, 1)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			c.A = uint8(float64(c.A)*k + 0.5)
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst
}

// Layout renders the composition and writes it.
func Layout(req CanvasRequest) Result {
	if strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输出路径")
	}
	img, err := RenderCanvas(req)
	if err != nil {
		return Fail("LAYOUT_FAILED", err.Error())
	}
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	if err := codec.Save(img, req.OutputPath, codec.SaveOptions{Quality: quality}); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	res := OK()
	res.OutputPath = req.OutputPath
	res.Width = img.Bounds().Dx()
	res.Height = img.Bounds().Dy()
	res.Count = len(req.Layers)
	return res
}

// canvasPresets are the standard arrangement templates the page offers. They are
// plain layer lists, so "apply a template" is just "replace the layer slice".
type CanvasPreset struct {
	Key   string
	Label string
	// Layers is a function of the canvas size, so a template scales with it.
	Build func(w, h int) []CanvasLayer
}

// CanvasPresets lists the built-in layout templates.
var CanvasPresets = []CanvasPreset{
	{Key: "center", Label: "居中标题", Build: func(w, h int) []CanvasLayer {
		return []CanvasLayer{
			{Kind: LayerText, Name: "标题", X: float64(w) * 0.1, Y: float64(h) * 0.38,
				W: float64(w) * 0.8, H: float64(h) * 0.24, Text: "标题", FontSize: h / 12,
				Bold: true, Align: "center", Color: "#111827", Visible: true, AutoHeight: true},
		}
	}},
	{Key: "photo-card", Label: "图文卡片", Build: func(w, h int) []CanvasLayer {
		return []CanvasLayer{
			{Kind: LayerShape, Name: "卡片底", X: 40, Y: 40,
				W: float64(w) - 80, H: float64(h) - 80, Shape: ShapeRect,
				Fill: "#FFFFFF", Radius: 24, Visible: true},
			{Kind: LayerImage, Name: "配图", X: 80, Y: 80,
				W: float64(w) - 160, H: float64(h) * 0.55, Fit: CollageCover, Visible: true},
			{Kind: LayerText, Name: "标题", X: 80, Y: float64(h)*0.55 + 120,
				W: float64(w) - 160, H: 80, Text: "标题文字", FontSize: 42,
				Bold: true, Color: "#111827", Visible: true, AutoHeight: true},
		}
	}},
	{Key: "caption", Label: "上图下注", Build: func(w, h int) []CanvasLayer {
		return []CanvasLayer{
			{Kind: LayerImage, Name: "图片", X: 0, Y: 0,
				W: float64(w), H: float64(h) * 0.75, Fit: CollageCover, Visible: true},
			{Kind: LayerText, Name: "说明", X: 40, Y: float64(h) * 0.75,
				W: float64(w) - 80, H: float64(h) * 0.25, Text: "说明文字",
				FontSize: 30, Color: "#4B5563", Visible: true, AutoHeight: true},
		}
	}},
}

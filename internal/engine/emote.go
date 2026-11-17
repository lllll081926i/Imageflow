package engine

// emote.go builds stickers and memes: a picture, a caption, and — for an animated
// source — the same caption over a chosen frame range.
//
// The animation path deliberately reuses the frame model of the GIF engine instead
// of growing a second animation pipeline: a meme is a text layer over the frames
// the animation tool already knows how to compose, re-encode and loop.

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// EmoteTemplate names one caption arrangement.
const (
	EmoteTopBottom = "top_bottom"
	EmoteCenter    = "center"
	EmoteBubble    = "bubble"
)

// EmoteRequest is one sticker render.
type EmoteRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`

	Template   string `json:"template"`
	TopText    string `json:"top_text"`
	BottomText string `json:"bottom_text"`
	CenterText string `json:"center_text"`

	Font        string `json:"font"`
	FontSize    int    `json:"font_size"`
	Color       string `json:"color"`
	Stroke      int    `json:"stroke"`
	StrokeColor string `json:"stroke_color"`
	Shadow      bool   `json:"shadow"`
	// BubbleColor fills the dialogue bubble template.
	BubbleColor string `json:"bubble_color"`

	// FrameStart and FrameEnd limit the caption to a frame range of an animation.
	// A zero range means every frame.
	FrameStart int `json:"frame_start"`
	FrameEnd   int `json:"frame_end"`

	OutputFormat string `json:"output_format"`
	Quality      int    `json:"quality"`
	Loop         int    `json:"loop"`
	ApplyLoop    bool   `json:"apply_loop"`
}

// RenderEmote draws the caption onto one already-decoded picture.
func RenderEmote(img image.Image, req EmoteRequest) (image.Image, error) {
	if img == nil {
		return nil, fmt.Errorf("缺少输入图像")
	}
	b := img.Bounds()
	canvas := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(canvas, canvas.Bounds(), img, b.Min, draw.Src)

	size := req.FontSize
	if size <= 0 {
		// The caption scales with the picture so a 400 px meme and a 1600 px one
		// both stay readable.
		size = clampInt(b.Dy()/8, 14, 200)
	}
	col := markColor(req.Color)
	if strings.TrimSpace(req.Color) == "" {
		col = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	strokeCol := markColor(req.StrokeColor)
	if strings.TrimSpace(req.StrokeColor) == "" {
		strokeCol = color.NRGBA{A: 255}
	}
	opts := TextOptions{
		Family: req.Font, Size: size, Color: col, Bold: true,
		Stroke: clampInt(req.Stroke, 0, 20), StrokeCol: strokeCol,
		Shadow:    req.Shadow,
		ShadowCol: color.NRGBA{A: 160},
	}

	switch strings.ToLower(strings.TrimSpace(req.Template)) {
	case EmoteBubble:
		if err := drawBubble(canvas, req); err != nil {
			return nil, err
		}
	case EmoteCenter:
		if err := drawCenteredCaption(canvas, req.CenterText, opts, size); err != nil {
			return nil, err
		}
	default: // top and bottom
		if err := drawCaption(canvas, req.TopText, opts, size, true); err != nil {
			return nil, err
		}
		if err := drawCaption(canvas, req.BottomText, opts, size, false); err != nil {
			return nil, err
		}
	}
	return canvas, nil
}

// drawCaption paints one line block at the top or the bottom edge, in caps, the
// way a meme is usually captioned.
func drawCaption(canvas *image.NRGBA, text string, opts TextOptions, size int, top bool) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	face, err := FontFace(opts.Family, opts.Size, 72, true)
	if err != nil {
		return err
	}
	b := canvas.Bounds()
	lines := WrapText(strings.ToUpper(text), face, float32(b.Dx())*0.92)
	lineHeight := float32(float64(size) * 1.25)
	blockH := float32(len(lines)) * lineHeight
	var y float32
	if top {
		y = float32(size) * 0.35
	} else {
		y = float32(b.Dy()) - blockH - float32(size)*0.15
	}
	for i, line := range lines {
		width := MeasureText(line, face)
		x := (float32(b.Dx()) - width) / 2
		baseline := y + float32(i)*lineHeight + float32(size)
		if _, err := DrawText(canvas, line, x, baseline, opts); err != nil {
			return err
		}
	}
	return nil
}

// drawCenteredCaption paints a centred badge in the middle of the picture.
func drawCenteredCaption(canvas *image.NRGBA, text string, opts TextOptions, size int) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	face, err := FontFace(opts.Family, opts.Size, 72, true)
	if err != nil {
		return err
	}
	b := canvas.Bounds()
	lines := WrapText(text, face, float32(b.Dx())*0.8)
	lineHeight := float32(float64(size) * 1.25)
	blockH := float32(len(lines)) * lineHeight
	startY := (float32(b.Dy()) - blockH) / 2
	for i, line := range lines {
		width := MeasureText(line, face)
		x := (float32(b.Dx()) - width) / 2
		baseline := startY + float32(i)*lineHeight + float32(size)
		if _, err := DrawText(canvas, line, x, baseline, opts); err != nil {
			return err
		}
	}
	return nil
}

// drawBubble paints a rounded speech bubble with a tail and the caption inside.
func drawBubble(canvas *image.NRGBA, req EmoteRequest) error {
	b := canvas.Bounds()
	pad := maxInt(16, b.Dy()/12)
	boxW := b.Dx() - pad*2
	boxH := b.Dy()/3 + pad
	if boxW < 16 || boxH < 16 {
		return fmt.Errorf("画布太小，无法放置对话气泡")
	}
	fill := markColor(req.BubbleColor)
	if strings.TrimSpace(req.BubbleColor) == "" {
		fill = color.NRGBA{R: 255, G: 255, B: 255, A: 245}
	}
	x0, y0 := pad, b.Dy()-boxH-pad
	radius := minInt(boxH/3, boxW/4)
	// The bubble body, then a triangular tail pointing down to its speaker.
	for y := y0; y < y0+boxH; y++ {
		for x := x0; x < x0+boxW; x++ {
			if !insideRounded(x-x0, y-y0, boxW, boxH, float64(radius)) {
				continue
			}
			blendPixel(canvas, x, y, fill, 255)
		}
	}
	tailX := x0 + boxW/4
	tailW := maxInt(12, boxH/3)
	fillTriangle(canvas, float64(tailX), float64(y0+boxH-2),
		float64(tailX+tailW), float64(y0+boxH-2),
		float64(tailX+tailW/2), float64(y0+boxH+tailW), fill)

	size := req.FontSize
	if size <= 0 {
		size = clampInt(boxH/3, 12, 120)
	}
	col := markColor(req.Color)
	if strings.TrimSpace(req.Color) == "" {
		col = color.NRGBA{R: 17, G: 24, B: 39, A: 255}
	}
	face, err := FontFace(req.Font, size, 72, true)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(req.CenterText)
	if text == "" {
		text = strings.TrimSpace(req.BottomText)
	}
	lines := WrapText(text, face, float32(boxW-pad*2))
	lineHeight := float32(float64(size) * 1.25)
	total := float32(len(lines)) * lineHeight
	startY := float32(y0) + (float32(boxH)-total)/2
	for i, line := range lines {
		width := MeasureText(line, face)
		x := float32(x0) + (float32(boxW)-width)/2
		baseline := startY + float32(i)*lineHeight + float32(size)
		if _, err := DrawText(canvas, line, x, baseline, TextOptions{
			Family: req.Font, Size: size, Color: col, Bold: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Emote renders one sticker and writes it. A still source produces one picture; an
// animation produces an animation whose caption is limited to the requested frame
// range.
func Emote(ctx context.Context, req EmoteRequest) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return Fail("CANCELLED", "已取消")
	}
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	// Animated sources go through the frame model; anything else is a still.
	if anim, err := LoadAnimation(req.InputPath); err == nil && len(anim.Frames) > 1 {
		return emoteAnimation(ctx, anim, req)
	}
	opened, err := codec.OpenSRGB(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	img, err := RenderEmote(opened.Image, req)
	if err != nil {
		return Fail("EMOTE_FAILED", err.Error())
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(extOfPath(req.OutputPath), "."))
	}
	if format == "" {
		format = "png"
	}
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}
	if ctx.Err() != nil {
		return Fail("CANCELLED", "已取消")
	}
	if err := codec.SaveContext(ctx, img, req.OutputPath, codec.SaveOptions{Quality: quality, SRGB: true}); err != nil {
		if ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		return Fail("WRITE_FAILED", err.Error())
	}
	res := OK()
	res.OutputPath = req.OutputPath
	res.Width = img.Bounds().Dx()
	res.Height = img.Bounds().Dy()
	res.Count = 1
	return res
}

// emoteAnimation captions a frame range and writes an animation back.
func emoteAnimation(ctx context.Context, anim *Animation, req EmoteRequest) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx != nil && ctx.Err() != nil {
		return Fail("CANCELLED", "已取消")
	}
	start, end := req.FrameStart, req.FrameEnd
	if start < 0 {
		start = 0
	}
	if end <= 0 || end > len(anim.Frames) {
		end = len(anim.Frames)
	}
	if start >= end {
		return Fail("EMOTE_BAD_RANGE", fmt.Sprintf("帧范围无效：%d–%d（共 %d 帧）", start, end, len(anim.Frames)))
	}
	for i := range anim.Frames {
		if ctx != nil && ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		if i < start || i >= end {
			continue
		}
		captioned, err := RenderEmote(anim.Frames[i].Image, req)
		if err != nil {
			return Fail("EMOTE_FAILED", fmt.Sprintf("第 %d 帧：%v", i+1, err))
		}
		anim.Frames[i].Image = captioned
	}
	if req.ApplyLoop {
		anim.Loop = req.Loop
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(extOfPath(req.OutputPath), "."))
	}
	if format == "" {
		format = "gif"
	}
	quality := req.Quality
	if quality <= 0 {
		quality = 90
	}
	if err := animationSize(anim.Width, anim.Height, len(anim.Frames)); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	switch format {
	case "gif":
		if err := writeGIFContext(ctx, anim, req.OutputPath, quality); err != nil {
			if ctx.Err() != nil {
				return Fail("CANCELLED", "已取消")
			}
			return Fail("WRITE_FAILED", err.Error())
		}
	case "apng", "png":
		if err := writeAPNGContext(ctx, anim, req.OutputPath); err != nil {
			if ctx.Err() != nil {
				return Fail("CANCELLED", "已取消")
			}
			return Fail("WRITE_FAILED", err.Error())
		}
	case "webp":
		if err := writeAnimatedWebPContext(ctx, anim, req.OutputPath, quality); err != nil {
			if ctx.Err() != nil {
				return Fail("CANCELLED", "已取消")
			}
			return Fail("WRITE_FAILED", err.Error())
		}
	default:
		return Fail("EMOTE_BAD_FORMAT", "不支持的动图输出格式："+format)
	}
	res := OK()
	res.OutputPath = req.OutputPath
	res.Width = anim.Width
	res.Height = anim.Height
	res.Count = len(anim.Frames)
	res.Message = fmt.Sprintf("第 %d–%d 帧加字", start+1, end)
	return res
}

// extOfPath returns the extension of a path including the dot.
func extOfPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' || path[i] == '\\' {
			return ""
		}
	}
	return ""
}

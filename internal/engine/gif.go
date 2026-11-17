package engine

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/webp"
	"github.com/kettek/apng"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// Frame is one animation frame with its display duration in milliseconds.
type Frame struct {
	Image image.Image
	Delay int
}

// Animation is a decoded GIF/APNG/WebP animation.
type Animation struct {
	Frames []Frame
	Loop   int
	Format string
	Width  int
	Height int
}

// GIFRequest is the GIF tool payload.
type GIFRequest struct {
	Action       string  `json:"action"`
	InputPath    string  `json:"input_path"`
	OutputPath   string  `json:"output_path"`
	OutputDir    string  `json:"output_dir"`
	OutputFormat string  `json:"output_format"`
	FrameStart   int     `json:"frame_start"`
	FrameEnd     int     `json:"frame_end"`
	SpeedFactor  float64 `json:"speed_factor"`
	Quality      int     `json:"quality"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	MaintainAR   bool    `json:"maintain_aspect"`
	// Loop is the animation's repeat count; ApplyLoop decides whether it is
	// written to the result.
	//
	// A plain int could not carry both meanings: 0 is a real value (loop
	// forever), so `if Loop != 0` made the panel's 0 = 无限 setting impossible to
	// apply — asking for an infinite loop kept the source's finite count.
	Loop      int  `json:"loop"`
	ApplyLoop bool `json:"apply_loop"`
}

// ProbeAnimation reports the frame count and canvas size.
func ProbeAnimation(path string) (frames, width, height int, format string, err error) {
	a, err := LoadAnimation(path)
	if err != nil {
		return 0, 0, 0, "", err
	}
	return len(a.Frames), a.Width, a.Height, a.Format, nil
}

// LoadAnimation decodes any of the three supported animation containers.
func LoadAnimation(path string) (*Animation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rawInput, err := readAnimation(f)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(rawInput)
	head := make([]byte, 16)
	n, _ := r.Read(head)
	head = head[:n]

	switch {
	case n >= 6 && (string(head[:6]) == "GIF87a" || string(head[:6]) == "GIF89a"):
		if _, err := r.Seek(0, 0); err != nil {
			return nil, err
		}
		g, err := gif.DecodeAll(r)
		if err != nil {
			return nil, err
		}
		// A file that claims GIF but carries a single frame is a still image; the
		// tool must reject it rather than "process" it as a one-frame animation.
		if len(g.Image) < 2 {
			return nil, fmt.Errorf("不是动图: GIF 只有 %d 帧", len(g.Image))
		}
		out := &Animation{Loop: g.LoopCount, Format: "GIF", Width: g.Config.Width, Height: g.Config.Height}
		composed := composeFrames(g.Image, g.Disposal, g.Config.Width, g.Config.Height)
		for i, frame := range composed {
			delay := 0
			if i < len(g.Delay) {
				delay = g.Delay[i] * 10
			}
			out.Frames = append(out.Frames, Frame{Image: frame, Delay: delay})
		}
		return out, nil
	case n >= 3 && head[0] == 0x89 && string(head[1:3]) == "PN":
		if _, err := r.Seek(0, 0); err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		// A plain PNG carries no animation control chunk; treating it as a
		// one-frame animation would silently accept still images.
		if !bytes.Contains(raw, []byte("acTL")) {
			return nil, fmt.Errorf("不是动图: PNG 缺少 acTL 块")
		}
		a, err := apng.DecodeAll(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if len(a.Frames) < 2 {
			return nil, fmt.Errorf("不是动图: APNG 只有 %d 帧", len(a.Frames))
		}
		for _, frame := range a.Frames {
			if frame.Image == nil {
				return nil, fmt.Errorf("APNG 帧缺少图像数据")
			}
		}
		out := &Animation{Loop: int(a.LoopCount), Format: "APNG"}
		if len(a.Frames) > 0 {
			b := a.Frames[0].Image.Bounds()
			out.Width, out.Height = b.Dx(), b.Dy()
		}
		composed := composeAPNG(a.Frames, out.Width, out.Height)
		for i, frame := range composed {
			delay := 0
			if i < len(a.Frames) {
				delay = int(a.Frames[i].GetDelay() * 1000)
			}
			out.Frames = append(out.Frames, Frame{Image: frame, Delay: delay})
		}
		return out, nil
	case n >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		if _, err := r.Seek(0, 0); err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		if !bytes.Contains(raw, []byte("ANIM")) {
			return nil, fmt.Errorf("不是动图: WebP 缺少 ANIM 块")
		}
		a, err := webp.DecodeAll(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if len(a.Image) < 2 {
			return nil, fmt.Errorf("不是动图: WebP 只有 %d 帧", len(a.Image))
		}
		out := &Animation{Loop: int(a.LoopCount), Format: "WEBP"}
		for i, img := range a.Image {
			if out.Width == 0 {
				b := img.Bounds()
				out.Width, out.Height = b.Dx(), b.Dy()
			}
			delay := 0
			if i < len(a.Delay) {
				delay = a.Delay[i]
			}
			out.Frames = append(out.Frames, Frame{Image: img, Delay: delay})
		}
		return out, nil
	}
	return nil, fmt.Errorf("不是支持的动图格式")
}

// composeFrames flattens a GIF's optimised frames onto one running canvas.
//
// GIF encoders store each frame as only the rectangle that changed, and a
// disposal method tells the viewer what to do with that rectangle afterwards.
// Drawing every frame onto a fresh transparent canvas instead drops the pixels
// the previous frames left on screen, which shows up as holes in exported
// frames and in any re-encode.
func composeFrames(frames []*image.Paletted, disposals []byte, width, height int) []image.Image {
	out := make([]image.Image, 0, len(frames))
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i, frame := range frames {
		// Restore-to-previous needs the canvas as it was before this frame.
		var saved *image.NRGBA
		if i < len(disposals) && disposals[i] == gifDisposalPrevious {
			saved = cloneRect(canvas, frame.Bounds())
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		snapshot := image.NewNRGBA(canvas.Bounds())
		copy(snapshot.Pix, canvas.Pix)
		out = append(out, snapshot)

		switch {
		case i < len(disposals) && disposals[i] == gifDisposalBackground:
			// Clear the frame's rectangle back to transparent.
			draw.Draw(canvas, frame.Bounds(), &image.Uniform{color.NRGBA{}}, frame.Bounds().Min, draw.Src)
		case saved != nil:
			draw.Draw(canvas, saved.Bounds(), saved, saved.Bounds().Min, draw.Src)
		}
	}
	return out
}

const (
	gifDisposalNone       = 0
	gifDisposalBackground = 2
	gifDisposalPrevious   = 3
)

// composeAPNG flattens APNG frames the same way composeFrames does for GIF: a
// frame may carry only a sub-rectangle, and its dispose/blend operation decides
// what happens to the canvas afterwards.
func composeAPNG(frames []apng.Frame, width, height int) []image.Image {
	out := make([]image.Image, 0, len(frames))
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	for _, frame := range frames {
		if frame.Image == nil {
			continue
		}
		dst := image.Rect(
			frame.XOffset,
			frame.YOffset,
			frame.XOffset+frame.Image.Bounds().Dx(),
			frame.YOffset+frame.Image.Bounds().Dy(),
		)
		op := draw.Over
		var saved *image.NRGBA
		if frame.DisposeOp == apng.DISPOSE_OP_PREVIOUS {
			saved = cloneRect(canvas, dst)
		}
		if frame.BlendOp == apng.BLEND_OP_SOURCE {
			op = draw.Src
		}
		draw.Draw(canvas, dst, frame.Image, frame.Image.Bounds().Min, op)
		snapshot := image.NewNRGBA(canvas.Bounds())
		copy(snapshot.Pix, canvas.Pix)
		out = append(out, snapshot)

		if frame.DisposeOp == apng.DISPOSE_OP_BACKGROUND {
			draw.Draw(canvas, dst, &image.Uniform{color.NRGBA{}}, dst.Min, draw.Src)
		} else if saved != nil {
			draw.Draw(canvas, saved.Bounds(), saved, saved.Bounds().Min, draw.Src)
		}
	}
	return out
}

// cloneRect copies one rectangle of src into a new image at the same origin.
func cloneRect(src *image.NRGBA, rect image.Rectangle) *image.NRGBA {
	if rect.Empty() {
		return nil
	}
	dst := image.NewNRGBA(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		copy(
			dst.Pix[(y-rect.Min.Y)*dst.Stride:],
			src.Pix[(y-src.Rect.Min.Y)*src.Stride+(rect.Min.X-src.Rect.Min.X)*4:],
		)
	}
	return dst
}

// RunGIF dispatches one GIF tool action.
func RunGIF(req GIFRequest) Result {
	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "get_frame_count", "probe":
		if req.InputPath == "" {
			return Fail("GIF_BAD_REQUEST", "缺少输入路径")
		}
		frames, w, h, format, err := ProbeAnimation(req.InputPath)
		if err != nil {
			return Fail("GIF_GET_FRAME_COUNT_FAILED", err.Error())
		}
		out := OK()
		out.Count = frames
		out.Width = w
		out.Height = h
		out.Message = format
		return out
	case "export_frames":
		return exportFrames(req)
	case "", "reverse":
		return transformAnimation(req, func(a *Animation) error { reverseFrames(a); return nil })
	case "change_speed":
		factor := req.SpeedFactor
		if factor <= 0 {
			factor = 1
		}
		return transformAnimation(req, func(a *Animation) error {
			for i := range a.Frames {
				d := int(float64(a.Frames[i].Delay) / factor)
				if d < 10 {
					d = 10
				}
				a.Frames[i].Delay = d
			}
			return nil
		})
	case "resize":
		return transformAnimation(req, func(a *Animation) error { return resizeFrames(a, req) })
	case "compress":
		// Re-encoding with a smaller palette is the compression: the writer
		// derives the colour count from req.Quality.
		return transformAnimation(req, func(a *Animation) error { return nil })
	}
	return Fail("GIF_BAD_REQUEST", "未知的操作: "+req.Action)
}

func reverseFrames(a *Animation) {
	for i, j := 0, len(a.Frames)-1; i < j; i, j = i+1, j-1 {
		a.Frames[i], a.Frames[j] = a.Frames[j], a.Frames[i]
	}
}

func resizeFrames(a *Animation, req GIFRequest) error {
	if req.Width <= 0 && req.Height <= 0 {
		return nil
	}
	w, h := req.Width, req.Height
	if req.MaintainAR {
		if w > 0 {
			h = 0
		} else if h > 0 {
			w = 0
		}
	}
	width, height := float64(w), float64(h)
	if w <= 0 {
		width = math.Max(1, math.Floor(float64(a.Width)*height/float64(a.Height)+0.5))
	}
	if h <= 0 {
		height = math.Max(1, math.Floor(float64(a.Height)*width/float64(a.Width)+0.5))
	}
	if width > output.MaxOutputSide || height > output.MaxOutputSide {
		return output.ErrSizeExceeded
	}
	if err := animationSize(int(width), int(height), len(a.Frames)); err != nil {
		return err
	}
	for i := range a.Frames {
		a.Frames[i].Image = resizeImage(a.Frames[i].Image, int(width), int(height))
	}
	b := a.Frames[0].Image.Bounds()
	a.Width, a.Height = b.Dx(), b.Dy()
	return nil
}

func exportFrames(req GIFRequest) Result {
	if req.InputPath == "" || req.OutputDir == "" {
		return Fail("GIF_BAD_REQUEST", "缺少输入路径或输出目录")
	}
	animation, err := LoadAnimation(req.InputPath)
	if err != nil {
		return Fail("GIF_DECODE_FAILED", err.Error())
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = "png"
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	start, end := req.FrameStart, req.FrameEnd
	if start < 0 {
		start = 0
	}
	if end <= 0 || end > len(animation.Frames) {
		end = len(animation.Frames)
	}
	if start >= end {
		return Fail("GIF_BAD_REQUEST", fmt.Sprintf("帧范围无效: %d-%d（共 %d 帧）", start, end, len(animation.Frames)))
	}
	base := strings.TrimSuffix(filepath.Base(req.InputPath), filepath.Ext(req.InputPath))
	var written []string
	for i := start; i < end; i++ {
		name := fmt.Sprintf("%s_frame_%04d.%s", base, i+1, canonicalExt(format))
		target := filepath.Join(req.OutputDir, name)
		target, err = uniquePath(target)
		if err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		quality := req.Quality
		if quality <= 0 {
			quality = 95
		}
		if err := codec.Save(animation.Frames[i].Image, target, codec.SaveOptions{Quality: quality, MetadataSource: req.InputPath}); err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		written = append(written, target)
	}
	out := OK()
	out.OutputDir = req.OutputDir
	out.Outputs = written
	out.Count = len(written)
	return out
}

func transformAnimation(req GIFRequest, mutation func(*Animation) error) Result {
	if req.InputPath == "" || req.OutputPath == "" {
		return Fail("GIF_BAD_REQUEST", "缺少输入或输出路径")
	}
	animation, err := LoadAnimation(req.InputPath)
	if err != nil {
		return Fail("GIF_DECODE_FAILED", err.Error())
	}
	if len(animation.Frames) == 0 {
		return Fail("GIF_NO_FRAMES", "没有可用的帧")
	}
	if err := mutation(animation); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	if req.ApplyLoop {
		animation.Loop = req.Loop
	}
	format := strings.ToLower(strings.TrimSpace(req.OutputFormat))
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(filepath.Ext(req.OutputPath), "."))
	}
	if format == "" {
		format = "gif"
	}
	target := ensureExtension(req.OutputPath, format)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	if err := animationSize(animation.Width, animation.Height, len(animation.Frames)); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	switch format {
	case "gif":
		err = writeGIF(animation, target, req.Quality)
	case "apng", "png":
		err = writeAPNG(animation, target)
	case "webp":
		err = writeAnimatedWebP(animation, target, req.Quality)
	default:
		return Fail("GIF_BAD_REQUEST", "不支持的输出格式: "+format)
	}
	if err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Count = len(animation.Frames)
	out.Width = animation.Width
	out.Height = animation.Height
	if st, e := os.Stat(target); e == nil {
		out.OutputSize = st.Size()
	}
	return out
}

func loopValue(loop int) int {
	if loop < 0 {
		return 0
	}
	return loop
}

func writeGIF(a *Animation, target string, quality int) error {
	return writeGIFContext(context.Background(), a, target, quality)
}

func writeGIFContext(ctx context.Context, a *Animation, target string, quality int) error {
	colors := gifColorsForQuality(quality)
	paletted := make([]*image.Paletted, 0, len(a.Frames))
	delays := make([]int, 0, len(a.Frames))
	for _, frame := range a.Frames {
		if err := ctx.Err(); err != nil {
			return err
		}
		paletted = append(paletted, toPaletted(frame.Image, colors))
		d := frame.Delay / 10
		if d < 2 {
			d = 2
		}
		delays = append(delays, d)
	}
	out := &gif.GIF{
		Image:     paletted,
		Delay:     delays,
		LoopCount: loopValue(a.Loop),
		Config:    image.Config{ColorModel: paletted[0].Palette, Width: a.Width, Height: a.Height},
	}
	return codec.WriteAtomic(target, func(file *os.File) error {
		if err := gif.EncodeAll(animationWriter{ctx, file}, out); err != nil {
			return err
		}
		return ctx.Err()
	})
}

// gifColorsForQuality maps the tool's quality slider onto a palette size. GIF
// carries at most 256 entries and one of them is the transparent key, so the
// usable colour count runs from 8 to 255.
func gifColorsForQuality(quality int) int {
	if quality <= 0 {
		quality = 90
	}
	quality = clampInt(quality, 1, 100)
	return clampInt(8+(quality-1)*247/99, 8, 255)
}

func writeAPNG(a *Animation, target string) error {
	return writeAPNGContext(context.Background(), a, target)
}

func writeAPNGContext(ctx context.Context, a *Animation, target string) error {
	out := apng.APNG{LoopCount: uint(loopValue(a.Loop))}
	for _, frame := range a.Frames {
		if err := ctx.Err(); err != nil {
			return err
		}
		delayMs := frame.Delay
		if delayMs <= 0 {
			delayMs = 100
		}
		divisor, remainder := delayMs, 1000
		for remainder != 0 {
			divisor, remainder = remainder, divisor%remainder
		}
		numerator, denominator := delayMs/divisor, 1000/divisor
		if numerator > math.MaxUint16 {
			return fmt.Errorf("APNG 无法精确表示 %d 毫秒的帧延迟", delayMs)
		}
		out.Frames = append(out.Frames, apng.Frame{
			Image:            frame.Image,
			DelayNumerator:   uint16(numerator),
			DelayDenominator: uint16(denominator),
			DisposeOp:        apng.DISPOSE_OP_NONE,
			BlendOp:          apng.BLEND_OP_SOURCE,
		})
	}
	return codec.WriteAtomic(target, func(file *os.File) error {
		if err := apng.Encode(animationWriter{ctx, file}, out); err != nil {
			return err
		}
		return ctx.Err()
	})
}

func writeAnimatedWebP(a *Animation, target string, quality int) error {
	return writeAnimatedWebPContext(context.Background(), a, target, quality)
}

func writeAnimatedWebPContext(ctx context.Context, a *Animation, target string, quality int) error {
	if quality <= 0 {
		quality = 80
	}
	out := &webp.WEBP{LoopCount: int(loopValue(a.Loop))}
	for _, frame := range a.Frames {
		if err := ctx.Err(); err != nil {
			return err
		}
		d := frame.Delay
		if d <= 0 {
			d = 100
		}
		out.Image = append(out.Image, frame.Image)
		out.Delay = append(out.Delay, d)
	}
	return codec.WriteAtomic(target, func(w *os.File) error {
		if err := webp.EncodeAll(animationWriter{ctx, w}, out, webp.Options{Quality: quality, Method: 4}); err != nil {
			return err
		}
		return ctx.Err()
	})
}

type animationWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (writer animationWriter) Write(data []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		return 0, err
	}
	return writer.writer.Write(data)
}

// adaptivePalette builds a palette of at most colors entries from the frame's
// own colours, which reproduces a photographic gradient far better than a fixed
// lattice of evenly spaced RGB values.
func adaptivePalette(img image.Image, colors int) color.Palette {
	samples, _ := quantize(imaging.Fit(img, 128, 128, imaging.Linear), colors)
	if len(samples) == 0 {
		return webPalette
	}
	pal := make(color.Palette, 0, len(samples)+1)
	// Index 0 is the transparent key, as GIF viewers expect.
	pal = append(pal, color.NRGBA{})
	for _, c := range samples {
		pal = append(pal, c)
		if len(pal) >= 256 {
			break
		}
	}
	return pal
}

// toPaletted converts an image into a paletted frame of at most colors entries.
func toPaletted(img image.Image, colors int) *image.Paletted {
	pal := adaptivePalette(img, colors)
	b := img.Bounds()
	paletted := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), pal)
	cache := make(map[uint32]uint8, 4096)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			key := uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
			idx, ok := cache[key]
			if !ok {
				idx = nearestIndexPalette(c, paletted.Palette)
				if len(cache) < 1<<18 {
					cache[key] = idx
				}
			}
			paletted.SetColorIndex(x-b.Min.X, y-b.Min.Y, idx)
		}
	}
	return paletted
}

var webPalette = func() color.Palette {
	pal := color.Palette{color.NRGBA{}}
	steps := []uint8{0, 42, 85, 128, 170, 213, 255}
outer:
	for _, r := range steps {
		for _, g := range steps {
			for _, b := range steps {
				if len(pal) >= 255 {
					break outer
				}
				pal = append(pal, color.NRGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	for len(pal) < 256 {
		pal = append(pal, color.NRGBA{R: 255, G: 255, B: 255, A: 128})
	}
	return pal
}()

func nearestIndexPalette(c color.NRGBA, pal color.Palette) uint8 {
	if c.A < 128 {
		return 0
	}
	best := 1
	bestDist := 1 << 30
	for i := 1; i < len(pal); i++ {
		pc := color.NRGBAModel.Convert(pal[i]).(color.NRGBA)
		dr := int(c.R) - int(pc.R)
		dg := int(c.G) - int(pc.G)
		db := int(c.B) - int(pc.B)
		d := dr*dr + dg*dg + db*db
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return uint8(best)
}

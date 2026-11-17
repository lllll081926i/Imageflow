package engine

// pixel.go implements the pixel-art and dithering engine.
//
// Two claims are kept apart on purpose, because the PRD treats them as separate
// acceptance criteria: "every pixel is one of two colours" and "the file is
// encoded at one bit per pixel". This file produces the first, and the writing
// step delegates to internal/png1bit for the second.

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
	"github.com/lllll081926i/imageflow-go/internal/png1bit"
)

// Dither modes.
const (
	DitherNone     = "none"
	DitherBayer    = "bayer"
	DitherFloyd    = "floyd"
	DitherAtkinson = "atkinson"
)

// DitherModes lists the dithering algorithms the page offers, in order.
var DitherModes = []string{DitherNone, DitherBayer, DitherFloyd, DitherAtkinson}

// DitherLabels are the Chinese labels of DitherModes.
var DitherLabels = []string{"无抖动", "Bayer 有序", "Floyd–Steinberg", "Atkinson"}

// PixelRequest is one pixel-art render.
type PixelRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`

	// Width and Height are the target pixel dimensions. One of them may be zero,
	// in which case the source aspect ratio derives it.
	Width  int `json:"width"`
	Height int `json:"height"`
	// FitMode is cover, contain or stretch.
	FitMode string `json:"fit_mode"`

	// Colors is 2, 4, 8 or 16.
	Colors int `json:"colors"`
	// Gray asks for a monochrome palette with Colors levels.
	Gray bool `json:"gray"`
	// StrictBlackWhite collapses to exactly two values.
	StrictBlackWhite bool `json:"strict_bw"`
	// Black and White are the two colours of the strict palette.
	Black string `json:"black"`
	White string `json:"white"`
	// TrueBitDepth asks for a real bitDepth=1 PNG. It is only honoured for the
	// strict two-colour mode and a PNG output.
	TrueBitDepth bool `json:"true_bit_depth"`

	Dither    string  `json:"dither"`
	Strength  float64 `json:"strength"`
	Threshold int     `json:"threshold"`
	Invert    bool    `json:"invert"`
	// Background composites translucent source pixels before quantisation, so a
	// transparent logo does not turn into a black square.
	Background string `json:"background"`

	Quality int `json:"quality"`
}

// PixelResult carries the pixel-art render back to the page.
type PixelResult struct {
	Result
	// ColorsUsed is how many distinct colours the output really contains.
	ColorsUsed int
	// BitDepth is the PNG bit depth actually written, 1 or 8.
	BitDepth int
}

// Pixelate renders one picture as pixel art.
func Pixelate(req PixelRequest) PixelResult {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return PixelResult{Result: Fail("BAD_INPUT", "缺少输入或输出路径")}
	}
	opened, err := codec.OpenSRGB(req.InputPath)
	if err != nil {
		return PixelResult{Result: Fail("READ_FAILED", err.Error())}
	}
	if req.TrueBitDepth && req.StrictBlackWhite {
		// The user asked for a one-bit file. Only PNG can carry one, so a JPEG or
		// WebP target is refused with the reason instead of quietly producing an
		// eight-bit file that looks like the requested one.
		if normalizeFormat(strings.TrimPrefix(filepath.Ext(req.OutputPath), ".")) != "png" {
			return PixelResult{Result: Fail("PIXEL_BAD_FORMAT", "真正的 1-bit 输出只能写 PNG；请改用 PNG，或关闭「1-bit 编码」")}
		}
	}
	img, colors, depth, err := RenderPixelArt(opened.Image, req)
	if err != nil {
		return PixelResult{Result: Fail("PIXEL_FAILED", err.Error())}
	}
	target := req.OutputPath
	format := normalizeFormat(strings.TrimPrefix(filepath.Ext(target), "."))
	if depth == 1 {
		// A true 1-bit file is a PNG by definition: no other container in the
		// ladder carries a one-bit greyscale or indexed image.
		if format != "png" {
			return PixelResult{Result: Fail("PIXEL_BAD_FORMAT", "真正的 1-bit 输出只能写 PNG")}
		}
		if err := writeOneBitPNG(img, req, target); err != nil {
			return PixelResult{Result: Fail("WRITE_FAILED", err.Error())}
		}
	} else {
		quality := req.Quality
		if quality <= 0 {
			quality = 95
		}
		if err := codec.Save(img, target, codec.SaveOptions{Quality: quality, SRGB: true}); err != nil {
			return PixelResult{Result: Fail("WRITE_FAILED", err.Error())}
		}
	}
	res := OK()
	res.OutputPath = target
	res.Width = img.Bounds().Dx()
	res.Height = img.Bounds().Dy()
	return PixelResult{Result: res, ColorsUsed: colors, BitDepth: depth}
}

// RenderPixelArt scales, quantises and dithers, returning the picture, the number
// of distinct colours it contains and the intended PNG bit depth.
func RenderPixelArt(src image.Image, req PixelRequest) (image.Image, int, int, error) {
	if src == nil {
		return nil, 0, 0, fmt.Errorf("缺少输入图像")
	}
	w, h := req.Width, req.Height
	if w <= 0 && h <= 0 {
		w, h = 400, 300
	}
	b := src.Bounds()
	if w <= 0 {
		w = maxInt(1, int(float64(b.Dx())*float64(h)/float64(b.Dy())+0.5))
	}
	if h <= 0 {
		h = maxInt(1, int(float64(b.Dy())*float64(w)/float64(b.Dx())+0.5))
	}
	if err := output.CheckSize(w, h); err != nil {
		return nil, 0, 0, err
	}
	// Composite translucent pixels before anything else: a transparent logo would
	// otherwise quantise its clear pixels as black.
	base := compositeOnto(src, req.Background)

	var scaled image.Image
	switch strings.ToLower(strings.TrimSpace(req.FitMode)) {
	case "contain":
		inner := imaging.Fit(base, w, h, imaging.Lanczos)
		canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
		bg := markColor(req.Background)
		if strings.TrimSpace(req.Background) == "" {
			bg = color.NRGBA{A: 255}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				canvas.SetNRGBA(x, y, bg)
			}
		}
		dx, dy := (w-inner.Bounds().Dx())/2, (h-inner.Bounds().Dy())/2
		for y := 0; y < inner.Bounds().Dy(); y++ {
			for x := 0; x < inner.Bounds().Dx(); x++ {
				canvas.SetNRGBA(dx+x, dy+y, color.NRGBAModel.Convert(inner.At(inner.Bounds().Min.X+x, inner.Bounds().Min.Y+y)).(color.NRGBA))
			}
		}
		scaled = canvas
	case "stretch":
		scaled = imaging.Resize(base, w, h, imaging.NearestNeighbor)
	default:
		scaled = imaging.Fill(base, w, h, imaging.Center, imaging.NearestNeighbor)
	}

	threshold := req.Threshold
	if threshold < 0 || threshold > 255 {
		threshold = 128
	}
	depth := 8
	var out *image.NRGBA
	var colorsUsed int
	if req.StrictBlackWhite {
		out, colorsUsed = strictBW(scaled, ditherMode(req.Dither), clampF(req.Strength, 0, 100)/100, threshold, req.Invert)
		depth = 8
		if req.TrueBitDepth && strings.EqualFold(filepath.Ext(req.OutputPath), ".png") {
			depth = 1
		}
	} else {
		levels := req.Colors
		if levels < 2 {
			levels = 2
		}
		if levels > 16 {
			levels = 16
		}
		out, colorsUsed = quantiseTo(scaled, levels, req.Gray, ditherMode(req.Dither),
			clampF(req.Strength, 0, 100)/100, threshold, req.Invert)
	}
	return out, colorsUsed, depth, nil
}

// ditherMode resolves the requested dithering name.
func ditherMode(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case DitherBayer, DitherFloyd, DitherAtkinson:
		return strings.ToLower(strings.TrimSpace(name))
	}
	return DitherNone
}

// compositeOnto flattens translucency onto a background colour.
func compositeOnto(src image.Image, background string) image.Image {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	bg := markColor(background)
	if strings.TrimSpace(background) == "" {
		bg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			out.SetNRGBA(x, y, compositeRGB(c, bg))
		}
	}
	return out
}

// compositeRGB lays c over an opaque background, returning an opaque colour.
func compositeRGB(c, bg color.NRGBA) color.NRGBA {
	a := float64(c.A) / 255
	mix := func(s, d uint8) uint8 { return uint8(float64(s)*a + float64(d)*(1-a) + 0.5) }
	return color.NRGBA{R: mix(c.R, bg.R), G: mix(c.G, bg.G), B: mix(c.B, bg.B), A: 255}
}

// strictBW collapses the picture to exactly two colours with an optional dither.
func strictBW(src image.Image, mode string, strength float64, threshold int, invert bool) (*image.NRGBA, int) {
	// 2 luminance levels: the quantiser is the same code path the multi-colour
	// mode uses, so the dither patterns match between the two modes.
	out, _ := quantiseTo(src, 2, true, mode, strength, threshold, invert)
	// Force the two exact endpoints: a grey level must not survive the dither.
	seen := map[uint8]struct{}{}
	b := out.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := out.NRGBAAt(x, y)
			if c.R > 127 {
				c = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			} else {
				c = color.NRGBA{A: 255}
			}
			out.SetNRGBA(x, y, c)
			seen[c.R] = struct{}{}
		}
	}
	return out, len(seen)
}

// quantiseTo reduces the picture to a palette of `levels` luminance (or colour)
// steps, distributing the error with the requested dither.
func quantiseTo(src image.Image, levels int, gray bool, mode string, strength float64, threshold int, invert bool) (*image.NRGBA, int) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	palette := buildPalette(levels, gray)
	var colors []color.NRGBA
	if !gray {
		colors = pixelPalette(src, levels)
	}
	// The error buffer is per channel and in source units, so an error diffused
	// from one pixel to its neighbour is added before quantisation.
	errR := make([][]float64, 0)
	if mode == DitherFloyd || mode == DitherAtkinson {
		errR = make([][]float64, h)
		for i := range errR {
			errR[i] = make([]float64, w)
		}
	}
	// A separate error plane per channel, allocated together.
	var errG, errB [][]float64
	if mode == DitherFloyd || mode == DitherAtkinson {
		errG = make([][]float64, h)
		errB = make([][]float64, h)
		for i := range errG {
			errG[i] = make([]float64, w)
			errB[i] = make([]float64, w)
		}
	}
	seen := map[uint32]struct{}{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			v := [3]float64{float64(c.R), float64(c.G), float64(c.B)}
			if gray {
				lum := 0.299*v[0] + 0.587*v[1] + 0.114*v[2]
				v = [3]float64{lum, lum, lum}
			}
			if mode == DitherFloyd || mode == DitherAtkinson {
				v[0] += errR[y][x] * strength
				v[1] += errG[y][x] * strength
				v[2] += errB[y][x] * strength
			}
			if mode == DitherBayer {
				// Ordered dithering offsets the value by a fraction of one palette
				// step, taken from the Bayer matrix.
				step := 255.0 / float64(levels-1)
				offset := (bayerValue(x, y) - 0.5) * step * strength
				v[0] += offset
				v[1] += offset
				v[2] += offset
			}
			q := [3]uint8{}
			if gray && levels == 2 {
				// A two-level monochrome image has exactly one decision to make per
				// pixel, and the threshold is that decision. Quantising to an evenly
				// spaced 0/255 palette would ignore the slider the page offers, which is
				// what made 亮度阈值 look broken.
				lum := 0.299*v[0] + 0.587*v[1] + 0.114*v[2]
				white := lum >= float64(threshold)
				if invert {
					white = !white
				}
				level := uint8(0)
				if white {
					level = 255
				}
				q[0], q[1], q[2] = level, level, level
				dst := color.NRGBA{R: q[0], G: q[1], B: q[2], A: 255}
				out.SetNRGBA(x, y, dst)
				seen[uint32(dst.R)<<16|uint32(dst.G)<<8|uint32(dst.B)] = struct{}{}
				if mode == DitherFloyd || mode == DitherAtkinson {
					for ch := 0; ch < 3; ch++ {
						diffuse(v[ch]-float64(q[ch]), x, y, w, h, mode, errR, errG, errB, ch)
					}
				}
				continue
			}
			if gray {
				for channel := range q {
					q[channel] = palette[nearestPaletteIndex(v[channel], palette)]
				}
			} else {
				chosen := nearestPixelColor(v, colors)
				q = [3]uint8{chosen.R, chosen.G, chosen.B}
			}
			if gray {
				if invert {
					q[0] = 255 - q[0]
				}
				q[1], q[2] = q[0], q[0]
			} else if invert {
				q[0], q[1], q[2] = 255-q[0], 255-q[1], 255-q[2]
			}
			dst := color.NRGBA{R: q[0], G: q[1], B: q[2], A: 255}
			out.SetNRGBA(x, y, dst)
			seen[uint32(dst.R)<<16|uint32(dst.G)<<8|uint32(dst.B)] = struct{}{}
			if mode == DitherFloyd || mode == DitherAtkinson {
				for ch := 0; ch < 3; ch++ {
					err := v[ch] - float64(q[ch])
					diffuse(err, x, y, w, h, mode, errR, errG, errB, ch)
				}
			}
		}
	}
	_ = threshold
	return out, len(seen)
}

// diffuse spreads one channel's quantisation error to the neighbouring pixels.
func diffuse(err float64, x, y, w, h int, mode string, errR, errG, errB [][]float64, ch int) {
	plane := errR
	if ch == 1 {
		plane = errG
	} else if ch == 2 {
		plane = errB
	}
	add := func(px, py int, factor float64) {
		if px < 0 || py < 0 || px >= w || py >= h {
			return
		}
		plane[py][px] += err * factor
	}
	switch mode {
	case DitherFloyd:
		add(x+1, y, 7.0/16)
		add(x-1, y+1, 3.0/16)
		add(x, y+1, 5.0/16)
		add(x+1, y+1, 1.0/16)
	case DitherAtkinson:
		// Atkinson diffuses only 6/8 of the error, which is what makes it keep
		// more contrast than Floyd–Steinberg.
		add(x+1, y, 1.0/8)
		add(x+2, y, 1.0/8)
		add(x-1, y+1, 1.0/8)
		add(x, y+1, 1.0/8)
		add(x+1, y+1, 1.0/8)
		add(x, y+2, 1.0/8)
	}
}

// buildPalette builds the evenly spaced luminance (or colour) steps.
func buildPalette(levels int, gray bool) []uint8 {
	out := make([]uint8, levels)
	for i := 0; i < levels; i++ {
		out[i] = uint8(float64(i)*255/float64(levels-1) + 0.5)
	}
	return out
}

// nearestPaletteIndex finds the closest step.
func nearestPaletteIndex(v float64, palette []uint8) int {
	best := 0
	bestDist := math1e9
	for i, p := range palette {
		d := v - float64(p)
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

const math1e9 = 1e18

// bayerValue returns the 4×4 ordered-dither threshold at one pixel, in 0..1.
func bayerValue(x, y int) float64 {
	matrix := [4][4]int{
		{0, 8, 2, 10},
		{12, 4, 14, 6},
		{3, 11, 1, 9},
		{15, 7, 13, 5},
	}
	return (float64(matrix[y%4][x%4]) + 0.5) / 16
}

// writeOneBitPNG writes the picture as a real bitDepth=1 PNG.
func writeOneBitPNG(img image.Image, req PixelRequest, target string) error {
	black := markColor(req.Black)
	if strings.TrimSpace(req.Black) == "" {
		black = color.NRGBA{A: 255}
	}
	white := markColor(req.White)
	if strings.TrimSpace(req.White) == "" {
		white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	mask := png1bit.FromImage(img, 128, req.Invert, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	raw, err := png1bit.Encode(mask, png1bit.Options{ColorType: png1bit.Gray, Black: black, White: white})
	if err != nil {
		return err
	}
	return writeBytes(target, raw)
}

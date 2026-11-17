package engine

// filters.go implements the traditional filter library.
//
// Every filter is a pure function of the pixels, and every filter honours the
// strength control: at 0% the result equals the source within one code value,
// which is what the PRD asks the acceptance test to assert. Nothing here claims
// to be an AI restoration effect.

import (
	"image"
	"image/color"
	"math"
)

// FilterRequest is one filter application, optionally combined with another.
type FilterRequest struct {
	// Kind selects the filter family.
	Kind string `json:"kind"`
	// Strength is 0..100.
	Strength float64 `json:"strength"`
	// Amount is the filter's own primary parameter, 0..100.
	Amount float64 `json:"amount"`
	// Radius drives the blur family, in pixels.
	Radius float64 `json:"radius"`
	// Levels is the palette size of the posterise filter.
	Levels int `json:"levels"`
	// Angle is the direction of the motion blur, in degrees.
	Angle float64 `json:"angle"`
}

// FilterKind names one filter in the catalogue the page lists.
type FilterKind struct {
	Key    string
	Label  string
	Group  string
	Params string
}

// FilterCatalog is the filter ladder, in the order the page shows it. Group is
// the panel's section, Params describes which control the filter uses.
var FilterCatalog = []FilterKind{
	{Key: "grayscale", Label: "黑白", Group: "基础"},
	{Key: "invert", Label: "反相", Group: "基础"},
	{Key: "sepia", Label: "复古", Group: "基础"},
	{Key: "warm", Label: "暖色", Group: "基础"},
	{Key: "cool", Label: "冷色", Group: "基础"},
	{Key: "grain", Label: "胶片颗粒", Group: "纹理", Params: "amount"},
	{Key: "vignette", Label: "暗角", Group: "纹理", Params: "amount"},
	{Key: "blur", Label: "高斯模糊", Group: "算法", Params: "radius"},
	{Key: "motion", Label: "运动模糊", Group: "算法", Params: "radius"},
	{Key: "sharpen", Label: "锐化", Group: "算法", Params: "amount"},
	{Key: "emboss", Label: "浮雕", Group: "算法"},
	{Key: "edges", Label: "边缘检测", Group: "算法"},
	{Key: "sketch", Label: "素描", Group: "算法"},
	{Key: "pixelate", Label: "像素化", Group: "算法", Params: "radius"},
	{Key: "posterize", Label: "减色", Group: "风格", Params: "levels"},
}

// FilterLabel resolves a key to the label the page prints.
func FilterLabel(key string) string {
	for _, k := range FilterCatalog {
		if k.Key == key {
			return k.Label
		}
	}
	return key
}

// ApplyFilter runs one filter at the requested strength.
func ApplyFilter(src image.Image, req FilterRequest) image.Image {
	if src == nil {
		return nil
	}
	strength := clampF(req.Strength, 0, 100) / 100
	if strength <= 0 {
		// 0% must be the source itself, not a blended near-copy: returning src
		// keeps the acceptance check exact.
		return src
	}
	filtered := runFilter(src, req)
	if strength >= 1 {
		return filtered
	}
	return blendImages(src, filtered, strength)
}

// runFilter dispatches to one filter at full strength.
func runFilter(src image.Image, req FilterRequest) image.Image {
	switch req.Kind {
	case "grayscale":
		return mapPixels(src, func(c color.NRGBA) color.NRGBA {
			l := uint8(0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B) + 0.5)
			return color.NRGBA{R: l, G: l, B: l, A: c.A}
		})
	case "invert":
		return mapPixels(src, func(c color.NRGBA) color.NRGBA {
			return color.NRGBA{R: 255 - c.R, G: 255 - c.G, B: 255 - c.B, A: c.A}
		})
	case "sepia":
		return mapPixels(src, func(c color.NRGBA) color.NRGBA {
			r := 0.393*float64(c.R) + 0.769*float64(c.G) + 0.189*float64(c.B)
			g := 0.349*float64(c.R) + 0.686*float64(c.G) + 0.168*float64(c.B)
			b := 0.272*float64(c.R) + 0.534*float64(c.G) + 0.131*float64(c.B)
			return color.NRGBA{R: clampTo8(r), G: clampTo8(g), B: clampTo8(b), A: c.A}
		})
	case "warm":
		return mapPixels(src, func(c color.NRGBA) color.NRGBA {
			return color.NRGBA{
				R: clampTo8(float64(c.R) * 1.10),
				G: clampTo8(float64(c.G) * 1.02),
				B: clampTo8(float64(c.B) * 0.88),
				A: c.A,
			}
		})
	case "cool":
		return mapPixels(src, func(c color.NRGBA) color.NRGBA {
			return color.NRGBA{
				R: clampTo8(float64(c.R) * 0.88),
				G: clampTo8(float64(c.G) * 1.0),
				B: clampTo8(float64(c.B) * 1.12),
				A: c.A,
			}
		})
	case "grain":
		return grainFilter(src, clampF(req.Amount, 0, 100)/100)
	case "vignette":
		return vignetteFilter(src, clampF(req.Amount, 0, 100)/100)
	case "blur":
		return boxBlur(src, int(clampF(req.Radius, 1, 64)))
	case "motion":
		return motionBlur(src, int(clampF(req.Radius, 1, 64)), req.Angle)
	case "sharpen":
		return sharpenFilter(src, clampF(req.Amount, 0, 100)/100)
	case "emboss":
		return embossFilter(src)
	case "edges":
		return edgeFilter(src)
	case "sketch":
		// A sketch is the edge magnitude inverted, with the paper staying white.
		edges := edgeFilter(src)
		return mapPixels(edges, func(c color.NRGBA) color.NRGBA {
			l := 255 - c.R
			return color.NRGBA{R: l, G: l, B: l, A: c.A}
		})
	case "pixelate":
		return pixelateFilter(src, int(clampF(req.Radius, 2, 128)))
	case "posterize":
		levels := req.Levels
		if levels < 2 || levels > 32 {
			levels = 6
		}
		return posterizeFilter(src, levels)
	}
	return src
}

// mapPixels applies fn to every pixel and returns an NRGBA copy.
func mapPixels(src image.Image, fn func(color.NRGBA) color.NRGBA) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			dst.SetNRGBA(x, y, fn(c))
		}
	}
	return dst
}

// blendImages mixes two same-sized images: k = 0 keeps base, k = 1 keeps top.
func blendImages(base, top image.Image, k float64) *image.NRGBA {
	b := base.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	k = clampF(k, 0, 1)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			a := color.NRGBAModel.Convert(base.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			c := color.NRGBAModel.Convert(top.At(top.Bounds().Min.X+x, top.Bounds().Min.Y+y)).(color.NRGBA)
			mix := func(p, q uint8) uint8 { return uint8(float64(p) + (float64(q)-float64(p))*k + 0.5) }
			dst.SetNRGBA(x, y, color.NRGBA{
				R: mix(a.R, c.R), G: mix(a.G, c.G), B: mix(a.B, c.B), A: mix(a.A, c.A),
			})
		}
	}
	return dst
}

// grainFilter adds a deterministic noise field. A fixed seed keeps the result
// reproducible, which the PRD requires of any effect that generates pixels.
func grainFilter(src image.Image, amount float64) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	// A small linear congruential generator: the exact quality of the noise does
	// not matter, the reproducibility does.
	var state uint32 = 0x9E3779B9
	next := func() float64 {
		state = state*1664525 + 1013904223
		return float64(state>>16)/65535.0*2 - 1
	}
	scale := amount * 48
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			n := next() * scale
			dst.SetNRGBA(x, y, color.NRGBA{
				R: clampTo8(float64(c.R) + n),
				G: clampTo8(float64(c.G) + n),
				B: clampTo8(float64(c.B) + n),
				A: c.A,
			})
		}
	}
	return dst
}

// vignetteFilter darkens towards the corners.
func vignetteFilter(src image.Image, amount float64) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	cx, cy := float64(b.Dx())/2, float64(b.Dy())/2
	maxR := math.Hypot(cx, cy)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			d := math.Hypot(float64(x)-cx, float64(y)-cy) / maxR
			// Full strength at the centre, nothing beyond 85% of the radius.
			k := 1 - amount*math.Pow(clampF(d, 0, 1), 2)*1.2
			if k < 0 {
				k = 0
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: clampTo8(float64(c.R) * k),
				G: clampTo8(float64(c.G) * k),
				B: clampTo8(float64(c.B) * k),
				A: c.A,
			})
		}
	}
	return dst
}

// boxBlur is a separable box blur run three times, which approximates a Gaussian
// closely enough for a preview and stays O(1) per pixel in the radius.
func boxBlur(src image.Image, radius int) *image.NRGBA {
	if radius < 1 {
		return mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	}
	cur := mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	for pass := 0; pass < 3; pass++ {
		cur = blurAxis(cur, radius, true)
		cur = blurAxis(cur, radius, false)
	}
	return cur
}

// blurAxis blurs along one axis with a sliding sum.
//
// The window clamps at the image border instead of dropping out-of-range samples:
// dropping them divides by a wider span than the samples that actually landed in
// the window, which quietly darkens every edge pixel.
func blurAxis(src *image.NRGBA, radius int, horizontal bool) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(src.Bounds())
	lines, length := h, w
	if !horizontal {
		lines, length = w, h
	}
	span := 2*radius + 1
	at := func(line, i int) color.NRGBA {
		if i < 0 {
			i = 0
		}
		if i >= length {
			i = length - 1
		}
		if horizontal {
			return src.NRGBAAt(i, line)
		}
		return src.NRGBAAt(line, i)
	}
	set := func(line, i int, c color.NRGBA) {
		if horizontal {
			dst.SetNRGBA(i, line, c)
			return
		}
		dst.SetNRGBA(line, i, c)
	}
	for line := 0; line < lines; line++ {
		var sumR, sumG, sumB, sumA int
		for i := -radius; i <= radius; i++ {
			c := at(line, i)
			sumR += int(c.R)
			sumG += int(c.G)
			sumB += int(c.B)
			sumA += int(c.A)
		}
		for i := 0; i < length; i++ {
			set(line, i, color.NRGBA{
				R: uint8(sumR / span), G: uint8(sumG / span),
				B: uint8(sumB / span), A: uint8(sumA / span),
			})
			out := at(line, i-radius)
			in := at(line, i+radius+1)
			sumR += int(in.R) - int(out.R)
			sumG += int(in.G) - int(out.G)
			sumB += int(in.B) - int(out.B)
			sumA += int(in.A) - int(out.A)
		}
	}
	return dst
}

// motionBlur averages along one direction.
func motionBlur(src image.Image, radius int, angle float64) *image.NRGBA {
	if radius < 1 {
		return mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	}
	b := src.Bounds()
	sample := mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	dst := image.NewNRGBA(sample.Bounds())
	rad := angle * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			var sr, sg, sb, sa, n int
			for t := -radius; t <= radius; t++ {
				sx := x + int(float64(t)*dx+0.5)
				sy := y + int(float64(t)*dy+0.5)
				if sx < 0 || sy < 0 || sx >= b.Dx() || sy >= b.Dy() {
					continue
				}
				c := sample.NRGBAAt(sx, sy)
				sr += int(c.R)
				sg += int(c.G)
				sb += int(c.B)
				sa += int(c.A)
				n++
			}
			if n == 0 {
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(sr / n), G: uint8(sg / n), B: uint8(sb / n), A: uint8(sa / n),
			})
		}
	}
	return dst
}

// unsharpMask is a 3×3 kernel applied at the given amount.
func sharpenFilter(src image.Image, amount float64) *image.NRGBA {
	base := mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	blurred := boxBlur(base, 1)
	k := amount
	if k > 1 {
		k = 1
	}
	dst := image.NewNRGBA(base.Bounds())
	w, h := base.Bounds().Dx(), base.Bounds().Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := base.NRGBAAt(x, y)
			b := blurred.NRGBAAt(x, y)
			sharpen := func(v, blur uint8) uint8 {
				return clampTo8(float64(v) + (float64(v)-float64(blur))*k*1.5)
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: sharpen(c.R, b.R), G: sharpen(c.G, b.G), B: sharpen(c.B, b.B), A: c.A,
			})
		}
	}
	return dst
}

// greyOf is the luminance the convolution filters operate on.
func greyOf(src image.Image) ([]float64, int, int) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			out[y*w+x] = 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
		}
	}
	return out, w, h
}

// embossFilter applies a diagonal relief kernel.
func embossFilter(src image.Image) *image.NRGBA {
	grey, w, h := greyOf(src)
	base := mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	dst := image.NewNRGBA(base.Bounds())
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 128
		}
		return grey[y*w+x]
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := 128 + (at(x+1, y+1) - at(x-1, y-1))
			l := clampTo8(v)
			dst.SetNRGBA(x, y, color.NRGBA{R: l, G: l, B: l, A: base.NRGBAAt(x, y).A})
		}
	}
	return dst
}

// edgeFilter is the Sobel magnitude.
func edgeFilter(src image.Image) *image.NRGBA {
	grey, w, h := greyOf(src)
	base := mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	dst := image.NewNRGBA(base.Bounds())
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		return grey[y*w+x]
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gx := -at(x-1, y-1) + at(x+1, y-1) - 2*at(x-1, y) + 2*at(x+1, y) - at(x-1, y+1) + at(x+1, y+1)
			gy := -at(x-1, y-1) - 2*at(x, y-1) - at(x+1, y-1) + at(x-1, y+1) + 2*at(x, y+1) + at(x+1, y+1)
			mag := math.Hypot(gx, gy)
			l := clampTo8(mag)
			dst.SetNRGBA(x, y, color.NRGBA{R: l, G: l, B: l, A: base.NRGBAAt(x, y).A})
		}
	}
	return dst
}

// pixelateFilter averages square blocks.
func pixelateFilter(src image.Image, block int) *image.NRGBA {
	base := mapPixels(src, func(c color.NRGBA) color.NRGBA { return c })
	w, h := base.Bounds().Dx(), base.Bounds().Dy()
	dst := image.NewNRGBA(base.Bounds())
	for by := 0; by < h; by += block {
		for bx := 0; bx < w; bx += block {
			var sr, sg, sb, sa, n int
			for y := by; y < by+block && y < h; y++ {
				for x := bx; x < bx+block && x < w; x++ {
					c := base.NRGBAAt(x, y)
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
			for y := by; y < by+block && y < h; y++ {
				for x := bx; x < bx+block && x < w; x++ {
					dst.SetNRGBA(x, y, avg)
				}
			}
		}
	}
	return dst
}

// posterizeFilter quantises each channel to the requested number of levels.
func posterizeFilter(src image.Image, levels int) *image.NRGBA {
	if levels < 2 {
		levels = 2
	}
	step := 255.0 / float64(levels-1)
	return mapPixels(src, func(c color.NRGBA) color.NRGBA {
		q := func(v uint8) uint8 {
			i := math.Floor(float64(v)/step + 0.5)
			return clampTo8(i * step)
		}
		return color.NRGBA{R: q(c.R), G: q(c.G), B: q(c.B), A: c.A}
	})
}

package engine

// grade.go is the professional colour engine behind the 专业调色 page.
//
// The working space is documented in one place: the pixels are decoded to sRGB,
// exposure is applied in linear light (the only place where "EV" means anything),
// and every other control works on the gamma-encoded value, which is what a
// generic 8-bit pipeline can promise honestly. The page labels itself 8-bit for
// that reason instead of claiming an arbitrary high-bit-depth workflow.
//
// Every control is a pure function of the source pixels: applying a parameter set
// twice, or resetting one control, never accumulates quantisation error beyond
// the single final encode.

import (
	"image"
	"image/color"
	"math"
)

// GradeParams is the parameter set of one grading step. All fields default to
// neutral; a zero value is the identity.
type GradeParams struct {
	// Exposure is measured in stops, -5..5.
	Exposure float64
	// Brightness shifts the encoded value, -100..100.
	Brightness float64
	// Contrast pivots around mid grey, -100..100.
	Contrast float64
	// Highlights and Shadows recover the two ends, -100..100.
	Highlights float64
	Shadows    float64
	// Whites and Blacks move the clipping points, -100..100.
	Whites float64
	Blacks float64
	// Saturation and Vibrance, -100..100. Vibrance protects already saturated
	// colours, so skin tones do not go orange first.
	Saturation float64
	Vibrance   float64
	// Temperature and Tint are white-balance offsets, -100..100. They are
	// implemented as a gain on the channel pair the name implies, not as an
	// arbitrary per-channel bias presented as "colour temperature".
	Temperature float64
	Tint        float64
	// Levels describe the input black/gamma/white points and the output range.
	// Zero values mean "identity": 0, 1, 255, 0, 255.
	InputBlack  float64
	InputGamma  float64
	InputWhite  float64
	OutputBlack float64
	OutputWhite float64
	// Curve is an optional control-point curve applied after levels.
	Curve Curve
	// CurveChannel limits the curve to one channel: "" (all), "r", "g", "b".
	CurveChannel string
	// LUT is an optional 3D lookup table.
	LUT *LUT3D
	// LUTStrength is 0..100.
	LUTStrength float64
}

// Identity reports whether the parameters change nothing.
func (p GradeParams) Identity() bool {
	if p.Exposure != 0 || p.Brightness != 0 || p.Contrast != 0 ||
		p.Highlights != 0 || p.Shadows != 0 || p.Whites != 0 || p.Blacks != 0 ||
		p.Saturation != 0 || p.Vibrance != 0 || p.Temperature != 0 || p.Tint != 0 {
		return false
	}
	if p.InputBlack != 0 || (p.InputGamma != 0 && p.InputGamma != 1) || p.InputWhite != 0 ||
		p.OutputBlack != 0 || p.OutputWhite != 0 {
		return false
	}
	if len(p.Curve.Points) > 1 {
		return false
	}
	if p.LUT != nil && p.LUTStrength > 0 {
		return false
	}
	return true
}

// Grade applies params to src and returns a new NRGBA image.
//
// A neutral parameter set returns the source untouched, so a page can call Grade
// on every preview frame without a copy when nothing is set.
func Grade(src image.Image, p GradeParams) image.Image {
	if src == nil {
		return nil
	}
	if p.Identity() {
		return src
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))

	exposureGain := math.Pow(2, clampF(p.Exposure, -5, 5))
	contrast := clampF(p.Contrast, -100, 100) / 100
	brightness := clampF(p.Brightness, -100, 100) * (255.0 / 200.0)
	highlights := clampF(p.Highlights, -100, 100) / 100
	shadows := clampF(p.Shadows, -100, 100) / 100
	whites := clampF(p.Whites, -100, 100) / 100
	blacks := clampF(p.Blacks, -100, 100) / 100
	saturation := 1 + clampF(p.Saturation, -100, 100)/100
	vibrance := clampF(p.Vibrance, -100, 100) / 100
	temp := clampF(p.Temperature, -100, 100) / 100
	tint := clampF(p.Tint, -100, 100) / 100

	inBlack := clampF(defaultF(p.InputBlack, 0), 0, 254)
	inWhite := clampF(defaultF(p.InputWhite, 255), inBlack+1, 255)
	gamma := clampF(defaultF(p.InputGamma, 1), 0.1, 10)
	outBlack := clampF(defaultF(p.OutputBlack, 0), 0, 255)
	outWhite := clampF(defaultF(p.OutputWhite, 255), 0, 255)
	levelsSpan := inWhite - inBlack
	outSpan := outWhite - outBlack

	// The curve is expanded once into a 256-entry table per affected channel,
	// so the per-pixel cost is a lookup instead of a spline evaluation.
	curveTables := p.Curve.tables(p.CurveChannel)

	var lut *LUT3D
	lutStrength := 0.0
	if p.LUT != nil && p.LUTStrength > 0 {
		lut = p.LUT
		lutStrength = clampF(p.LUTStrength, 0, 100) / 100
	}

	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			// Straight-alpha channels: grading a premultiplied value would darken
			// every translucent edge, which is the classic black-halo bug.
			r, g, bl := float64(c.R), float64(c.G), float64(c.B)

			if exposureGain != 1 {
				r = linearToSRGB(srgbToLinear(r) * exposureGain)
				g = linearToSRGB(srgbToLinear(g) * exposureGain)
				bl = linearToSRGB(srgbToLinear(bl) * exposureGain)
			}
			r += brightness
			g += brightness
			bl += brightness

			if contrast != 0 {
				r = pivot(r, contrast)
				g = pivot(g, contrast)
				bl = pivot(bl, contrast)
			}
			if whites != 0 {
				r = moveEnd(r, whites, true)
				g = moveEnd(g, whites, true)
				bl = moveEnd(bl, whites, true)
			}
			if blacks != 0 {
				r = moveEnd(r, blacks, false)
				g = moveEnd(g, blacks, false)
				bl = moveEnd(bl, blacks, false)
			}
			if highlights != 0 || shadows != 0 {
				lum := 0.2126*r + 0.7152*g + 0.0722*bl
				delta := toneRecovery(lum, highlights, shadows)
				if delta != 0 {
					r += delta
					g += delta
					bl += delta
				}
			}
			if levelsSpan != 255 || inBlack != 0 || gamma != 1 || outSpan != 255 || outBlack != 0 {
				r = levels(r, inBlack, levelsSpan, gamma, outBlack, outSpan)
				g = levels(g, inBlack, levelsSpan, gamma, outBlack, outSpan)
				bl = levels(bl, inBlack, levelsSpan, gamma, outBlack, outSpan)
			}
			if temp != 0 || tint != 0 {
				// A positive temperature warms: red gains, blue loses. Tint moves
				// the green/magenta axis instead, which is why they are separate
				// controls rather than one "colour" slider.
				r *= 1 + temp*0.25
				bl *= 1 - temp*0.25
				g *= 1 - tint*0.18
				r *= 1 + tint*0.08
				bl *= 1 + tint*0.08
			}
			if saturation != 1 || vibrance != 0 {
				lum := 0.2126*r + 0.7152*g + 0.0722*bl
				if vibrance != 0 {
					// Vibrance scales with how unsaturated the pixel already is.
					span := (maxOf3(r, g, bl) - minOf3(r, g, bl)) / 255
					factor := vibrance * (1 - span)
					r = lum + (r-lum)*(1+factor)
					g = lum + (g-lum)*(1+factor)
					bl = lum + (bl-lum)*(1+factor)
				}
				if saturation != 1 {
					r = lum + (r-lum)*saturation
					g = lum + (g-lum)*saturation
					bl = lum + (bl-lum)*saturation
				}
			}
			if curveTables != nil {
				r = curveTables[0].lookup(r)
				g = curveTables[1].lookup(g)
				bl = curveTables[2].lookup(bl)
			}
			if lut != nil {
				lr, lg, lb := lut.Apply(r, g, bl)
				r = r + (lr-r)*lutStrength
				g = g + (lg-g)*lutStrength
				bl = bl + (lb-bl)*lutStrength
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: clampTo8(r),
				G: clampTo8(g),
				B: clampTo8(bl),
				A: c.A,
			})
		}
	}
	return dst
}

// pivot applies a contrast S-curve around mid grey. A positive amount stretches
// the range, a negative one compresses it; both keep 0 and 255 fixed so the
// endpoints do not drift.
func pivot(v, amount float64) float64 {
	k := 1 + amount
	m := v - 127.5
	return 127.5 + m*k
}

// moveEnd moves the white or black clipping point, leaving the opposite end
// alone: raising whites brightens everything above mid grey.
func moveEnd(v, amount float64, whiteEnd bool) float64 {
	if whiteEnd {
		if amount > 0 {
			return v + (255-v)*amount
		}
		return v + v*amount*0.5
	}
	if amount > 0 {
		return v - v*amount
	}
	return v - (255-v)*amount*0.5
}

// toneRecovery lifts shadows and pulls highlights without touching mid grey.
func toneRecovery(lum, highlights, shadows float64) float64 {
	var delta float64
	if shadows != 0 {
		// A bell centred at 0 with its weight on the dark end.
		weight := math.Pow(1-clampF(lum/255, 0, 1), 2)
		delta += shadows * 70 * weight
	}
	if highlights != 0 {
		weight := math.Pow(clampF(lum/255, 0, 1), 2)
		delta -= highlights * 70 * weight
	}
	return delta
}

// levels maps the input range onto the output range through a gamma.
func levels(v, inBlack, span, gamma, outBlack, outSpan float64) float64 {
	t := (v - inBlack) / span
	t = clampF(t, 0, 1)
	if gamma != 1 {
		t = math.Pow(t, 1/gamma)
	}
	return outBlack + t*outSpan
}

func srgbToLinear(v float64) float64 {
	t := clampF(v, 0, 255) / 255
	if t <= 0.04045 {
		return t / 12.92 * 255
	}
	return math.Pow((t+0.055)/1.055, 2.4) * 255
}

func linearToSRGB(v float64) float64 {
	t := clampF(v, 0, 255) / 255
	if t <= 0.0031308 {
		return t * 12.92 * 255
	}
	return (1.055*math.Pow(t, 1/2.4) - 0.055) * 255
}

func clampTo8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func clampF(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// defaultF returns v when it was set, otherwise the neutral value.
func defaultF(v, neutral float64) float64 {
	if v == 0 {
		return neutral
	}
	return v
}

func maxOf3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }
func minOf3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }

// Curve is a control-point tone curve. Points are in 0..255 on both axes and are
// kept sorted by X; a curve with fewer than two points is the identity.
type Curve struct {
	Points []CurvePoint `json:"points"`
}

// CurvePoint is one anchor of a tone curve.
type CurvePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// table is one expanded 256-entry lookup.
type curveTable [256]uint8

func (t curveTable) lookup(v float64) float64 {
	i := int(v + 0.5)
	if i < 0 {
		i = 0
	}
	if i > 255 {
		i = 255
	}
	return float64(t[i])
}

// tables expands the curve into per-channel lookups. A nil result means "no
// curve", which keeps the per-pixel loop free of a branch per channel.
func (c Curve) tables(channel string) *[3]curveTable {
	if len(c.Points) < 2 {
		return nil
	}
	base := c.expand()
	var out [3]curveTable
	names := [3]string{"r", "g", "b"}
	for ch := 0; ch < 3; ch++ {
		if channel != "" && channel != names[ch] {
			out[ch] = identityTable()
			continue
		}
		out[ch] = base
	}
	return &out
}

func identityTable() curveTable {
	var t curveTable
	for i := 0; i < 256; i++ {
		t[i] = uint8(i)
	}
	return t
}

// expand builds the 256-entry table with monotone cubic interpolation between the
// control points. Monotone rather than Catmull-Rom on purpose: an overshooting
// spline makes a tone curve dip below its own anchor, which shows up as a
// reversal in a gradient.
func (c Curve) expand() curveTable {
	pts := append([]CurvePoint(nil), c.Points...)
	// Sort by X with a small insertion sort: a curve has a handful of points.
	for i := 1; i < len(pts); i++ {
		for j := i; j > 0 && pts[j].X < pts[j-1].X; j-- {
			pts[j], pts[j-1] = pts[j-1], pts[j]
		}
	}
	// The endpoints are clamped to the axes: a curve that starts above 0 or ends
	// below 255 would otherwise leave a flat, unpaintable band.
	pts[0].X = 0
	pts[len(pts)-1].X = 255
	for i := range pts {
		pts[i].Y = clampF(pts[i].Y, 0, 255)
	}
	var t curveTable
	for i := 0; i < 256; i++ {
		t[i] = uint8(clampF(evalCurve(pts, float64(i)), 0, 255) + 0.5)
	}
	return t
}

// evalCurve interpolates the sorted points at x.
func evalCurve(pts []CurvePoint, x float64) float64 {
	if x <= pts[0].X {
		return pts[0].Y
	}
	last := len(pts) - 1
	if x >= pts[last].X {
		return pts[last].Y
	}
	i := 0
	for i < last && pts[i+1].X < x {
		i++
	}
	a, b := pts[i], pts[i+1]
	dx := b.X - a.X
	if dx <= 0 {
		return a.Y
	}
	t := (x - a.X) / dx
	// Fritsch–Carlson monotone Hermite: the tangents are limited so one segment
	// cannot overshoot its own anchors, which is what would show as a reversal in
	// a smooth gradient.
	slope := (b.Y - a.Y) / dx
	m0 := clampF(tangentAt(pts, i), -3*slope, 3*slope)
	m1 := clampF(tangentAt(pts, i+1), -3*slope, 3*slope)
	t2 := t * t
	t3 := t2 * t
	h00 := 2*t3 - 3*t2 + 1
	h10 := t3 - 2*t2 + t
	h01 := -2*t3 + 3*t2
	h11 := t3 - t2
	return h00*a.Y + h10*dx*m0 + h01*b.Y + h11*dx*m1
}

// tangentAt is the central-difference slope at one point.
func tangentAt(pts []CurvePoint, i int) float64 {
	if i == 0 {
		return (pts[1].Y - pts[0].Y) / maxFloat64(pts[1].X-pts[0].X, 1e-6)
	}
	if i == len(pts)-1 {
		return (pts[i].Y - pts[i-1].Y) / maxFloat64(pts[i].X-pts[i-1].X, 1e-6)
	}
	return (pts[i+1].Y - pts[i-1].Y) / maxFloat64(pts[i+1].X-pts[i-1].X, 1e-6)
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// FilterPreset is one named grading preset offered by the page.
type FilterPreset struct {
	Key    string
	Label  string
	Params GradeParams
}

// GradePresets are the built-in looks. They are plain parameter sets, so every
// value stays visible and editable in the panel instead of being an opaque
// "application effect".
var GradePresets = []FilterPreset{
	{Key: "", Label: "原始", Params: GradeParams{}},
	{Key: "bright", Label: "明亮", Params: GradeParams{Exposure: 0.25, Contrast: 8, Vibrance: 12, Shadows: 10}},
	{Key: "vivid", Label: "鲜艳", Params: GradeParams{Contrast: 15, Saturation: 22, Vibrance: 18}},
	{Key: "soft", Label: "柔和", Params: GradeParams{Contrast: -12, Highlights: -15, Shadows: 18, Saturation: -8}},
	{Key: "warm", Label: "暖调", Params: GradeParams{Temperature: 28, Vibrance: 8}},
	{Key: "cool", Label: "冷调", Params: GradeParams{Temperature: -28, Contrast: 6}},
	{Key: "mono", Label: "黑白", Params: GradeParams{Saturation: -100, Contrast: 12}},
	{Key: "film", Label: "胶片", Params: GradeParams{Contrast: -6, Blacks: 18, Highlights: -10, Temperature: 12, Curve: Curve{Points: []CurvePoint{{0, 18}, {128, 132}, {255, 244}}}}},
}

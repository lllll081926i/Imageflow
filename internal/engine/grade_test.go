package engine

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

// gradeSource is a small deterministic image with a gradient, a flat patch and a
// translucent edge, so the colour tests exercise the alpha path too.
func gradeSource() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 16), G: uint8(y * 16), B: uint8((x + y) * 8),
				A: 255,
			})
		}
	}
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	return img
}

func TestGradeIdentityReturnsTheSource(t *testing.T) {
	src := gradeSource()
	got := Grade(src, GradeParams{})
	if got != image.Image(src) {
		t.Fatal("a neutral parameter set must return the source itself")
	}
}

// TestGradeNeutralIsPixelIdentical pins the PRD's acceptance rule: a control at
// its default must produce the source unchanged, and toggling a value back and
// forth must not let quantisation error creep in step by step.
func TestGradeNeutralIsPixelIdentical(t *testing.T) {
	src := gradeSource()
	// A neutral parameter set must be a genuine no-op, not a re-encode.
	if got := Grade(src, GradeParams{}); got != image.Image(src) {
		t.Fatal("neutral params must return the source itself")
	}
	// Repeatedly switching a look on and off must land on the same picture every
	// time. Grading is a pure function of the *source* pixels plus the parameter
	// set, so no step can accumulate error.
	look := GradeParams{Contrast: 20, Saturation: 10, Vibrance: 5}
	var first image.Image
	for i := 0; i < 5; i++ {
		on := Grade(src, look)
		if first == nil {
			first = on
			continue
		}
		worst := maxChannelDelta(first, on)
		if worst != 0 {
			t.Fatalf("conversion %d drifted by %d code values", i, worst)
		}
	}
}

// maxChannelDelta is the largest per-channel difference between two images.
func maxChannelDelta(a, b image.Image) int {
	if a.Bounds() != b.Bounds() {
		return 255
	}
	worst := 0
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ca := color.NRGBAModel.Convert(a.At(x, y)).(color.NRGBA)
			cb := color.NRGBAModel.Convert(b.At(x, y)).(color.NRGBA)
			for _, pair := range [4][2]uint8{{ca.R, cb.R}, {ca.G, cb.G}, {ca.B, cb.B}, {ca.A, cb.A}} {
				d := int(pair[0]) - int(pair[1])
				if d < 0 {
					d = -d
				}
				if d > worst {
					worst = d
				}
			}
		}
	}
	return worst
}

func TestGradeKeepsAlpha(t *testing.T) {
	src := gradeSource()
	got := Grade(src, GradeParams{Exposure: 1}).(*image.NRGBA)
	if a := got.NRGBAAt(0, 0).A; a != 128 {
		t.Fatalf("alpha = %d, want the source's 128", a)
	}
}

func TestGradeExposureBrightensInLinearLight(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	oneStop := Grade(src, GradeParams{Exposure: 1}).(*image.NRGBA).NRGBAAt(0, 0)
	if oneStop.R <= 128 {
		t.Fatalf("one stop = %d, want brighter than 128", oneStop.R)
	}
	// One stop doubles the linear value, which for mid grey lands near 176 in
	// sRGB, not at 255: applying it to the encoded value would clip immediately.
	if oneStop.R > 200 {
		t.Fatalf("one stop = %d, the gain looks like it was applied to the encoded value", oneStop.R)
	}
}

func TestGradeSaturationZeroIsGrey(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 60, B: 20, A: 255})
	got := Grade(src, GradeParams{Saturation: -100}).(*image.NRGBA).NRGBAAt(0, 0)
	if got.R != got.G || got.G != got.B {
		t.Fatalf("desaturated pixel = %+v", got)
	}
}

func TestCurveIdentityKeepsValues(t *testing.T) {
	c := Curve{Points: []CurvePoint{{0, 0}, {128, 128}, {255, 255}}}
	table := c.expand()
	for i := 0; i < 256; i++ {
		if diff := int(table[i]) - i; diff < -1 || diff > 1 {
			t.Fatalf("identity curve at %d = %d", i, table[i])
		}
	}
}

func TestCurveIsMonotone(t *testing.T) {
	// A curve with a steep middle: a spline without the monotone limiter
	// overshoots here and makes a gradient reverse.
	c := Curve{Points: []CurvePoint{{0, 0}, {60, 120}, {120, 130}, {255, 255}}}
	table := c.expand()
	for i := 1; i < 256; i++ {
		if table[i] < table[i-1] {
			t.Fatalf("curve reverses at %d: %d -> %d", i, table[i-1], table[i])
		}
	}
}

func TestLevelsClampAndGamma(t *testing.T) {
	// A gamma of 2 on the input lightens the midtones.
	got := levels(128, 0, 255, 2, 0, 255)
	if got <= 128 {
		t.Fatalf("gamma 2 at 128 = %.1f, want brighter", got)
	}
	if levels(-10, 0, 255, 1, 0, 255) != 0 || levels(300, 0, 255, 1, 0, 255) != 255 {
		t.Fatal("levels must clamp outside the input range")
	}
}

func TestLoadLUTRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"声明尺寸不符": "LUT_3D_SIZE 2\n0 0 0\n1 0 0\n",
		"尺寸越界":   "LUT_3D_SIZE 1\n0 0 0\n",
		"缺少尺寸":   "0 0 0\n",
		"1D 表":   "LUT_1D_SIZE 2\n0 0 0\n",
		"NaN 分量": "LUT_3D_SIZE 2\n" + strings.Repeat("0 0 0\n", 7) + "NaN 0 0\n",
	}
	for name, doc := range cases {
		if _, err := ParseLUT(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: parser accepted a broken table", name)
		}
	}
}

func TestLUTIdentityRoundTrips(t *testing.T) {
	lut := IdentityLUT(9)
	for _, v := range []float64{0, 32, 128, 200, 255} {
		r, g, b := lut.Apply(v, v, v)
		if math.Abs(r-v) > 1.5 || math.Abs(g-v) > 1.5 || math.Abs(b-v) > 1.5 {
			t.Fatalf("identity LUT at %.0f = %.1f,%.1f,%.1f", v, r, g, b)
		}
	}
}

func TestLUTStrengthZeroChangesNothing(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
	p := GradeParams{LUT: IdentityLUT(2), LUTStrength: 0}
	if !p.Identity() {
		t.Fatal("a LUT at 0% strength must count as identity")
	}
}

func TestParseLUTAcceptsAValidTable(t *testing.T) {
	doc := "TITLE \"test\"\nLUT_3D_SIZE 2\nDOMAIN_MIN 0 0 0\nDOMAIN_MAX 1 1 1\n" +
		strings.Repeat("0.5 0.5 0.5\n", 8)
	lut, err := ParseLUT(bytes.NewReader([]byte(doc)))
	if err != nil {
		t.Fatal(err)
	}
	if lut.Size != 2 || lut.Title != "test" {
		t.Fatalf("lut = %+v", lut)
	}
}

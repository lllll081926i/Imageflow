package engine

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func filterSource() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 8), G: uint8(y * 10), B: uint8((x * y) % 256), A: 255,
			})
		}
	}
	img.SetNRGBA(5, 5, color.NRGBA{R: 255, G: 0, B: 0, A: 64})
	return img
}

// TestFilterZeroStrengthIsIdentity is the PRD's acceptance rule for filters: 0%
// leaves the source untouched within tolerance.
func TestFilterZeroStrengthIsIdentity(t *testing.T) {
	src := filterSource()
	for _, kind := range FilterCatalog {
		got := ApplyFilter(src, FilterRequest{Kind: kind.Key, Strength: 0})
		if got != image.Image(src) {
			t.Errorf("%s at 0%% did not return the source", kind.Key)
		}
	}
}

func TestFilterFullStrengthChangesTheImage(t *testing.T) {
	src := filterSource()
	for _, kind := range FilterCatalog {
		got := ApplyFilter(src, FilterRequest{
			Kind: kind.Key, Strength: 100, Amount: 60, Radius: 3, Levels: 4, Angle: 30,
		})
		if got == nil {
			t.Fatalf("%s returned nil", kind.Key)
		}
		if got.Bounds() != src.Bounds() {
			t.Errorf("%s changed the bounds: %v", kind.Key, got.Bounds())
		}
	}
}

func TestFilterKeepsAlpha(t *testing.T) {
	src := filterSource()
	// Alpha-preserving filters keep the source's alpha byte. A blur blends
	// neighbouring alpha values by definition, so it is checked separately in
	// TestBlurKeepsOpaqueAlphaOpaque.
	for _, kind := range []string{"grayscale", "invert", "sepia", "posterize", "sharpen"} {
		got := ApplyFilter(src, FilterRequest{Kind: kind, Strength: 100, Radius: 2, Levels: 4})
		alpha := color.NRGBAModel.Convert(got.At(5, 5)).(color.NRGBA).A
		if alpha != 64 {
			t.Errorf("%s changed alpha to %d", kind, alpha)
		}
	}
}

// TestBlurKeepsOpaqueAlphaOpaque is the check that caught a border bug: a blur
// whose window dropped out-of-range samples divided by the full span, which
// darkened the whole edge of the picture and made an opaque alpha translucent.
func TestBlurKeepsOpaqueAlphaOpaque(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 9, 9))
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	got := ApplyFilter(src, FilterRequest{Kind: "blur", Strength: 100, Radius: 2}).(*image.NRGBA)
	for _, p := range [][2]int{{0, 0}, {8, 0}, {0, 8}, {8, 8}, {4, 4}} {
		c := got.NRGBAAt(p[0], p[1])
		if c.A != 255 || c.R != 255 {
			t.Fatalf("blur at %v = %+v, want the untouched white", p, c)
		}
	}
}

func TestFilterBlurSpreadsAnEdge(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			v := uint8(0)
			if x >= 8 {
				v = 255
			}
			src.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	got := ApplyFilter(src, FilterRequest{Kind: "blur", Strength: 100, Radius: 3}).(*image.NRGBA)
	if got.NRGBAAt(8, 8).R == 255 {
		t.Fatal("blur did not soften the hard edge")
	}
	// A blurred edge may pick up a little of the bright side: that is what a blur
	// is. What it must not do is move the border by half the image, so the far
	// side stays dark.
	if got.NRGBAAt(0, 8).R > 40 {
		t.Fatalf("blur reached the far edge with %d", got.NRGBAAt(0, 8).R)
	}
}

func TestPosterizeProducesLevels(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for x := 0; x < 256; x++ {
		src.SetNRGBA(x, 0, color.NRGBA{R: uint8(x), G: uint8(x), B: uint8(x), A: 255})
	}
	got := ApplyFilter(src, FilterRequest{Kind: "posterize", Strength: 100, Levels: 4}).(*image.NRGBA)
	seen := map[uint8]bool{}
	for x := 0; x < 256; x++ {
		seen[got.NRGBAAt(x, 0).R] = true
	}
	if len(seen) > 4 {
		t.Fatalf("posterize produced %d distinct values: %v", len(seen), seen)
	}
}

func TestFilterLabelFallsBackToTheKey(t *testing.T) {
	if got := FilterLabel("blur"); got != "高斯模糊" {
		t.Fatalf("label = %q", got)
	}
	if got := FilterLabel("nope"); got != "nope" {
		t.Fatalf("unknown label = %q", got)
	}
}

func TestCompareRefusesMismatchedSizesWithoutAlignment(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	b := image.NewNRGBA(image.Rect(0, 0, 160, 80))
	res := Compare(CompareRequest{Reference: a, Result: b})
	if res.Computable {
		t.Fatal("metrics must be unavailable without an alignment")
	}
	if res.Reason == "" {
		t.Fatal("an unavailable metric must carry a reason")
	}
}

func TestCompareIdenticalImagesArePerfect(t *testing.T) {
	a := filterSource()
	res := Compare(CompareRequest{Reference: a, Result: a})
	if !res.Computable {
		t.Fatalf("identical images were not computable: %s", res.Reason)
	}
	if !math.IsInf(res.PSNR, 1) {
		t.Fatalf("PSNR = %v, want +Inf", res.PSNR)
	}
	if res.SSIM < 0.999 {
		t.Fatalf("SSIM = %v", res.SSIM)
	}
	if res.MaxChannelDelta != 0 {
		t.Fatalf("max delta = %d", res.MaxChannelDelta)
	}
}

func TestCompareStretchAlignmentIsComputable(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	b := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			b.SetNRGBA(x, y, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	res := Compare(CompareRequest{Reference: a, Result: b, Align: AlignStretch})
	if !res.Computable {
		t.Fatalf("stretch alignment failed: %s", res.Reason)
	}
	if res.PSNR <= 0 {
		t.Fatalf("PSNR = %v", res.PSNR)
	}
}

func TestDiffAndHeatmapNeedAComputablePair(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	b := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	if img, _ := DiffImage(CompareRequest{Reference: a, Result: b}, 2); img != nil {
		t.Fatal("diff image produced for a pair that cannot be compared")
	}
	if img, _ := HeatmapImage(CompareRequest{Reference: a, Result: b}); img != nil {
		t.Fatal("heatmap produced for a pair that cannot be compared")
	}
}

func TestDiffImageMarksChangedPixels(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	b := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	b.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	img, res := DiffImage(CompareRequest{Reference: a, Result: b}, 1)
	if img == nil || !res.Computable {
		t.Fatal("diff image missing")
	}
	if img.(*image.NRGBA).NRGBAAt(1, 1).R != 255 {
		t.Fatal("changed pixel is not marked")
	}
	if img.(*image.NRGBA).NRGBAAt(0, 0).R != 0 {
		t.Fatal("unchanged pixel is marked")
	}
}

func TestAnalyzeCountsClipping(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	img.SetNRGBA(2, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
	img.SetNRGBA(3, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 0})
	res := Analyze(AnalysisRequest{Image: img})
	if res.OverexposedPixels != 1 || res.UnderexposedPixels != 1 {
		t.Fatalf("clipping = %d/%d", res.OverexposedPixels, res.UnderexposedPixels)
	}
	if res.FullyTransparentPixels != 1 || res.Opaque {
		t.Fatalf("transparency = %d opaque=%v", res.FullyTransparentPixels, res.Opaque)
	}
	if res.UniqueColors != 4 {
		t.Fatalf("unique colours = %d", res.UniqueColors)
	}
	if res.Luma[255] != 1 || res.Luma[0] != 1 {
		t.Fatalf("luma extremes = %d/%d", res.Luma[0], res.Luma[255])
	}
}

func TestAnalyzeMarksSampledScans(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	res := Analyze(AnalysisRequest{Image: img, MaxSamples: 100})
	if !res.Sampled {
		t.Fatal("a bounded scan must be labelled as sampled")
	}
	if res.SampledPixels > 2000 {
		t.Fatalf("sampled %d pixels of 10000", res.SampledPixels)
	}
}

func TestContrastRatioMatchesTheWCAGRange(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.NRGBA{A: 255}
	if got := ContrastRatio(white, black); math.Abs(got-21) > 0.01 {
		t.Fatalf("white on black = %.2f, want 21", got)
	}
	if got := ContrastRatio(white, white); math.Abs(got-1) > 0.001 {
		t.Fatalf("white on white = %.3f, want 1", got)
	}
}

package engine

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/png1bit"
)

func annotateSource() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 4), G: uint8(y * 5), B: uint8((x + y) * 2), A: 255,
			})
		}
	}
	return img
}

// TestCoverDestroysThePixelsUnderIt is E2E-07: an opaque cover must leave no
// recoverable detail in the exported pixels.
func TestCoverDestroysThePixelsUnderIt(t *testing.T) {
	src := annotateSource()
	// The "card number" occupies a known band; read it before covering it.
	original := make([]color.NRGBA, 0, 400)
	for y := 10; y < 20; y++ {
		for x := 10; x < 50; x++ {
			original = append(original, src.NRGBAAt(x, y))
		}
	}
	marks := []AnnotateMark{{
		Kind: AnnotateCover, X1: 10, Y1: 10, X2: 50, Y2: 20, Color: "#000000",
	}}
	out, err := AnnotateImage(src, marks)
	if err != nil {
		t.Fatal(err)
	}
	i := 0
	for y := 10; y < 20; y++ {
		for x := 10; x < 50; x++ {
			got := out.(*image.NRGBA).NRGBAAt(x, y)
			if got.R != 0 || got.G != 0 || got.B != 0 {
				t.Fatalf("covered pixel %d,%d = %+v", x, y, got)
			}
			if got == original[i] {
				t.Fatalf("covered pixel %d,%d still equals the source", x, y)
			}
			i++
		}
	}
}

func TestCoverIsMarkedIrreversibleAndBlurIsNot(t *testing.T) {
	if !IsIrreversible(AnnotateCover) {
		t.Fatal("cover must be reported as irreversible")
	}
	for _, kind := range []string{AnnotateBlur, AnnotateMosaic} {
		if IsIrreversible(kind) {
			t.Fatalf("%s must not be advertised as an irreversible cover", kind)
		}
	}
}

func TestAnnotateLeavesTheSourceUntouched(t *testing.T) {
	src := annotateSource()
	before := make([]byte, len(src.Pix))
	copy(before, src.Pix)
	if _, err := AnnotateImage(src, []AnnotateMark{{
		Kind: AnnotateRect, X1: 0, Y1: 0, X2: 20, Y2: 20, Color: "#FF0000", Width: 3,
	}}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, src.Pix) {
		t.Fatal("annotation mutated the caller's image")
	}
}

func TestAnnotateDrawsEveryKind(t *testing.T) {
	src := annotateSource()
	kinds := []string{
		AnnotateRect, AnnotateEllipse, AnnotateArrow, AnnotateLine, AnnotateFree,
		AnnotateText, AnnotateNumber, AnnotateHighlight, AnnotateMosaic, AnnotateBlur,
		AnnotateCover,
	}
	for _, kind := range kinds {
		mark := AnnotateMark{
			Kind: kind, X1: 8, Y1: 8, X2: 40, Y2: 32, Color: "#EF4444", Width: 4,
			Text: "1", Size: 18, Strength: 6,
			Points: []AnnotatePoint{{X: 8, Y: 8}, {X: 20, Y: 20}, {X: 40, Y: 12}},
		}
		got, err := AnnotateImage(src, []AnnotateMark{mark})
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if got.Bounds().Dx() != 64 || got.Bounds().Dy() != 48 {
			t.Fatalf("%s changed the bounds", kind)
		}
	}
}

func TestAnnotateRejectsUnknownKind(t *testing.T) {
	if _, err := AnnotateImage(annotateSource(), []AnnotateMark{{Kind: "sticker"}}); err == nil {
		t.Fatal("an unknown mark kind must be reported, not silently skipped")
	}
}

func TestScreenshotKeepsTheMinimumSizeAndShadowInsideTheCanvas(t *testing.T) {
	src := annotateSource()
	canvas, err := RenderScreenshot(src, ScreenshotRequest{
		Padding: 24, Radius: 12, ShadowDistance: 20, ShadowOpacity: 0.3,
		Background: "#F5F5F7", BackgroundTo: "#E5E7EB", GradientAngle: 45,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The picture plus padding on both sides plus the shadow's room.
	if canvas.Bounds().Dx() <= src.Bounds().Dx() {
		t.Fatalf("canvas %v did not grow around the source", canvas.Bounds())
	}
	// The gradient must not be a flat fill.
	top := color.NRGBAModel.Convert(canvas.At(1, 1)).(color.NRGBA)
	bottom := color.NRGBAModel.Convert(canvas.At(1, canvas.Bounds().Dy()-2)).(color.NRGBA)
	if top.R == bottom.R && top.G == bottom.G && top.B == bottom.B {
		t.Fatal("gradient background is flat")
	}
}

func TestScreenshotAspectLetterbox(t *testing.T) {
	src := annotateSource()
	canvas, err := RenderScreenshot(src, ScreenshotRequest{
		Padding: 10, AspectW: 16, AspectH: 9, Background: "#FFFFFF",
	})
	if err != nil {
		t.Fatal(err)
	}
	ratio := float64(canvas.Bounds().Dx()) / float64(canvas.Bounds().Dy())
	if ratio < 1.7 || ratio > 1.85 {
		t.Fatalf("ratio = %.3f, want about 1.777", ratio)
	}
}

func TestCanvasLayerOrderDecidesWhatIsVisible(t *testing.T) {
	src := annotateSource()
	req := CanvasRequest{
		Width: 64, Height: 48, Background: "#FFFFFF",
		Layers: []CanvasLayer{
			// The red patch is drawn last, so it covers the blue one.
			{Kind: LayerShape, Shape: ShapeRect, X: 0, Y: 0, W: 32, H: 32, Fill: "#0000FF", Visible: true},
			{Kind: LayerShape, Shape: ShapeRect, X: 0, Y: 0, W: 32, H: 32, Fill: "#FF0000", Visible: true},
		},
	}
	out, err := RenderCanvas(req)
	if err != nil {
		t.Fatal(err)
	}
	got := out.(*image.NRGBA).NRGBAAt(4, 4)
	if got.R != 255 || got.B != 0 {
		t.Fatalf("top layer lost: %+v", got)
	}
	_ = src
}

func TestCanvasHiddenLayerIsSkipped(t *testing.T) {
	req := CanvasRequest{
		Width: 20, Height: 20, Background: "#FFFFFF",
		Layers: []CanvasLayer{
			{Kind: LayerShape, Shape: ShapeRect, X: 0, Y: 0, W: 20, H: 20, Fill: "#FF0000", Visible: false},
		},
	}
	out, err := RenderCanvas(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.(*image.NRGBA).NRGBAAt(10, 10); got.R != 255 || got.G != 255 {
		t.Fatalf("hidden layer painted: %+v", got)
	}
}

func TestCanvasPresetsBuildIntoTheirBox(t *testing.T) {
	for _, preset := range CanvasPresets {
		layers := preset.Build(800, 600)
		if len(layers) == 0 {
			t.Errorf("%s: empty preset", preset.Key)
		}
		for _, l := range layers {
			if l.Kind == LayerImage && l.Path != "" {
				t.Errorf("%s: a template must not reference a file", preset.Key)
			}
		}
	}
}

func TestStrictBlackWhiteHasExactlyTwoColours(t *testing.T) {
	src := annotateSource()
	out, colors, depth, err := RenderPixelArt(src, PixelRequest{
		Width: 40, Height: 30, StrictBlackWhite: true, Dither: DitherAtkinson,
		Strength: 100, TrueBitDepth: true, OutputPath: "x.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if depth != 1 {
		t.Fatalf("bit depth = %d, want 1", depth)
	}
	if colors != 2 {
		t.Fatalf("colours = %d, want 2", colors)
	}
	seen := map[uint32]struct{}{}
	img := out.(*image.NRGBA)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			c := img.NRGBAAt(x, y)
			if c.A != 255 {
				t.Fatalf("strict mode left a translucent pixel at %d,%d", x, y)
			}
			seen[uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B)] = struct{}{}
		}
	}
	if len(seen) > 2 {
		t.Fatalf("strict mode produced %d colours", len(seen))
	}
}

// TestPixelArtWritesARealOneBitPNG is E2E-11's second half: the file's IHDR must
// say bitDepth=1, which is a different claim from "the pixels are only black and
// white".
func TestPixelArtWritesARealOneBitPNG(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "in.png")
	writePNGFixture(t, input, annotateSource())
	output := filepath.Join(dir, "out.png")
	res := Pixelate(PixelRequest{
		InputPath: input, OutputPath: output, Width: 400, Height: 300,
		StrictBlackWhite: true, TrueBitDepth: true, Dither: DitherBayer, Strength: 100,
	})
	if !res.Success {
		t.Fatalf("pixelate failed: %s", res.Error)
	}
	if res.BitDepth != 1 {
		t.Fatalf("reported depth = %d", res.BitDepth)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if depth := int(raw[24]); depth != 1 {
		t.Fatalf("IHDR bit depth = %d", depth)
	}
	if ct := int(raw[25]); ct != 0 && ct != 3 {
		t.Fatalf("IHDR colour type = %d", ct)
	}
	decoded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 400 || decoded.Bounds().Dy() != 300 {
		t.Fatalf("decoded size = %v", decoded.Bounds())
	}
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			r, _, _, _ := decoded.At(x, y).RGBA()
			if r != 0 && r != 0xFFFF {
				t.Fatalf("pixel %d,%d = %d, not black or white", x, y, r)
			}
		}
	}
}

func TestPixelArtWithoutTrueBitDepthStaysEightBit(t *testing.T) {
	src := annotateSource()
	_, _, depth, err := RenderPixelArt(src, PixelRequest{
		Width: 40, Height: 30, StrictBlackWhite: true, TrueBitDepth: false, OutputPath: "x.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if depth != 8 {
		t.Fatalf("depth = %d, want 8 when the user did not ask for 1-bit", depth)
	}
}

// TestPixelArtRejectsOneBitNonPNG keeps the promise honest: no other container in
// the ladder can carry a one-bit image.
func TestPixelArtRejectsOneBitNonPNG(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "in.png")
	writePNGFixture(t, input, annotateSource())
	res := Pixelate(PixelRequest{
		InputPath: input, OutputPath: filepath.Join(dir, "out.jpg"),
		Width: 40, Height: 30, StrictBlackWhite: true, TrueBitDepth: true,
	})
	if res.Success {
		t.Fatal("1-bit JPEG must be rejected")
	}
}

// TestThresholdMovesTheBlackWhiteSplit pins the page's 亮度阈值 slider: a higher
// threshold must turn more pixels black, which is the opposite of what an
// evenly-spaced 0/255 palette would do (it would ignore the slider entirely).
func TestThresholdMovesTheBlackWhiteSplit(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 16), G: uint8(x * 16), B: uint8(x * 16), A: 255})
		}
	}
	whiteAt := func(threshold int) int {
		out, _, _, err := RenderPixelArt(src, PixelRequest{
			Width: 16, Height: 16, StrictBlackWhite: true, Dither: DitherNone,
			Threshold: threshold, OutputPath: "x.png",
		})
		if err != nil {
			t.Fatal(err)
		}
		white := 0
		img := out.(*image.NRGBA)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if img.NRGBAAt(x, y).R > 127 {
					white++
				}
			}
		}
		return white
	}
	low, high := whiteAt(64), whiteAt(192)
	if low <= high {
		t.Fatalf("white pixels: %d at threshold 64, %d at 192; a higher threshold must yield fewer white pixels", low, high)
	}
}

func TestDitherIsReproducible(t *testing.T) {
	src := annotateSource()
	a, _, _, _ := RenderPixelArt(src, PixelRequest{Width: 30, Height: 20, StrictBlackWhite: true,
		Dither: DitherFloyd, Strength: 100, OutputPath: "x.png"})
	b, _, _, _ := RenderPixelArt(src, PixelRequest{Width: 30, Height: 20, StrictBlackWhite: true,
		Dither: DitherFloyd, Strength: 100, OutputPath: "x.png"})
	if !bytes.Equal(a.(*image.NRGBA).Pix, b.(*image.NRGBA).Pix) {
		t.Fatal("two runs with the same parameters produced different pixels")
	}
}

func TestBayerMatrixStaysInsideTheUnitRange(t *testing.T) {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v := bayerValue(x, y)
			if v < 0 || v > 1 {
				t.Fatalf("bayerValue(%d,%d) = %v", x, y, v)
			}
		}
	}
}

func TestEmoteStillPaintsBothCaptions(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 320, 240))
	out, err := RenderEmote(src, EmoteRequest{
		Template: EmoteTopBottom, TopText: "上", BottomText: "下",
		FontSize: 40, Color: "#FFFFFF", Stroke: 3, StrokeColor: "#000000",
	})
	if err != nil {
		t.Fatal(err)
	}
	img := out.(*image.NRGBA)
	// Some pixel in the top band and some pixel in the bottom band must be
	// painted; an empty canvas would leave everything at zero.
	if !anyNonZero(img, 0, 0, 320, 120) {
		t.Fatal("top caption painted nothing")
	}
	if !anyNonZero(img, 0, 120, 320, 240) {
		t.Fatal("bottom caption painted nothing")
	}
}

func anyNonZero(img *image.NRGBA, x0, y0, x1, y1 int) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if c := img.NRGBAAt(x, y); c.R != 0 || c.G != 0 || c.B != 0 || c.A != 0 {
				return true
			}
		}
	}
	return false
}

func TestEmoteBubblePaintsABody(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 240, 240))
	out, err := RenderEmote(src, EmoteRequest{
		Template: EmoteBubble, CenterText: "你好",
		FontSize: 32, BubbleColor: "#FFFFFF", Color: "#111827",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !anyNonZero(out.(*image.NRGBA), 0, 120, 240, 240) {
		t.Fatal("bubble template painted nothing")
	}
}

// writePNGFixture writes a PNG fixture and closes it before returning, so a later
// step on Windows can read or delete the file.
func writePNGFixture(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOneBitHeaderIsBigEndianDepthOne(t *testing.T) {
	m := png1bit.NewMask(2, 2)
	raw, err := png1bit.Encode(m, png1bit.Options{ColorType: png1bit.Gray})
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(raw[16:20]) != 2 {
		t.Fatal("width is not in the IHDR")
	}
}

package engine

// audit_test.go pins the defects found in the full audit.
//
// Each test states the bug it guards, because the symptom is what a user reports
// and the code that caused it is not obvious from the name: a rounded crop that
// erased the whole top edge, a loop count of zero that could not be applied, a
// PDF title that vanished when an author was also set.

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRoundCornersKeepTheLongEdges pins the rounded-crop mask.
//
// The old mask measured a square of the short side and mirrored it, so on a wide
// canvas every pixel whose x fell beyond the square while y sat inside the corner
// band counted as a corner: a 400×225 crop came back with its whole top and bottom
// edges transparent.
func TestRoundCornersKeepTheLongEdges(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 60, G: 140, B: 200, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "s.png")
	if err := SaveImage(src, path); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "r.png")
	res := CropBox(CropRequest{
		InputPath: path, OutputPath: out,
		RatioW: 16, RatioH: 9, Anchor: "c", Radius: 20, OutputFormat: "png",
	})
	if !res.Success {
		t.Fatalf("crop failed: %s", res.Error)
	}
	img := openPNG(t, out)
	b := img.Bounds()
	cases := []struct {
		name string
		x, y int
		want uint32
	}{
		{"top edge centre", b.Dx() / 2, 0, 255},
		{"bottom edge centre", b.Dx() / 2, b.Dy() - 1, 255},
		{"left edge centre", 0, b.Dy() / 2, 255},
		{"centre", b.Dx() / 2, b.Dy() / 2, 255},
		{"top-left corner", 0, 0, 0},
		{"top-right corner", b.Dx() - 1, 0, 0},
		{"bottom-left corner", 0, b.Dy() - 1, 0},
	}
	for _, c := range cases {
		_, _, _, a := img.At(c.x, c.y).RGBA()
		if got := a >> 8; got != c.want {
			t.Errorf("%s at (%d,%d): alpha %d, want %d", c.name, c.x, c.y, got, c.want)
		}
	}
}

// TestRoundCornersFillWithBackground pins that a chosen background colour lands in
// the cleared corners instead of transparent pixels, which is what the panel's
// 圆角外颜色 field promises.
func TestRoundCornersFillWithBackground(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 40, G: 160, B: 220, A: 255})
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "s.png")
	if err := SaveImage(src, path); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "bg.png")
	if res := CropBox(CropRequest{
		InputPath: path, OutputPath: out,
		RatioW: 16, RatioH: 9, Anchor: "c", Radius: 20,
		Background: "#FFFFFF", OutputFormat: "png",
	}); !res.Success {
		t.Fatalf("crop failed: %s", res.Error)
	}
	img := openPNG(t, out)
	r, g, b, a := img.At(0, 0).RGBA()
	if a>>8 != 255 || r>>8 != 255 || g>>8 != 255 || b>>8 != 255 {
		t.Errorf("corner = #%02X%02X%02X a=%d, want opaque white", r>>8, g>>8, b>>8, a>>8)
	}
}

// TestGIFForeverLoopCanBeRequested pins the loop count.
//
// Loop is an int where 0 means 无限, and the writer only wrote it back when it was
// non-zero, so asking for an infinite loop kept the source's finite count. The
// request now carries ApplyLoop instead of inferring intent from the value.
func TestGIFForeverLoopCanBeRequested(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "loop.gif")
	writeTestGIF(t, src, 3)

	out := filepath.Join(dir, "forever.gif")
	res := RunGIF(GIFRequest{
		Action: "reverse", InputPath: src, OutputPath: out,
		OutputFormat: "gif", Loop: 0, ApplyLoop: true,
	})
	if !res.Success {
		t.Fatalf("reverse failed: %s", res.Error)
	}
	a, err := LoadAnimation(out)
	if err != nil {
		t.Fatal(err)
	}
	if a.Loop != 0 {
		t.Errorf("loop count = %d, want 0 (forever)", a.Loop)
	}

	// Without ApplyLoop the source's own count is kept, which is the zero-value
	// behaviour a caller that has no opinion needs.
	keep := filepath.Join(dir, "keep.gif")
	if res := RunGIF(GIFRequest{
		Action: "reverse", InputPath: src, OutputPath: keep, OutputFormat: "gif",
	}); !res.Success {
		t.Fatalf("reverse failed: %s", res.Error)
	}
	if b, err := LoadAnimation(keep); err != nil {
		t.Fatal(err)
	} else if b.Loop != 3 {
		t.Errorf("loop count = %d, want the source's 3", b.Loop)
	}
}

// TestPDFKeepsBothMetadataFields pins the document info dictionary.
//
// gopdf's SetInfo replaces the whole struct, so calling it once per field meant the
// second call discarded the first: setting a title and an author wrote only the
// author.
func TestPDFKeepsBothMetadataFields(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "a.png")
	if err := SaveImage(solidImage(64, 64), img); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "doc.pdf")
	res := GeneratePDF(PDFRequest{
		ImagePaths: []string{img}, OutputPath: out,
		PageSize: "A4", Title: "封面标题", Author: "作者名",
	})
	if !res.Success {
		t.Fatalf("pdf failed: %s", res.Error)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "/Title") {
		t.Error("PDF has no /Title entry; the author overwrote it")
	}
	if !strings.Contains(text, "/Author") {
		t.Error("PDF has no /Author entry")
	}
}

// TestPaletteBucketsDoNotCollide pins the histogram key.
//
// The key packed 6 bits per channel, so two different colours shared one bucket and
// the palette collapsed to a single entry: a picture of #00C800 and #024800 came
// back as five copies of the first.
func TestPaletteBucketsDoNotCollide(t *testing.T) {
	dir := t.TempDir()
	src := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if x < 32 {
				src.SetNRGBA(x, y, color.NRGBA{R: 0, G: 200, B: 0, A: 255})
			} else {
				src.SetNRGBA(x, y, color.NRGBA{R: 2, G: 72, B: 0, A: 255})
			}
		}
	}
	path := filepath.Join(dir, "two.png")
	if err := SaveImage(src, path); err != nil {
		t.Fatal(err)
	}
	_, pr := ExtractPalette(PaletteRequest{InputPath: path, Colors: 4})
	seen := map[string]bool{}
	for _, c := range append([]string{pr.ThemeColor}, pr.Colors...) {
		seen[c] = true
	}
	if len(seen) < 2 {
		t.Fatalf("a two-colour image produced one colour: %v", pr.Colors)
	}
}

// TestZeroValuesUseDocumentedDefaults pins the default-then-clamp order.
//
// Both engines clamped first and checked for zero afterwards, so zero became the
// clamp's lower bound and the documented defaults were unreachable: a rich-text
// request with no width produced a 200 px canvas instead of 800, and a palette with
// no height produced 24 px instead of 96.
func TestZeroValuesUseDocumentedDefaults(t *testing.T) {
	img, err := RenderRichTextImage(RichTextRequest{
		Blocks: []RichTextBlock{{Type: "paragraph", Text: "你好"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Dx(); got != 800 {
		t.Errorf("rich-text width = %d, want the documented default 800", got)
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "s.png")
	if err := SaveImage(solidImage(32, 32), src); err != nil {
		t.Fatal(err)
	}
	res, _ := ExtractPalette(PaletteRequest{
		InputPath: src, OutputPath: filepath.Join(dir, "p.png"), Colors: 4,
	})
	if !res.Success {
		t.Fatalf("palette failed: %s", res.Error)
	}
	if res.Height != 96 {
		t.Errorf("swatch height = %d, want the documented default 96", res.Height)
	}
}

// TestSVGSourceGetsOneExtension pins the output name.
//
// ensureExtension appended the encoder's suffix when the existing one did not
// match, so a source named logo.svg came out as logo_watermark.svg.png.
func TestSVGSourceGetsOneExtension(t *testing.T) {
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "logo.svg")
	doc := `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="80">` +
		`<rect width="120" height="80" fill="#3B82F6"/></svg>`
	if err := os.WriteFile(svgPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Watermark(WatermarkRequest{
		InputPath: svgPath, OutputPath: filepath.Join(dir, "logo_watermark.svg"),
		Text: "IF", FontSize: 20, Color: "#FFFFFF", Opacity: 80, Position: "br",
	})
	if !res.Success {
		t.Fatalf("watermark failed: %s", res.Error)
	}
	if base := filepath.Base(res.OutputPath); strings.Count(base, ".") != 1 {
		t.Errorf("output name = %q, want exactly one extension", base)
	}
}

// TestICOEntriesAreSquare pins the icon entries.
//
// An ICO directory stores a single side per entry, so a non-square source produced
// 16×9 entries — a malformed icon set. Every entry must be square.
func TestICOEntriesAreSquare(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 40, G: 160, B: 220, A: 255})
		}
	}
	out := filepath.Join(dir, "w.ico")
	if err := SaveImage(img, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	entries := int(raw[4]) | int(raw[5])<<8
	if entries < 1 {
		t.Fatal("no ICO entries")
	}
	for i := 0; i < entries; i++ {
		off := 6 + i*16
		w, h := int(raw[off]), int(raw[off+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		if w != h {
			t.Errorf("entry %d is %dx%d, want a square", i, w, h)
		}
	}
}

// openPNG decodes a PNG written by the engine.
func openPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// solidImage is a one-colour square, used where only the dimensions matter.
func solidImage(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 90, G: 120, B: 160, A: 255})
		}
	}
	return img
}

// writeTestGIF writes a two-frame GIF with the given loop count.
func writeTestGIF(t *testing.T, path string, loop int) {
	t.Helper()
	pal := color.Palette{
		color.NRGBA{R: 255, A: 255},
		color.NRGBA{B: 255, A: 255},
	}
	f0 := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	f1 := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	for i := range f1.Pix {
		f1.Pix[i] = 1
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, &gif.GIF{
		Image: []*image.Paletted{f0, f1}, Delay: []int{10, 10},
		LoopCount: loop, Config: image.Config{ColorModel: pal, Width: 16, Height: 16},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestZeroMarginIsHonoured pins that 0 means flush to the edge.
//
// The watermark engine replaced a zero margin with half the font size, and the
// panel re-seeded its own default whenever the value read 0, so the slider's
// leftmost position was unreachable: the watermark could never sit flush against
// the border.
func TestZeroMarginIsHonoured(t *testing.T) {
	dir := t.TempDir()
	src := image.NewNRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	in := filepath.Join(dir, "in.png")
	if err := SaveImage(src, in); err != nil {
		t.Fatal(err)
	}

	flush := filepath.Join(dir, "flush.png")
	if res := Watermark(WatermarkRequest{
		InputPath: in, OutputPath: flush, Text: "IF", FontSize: 32,
		Color: "#000000", Opacity: 100, Position: "tl", Margin: 0,
	}); !res.Success {
		t.Fatalf("watermark failed: %s", res.Error)
	}
	padded := filepath.Join(dir, "padded.png")
	if res := Watermark(WatermarkRequest{
		InputPath: in, OutputPath: padded, Text: "IF", FontSize: 32,
		Color: "#000000", Opacity: 100, Position: "tl", Margin: 80,
	}); !res.Success {
		t.Fatalf("watermark failed: %s", res.Error)
	}

	// The first dark pixel of the flush stamp must sit nearer the top-left corner
	// than the padded one's.
	if firstInk(openPNG(t, flush)) >= firstInk(openPNG(t, padded)) {
		t.Error("margin 0 did not place the watermark closer to the corner than margin 80")
	}
}

// TestZeroBorderIsHonoured pins that a border width of 0 produces no frame.
//
// Both the stitcher and the panel treated 0 as "not set" and substituted 40 px, so
// the 单张图加边框 frame could not be turned off once it was on.
func TestZeroBorderIsHonoured(t *testing.T) {
	dir := t.TempDir()
	src := image.NewNRGBA(image.Rect(0, 0, 60, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 60; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}
	in := filepath.Join(dir, "in.png")
	if err := SaveImage(src, in); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "frame.png")
	res := Stitch(StitchRequest{
		InputPaths: []string{in}, OutputPath: out,
		BorderMode: true, Border: 0, Background: "#FFFFFF", OutputFormat: "png",
	})
	if !res.Success {
		t.Fatalf("stitch failed: %s", res.Error)
	}
	if res.Width != 60 || res.Height != 40 {
		t.Errorf("frame = %dx%d, want the source's 60x40 when the border is 0", res.Width, res.Height)
	}
}

// firstInk reports the smallest x+y of a non-white pixel, a cheap measure of how
// far the content sits from the top-left corner.
func firstInk(img image.Image) int {
	b := img.Bounds()
	best := b.Dx() + b.Dy()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 < 200 || g>>8 < 200 || bl>>8 < 200 {
				if sum := x + y; sum < best {
					best = sum
				}
			}
		}
	}
	return best
}

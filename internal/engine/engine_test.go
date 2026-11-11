package engine_test

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// samplePNG draws a recognizable test image so format and geometry checks can
// assert on real pixels.
func samplePNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 255 / max(w-1, 1)),
				G: uint8(y * 255 / max(h-1, 1)),
				B: 128,
				A: 255,
			})
		}
	}
	path := filepath.Join(dir, "sample.png")
	if err := codec.Save(img, path, codec.SaveOptions{}); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	return path
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mustExist(t *testing.T, path string) int64 {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected output %s: %v", path, err)
	}
	if st.Size() == 0 {
		t.Fatalf("output %s is empty", path)
	}
	return st.Size()
}

func TestConvertWritesEveryFormat(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 120, 80)
	for _, format := range []string{"jpg", "png", "webp", "tiff", "bmp", "ico"} {
		out := filepath.Join(dir, "out."+format)
		res := engine.Convert(engine.ConvertRequest{
			InputPath:  src,
			OutputPath: out,
			Format:     format,
			Quality:    90,
		})
		if !res.Success {
			t.Fatalf("%s: %s (%s)", format, res.Error, res.ErrorCode)
		}
		mustExist(t, res.OutputPath)
		if res.Width != 120 || res.Height != 80 {
			t.Errorf("%s: size %dx%d", format, res.Width, res.Height)
		}
		opened, err := codec.Open(res.OutputPath)
		if err != nil {
			t.Fatalf("%s: reopen: %v", format, err)
		}
		// ICO holds square entries, so it is the one target that changes the shape
		// of a non-square source on purpose.
		if format == "ico" {
			if opened.Width != opened.Height || opened.Width < 16 {
				t.Errorf("ico: reopened %dx%d", opened.Width, opened.Height)
			}
			continue
		}
		if opened.Width != 120 || opened.Height != 80 {
			t.Errorf("%s: reopened %dx%d", format, opened.Width, opened.Height)
		}
	}
}

func TestConvertResizeModes(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 200, 100)
	cases := []struct {
		name string
		spec engine.ConvertRequest
		w, h int
	}{
		{"percent", engine.ConvertRequest{ResizeMode: "percent", ScalePercent: 50}, 100, 50},
		{"fit", engine.ConvertRequest{ResizeMode: "fit", Width: 100, Height: 100, MaintainAR: true}, 100, 50},
		{"exact", engine.ConvertRequest{ResizeMode: "exact", Width: 60, Height: 60}, 60, 60},
		{"long_edge", engine.ConvertRequest{ResizeMode: "long_edge", LongEdge: 100}, 100, 50},
	}
	for _, c := range cases {
		out := filepath.Join(dir, c.name+".png")
		req := c.spec
		req.InputPath = src
		req.OutputPath = out
		req.Format = "png"
		res := engine.Convert(req)
		if !res.Success {
			t.Fatalf("%s: %s", c.name, res.Error)
		}
		if res.Width != c.w || res.Height != c.h {
			t.Errorf("%s: got %dx%d want %dx%d", c.name, res.Width, res.Height, c.w, c.h)
		}
	}
}

func TestConvertRejectsMissingInput(t *testing.T) {
	res := engine.Convert(engine.ConvertRequest{OutputPath: "x.png", Format: "png"})
	if res.Success || res.ErrorCode != "BAD_INPUT" {
		t.Fatalf("got %+v", res)
	}
}

func TestResizeTool(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 300, 200)
	out := filepath.Join(dir, "resized.png")
	res := engine.Resize(engine.ResizeRequest{
		InputPath: src, OutputPath: out, Mode: "long_edge", Value: 100,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Width != 100 || res.Height != 67 {
		t.Fatalf("got %dx%d", res.Width, res.Height)
	}
}

func TestGridCutter(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 90, 90)
	outDir := filepath.Join(dir, "grid")
	res := engine.CutGrid(engine.GridRequest{
		InputPath: src, OutputDir: outDir, GridSize: 3, OutputFormat: "png", BaseName: "tile",
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Count != 9 || len(res.Outputs) != 9 {
		t.Fatalf("count = %d", res.Count)
	}
	for _, p := range res.Outputs {
		mustExist(t, p)
	}
	opened, err := codec.Open(res.Outputs[0])
	if err != nil {
		t.Fatal(err)
	}
	if opened.Width != 30 || opened.Height != 30 {
		t.Errorf("cell %dx%d", opened.Width, opened.Height)
	}
}

func TestSplitSquareAndCover(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 200, 100)
	for _, mode := range []string{"square", "cover"} {
		outDir := filepath.Join(dir, "split_"+mode)
		res := engine.Split(engine.SplitRequest{
			InputPath: src, OutputDir: outDir, Rows: 2, Cols: 4,
			Mode: mode, OutputFormat: "png", BaseName: "p",
		})
		if !res.Success {
			t.Fatalf("%s: %s", mode, res.Error)
		}
		if res.Count != 8 {
			t.Fatalf("%s: count %d", mode, res.Count)
		}
		opened, err := codec.Open(res.Outputs[0])
		if err != nil {
			t.Fatal(err)
		}
		if mode == "square" {
			// 100x100 square cut into 4 cols gives 25x50 cells.
			if opened.Width != 25 || opened.Height != 50 {
				t.Errorf("square cell %dx%d", opened.Width, opened.Height)
			}
		}
	}
}

func TestStitchVerticalAndHorizontal(t *testing.T) {
	dir := t.TempDir()
	a := samplePNG(t, dir, 100, 50)
	b := filepath.Join(dir, "b.png")
	img, _ := codec.Open(a)
	if err := codec.Save(img.Image, b, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, direction := range []string{"vertical", "horizontal"} {
		out := filepath.Join(dir, "stitch_"+direction+".png")
		res := engine.Stitch(engine.StitchRequest{
			InputPaths:   []string{a, b},
			OutputPath:   out,
			Direction:    direction,
			TargetWidth:  100,
			TargetHeight: 100,
			Gap:          10,
			Background:   "#FFFFFF",
			OutputFormat: "png",
		})
		if !res.Success {
			t.Fatalf("%s: %s", direction, res.Error)
		}
		mustExist(t, out)
		if direction == "vertical" {
			if res.Height != 50+10+50 {
				t.Errorf("vertical height = %d", res.Height)
			}
		} else if res.Width != 200+10+200 {
			// Horizontal stitching scales each input to the target height of
			// 100, so a 100x50 source becomes 200x100.
			t.Errorf("horizontal width = %d", res.Width)
		}
	}
}

func TestStitchFrameMode(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 100, 40)
	out := filepath.Join(dir, "framed.png")
	res := engine.Stitch(engine.StitchRequest{
		InputPaths: []string{src}, OutputPath: out, BorderMode: true, Border: 20,
		Background: "#FF0000", OutputFormat: "png",
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Width != 140 || res.Height != 80 {
		t.Fatalf("got %dx%d", res.Width, res.Height)
	}
	opened, _ := codec.Open(out)
	r, g, bl, _ := opened.Image.At(2, 2).RGBA()
	if r>>8 < 200 || g>>8 > 60 || bl>>8 > 60 {
		t.Errorf("border pixel = %d,%d,%d", r>>8, g>>8, bl>>8)
	}
}

func TestWatermarkWritesPixels(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 400, 300)
	out := filepath.Join(dir, "wm.png")
	res := engine.Watermark(engine.WatermarkRequest{
		InputPath: src, OutputPath: out, Text: "ImageFlow",
		FontSize: 40, Color: "#FF0000", Opacity: 100, Position: "c",
	})
	if !res.Success {
		t.Fatalf("%s (%s)", res.Error, res.ErrorCode)
	}
	mustExist(t, out)
	plain, _ := codec.Open(src)
	marked, _ := codec.Open(out)
	diff := 0
	for y := 0; y < 300; y += 3 {
		for x := 0; x < 400; x += 3 {
			r1, g1, b1, _ := plain.Image.At(x, y).RGBA()
			r2, g2, b2, _ := marked.Image.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Fatal("watermark changed no pixels")
	}
}

func TestWatermarkTiled(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 300, 300)
	out := filepath.Join(dir, "tiled.png")
	res := engine.Watermark(engine.WatermarkRequest{
		InputPath: src, OutputPath: out, Text: "IF", FontSize: 24,
		Color: "#000000", Opacity: 90, Tiled: true, TileGapX: 80, TileGapY: 80,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	mustExist(t, out)
}

func TestWatermarkEmptyTextFails(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 100, 100)
	res := engine.Watermark(engine.WatermarkRequest{
		InputPath: src, OutputPath: filepath.Join(dir, "x.png"), Text: "  ",
	})
	if res.Success || res.ErrorCode != "WM_NO_TEXT" {
		t.Fatalf("got %+v", res)
	}
}

func TestPaletteExtraction(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 200, 200)
	out := filepath.Join(dir, "palette.png")
	res, palette := engine.ExtractPalette(engine.PaletteRequest{
		InputPath: src, OutputPath: out, SwatchHeight: 80, IncludeText: true,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if palette.ThemeColor == "" || len(palette.Colors) != 10 {
		t.Fatalf("palette = %+v", palette)
	}
	mustExist(t, out)
}

func TestPDFGeneration(t *testing.T) {
	dir := t.TempDir()
	one := samplePNG(t, dir, 300, 200)
	two := filepath.Join(dir, "two.png")
	img, _ := codec.Open(one)
	if err := codec.Save(img.Image, two, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "doc.pdf")
	res := engine.GeneratePDF(engine.PDFRequest{
		ImagePaths: []string{one, two},
		OutputPath: out,
		Layout:     "2x2",
		PageSize:   "A4",
		Landscape:  false,
		FitMode:    "contain",
		Margin:     36,
		Title:      "Test",
		Author:     "ImageFlow",
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	size := mustExist(t, out)
	if size < 1000 {
		t.Fatalf("pdf suspiciously small: %d", size)
	}
	head := make([]byte, 5)
	f, _ := os.Open(out)
	_, _ = f.Read(head)
	f.Close()
	if string(head) != "%PDF-" {
		t.Fatalf("not a pdf: %q", head)
	}
}

func TestPDFEncryptionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 200, 200)
	out := filepath.Join(dir, "locked.pdf")
	res := engine.GeneratePDF(engine.PDFRequest{
		ImagePaths: []string{src}, OutputPath: out, Layout: "single",
		PageSize: "A4", Landscape: false, FitMode: "contain", UserPasswd: "secret",
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	mustExist(t, out)
	raw, _ := os.ReadFile(out)
	if !containsASCII(raw, "Encrypt") {
		t.Error("pdf has no encryption dictionary")
	}
}

func containsASCII(data []byte, want string) bool {
	return len(want) == 0 || indexOfBytes(data, []byte(want)) >= 0
}

func indexOfBytes(haystack, needle []byte) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func TestRichTextRender(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "rich.png")
	res := engine.RenderRichText(engine.RichTextRequest{
		Blocks: []engine.RichTextBlock{
			{Type: "heading", Level: 1, Text: "标题 Heading"},
			{Type: "paragraph", Text: "这是一段中文正文，用于验证自动换行。"},
			{Type: "list", Items: []string{"第一项", "第二项"}, Ordered: true},
			{Type: "quote", Text: "引用文字"},
		},
		OutputPath: out,
		Width:      520,
		Padding:    32,
		Background: "#FFFFFF",
		TextColor:  "#1F2937",
		FontSize:   16,
		LineHeight: 1.6,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Width != 520 {
		t.Errorf("width = %d", res.Width)
	}
	if res.Height < 200 {
		t.Errorf("height = %d, layout probably collapsed", res.Height)
	}
	mustExist(t, out)
	opened, _ := codec.Open(out)
	dark := 0
	for y := 0; y < opened.Height; y += 2 {
		for x := 0; x < opened.Width; x += 2 {
			r, _, _, _ := opened.Image.At(x, y).RGBA()
			if r>>8 < 120 {
				dark++
			}
		}
	}
	if dark < 50 {
		t.Fatalf("only %d dark pixels, text missing", dark)
	}
}

func TestRichTextRejectsEmpty(t *testing.T) {
	res := engine.RenderRichText(engine.RichTextRequest{OutputPath: "x.png"})
	if res.Success || res.ErrorCode != "RT_NO_BLOCKS" {
		t.Fatalf("got %+v", res)
	}
}

func TestSVGConversion(t *testing.T) {
	dir := t.TempDir()
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100" viewBox="0 0 200 100">
<rect width="200" height="100" fill="#1e90ff"/>
<circle cx="100" cy="50" r="30" fill="#ffffff"/>
</svg>`
	src := filepath.Join(dir, "shape.svg")
	if err := os.WriteFile(src, []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "shape.png")
	res := engine.Convert(engine.ConvertRequest{InputPath: src, OutputPath: out, Format: "png"})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Width != 200 || res.Height != 100 {
		t.Fatalf("svg size %dx%d", res.Width, res.Height)
	}
	opened, _ := codec.Open(out)
	r, g, b, _ := opened.Image.At(100, 50).RGBA()
	if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
		t.Errorf("circle pixel not white: %d,%d,%d", r>>8, g>>8, b>>8)
	}
}

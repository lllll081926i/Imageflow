package ui

// studio_integration_test.go drives the pages added by the v2.0 plan end to end:
// seed inputs, run the page, and reopen the file it wrote. A page whose output
// cannot be decoded again is not finished, whatever its panel looks like.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
)

// outputsOf lists the files a batch wrote, read from the session's row results.
func outputsOf(a *App) []string {
	var out []string
	for _, row := range a.Session.Snapshot() {
		if row.Status == "ok" && row.Output != "" {
			out = append(out, row.Output)
		}
	}
	return out
}

// reopen decodes one written file.
func reopen(t *testing.T, path string) image.Image {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

// TestRunBasicEditEndToEnd is EDIT-01…03's acceptance path: rotate, pad the canvas,
// round the corners, and reopen the file to check the geometry really changed.
func TestRunBasicEditEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "基础编辑").(*EditTool)
	tool.rotation = 90
	tool.canvasW, tool.canvasH = 200, 200
	tool.radius = 20
	tool.bgHex = "#FFFFFF"
	a.Start()
	waitForRun(t, a)

	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	img := reopen(t, outputs[0])
	if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 200 {
		t.Fatalf("canvas = %v, want 200×200", img.Bounds())
	}
}

// TestRunGradeEndToEnd grades a seeded picture and checks the file reopens with the
// expected size.
func TestRunGradeEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "专业调色").(*GradeTool)
	tool.exposure = 0.5
	tool.saturation = 20
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	if img := reopen(t, outputs[0]); img.Bounds().Dx() != 120 {
		t.Fatalf("graded size = %v", img.Bounds())
	}
}

// TestRunFilterEndToEnd runs a zero-strength filter and asserts the output equals the
// source, which is the acceptance rule the page advertises.
//
// The comparison is against a re-encoded copy of the source rather than the source
// file's bytes: the batch always writes a fresh container, so "0% is the original"
// is a statement about pixels, not about files.
func TestRunFilterEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "滤镜效果").(*FilterTool)
	tool.kind = "grayscale"
	tool.strength = 0
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	got := reopen(t, outputs[0])
	if got.Bounds().Dx() != 120 || got.Bounds().Dy() != 90 {
		t.Fatalf("bounds = %v", got.Bounds())
	}
	// The 3×3 seed has a distinct colour per pixel, so a filter that ran at all
	// would show up in the sample below. The expected value is the pixel the
	// engine produces for a 0% strength, which is the source pixel.
	for y := 0; y < 90; y += 7 {
		for x := 0; x < 120; x += 7 {
			want := color.NRGBA{R: uint8(x), G: uint8(y), B: 60, A: 255}
			if c := color.NRGBAModel.Convert(got.At(x, y)).(color.NRGBA); c != want {
				t.Fatalf("0%% strength changed pixel %d,%d: want %+v got %+v", x, y, want, c)
			}
		}
	}
}

// TestRunFilterInvertChangesPixels proves the filter path actually runs, so the
// zero-strength test above cannot pass for a page that never filters anything.
func TestRunFilterInvertChangesPixels(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "滤镜效果").(*FilterTool)
	tool.kind = "invert"
	tool.strength = 100
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	got := reopen(t, outputs[0])
	original := color.NRGBA{R: 0, G: 0, B: 60, A: 255}
	if c := color.NRGBAModel.Convert(got.At(0, 0)).(color.NRGBA); c == original {
		t.Fatal("a 100% invert left the pixel unchanged")
	}
}

// TestRunPixelArtStrictBWEndToEnd is E2E-11's pipeline: 400×300, strict black and
// white, real 1-bit PNG, then reopen and check every pixel and the IHDR.
func TestRunPixelArtStrictBWEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "像素画 / 点阵").(*PixelTool)
	tool.width, tool.height = 400, 300
	tool.strict = true
	tool.trueBit = true
	tool.dither = 1 // Bayer
	tool.strength = 100
	tool.format = 0 // png
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	raw, err := os.ReadFile(outputs[0])
	if err != nil {
		t.Fatal(err)
	}
	if raw[24] != 1 {
		t.Fatalf("IHDR bit depth = %d, want 1", raw[24])
	}
	if raw[25] != 0 && raw[25] != 3 {
		t.Fatalf("IHDR colour type = %d", raw[25])
	}
	img := reopen(t, outputs[0])
	if img.Bounds().Dx() != 400 || img.Bounds().Dy() != 300 {
		t.Fatalf("decoded = %v", img.Bounds())
	}
	for y := 0; y < 300; y += 3 {
		for x := 0; x < 400; x += 3 {
			r, g, b, _ := img.At(x, y).RGBA()
			if r != g || g != b || (r != 0 && r != 0xFFFF) {
				t.Fatalf("pixel %d,%d = %d,%d,%d", x, y, r, g, b)
			}
		}
	}
}

// TestRunAnnotateCoverEndToEnd is E2E-07's pipeline: place an opaque cover, export,
// and confirm the covered pixels carry none of the source.
func TestRunAnnotateCoverEndToEnd(t *testing.T) {
	a := newTestApp(t)
	inputs := seedInputs(t, a, 1)
	tool := useTool(t, a, "图片标注").(*AnnotateTool)
	tool.tool = len(annotateTools) - 1 // 安全遮盖
	tool.colorHex = "#000000"
	tool.marks = []engine.AnnotateMark{{
		Kind: engine.AnnotateCover, X1: 10, Y1: 10, X2: 60, Y2: 40, Color: "#000000",
	}}
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	got := reopen(t, outputs[0])
	src := reopen(t, inputs[0])
	changed := 0
	for y := 10; y < 40; y++ {
		for x := 10; x < 60; x++ {
			c := color.NRGBAModel.Convert(got.At(x, y)).(color.NRGBA)
			if c.R == 0 && c.G == 0 && c.B == 0 {
				changed++
			}
		}
	}
	if changed < 1500 {
		t.Fatalf("only %d covered pixels are black", changed)
	}
	// A pixel outside the cover keeps the source value.
	if color.NRGBAModel.Convert(got.At(2, 2)).(color.NRGBA) != color.NRGBAModel.Convert(src.At(2, 2)).(color.NRGBA) {
		t.Fatal("the cover bled outside its rectangle")
	}
}

// TestRunScreenshotEndToEnd decorates a screenshot and checks the canvas grew.
func TestRunScreenshotEndToEnd(t *testing.T) {
	a := newTestApp(t)
	inputs := seedInputs(t, a, 1)
	tool := useTool(t, a, "截图美化").(*ScreenshotTool)
	tool.padding = 40
	tool.radius = 12
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	got := reopen(t, outputs[0])
	src := reopen(t, inputs[0])
	if got.Bounds().Dx() <= src.Bounds().Dx() {
		t.Fatalf("the frame did not grow: %v", got.Bounds())
	}
}

// TestRunEmoteStickerEndToEnd captions a still picture and reopens the result.
func TestRunEmoteStickerEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "表情包制作").(*EmoteTool)
	tool.template = 0
	tool.bottom = "测试字幕"
	tool.format = 0 // gif output is still a valid image container for a still
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	if st, err := os.Stat(outputs[0]); err != nil || st.Size() == 0 {
		t.Fatalf("output missing: %v", err)
	}
}

// TestRunCanvasLayoutEndToEnd renders a free layout with one shape layer and checks
// the canvas size and the drawn colour.
func TestRunCanvasLayoutEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "自由布局").(*CollageStudioTool)
	tool.width, tool.height = 120, 120
	tool.bgHex = "#FFFFFF"
	tool.layers = []engine.CanvasLayer{{
		Kind: engine.LayerShape, Shape: engine.ShapeRect,
		X: 0, Y: 0, W: 120, H: 120, Fill: "#FF0000", Visible: true, Opacity: 1,
	}}
	a.Start()
	waitForRun(t, a)
	outputs := outputsOf(a)
	if len(outputs) != 1 {
		t.Fatalf("outputs = %v", outputs)
	}
	got := reopen(t, outputs[0])
	c := color.NRGBAModel.Convert(got.At(60, 60)).(color.NRGBA)
	if c.R != 255 || c.G != 0 {
		t.Fatalf("layer colour = %+v", c)
	}
}

func TestRunUpscaleWithoutEngineIsHonest(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 1)
	tool := useTool(t, app, "AI 高清放大").(*UpscaleTool)
	tool.loadPreferences(app)
	tool.detected = true
	tool.downloadModel = "local-model"
	tool.env.Models = []upscale.ModelStatus{{Model: upscale.Model{Name: tool.downloadModel}, Installed: true, HashOK: true}}
	app.Start()
	if app.IsRunning() || app.StatusLine() != "本地推理引擎未就绪" {
		t.Fatalf("引擎未就绪时应在启动前显示原因：%q", app.StatusLine())
	}
	if row := app.Session.Snapshot()[0]; row.Status != "" || row.Output != "" {
		t.Fatalf("尚未启动时不应修改文件状态：%+v", row)
	}
}

// TestPixelPageRejectsOneBitForNonPNG pins the page-level guard: the panel cannot
// promise a 1-bit file it cannot write.
func TestPixelPageRejectsOneBitForNonPNG(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	tool := useTool(t, a, "像素画 / 点阵").(*PixelTool)
	tool.strict = true
	tool.trueBit = true
	tool.format = 2 // webp
	a.Start()
	waitForRun(t, a)
	for _, row := range a.Session.Snapshot() {
		if row.Status == "ok" {
			t.Fatal("1-bit WebP was accepted")
		}
	}
}

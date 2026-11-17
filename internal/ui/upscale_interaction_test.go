package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	"github.com/egoist/mygo/transfer"
	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
)

func TestUpscaleMouseZoomPanAndReset(t *testing.T) {
	tool := &UpscaleTool{}
	tool.resetComparison()
	rect := myui.Rect{W: 800, H: 400}
	tool.comparisonPointer(myui.InputEvent{Kind: myui.InputScroll, DY: -100}, rect)
	if tool.zoom != 300 {
		t.Fatalf("滚轮未缩放：%v", tool.zoom)
	}
	tool.comparisonPointer(myui.InputEvent{Kind: myui.InputPointerDown, Button: 1, X: 400, Y: 200}, rect)
	tool.comparisonPointer(myui.InputEvent{Kind: myui.InputPointerMove, X: 640, Y: 320}, rect)
	tool.comparisonPointer(myui.InputEvent{Kind: myui.InputPointerUp, Button: 1}, rect)
	if tool.centerX >= .5 || tool.centerY >= .5 || tool.split != .5 {
		t.Fatal("右键未平移或错误移动了对比线")
	}
	for _, center := range []float64{0, .5, 1} {
		region := normalizedRegion(center, center, tool.zoom/100)
		if region.X < 0 || region.Y < 0 || region.X+region.W > 1.00001 || region.Y+region.H > 1.00001 {
			t.Fatalf("缩放源区域越界：%+v", region)
		}
	}
	tool.comparisonPointer(myui.InputEvent{Kind: myui.InputPointerDown, Clicks: 2}, rect)
	if tool.zoom != 100 || tool.centerX != .5 || tool.centerY != .5 || tool.split != .5 {
		t.Fatal("双击未复位")
	}
}

func TestUpscaleDividerDragShowsActualBeforeAndAfter(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	paths := seedInputs(t, app, 1)
	before := imaging.New(100, 50, color.NRGBA{R: 240, A: 255})
	after := imaging.New(400, 200, color.NRGBA{B: 240, A: 255})
	output := filepath.Join(t.TempDir(), "enhanced.png")
	for path, img := range map[string]*image.NRGBA{paths[0]: before, output: after} {
		if err := codec.Save(img, path, codec.SaveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	app.openTool(app.toolIndex(ToolUpscale))
	tool := app.ActiveTool().(*UpscaleTool)
	info, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	tool.results.Store(paths[0], upscaleResult{report: upscale.Report{OutputPath: output, Width: 400, Height: 200}, sourceSize: info.Size(), sourceMod: info.ModTime().UnixNano()})
	tester := myui.NewTester(app.View, 1440, 900)
	deadline := time.Now().Add(10 * time.Second)
	for {
		tester.Frame()
		pair, busy, err := tool.resultPreview.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if !busy && pair.after != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("对比图未就绪")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, label := range []string{"滑动", "放大镜", "重置视图", "缩放", "打开结果"} {
		if tester.HasText(label) {
			t.Fatalf("画布仍有已移除的控件：%s", label)
		}
	}
	canvas, ok := tester.Access("AI 对比画布")
	if !ok {
		t.Fatal("缺少对比画布")
	}
	rect := previewFitRect(canvas.Bounds, 100, 50)
	check := func(fraction float32, blue bool) {
		t.Helper()
		pixel := color.NRGBAModel.Convert(tester.Image().At(int(rect.X+rect.W*fraction), int(rect.Y+rect.H*.3))).(color.NRGBA)
		if blue && (pixel.B < 200 || pixel.R > 20) || !blue && (pixel.R < 200 || pixel.B > 20) {
			t.Fatalf("对比分区像素错误：位置 %.2f，期望优化后=%v，像素=%v", fraction, blue, pixel)
		}
	}
	check(.25, false)
	check(.75, true)
	drag := func(from, to float32) {
		tester.Press(rect.X+rect.W*from, rect.Y+rect.H/2)
		tester.Frame()
		tester.Move(rect.X+rect.W*to, rect.Y+rect.H/2)
		tester.Frame()
		if tool.split < float64(to)-.01 || tool.split > float64(to)+.01 {
			t.Fatalf("拖动期间分隔线未实时更新：期望 %.2f，实际 %.2f", to, tool.split)
		}
		if to < .25 {
			check(.25, true)
		}
		if to > .75 {
			check(.75, false)
		}
		tester.Release(rect.X+rect.W*to, rect.Y+rect.H/2)
		tester.Frame()
	}
	drag(.5, .1)
	check(.25, true)
	drag(.1, .9)
	check(.75, false)
	drag(.9, 0)
	check(.25, true)
	check(.75, true)
	drag(0, 1)
	check(.25, false)
	check(.75, false)
	tester.Scroll(rect.X+rect.W/2, rect.Y+rect.H/2, 0, -100)
	tester.Frame()
	if tool.zoom <= 100 {
		t.Fatal("画布未接收滚轮缩放")
	}
	tester.Press(rect.X+rect.W/2, rect.Y+rect.H/2)
	tester.Release(rect.X+rect.W/2, rect.Y+rect.H/2)
	tester.Press(rect.X+rect.W/2, rect.Y+rect.H/2)
	tester.Release(rect.X+rect.W/2, rect.Y+rect.H/2)
	tester.Frame()
	if tool.zoom != 100 {
		t.Fatal("画布双击未复位缩放")
	}
	check(.25, false)
	check(.75, true)
	dumpImage(t, tester.Image(), "upscale-comparison")
}

func TestUpscalePreferencesPersistAndAdvancedSectionCanClose(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	app.openTool(app.toolIndex(ToolUpscale))
	tool := app.ActiveTool().(*UpscaleTool)
	tester := myui.NewTester(app.View, 1440, 1700)
	for _, want := range []bool{true, false, true} {
		if err := tester.Click("高级选项"); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
		if tool.advanced != want {
			t.Fatal("高级选项不能反复展开或折叠")
		}
	}
	tool.scale, tool.double, tool.useCustomWidth, tool.customWidth = 6, true, true, 2048
	tool.gpuIDs, tool.tile, tool.compression = "0,1", 64, 30
	tool.persistPreferences(app)
	tool.flushPreferences(app)
	loaded := settings.Load()
	if loaded.Upscale != tool.preferences() {
		t.Fatalf("AI 参数未保存：%+v %+v", loaded.Upscale, tool.preferences())
	}
	restored := &UpscaleTool{}
	restored.ensure()
	other := NewApp(loaded)
	defer other.CancelStudioWork()
	restored.loadPreferences(other)
	if restored.preferences() != tool.preferences() {
		t.Fatal("重启后 AI 参数没有恢复")
	}
}

func TestUpscalePasteImportsClipboardImageBesideModels(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	app.Settings.AIModelsDir = t.TempDir()
	app.openTool(app.toolIndex(ToolUpscale))
	tester := myui.NewTester(app.View, 1440, 900)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, imaging.New(80, 40, color.NRGBA{G: 200, A: 255})); err != nil {
		t.Fatal(err)
	}
	tester.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, encoded.Bytes()))))
	if err := tester.Click("粘贴图片"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for app.Session.Len() == 0 {
		tester.Frame()
		if time.Now().After(deadline) {
			t.Fatal("剪贴板图片未导入：" + app.StatusLine())
		}
		time.Sleep(10 * time.Millisecond)
	}
	item := app.Session.Snapshot()[0]
	if filepath.Dir(filepath.Dir(item.Path)) != filepath.Join(app.Settings.AIModelsDir, ".clipboard") || item.Width != 80 || item.Height != 40 {
		t.Fatalf("剪贴板输入路径或尺寸错误：%+v", item)
	}
}

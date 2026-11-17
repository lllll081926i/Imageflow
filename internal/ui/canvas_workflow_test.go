package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestCanvasWorkflowIndependentResult(t *testing.T) {
	for _, name := range []string{"自由布局", "富文本制图"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			tool := useTool(t, app, name)
			if canvas, ok := tool.(*CollageStudioTool); ok {
				canvas.width, canvas.height = 160, 160
				canvas.addLayer(engine.LayerShape)
			}
			app.Start()
			if app.IsRunning() || app.StatusLine() != "先选择输出文件夹" {
				t.Fatalf("未选择输出位置时未阻止导出：%s", app.StatusLine())
			}
			dir := t.TempDir()
			if err := app.Session.UpdateSettings(func(next *settings.Settings) { next.DefaultOutputDir = dir }); err != nil {
				t.Fatal(err)
			}
			app.Start()
			waitForRun(t, app)
			output := app.completedOutputPath()
			if filepath.Dir(output) != dir {
				t.Fatalf("画布结果没有记住实际输出：%q，状态：%s", output, app.StatusLine())
			}
			if _, err := os.Stat(output); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(app.StatusLine(), dir) {
				t.Fatal("完成提示仍显示完整目录")
			}
			if _, caption := app.ProgressState(); caption != "" {
				t.Fatal("完成提示重复显示在进度条下方")
			}
			waitStudioImage(t, app, tool)
			tester := myui.NewTester(app.View, 960, 620)
			if _, found := tester.Find("查看结果"); !found {
				t.Fatal("无图片画布缺少结果入口")
			}
			if _, found := tester.Find("拖放区"); found {
				t.Fatal("无图片画布仍显示空队列")
			}
			if !tester.HasText("画布") {
				t.Fatal("无图片画布标题仍显示输入项")
			}
			dumpImage(t, tester.Image(), "canvas-result-"+tool.ID())
			app.openTool(app.toolIndex(ToolConvert))
			if app.completedOutputPath() != "" {
				t.Fatal("画布结果泄漏到其他工具")
			}
			if app.Preview() != nil {
				t.Fatal("独立画布结果污染了其他工具的源图预览")
			}
			app.openTool(app.toolIndex(tool.ID()))
			if app.completedOutputPath() != output {
				t.Fatal("返回画布后结果入口丢失")
			}
			blocked := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blocked, []byte("目录保护测试"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := app.Session.UpdateSettings(func(next *settings.Settings) { next.DefaultOutputDir = blocked }); err != nil {
				t.Fatal(err)
			}
			app.Start()
			waitForRun(t, app)
			if app.completedOutputPath() != "" || !strings.Contains(app.StatusLine(), "失败") {
				t.Fatalf("失败后仍沿用旧结果：%s", app.StatusLine())
			}
		})
	}
}

func TestCanvasWorkflowBatchReportsFailuresAndSkips(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 2)
	tool := useTool(t, app, "自由布局").(*CollageStudioTool)
	tool.width, tool.height = 160, 160
	tool.addLayer(engine.LayerImage)
	tool.layers[0].Path = filepath.Join(t.TempDir(), "missing.png")
	app.Start()
	waitForRun(t, app)
	if !strings.Contains(app.StatusLine(), "成功 0，失败 2") {
		t.Fatalf("失败导出仍显示全部完成：%s", app.StatusLine())
	}
	for _, row := range app.Session.Snapshot() {
		if row.Status != "failed" || row.Output != "" {
			t.Fatalf("失败行状态不准确：%+v", row)
		}
	}
	tool.layers = nil
	tool.addLayer(engine.LayerShape)
	app.Start()
	waitForRun(t, app)
	if err := app.Session.UpdateSettings(func(next *settings.Settings) { next.ConflictStrategy = settings.ConflictSkip }); err != nil {
		t.Fatal(err)
	}
	app.Start()
	waitForRun(t, app)
	if !strings.Contains(app.StatusLine(), "0 张，跳过 2") {
		t.Fatalf("跳过导出仍显示全部生成：%s", app.StatusLine())
	}
	for _, row := range app.Session.Snapshot() {
		if row.Status != "skipped" || row.Output != "" {
			t.Fatalf("跳过行状态不准确：%+v", row)
		}
	}
}

func TestCanvasWorkflowLayerControls(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	inputs := seedInputs(t, app, 2)
	tool := useTool(t, app, "自由布局").(*CollageStudioTool)
	app.selectBrowserPreview(inputs[1])
	waitForPreview(t, app, inputs[1])
	tester := myui.NewTester(func(c *myui.Context) {
		myui.Column(c).Width(384).Gap(10).Children(func() { tool.Panel(c, app) })
	}, 420, 2600)
	if err := tester.Click("加图片"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if len(tool.layers) != 1 || tool.layers[0].Path != inputs[1] {
		t.Fatal("新增图片图层未使用选中的图片")
	}
	button, found := tester.Find("选择")
	if !found || button.W < 40 || button.X+button.W > 384 {
		t.Fatalf("图片路径挤出了选择按钮：%+v", button)
	}
	if err := tester.Click("加文字"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if strings.Contains(tool.layers[1].Text, "双击") {
		t.Fatal("文字图层仍承诺未实现的双击编辑")
	}
	if err := tester.Click("居中标题"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	count := len(tool.layers)
	if err := tester.Click("自定义"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if tool.preset != "" || len(tool.layers) != count {
		t.Fatal("切回自定义破坏了现有图层")
	}
	for shape, label := range map[string]string{
		engine.ShapeRect: "形状：矩形", engine.ShapeEllipse: "形状：椭圆", engine.ShapeLine: "形状：直线",
	} {
		if got := layerLabel(engine.CanvasLayer{Kind: engine.LayerShape, Shape: shape}); got != label {
			t.Fatalf("形状名称仍显示内部枚举：%s", got)
		}
	}
}

func TestCanvasWorkflowEmptyPreview(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	tool := useTool(t, app, "自由布局").(*CollageStudioTool)
	tool.width, tool.height = 160, 160
	tool.addLayer(engine.LayerShape)
	tester := myui.NewTester(app.View, 960, 620)
	deadline := time.Now().Add(5 * time.Second)
	for app.Preview() == nil && time.Now().Before(deadline) {
		tester.Frame()
		time.Sleep(time.Millisecond)
	}
	tester.Frame()
	if _, found := tester.Find("预览图像"); !found {
		t.Fatal("无图片画布没有实时预览")
	}
	if _, found := tester.Find("拖放区"); found {
		t.Fatal("无图片画布仍占用空文件列表空间")
	}
	dumpImage(t, tester.Image(), "canvas-empty-live-preview")
}

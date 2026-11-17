package ui

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func writeInspectionGIF(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "animation.gif")
	handle, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	palette := color.Palette{color.Black, color.White}
	animation := &gif.GIF{}
	for range 4 {
		animation.Image = append(animation.Image, image.NewPaletted(image.Rect(0, 0, 24, 24), palette))
		animation.Delay = append(animation.Delay, 10)
	}
	encodeErr := gif.EncodeAll(handle, animation)
	closeErr := handle.Close()
	if encodeErr != nil || closeErr != nil {
		t.Fatalf("生成动画失败：%v，%v", encodeErr, closeErr)
	}
	return path
}

func TestInspectionDirectoryConflicts(t *testing.T) {
	for _, name := range []string{"尺寸调整", "宫格切图", "图片布局", "GIF 工具"} {
		for _, strategy := range []settings.ConflictStrategy{settings.ConflictSkip, settings.ConflictRename} {
			t.Run(name+"/"+string(strategy), func(t *testing.T) {
				app := newTestApp(t)
				defer app.CancelStudioWork()
				seedInputs(t, app, 0)
				if name == "GIF 工具" {
					app.AddPaths([]string{writeInspectionGIF(t)})
				} else {
					seedInputs(t, app, 1)
				}
				tool := useTool(t, app, name)
				suffix := ""
				switch current := tool.(type) {
				case *ResizeTool:
					current.mode, current.preset = 3, "24x24"
					suffix = "_sizes"
				case *GridTool:
					suffix = "_grid"
				case *LayoutTool:
					suffix = "_layout"
				case *GIFTool:
					current.action, current.frameStart, current.frameEnd = 4, 1, 3
					suffix = "_frames"
				}
				if err := app.Session.UpdateSettings(func(next *settings.Settings) { next.ConflictStrategy = strategy }); err != nil {
					t.Fatal(err)
				}
				row := app.Session.Snapshot()[0]
				existing := outputDirFor(app.Session.SettingsSnapshot(), row, suffix)
				if err := os.MkdirAll(existing, 0o700); err != nil {
					t.Fatal(err)
				}
				marker := filepath.Join(existing, "keep.txt")
				if err := os.WriteFile(marker, []byte("保留"), 0o600); err != nil {
					t.Fatal(err)
				}
				app.Start()
				waitForRun(t, app)
				result := app.Session.Snapshot()[0]
				if strategy == settings.ConflictSkip {
					if result.Status != "skipped" || result.Output != "" {
						t.Fatalf("已有目录没有跳过：%+v", result)
					}
				} else if result.Status != "ok" || filepath.Dir(result.Output) == existing {
					t.Fatalf("重命名没有隔离目录：%+v", result)
				}
				entries, err := os.ReadDir(existing)
				if err != nil || len(entries) != 1 || entries[0].Name() != "keep.txt" {
					t.Fatalf("原目录内容被改变：%v，%v", entries, err)
				}
			})
		}
	}
}

func TestInspectionGIFInvalidRangeAndAction(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 1)
	tool := useTool(t, app, "GIF 工具").(*GIFTool)
	for _, action := range []int{-1, 99} {
		tool.action = action
		_ = tool.PreviewCaption(app)
		if tool.action != 0 {
			t.Fatal("无效操作没有安全回退")
		}
	}
	tool.action, tool.frameStart, tool.frameEnd = 4, 2, 2
	tester := myui.NewTester(app.View, 960, 620)
	if !tester.HasText("结束帧必须大于起始帧") {
		t.Fatal("无效帧范围没有就地提示")
	}
	app.Start()
	if app.IsRunning() || app.StatusLine() != "结束帧必须大于起始帧" {
		t.Fatal("无效范围仍启动了处理")
	}
	dumpImage(t, tester.Image(), "inspection-gif-range")
}

func waitInspection(t *testing.T, tester *myui.Tester, busy func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for busy() && time.Now().Before(deadline) {
		tester.Frame()
		time.Sleep(time.Millisecond)
	}
	tester.Frame()
	if busy() {
		t.Fatal("分析或对比没有完成")
	}
}

func TestInspectionAnalysisUsesCurrentDecodedImage(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	inputs := seedInputs(t, app, 1)
	tool := useTool(t, app, "图像分析").(*AnalysisTool)
	tester := myui.NewTester(app.View, 1440, 900)
	tool.analyse(app, inputs[0])
	waitInspection(t, tester, func() bool { return tool.busy })
	if tool.sample == nil || tool.result.Width != 120 {
		t.Fatal("没有保留本次分析的源图")
	}
	picture := image.NewNRGBA(image.Rect(0, 0, 120, 90))
	picture.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	if err := saveImage(picture, inputs[0], 95); err != nil {
		t.Fatal(err)
	}
	tool.analyse(app, inputs[0])
	if tool.sample != nil || tool.result.Width != 0 {
		t.Fatal("重新分析时仍展示旧图像和统计")
	}
	waitInspection(t, tester, func() bool { return tool.busy })
	probe, ok := engine.SamplePixel(tool.sample, 0, 0)
	if !ok || probe.G != 255 || probe.R != 0 {
		t.Fatalf("同路径重分析仍读取旧缓存：%+v", probe)
	}
	tool.analyse(app, filepath.Join(t.TempDir(), "missing.png"))
	waitInspection(t, tester, func() bool { return tool.busy })
	if tool.sample != nil || tool.result.Width != 0 || tool.err == "" {
		t.Fatal("分析失败后残留旧结果")
	}
}

func TestInspectionCompareSwapAndReportEntry(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	inputs := seedInputs(t, app, 2)
	tool := useTool(t, app, "图片对比").(*CompareTool)
	tool.reference, tool.result = inputs[0], inputs[1]
	tester := myui.NewTester(app.View, 1440, 900)
	tool.recompute(app)
	waitInspection(t, tester, func() bool { return tool.busy })
	if err := tester.Click("交换两图"); err != nil {
		t.Fatal(err)
	}
	if tool.reference != inputs[1] || tool.result != inputs[0] {
		t.Fatal("交换按钮未更新两张来源")
	}
	waitInspection(t, tester, func() bool { return tool.busy })
	if !tool.metrics.Computable {
		t.Fatal("交换后没有重新计算")
	}
	app.Session.Mu.Lock()
	app.Session.Items[0].Status, app.Session.Items[0].Output = "ok", inputs[0]
	app.Session.Mu.Unlock()
	if app.completedOutputPath() != "" {
		t.Fatal("未导出报告时误显示其他任务的队列结果")
	}
	app.Start()
	report := app.completedOutputPath()
	if !strings.HasSuffix(report, ".txt") {
		t.Fatalf("报告入口被队列结果覆盖：%s", report)
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if _, found := tester.Find("查看结果"); !found {
		t.Fatal("报告保存后缺少结果入口")
	}
	dumpImage(t, tester.Image(), "inspection-compare-report")
	tool.reference = ""
	tool.recompute(app)
	if tool.metrics.Computable || tool.diff != nil {
		t.Fatal("缺少来源时仍保留旧指标或差异图")
	}
}

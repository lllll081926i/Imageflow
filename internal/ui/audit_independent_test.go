package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestAuditRunStateReleased(t *testing.T) {
	for _, name := range []string{"图片对比", "图像分析", "自由布局"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			seedInputs(t, app, 1)
			useTool(t, app, name)
			app.Start()
			if app.IsRunning() {
				t.Fatalf("%s 返回后 Running 仍为 true：%s", name, app.StatusLine())
			}
		})
	}
}

func TestAuditEditPreviewChanges(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 1)
	tool := useTool(t, app, "基础编辑").(*EditTool)
	src := image.NewNRGBA(image.Rect(0, 0, 120, 90))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	app.SetPreview(src)
	tool.ensure()
	tool.rotation = 90
	tester := myui.NewTester(app.View, 1440, 900)
	for frame := 0; frame < 3; frame++ {
		tester.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if got := app.Preview().Bounds(); got.Dx() != 90 || got.Dy() != 120 {
		t.Fatalf("旋转后预览仍为 %v", got)
	}
}

func TestAuditCanvasPreviewConnected(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 1)
	tool := useTool(t, app, "自由布局").(*CollageStudioTool)
	tool.ensure()
	tool.layers = []engine.CanvasLayer{{Kind: engine.LayerShape, Visible: true, W: 100, H: 100, Fill: "#FF0000"}}
	tester := myui.NewTester(app.View, 1440, 900)
	for frame := 0; frame < 3; frame++ {
		tester.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if tool.preview == nil && !tool.busy {
		t.Fatal("自由布局未发起合成预览，仍使用共享单图预览")
	}
}

func TestAuditAnnotationGesture(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 1)
	tool := useTool(t, app, "图片标注").(*AnnotateTool)
	src := image.NewNRGBA(image.Rect(0, 0, 120, 90))
	app.SetPreview(src)
	tester := myui.NewTester(app.View, 1440, 900)
	tester.Frame()
	for frame := 0; frame < 4; frame++ {
		tester.Frame()
	}
	info, ok := tester.Access("标注画布")
	if !ok {
		t.Fatal("找不到标注画布")
	}
	col, row := info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2
	tester.Press(col, row)
	tester.Frame()
	tester.Release(col+20, row+20)
	tester.Frame()
	if len(tool.marks) == 0 {
		t.Fatal("指针输入未创建标注")
	}
}

func TestAuditGradeRunSnapshot(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 12)
	tool := useTool(t, app, "专业调色").(*GradeTool)
	tool.ensure()
	app.Start()
	for step := 0; step < 400; step++ {
		tool.exposure = float64(step % 5)
		time.Sleep(time.Microsecond * 50)
	}
	waitForRun(t, app)
}

func TestAuditZeroDefaultsPersist(t *testing.T) {
	t.Run("截图美化", func(t *testing.T) {
		tool := &ScreenshotTool{}
		tool.ensure()
		tool.padding, tool.radius, tool.gradient = 0, 0, 0
		tool.ensure()
		if tool.padding != 0 || tool.radius != 0 || tool.gradient != 0 {
			t.Fatalf("零值被重置：padding=%v radius=%v angle=%v", tool.padding, tool.radius, tool.gradient)
		}
	})
	t.Run("像素画", func(t *testing.T) {
		tool := &PixelTool{}
		tool.ensure()
		tool.strength, tool.threshold = 0, 0
		tool.ensure()
		if tool.strength != 0 || tool.threshold != 0 {
			t.Fatalf("零值被重置：strength=%v threshold=%v", tool.strength, tool.threshold)
		}
	})
	t.Run("GPU零号", func(t *testing.T) {
		tool := &UpscaleTool{}
		tool.ensure()
		tool.gpu = 0
		tool.ensure()
		if tool.gpu != 0 {
			t.Fatalf("GPU 0 被改为 %v", tool.gpu)
		}
	})
}

func TestAuditCompareReportHonorsSkip(t *testing.T) {
	app := newTestApp(t)
	inputs := seedInputs(t, app, 1)
	next := app.Session.SettingsSnapshot()
	next.ConflictStrategy = settings.ConflictSkip
	if err := app.Session.SetSettings(next); err != nil {
		t.Fatal(err)
	}
	tool := useTool(t, app, "图片对比").(*CompareTool)
	tool.reference, tool.result = inputs[0], inputs[0]
	tool.metrics = engine.CompareResult{Computable: true, Width: 120, Height: 90}
	path := filepath.Join(app.outputFolder(), "compare-report.txt")
	before := []byte("existing report")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	tool.Run(app)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("设置为跳过，已有对比报告仍被覆盖")
	}
}

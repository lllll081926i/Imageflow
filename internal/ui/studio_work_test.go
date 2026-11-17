package ui

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

func waitStudioImage(t *testing.T, app *App, tool Tool) image.Image {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		app.requestStudioPreview(tool)
		img, busy, err := app.studioPreview.snapshot()
		if !busy {
			if err != nil {
				t.Fatal(err)
			}
			if img == nil {
				t.Fatal("预览结果为空")
			}
			app.requestStudioPreview(tool)
			return app.Preview()
		}
		if time.Now().After(deadline) {
			t.Fatal("预览未完成")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStudioPreviewsRenderEachTool(t *testing.T) {
	for _, name := range []string{"基础编辑", "截图美化", "专业调色", "滤镜效果", "像素画 / 点阵", "表情包制作", "自由布局"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			tool := useTool(t, app, name)
			source := image.NewNRGBA(image.Rect(0, 0, 24, 16))
			for row := 0; row < 16; row++ {
				for column := 0; column < 24; column++ {
					source.SetNRGBA(column, row, color.NRGBA{R: 160, G: 80, B: 40, A: 255})
				}
			}
			app.SetPreview(source)
			switch current := tool.(type) {
			case *EditTool:
				current.ensure()
				current.rotation = 90
			case *ScreenshotTool:
				current.ensure()
				current.padding = 2
				current.radius = 0
			case *GradeTool:
				current.ensure()
				current.exposure = 1
			case *FilterTool:
				current.ensure()
				current.kind = "invert"
				current.strength = 100
			case *PixelTool:
				current.ensure()
				current.width, current.height = 8, 8
			case *EmoteTool:
				current.ensure()
				current.bottom = "测试"
			case *CollageStudioTool:
				current.ensure()
				current.width, current.height = 32, 32
				current.layers = []engine.CanvasLayer{{Kind: engine.LayerShape, Shape: engine.ShapeRect, W: 10, H: 10, Fill: "#FF0000", Visible: true, Opacity: 1}}
			}
			result := waitStudioImage(t, app, tool)
			if result == source {
				t.Fatal("仍显示原图而非渲染结果")
			}
			if source.NRGBAAt(0, 0) != (color.NRGBA{R: 160, G: 80, B: 40, A: 255}) {
				t.Fatal("预览修改了源图")
			}
			if name == "基础编辑" && result.Bounds() != image.Rect(0, 0, 16, 24) {
				t.Fatalf("旋转预览错误：%v", result.Bounds())
			}
			if name == "专业调色" {
				red, _, _, _ := result.At(0, 0).RGBA()
				if red <= 160*257 {
					t.Fatal("调色预览未应用曝光")
				}
			}
			if name == "截图美化" && result.Bounds() != image.Rect(0, 0, 28, 20) {
				t.Fatalf("留白预览错误：%v", result.Bounds())
			}
			app.CancelStudioWork()
		})
	}
}

func TestStudioJobRejectsObsoleteResults(t *testing.T) {
	app := newTestApp(t)
	var job studioJob[int]
	defer job.stop()
	started, release := make(chan struct{}), make(chan struct{})
	job.request(app, "旧请求", func(context.Context) (int, error) { close(started); <-release; return 1, nil })
	<-started
	job.request(app, "新请求", func(context.Context) (int, error) { return 2, nil })
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		value, busy, err := job.snapshot()
		if !busy {
			if value != 2 || err != nil {
				t.Fatalf("发布了过期结果：%d %v", value, err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("新请求未完成")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStudioPreviewDoesNotAccumulateTransforms(t *testing.T) {
	app := newTestApp(t)
	tool := useTool(t, app, "基础编辑").(*EditTool)
	tool.ensure()
	tool.rotation = 90
	app.SetPreview(image.NewNRGBA(image.Rect(0, 0, 24, 16)))
	waitStudioImage(t, app, tool)
	tool.rotation = 180
	if got := waitStudioImage(t, app, tool).Bounds(); got != image.Rect(0, 0, 24, 16) {
		t.Fatalf("旋转叠加到了旧预览：%v", got)
	}
	app.SetPreview(image.NewNRGBA(image.Rect(0, 0, 30, 10)))
	if app.Preview().Bounds() != image.Rect(0, 0, 30, 10) {
		t.Fatal("换图后仍显示旧结果")
	}
	if got := waitStudioImage(t, app, tool).Bounds(); got != image.Rect(0, 0, 30, 10) {
		t.Fatalf("换图预览错误：%v", got)
	}
	app.CancelStudioWork()
}

func TestBatchContextCancelsActiveTask(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 1)
	started := make(chan struct{})
	app.SetRunning(true)
	app.runBatchContext("取消测试", func(ctx context.Context, _ int, _ batch.Item) engine.Result {
		close(started)
		<-ctx.Done()
		return engine.Fail("CANCELLED", "已取消")
	})
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("任务未启动")
	}
	app.Cancel()
	waitForRun(t, app)
	if row := app.Session.Snapshot()[0]; row.Status != "cancelled" {
		t.Fatalf("取消状态错误：%+v", row)
	}
	if strings.Contains(app.StatusLine(), "完成：") {
		t.Fatalf("取消被报告为完成：%s", app.StatusLine())
	}
}

func TestZeroValuesSurvivePanelFrames(t *testing.T) {
	app := newTestApp(t)
	tool := useTool(t, app, "专业调色").(*GradeTool)
	tool.ensure()
	tool.curveMiddle, tool.lutStrength, tool.exposure = 0, 0, 1
	tester := myui.NewTester(app.View, 1440, 900)
	tester.Frame()
	tester.Frame()
	if tool.curveMiddle != 0 || tool.lutStrength != 0 || tool.exposure != 1 {
		t.Fatal("面板重建覆盖了调色参数")
	}
	emote := useTool(t, app, "表情包制作").(*EmoteTool)
	emote.ensure()
	emote.bottom, emote.top, emote.center, emote.stroke = "", "", "", 0
	tester.Frame()
	tester.Frame()
	if emote.bottom != "" || emote.stroke != 0 {
		t.Fatal("面板重建覆盖了空字幕或零描边")
	}
}

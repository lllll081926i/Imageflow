package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestClipboardCopyCommitsOnUIQueue(t *testing.T) {
	app := newTestApp(t)
	inputs := seedInputs(t, app, 1)
	tool := useTool(t, app, "图片拼图").(*CollageTool)
	var services myui.Services
	tester := myui.NewTester(func(ctx *myui.Context) { services = ctx.Services() }, 64, 64)
	request := engine.CollageRequest{Cells: []engine.CollageCell{{Path: inputs[0]}}, LongEdge: 32, AspectW: 1, AspectH: 1}
	queue := make(chan func(), 1)
	tool.startCopy(app, request, services, func(callback func()) { queue <- callback })
	select {
	case commit := <-queue:
		if _, ok := tester.ClipboardData().Preferred(transfer.PNG); ok {
			t.Fatal("窗口提交前写入了剪贴板")
		}
		if !tool.copying {
			t.Fatal("窗口提交前提前结束复制")
		}
		commit()
	case <-time.After(10 * time.Second):
		t.Fatal("后台复制未提交结果")
	}
	if _, ok := tester.ClipboardData().Preferred(transfer.PNG); !ok || tool.copying {
		t.Fatal("窗口提交后未完成复制")
	}
}

func TestCollageGroupsKeepIndividualOutcomes(t *testing.T) {
	for _, outcome := range []string{"失败", "跳过", "取消", "提交后取消", "异常"} {
		t.Run(outcome, func(t *testing.T) {
			app := newTestApp(t)
			seedInputs(t, app, 3)
			app.SetRunning(true)
			cancel := app.beginWholeList()
			rows := app.Session.Snapshot()
			groups := [][]batch.Item{{rows[0]}, {rows[1]}, {rows[2]}}
			requests := make([]engine.CollageRequest, len(groups))
			for index := range requests {
				requests[index] = engine.CollageRequest{Cells: []engine.CollageCell{{Path: rows[index].Path}}, Format: "png"}
			}
			calls := 0
			app.runCollageGroups(groups, requests, cancel, func(_ context.Context, request engine.CollageRequest) engine.Result {
				calls++
				if calls == 2 {
					switch outcome {
					case "失败":
						return engine.Fail("TEST_FAILED", "测试失败")
					case "跳过":
						return engine.Skip("测试跳过")
					case "取消":
						cancel.Cancel()
						return engine.Fail("CANCELLED", "已取消")
					case "提交后取消":
						cancel.Cancel()
					case "异常":
						panic("测试异常")
					}
				}
				result := engine.OK()
				result.OutputPath = request.OutputPath
				return result
			})
			actual := app.Session.Snapshot()
			wantSecond, wantThird := "failed", "ok"
			switch outcome {
			case "跳过":
				wantSecond = "skipped"
			case "取消":
				wantSecond, wantThird = "cancelled", "cancelled"
			case "提交后取消":
				wantSecond, wantThird = "ok", "cancelled"
			}
			if actual[0].Status != "ok" || actual[0].Output == "" || actual[1].Status != wantSecond || actual[2].Status != wantThird {
				t.Fatalf("分组结果错误：%+v", actual)
			}
			if app.IsRunning() || app.Session.Cancel != nil {
				t.Fatal("分组结束未释放运行状态")
			}
		})
	}
}

func TestIconDirectoryConflictPolicies(t *testing.T) {
	for _, strategy := range []settings.ConflictStrategy{settings.ConflictSkip, settings.ConflictRename, settings.ConflictOverwrite} {
		t.Run(string(strategy), func(t *testing.T) {
			app := newTestApp(t)
			seedInputs(t, app, 1)
			config := app.Session.SettingsSnapshot()
			config.ConflictStrategy = strategy
			if err := app.Session.SetSettings(config); err != nil {
				t.Fatal(err)
			}
			row := app.Session.Snapshot()[0]
			existing := outputDirFor(config, row, "_icons")
			if err := os.MkdirAll(existing, 0700); err != nil {
				t.Fatal(err)
			}
			first, skipped := app.outputDirectoryFor(row, "_icons")
			if skipped != (strategy == settings.ConflictSkip) {
				t.Fatal("目录跳过策略错误")
			}
			if strategy == settings.ConflictRename {
				second, _ := app.outputDirectoryFor(row, "_icons")
				if first == existing || first == second {
					t.Fatal("目录重命名未隔离同名输入")
				}
			} else if first != existing {
				t.Fatal("非重命名策略改变了目录")
			}
		})
	}
}

func TestPreviewPublishesOnlyCurrentResult(t *testing.T) {
	app := newTestApp(t)
	inputs := seedInputs(t, app, 1)
	tool := useTool(t, app, "图片拼图").(*CollageTool)
	request := engine.CollageRequest{Cells: []engine.CollageCell{{Path: inputs[0]}}, LongEdge: 32, AspectW: 1, AspectH: 1}
	queue := make(chan func(), 4)
	publish := func(callback func()) { queue <- callback }
	receive := func() func() {
		select {
		case callback := <-queue:
			return callback
		case <-time.After(10 * time.Second):
			t.Fatal("后台预览未提交结果")
			return nil
		}
	}
	tool.startPreview(app, request, publish)
	old := receive()
	if !tool.previewBusy || tool.previewBitmap != nil {
		t.Fatal("后台直接修改了预览状态")
	}
	tool.startPreview(app, request, publish)
	current := receive()
	old()
	if !tool.previewBusy || tool.previewBitmap != nil {
		t.Fatal("旧预览覆盖了新请求")
	}
	current()
	if tool.previewBusy || tool.previewBitmap == nil {
		t.Fatal("当前预览未在提交时显示")
	}
	tool.startPreview(app, request, publish)
	closed := receive()
	app.CancelCollageWork()
	closed()
	if tool.previewBusy {
		t.Fatal("关闭后仍显示预览处理中")
	}
}

func TestRegressionSkippedCollageGroup(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 4)
	tool := useTool(t, app, "图片拼图").(*CollageTool)
	tool.presetID, tool.mode, tool.longEdge = "2-h", collagePerSlot, 64
	config := app.Session.SettingsSnapshot()
	config.ConflictStrategy = settings.ConflictSkip
	if err := app.Session.SetSettings(config); err != nil {
		t.Fatal(err)
	}
	rows := app.Session.Snapshot()
	existing := batch.BuildOutputPath(config, rows[0].Path, rows[0].RelPath, "collage", "png", 1)
	if err := os.WriteFile(existing, []byte("保留原文件"), 0600); err != nil {
		t.Fatal(err)
	}
	app.Start()
	waitForRun(t, app)
	rows = app.Session.Snapshot()
	if rows[0].Status != "skipped" || rows[1].Status != "skipped" {
		t.Fatalf("第一组应保持 skipped，实际为 %s / %s；输出 %s", rows[0].Status, rows[1].Status, rows[0].Output)
	}
}

func TestRegressionEmptyPresetReleasesRunning(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 1)
	tool := useTool(t, app, "尺寸调整").(*ResizeTool)
	tool.mode, tool.preset = 3, "无效尺寸"
	app.Start()
	if app.IsRunning() {
		t.Fatal("参数校验失败后 Running 仍为 true，后续 Start 无法执行")
	}
}

func TestRegressionRemovedRowKeepsResultIdentity(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 2)
	rows := app.Session.Snapshot()
	entered, release, exited := make(chan struct{}), make(chan struct{}), make(chan struct{})
	app.SetRunning(true)
	app.runBatch("审查", func(index int, item batch.Item) engine.Result {
		if index == 0 {
			close(entered)
			<-release
			close(exited)
		} else {
			<-exited
		}
		result := engine.OK()
		result.OutputPath = item.Path + ".result"
		return result
	})
	<-entered
	app.removeByPath(rows[0].Path)
	close(release)
	waitForRun(t, app)
	remaining := app.Session.Snapshot()[0]
	if remaining.Output != rows[1].Path+".result" {
		t.Fatalf("第二张图片拿到了第一张的结果：%s", remaining.Output)
	}
}

func TestRegressionAPNGHonorsSkip(t *testing.T) {
	app := newTestApp(t)
	inputs := seedInputs(t, app, 1)
	palette := color.Palette{color.Black, color.White}
	input := inputs[0] + ".gif"
	handle, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(handle, &gif.GIF{Image: []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 2, 2), palette), image.NewPaletted(image.Rect(0, 0, 2, 2), palette)}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	handle.Close()
	app.Session.Clear()
	app.AddPaths([]string{input})
	tool := useTool(t, app, "GIF 工具").(*GIFTool)
	tool.ensure()
	tool.format = "apng"
	config := app.Session.SettingsSnapshot()
	config.ConflictStrategy = settings.ConflictSkip
	app.Session.SetSettings(config)
	row := app.Session.Snapshot()[0]
	planned := batch.BuildOutputPath(config, row.Path, row.RelPath, gifActionKeys[tool.action], "png", 1)
	actual := planned[:len(planned)-len(filepath.Ext(planned))] + ".apng"
	sentinel := []byte("现有 APNG 文件不得覆盖")
	os.WriteFile(actual, sentinel, 0600)
	app.Start()
	waitForRun(t, app)
	got, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Fatalf("skip 策略仍覆盖了 %s", actual)
	}
}

func TestRegressionIconHonorsSkip(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 1)
	useTool(t, app, "图标生成")
	config := app.Session.SettingsSnapshot()
	config.ConflictStrategy = settings.ConflictSkip
	app.Session.SetSettings(config)
	row := app.Session.Snapshot()[0]
	dir := outputDirFor(config, row, "_icons")
	os.MkdirAll(dir, 0700)
	target := filepath.Join(dir, "icon-16x16.png")
	sentinel := []byte("现有图标不得覆盖")
	os.WriteFile(target, sentinel, 0600)
	app.Start()
	waitForRun(t, app)
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Fatalf("skip 策略仍覆盖了 %s", target)
	}
}

func TestRegressionCollageRunWhileEditing(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 4)
	tool := useTool(t, app, "图片拼图").(*CollageTool)
	tool.presetID, tool.mode, tool.longEdge = "2-h", collagePerSlot, 128
	app.Start()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		tool.askRedraw()
		runtime.Gosched()
	}
	waitForRun(t, app)
}

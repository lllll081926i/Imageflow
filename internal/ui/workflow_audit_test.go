package ui

import (
	"path/filepath"
	"testing"

	myui "github.com/egoist/mygo/ui"
)

func TestWorkflowDeleteSelectedInput(t *testing.T) {
	for layout, name := range []string{"列表", "网格", "紧凑"} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			inputs := seedInputs(t, app, 3)
			app.openTool(app.toolIndex(ToolConvert))
			app.fileLayout = layout
			tester := myui.NewTester(app.View, 1440, 900)
			if err := tester.Click("预览 " + filepath.Base(inputs[1])); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			tester.Key(0, myui.KeyDelete)
			tester.Frame()
			rows := app.Session.Snapshot()
			if len(rows) != 2 || rows[0].Path != inputs[0] || rows[1].Path != inputs[2] {
				t.Fatalf("Delete 未只移除选中的图片：%+v", rows)
			}
			waitForPreview(t, app, inputs[2])
			tester.Frame()
			app.SetRunning(true)
			tester.Frame()
			tester.Key(0, myui.KeyDelete)
			if app.Session.Len() != 2 {
				t.Fatal("处理中仍可按 Delete 移除图片")
			}
			app.SetRunning(false)
			tester.Frame()
			if layout == 1 {
				dumpImage(t, tester.Image(), "workflow-result-queue")
			}
		})
	}
}

func TestWorkflowClearProtectsActiveRun(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	seedInputs(t, app, 2)
	app.SetRunning(true)
	tester := myui.NewTester(app.View, 1440, 900)
	if err := tester.Click("清空"); err != nil {
		t.Fatal(err)
	}
	app.clearInputs()
	if app.Session.Len() != 2 || app.Preview() == nil {
		t.Fatal("处理中清空了队列或预览")
	}
	app.SetRunning(false)
	tester.Frame()
	if err := tester.Click("清空"); err != nil {
		t.Fatal(err)
	}
	if app.Session.Len() != 0 || app.Preview() != nil {
		t.Fatal("空闲时清空未生效")
	}
}

func TestWorkflowResultEntry(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	inputs := seedInputs(t, app, 2)
	app.openTool(app.toolIndex(ToolConvert))
	tester := myui.NewTester(app.View, 1440, 900)
	if _, found := tester.Find("取消"); found {
		t.Fatal("空闲时仍显示无效取消按钮")
	}
	if _, found := tester.Find("查看结果"); found {
		t.Fatal("未导出就显示查看结果")
	}
	app.Start()
	waitForRun(t, app)
	tester.Frame()
	if _, found := tester.Find("查看结果"); !found {
		t.Fatal("完成后缺少结果入口")
	}
	dumpImage(t, tester.Image(), "workflow-completed-result")
	rows := app.Session.Snapshot()
	app.selectBrowserPreview(inputs[1])
	waitForPreview(t, app, inputs[1])
	if got := app.completedOutputPath(); got != rows[1].Output {
		t.Fatalf("未定位所选图片的实际结果：%s", got)
	}
	app.Session.Mu.Lock()
	app.Session.Items[1].Status = "failed"
	app.Session.Mu.Unlock()
	if got := app.completedOutputPath(); got != rows[0].Output {
		t.Fatalf("所选图片失败后未回退到成功结果：%s", got)
	}
}

func TestWorkflowPreviewSkipsDuplicateSelection(t *testing.T) {
	app := newTestApp(t)
	inputs := seedInputs(t, app, 2)
	version := app.previewVersion
	for range 5 {
		app.selectBrowserPreview(inputs[0])
	}
	if app.previewVersion != version {
		t.Fatal("重复选择同一图片触发重复解码")
	}
	app.previewLoadMu.Lock()
	app.selectBrowserPreview(inputs[1])
	version = app.previewVersion
	app.selectBrowserPreview(inputs[1])
	unchanged := app.previewVersion == version
	app.previewLoadMu.Unlock()
	if !unchanged {
		t.Fatal("正在加载的图片触发重复解码")
	}
	waitForPreview(t, app, inputs[1])
	app.previewMu.Lock()
	app.lastPreview = nil
	app.previewMu.Unlock()
	app.selectBrowserPreview(inputs[1])
	waitForPreview(t, app, inputs[1])
	if app.previewVersion == version {
		t.Fatal("失败预览无法重试")
	}
}

func TestWorkflowDeleteDoesNotStealParameterInput(t *testing.T) {
	app := newTestApp(t)
	seedInputs(t, app, 2)
	value := "参数"
	tester := myui.NewTester(func(c *myui.Context) {
		myui.Row(c).Fill().Children(func() {
			app.inputCard(c, newStyle(false))
			myui.TextInput(c, &value).Width(200).Height(40).Label("参数文本")
		})
	}, 1000, 800)
	if err := tester.Click("参数文本"); err != nil {
		t.Fatal(err)
	}
	tester.Key(0, myui.KeyDelete)
	if app.Session.Len() != 2 {
		t.Fatal("参数输入框的 Delete 移除了队列图片")
	}
}

func TestWorkflowFilterZeroAmountSurvives(t *testing.T) {
	for _, kind := range []string{"grain", "vignette", "sharpen"} {
		t.Run(kind, func(t *testing.T) {
			app := newTestApp(t)
			defer app.CancelStudioWork()
			tool := useTool(t, app, "滤镜效果").(*FilterTool)
			tool.ensure()
			tool.kind, tool.amount = kind, 0
			tester := myui.NewTester(app.View, 1440, 900)
			for range 4 {
				tester.Frame()
			}
			if tool.amount != 0 || tool.request().Amount != 0 {
				t.Fatal("零幅度被默认值覆盖")
			}
		})
	}
}

package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestReleasePagesAvoidExtraExplanations(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	for _, tool := range app.Tools {
		tester := acceptancePanel(app, tool)
		for _, text := range tester.Texts() {
			for _, unwanted := range []string{"分开验收", "按线性光计算", "不是模型本身的输出", "不能替代人工视觉评估", "与调色页的取样口径一致", "普通地址：", "国内加速："} {
				if strings.Contains(text, unwanted) {
					t.Fatalf("%s 仍有多余说明：%s", tool.Name(), text)
				}
			}
		}
	}
	app.ShowHome = true
	tester := myui.NewTester(app.View, 1440, 900)
	for _, text := range []string{"全程不联网", "先从这里开始", "不读取你的图片内容"} {
		if tester.HasText(text) {
			t.Fatalf("首页仍有多余说明：%s", text)
		}
	}
}

func TestReleaseAnnotatePreviewMatchesExport(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	tool := useTool(t, app, "图片标注").(*AnnotateTool)
	source := image.NewNRGBA(image.Rect(0, 0, 120, 90))
	for row := 0; row < 90; row++ {
		for column := 0; column < 120; column++ {
			source.SetNRGBA(column, row, color.NRGBA{R: 255, A: 255})
		}
	}
	app.SetPreview(source)
	tool.marks = []engine.AnnotateMark{{Kind: engine.AnnotateCover, X1: 10, Y1: 10, X2: 80, Y2: 60, Color: "#000000"}}
	preview := waitStudioImage(t, app, tool)
	if pixel := color.NRGBAModel.Convert(preview.At(30, 30)).(color.NRGBA); pixel != (color.NRGBA{A: 255}) {
		t.Fatalf("安全遮盖预览没有实际填充：%v", pixel)
	}
	if source.NRGBAAt(30, 30).R != 255 {
		t.Fatal("标注预览修改了源图")
	}
	tool.marks = nil
	if pixel := color.NRGBAModel.Convert(waitStudioImage(t, app, tool).At(30, 30)).(color.NRGBA); pixel.R != 255 {
		t.Fatal("删除标注后仍显示旧遮盖")
	}
}

func TestReleaseImportRemoveAndKeyboard(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	paths := seedInputs(t, app, 2)
	app.Settings.UIReducedMotion = true
	tester := myui.NewTester(app.View, 1440, 900)
	if err := tester.Click("预览 " + app.Session.Snapshot()[0].Name); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Key(0, myui.KeyRight)
	tester.Frame()
	if app.selectedPreviewPath() != paths[1] {
		t.Fatal("方向键没有切换到第二张图片")
	}
	app.removeByPath(paths[1])
	tester.Frame()
	if app.Session.Len() != 1 {
		t.Fatal("移除文件后列表未更新")
	}
	if added := app.AddPaths([]string{paths[0]}); added != 0 || app.Session.Len() != 1 {
		t.Fatal("重复导入未去重")
	}
	if err := tester.Click("清空"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if app.Session.Len() != 0 {
		t.Fatal("清空操作没有清空输入列表")
	}
}

func TestReleaseSettingsScrollAndSave(t *testing.T) {
	for _, width := range []int{960, 1440} {
		app := newTestApp(t)
		app.ShowSettings = true
		tester := myui.NewTester(app.View, width, 620)
		clicked := false
		for attempt := 0; attempt < 20; attempt++ {
			if control, found := tester.AccessByRole(myui.RoleSwitch, "后台运行"); found && !control.Offscreen && control.Bounds.Y > 68 && control.Bounds.Y+control.Bounds.H < 500 {
				tester.ClickAt(control.Bounds.X+control.Bounds.W/2, control.Bounds.Y+control.Bounds.H/2)
				tester.Frame()
				clicked = true
				break
			}
			tester.Scroll(float32(width-200), 300, 0, 180)
			tester.Frame()
		}
		if !clicked || !app.Settings.BackgroundRunning {
			t.Fatalf("宽度 %d：设置页滚动后无法修改后台运行", width)
		}
		if err := tester.Click("保存设置"); err != nil {
			t.Fatalf("宽度 %d：固定保存按钮不可操作：%v", width, err)
		}
		if !settings.Load().BackgroundRunning {
			t.Fatal("界面修改没有保存到设置文件")
		}
		app.CancelStudioWork()
	}
}

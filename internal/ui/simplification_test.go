package ui

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func TestSettingsHideTechnicalSections(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	app.ShowSettings = true
	tester := myui.NewTester(app.View, 1440, 900)
	for _, text := range []string{"编解码能力", "模型与组件", "减弱动效", "模型保存目录", "本地推理引擎"} {
		if tester.HasText(text) {
			t.Fatalf("设置页仍显示已移除项目：%s", text)
		}
	}
	dumpFrame(t, app, "settings-simplified")
}

func TestUpscaleHidesManualInstallationControls(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	app.openTool(app.toolIndex(ToolUpscale))
	tester := myui.NewTester(app.View, 1440, 900)
	for _, text := range []string{"引擎可执行文件", "模型目录", "可信模型清单", "导入模型清单", "按默认源下载"} {
		if tester.HasText(text) {
			t.Fatalf("AI 页仍显示手动安装控件：%s", text)
		}
	}
	// The purpose travels with the name inside the menu, so the page no longer carries a
	// separate caption below it: the trigger shows the decorated label and nothing else
	// repeats it.
	if !tester.HasText("一键下载") {
		t.Fatal("AI 页缺少一键下载")
	}
	if !tester.HasText("upscayl-standard-4x（照片 / 通用）") {
		t.Fatal("模型用途未随模型名显示")
	}
	if tester.HasText("适合：照片 / 通用") {
		t.Fatal("模型用途仍在单独的说明行中重复")
	}
	dumpFrame(t, app, "upscale-simplified")
}

func TestActiveStudioGroupCanFoldAndReopen(t *testing.T) {
	app := newTestApp(t)
	app.Settings.UIReducedMotion = true
	app.openTool(app.toolIndex(ToolGrade))
	app.toggleGroup(GroupCore)
	app.toggleGroup(GroupCreator)
	tester := myui.NewTester(app.View, 1440, 900)
	if err := tester.Click("专业图像"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if app.groupExpanded(GroupStudio) || app.railToolRowY(app.Active) >= 0 || app.ActiveTool().ID() != ToolGrade {
		t.Fatal("专业图像分组未折叠或当前页面被关闭")
	}
	loaded := NewApp(settings.Load())
	if loaded.groupExpanded(GroupStudio) {
		t.Fatal("分组折叠状态未保存")
	}
	if err := tester.Click("专业图像"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !app.groupExpanded(GroupStudio) || app.railToolRowY(app.Active) < 0 {
		t.Fatal("专业图像分组未重新展开")
	}
	app.toggleGroup(GroupStudio)
	app.railCollapsed = true
	tester.Frame()
	if app.railToolCount() != len(app.Tools) {
		t.Fatal("收起侧栏后折叠分组中的工具无法访问")
	}
	for index, row := range app.railRows() {
		if row.Header || row.Y != float32(index*(railRowHeight+railRowGap)) {
			t.Fatalf("收起侧栏存在分组空隙：%+v", row)
		}
	}
	app.railCollapsed = false
	app.openTool(app.toolIndex(ToolGrade))
	if !app.groupExpanded(GroupStudio) {
		t.Fatal("从首页打开工具后未展开所属分组")
	}
}

func TestHomeCardsAreLarger(t *testing.T) {
	for _, width := range []int{960, 1440} {
		app := newTestApp(t)
		app.windowWidth = float32(width)
		tester := myui.NewTester(func(c *myui.Context) {
			app.homePage(c, newStyle(false))
		}, width, 900)
		card, found := tester.AccessByRole(myui.RoleButton, "图片压缩")
		if !found || card.Bounds.H < 132 || card.Bounds.W < 240 {
			t.Fatalf("宽度 %d：首页卡片过小：%+v", width, card)
		}
	}
}

func TestRailWheelNavigationWithoutScrollbar(t *testing.T) {
	app := newTestApp(t)
	app.Settings.UIReducedMotion = true
	tester := myui.NewTester(app.View, 1440, 900)
	for attempt := 0; attempt < 40; attempt++ {
		button, found := tester.AccessByRole(myui.RoleButton, "转 PDF")
		if found && !button.Offscreen && button.Bounds.Y > 20 && button.Bounds.Y+button.Bounds.H < 720 {
			if err := tester.Click("转 PDF"); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			if app.ActiveTool().ID() != ToolPDF {
				t.Fatal("侧栏滚动后未切换到目标页面")
			}
			return
		}
		tester.Wheel(110, 350, 0, 200)
		tester.Advance(200 * time.Millisecond)
		tester.Frame()
	}
	t.Fatal("隐藏滚动条后无法用滚轮访问末尾工具")
}

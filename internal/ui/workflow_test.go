package ui

import (
	"testing"

	myui "github.com/egoist/mygo/ui"
)

func TestPrimaryActionFollowsPageState(t *testing.T) {
	app := newTestApp(t)
	defer app.CancelStudioWork()
	compare := useTool(t, app, "图片对比").(*CompareTool)
	tester := myui.NewTester(app.View, 960, 620)
	check := func(label string, disabled bool) {
		t.Helper()
		tester.Frame()
		action, found := tester.AccessByRole(myui.RoleButton, label)
		if !found || action.Offscreen || action.Disabled != disabled {
			t.Fatalf("主操作 %q 状态错误：%+v", label, action)
		}
	}
	check("选择参考图", false)
	compare.reference = "reference.png"
	check("选择结果图", false)
	compare.result = "result.png"
	compare.busy = true
	check("正在计算指标…", true)
	compare.busy = false
	compare.metrics.Computable = true
	check("导出对比报告", false)
	gifTool := useTool(t, app, "GIF 工具").(*GIFTool)
	gifTool.action = 4
	check("导出帧图片", true)
	resize := useTool(t, app, "尺寸调整").(*ResizeTool)
	resize.ensure()
	resize.mode = 3
	check("导出多尺寸图片", true)
	useTool(t, app, "图像分析")
	check("选择要分析的图片", false)
}

package ui

// layout_test.go pins the shell's geometry to the design contract: a rail card
// of railWidth DIP, a 420 DIP parameter card, and an input card that takes the
// rest. Regressions in these numbers are what make the UI feel unlike the
// original, so they are asserted rather than eyeballed.

import (
	"testing"

	myui "github.com/egoist/mygo/ui"
)

// shellGeometry renders the real app and reads the boxes of its landmarks by
// their accessible labels.
func shellGeometry(t *testing.T, width, height int) map[string]myui.Rect {
	t.Helper()
	a := newTestApp(t)
	a.Active = 0
	a.ShowSettings = false
	tt := myui.NewTester(a.View, width, height)
	tt.Frame()

	out := map[string]myui.Rect{}
	for name, want := range map[string]string{
		"rail":     "工具卡",
		"input":    "输入卡",
		"drop":     "拖放区",
		"settings": "参数卡",
		"action":   a.actionLabel(),
	} {
		info, ok := tt.Access(want)
		if !ok {
			t.Fatalf("%s (%q) not found in the frame", name, want)
		}
		out[name] = info.Bounds
	}
	return out
}

func TestShellGeometryAtDefaultSize(t *testing.T) {
	boxes := shellGeometry(t, 1440, 900)

	rail := boxes["rail"]
	input := boxes["input"]
	settings := boxes["settings"]
	action := boxes["action"]

	// The rail hugs the left edge, under the title bar.
	if rail.X > 24 {
		t.Errorf("rail starts at x=%v, want a small margin", rail.X)
	}
	// Literal value, not the constant under test: this table is the design
	// contract for the 216 DIP rail, so a wrong constant cannot pass by
	// dragging the assertion along with it.
	if rail.W < 212 || rail.W > 220 {
		t.Errorf("rail width = %v, want 216", rail.W)
	}
	// The input card sits between the rail and the parameter card.
	if input.X <= rail.X+rail.W {
		t.Errorf("input card x=%v not right of the rail (%v+%v)", input.X, rail.X, rail.W)
	}
	if input.W < 300 {
		t.Errorf("input card squeezed to %v", input.W)
	}
	// The parameter card is on the right, 420 wide.
	if settings.X <= input.X {
		t.Errorf("settings x=%v not right of the input card (%v)", settings.X, input.X)
	}
	if settings.W < 415 || settings.W > 425 {
		t.Errorf("settings card width = %v, want 420", settings.W)
	}
	if settings.X+settings.W > 1440 {
		t.Errorf("settings card overflows the window: %v+%v", settings.X, settings.W)
	}
	// The action bar is under the parameters, at the bottom.
	if action.Y <= settings.Y {
		t.Errorf("action bar y=%v is not below the first parameter card (%v)", action.Y, settings.Y)
	}
	if action.Y+action.H > 900 {
		t.Errorf("action bar overflows the window: %v+%v", action.Y, action.H)
	}
	if action.W < 200 {
		t.Errorf("action bar width = %v", action.W)
	}
	// The cards start at the top of the content area: the window keeps its native
	// title bar, so the app draws no header of its own and the shell owns the
	// full client area.
	if rail.Y > 2 {
		t.Errorf("rail y=%v, want it at the top of the client area", rail.Y)
	}
}

// TestShellGeometryAtMinimumSize pins the narrow-window rule: at 960 DIP the rail
// folds itself to the icon strip so the three cards still fit, and the cards stay
// side by side with room to work in.
func TestShellGeometryAtMinimumSize(t *testing.T) {
	// The window's minimum is 960x620; the shell must still lay out.
	boxes := shellGeometry(t, 960, 620)
	rail := boxes["rail"]
	input := boxes["input"]
	settings := boxes["settings"]
	if rail.W > 76 {
		t.Errorf("rail width = %v at the minimum width, want the 72 DIP folded strip", rail.W)
	}
	if input.W < 100 {
		t.Errorf("input card squeezed to %v at the minimum width", input.W)
	}
	if settings.X+settings.W > 960 {
		t.Errorf("settings card overflows at the minimum width: %v+%v",
			settings.X, settings.W)
	}
}

// TestToolsUseWorkbenchColumns asserts every real tool page keeps the same three
// cards and the same actions.
func TestToolsUseWorkbenchColumns(t *testing.T) {
	a := newTestApp(t)
	for i := range a.Tools {
		a.Active = i
		a.ShowSettings = false
		tt := myui.NewTester(a.View, 1440, 900)
		for _, want := range []string{
			"工具卡", "输入卡", "参数卡", a.actionLabel(), "选择输出位置",
		} {
			if !tt.HasText(want) {
				t.Errorf("tool %d (%s) missing %q", i, a.Tools[i].Name(), want)
			}
		}
		if tt.HasText("取消") {
			t.Errorf("tool %d shows 取消 with an empty list", i)
		}
		if len(tt.Texts()) < 10 {
			t.Errorf("tool %d rendered only %d strings", i, len(tt.Texts()))
		}
		// A control torn by a too-narrow cell wraps its text; the accessible
		// name is still one string but the drawn text is not. Assert the panel
		// scrolls rather than clipping: the action bar must stay visible.
		if info, ok := tt.AccessByRole(myui.RoleButton, a.actionLabel()); !ok || info.Bounds.H < 30 {
			t.Errorf("tool %d action button is clipped: %+v", i, info.Bounds)
		}
	}
}

func TestStopAppearsOnlyWhileRunning(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 0
	tt := myui.NewTester(a.View, 1440, 900)
	if tt.HasText("取消") || tt.HasText("停止处理") {
		t.Fatal("空闲时显示了取消操作")
	}
	a.SetRunning(true)
	tt.Frame()
	if !tt.HasText("停止处理") {
		t.Fatal("处理中缺少停止操作")
	}
	a.SetRunning(false)
	tt.Frame()
	if tt.HasText("停止处理") {
		t.Fatal("处理结束后仍显示停止操作")
	}
}

// TestSettingsPageShape asserts the settings page keeps the shell's shape: one
// scrolling card of grouped sections with the save action pinned under it.
// There is no 返回工具 button: the rail's 全局设置 entry is already a toggle, and
// a second way out of the page was a duplicate control in the header.
func TestSettingsPageShape(t *testing.T) {
	a := newTestApp(t)
	a.ShowSettings = true
	tt := myui.NewTester(a.View, 1440, 900)
	for _, want := range []string{
		"全局设置", "保存设置", "输出命名", "输出位置",
		"并发与行为", "关于",
	} {
		if !tt.HasText(want) {
			t.Errorf("settings page missing %q", want)
		}
	}
	// The header must not carry a duplicate way out: selecting any tool in the
	// rail leaves the settings page, and 全局设置 toggles back to it.
	if tt.HasText("返回工具") {
		t.Error("settings header still shows 返回工具")
	}
}

func TestSettingsResponsiveColumns(t *testing.T) {
	wideApp := newTestApp(t)
	wideApp.ShowSettings = true
	wide := myui.NewTester(wideApp.View, 1440, 900)
	output, outputOK := wide.Access("输出设置")
	application, applicationOK := wide.Access("应用设置")
	if !outputOK || !applicationOK {
		t.Fatalf("wide settings columns missing: output=%v application=%v", outputOK, applicationOK)
	}
	if application.Bounds.X <= output.Bounds.X+output.Bounds.W {
		t.Errorf("wide settings columns overlap: output=%+v application=%+v", output.Bounds, application.Bounds)
	}
	if dh := output.Bounds.H - application.Bounds.H; dh < -1 || dh > 1 {
		t.Errorf("wide settings columns height mismatch: output=%v application=%v", output.Bounds.H, application.Bounds.H)
	}
	if wide.HasText("尚未检查更新") == false {
		t.Error("an unchecked update must not claim to be up to date")
	}

	narrowApp := newTestApp(t)
	narrowApp.ShowSettings = true
	narrow := myui.NewTester(narrowApp.View, 960, 620)
	if _, ok := narrow.Access("设置单列"); !ok {
		t.Fatal("narrow settings did not collapse to one column")
	}
	for _, want := range []string{"保存设置", "还原默认值"} {
		info, ok := narrow.Access(want)
		if !ok || info.Bounds.X < 0 || info.Bounds.X+info.Bounds.W > 960 {
			t.Errorf("narrow settings action %q is missing or clipped: %+v", want, info.Bounds)
		}
	}
}

func TestOperationFeedbackAndDisabledAction(t *testing.T) {
	a := newTestApp(t)
	a.SetStatus("转换失败：编码不可用")
	tt := myui.NewTester(a.View, 1440, 900)
	if !tt.HasText("转换失败：编码不可用") {
		t.Fatal("operation failure is not visible in the workbench")
	}
	if _, ok := tt.Access("转换失败：编码不可用"); !ok {
		t.Error("operation feedback has no accessible status")
	}
	action, ok := tt.Access(a.actionLabel())
	if !ok || !action.Disabled {
		t.Errorf("empty input action should be disabled: %+v", action)
	}
}

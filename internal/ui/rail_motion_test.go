package ui

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestRailCollapseKeepsCardsInStep pins the two halves of the shell to one
// movement. The rail's width is animated by AnimateTo on the parent Row, so the
// page beside it must take the new layout on that very frame. A Transition that
// animates position or size on the page container instead eases the same change
// a second time, with a different curve: the cards lag behind the rail and the
// collapse reads as two unrelated animations.
func TestRailCollapseKeepsCardsInStep(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	a.railCollapsed = true
	worst := float32(0)
	for i := 0; i < 30; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		rail, okR := tt.Access("工具卡")
		input, okI := tt.Access("输入卡")
		if !okR || !okI {
			continue
		}
		gap := input.Bounds.X - (rail.Bounds.X + rail.Bounds.W)
		if gap > worst {
			worst = gap
		}
		if gap < 14 || gap > 18 {
			t.Errorf("frame %d: input card sits %v DIP from the rail, want 16: the cards are out of step", i, gap)
		}
	}
	if worst == 0 {
		t.Fatal("no frame had both the rail and the input card")
	}

	// Expanding is the same gesture in reverse and must stay in step too.
	a.railCollapsed = false
	for i := 0; i < 30; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		rail, okR := tt.Access("工具卡")
		input, okI := tt.Access("输入卡")
		if !okR || !okI {
			continue
		}
		if gap := input.Bounds.X - (rail.Bounds.X + rail.Bounds.W); gap < 14 || gap > 18 {
			t.Errorf("expand frame %d: input card sits %v DIP from the rail, want 16", i, gap)
		}
	}
}

// TestReducedMotionSettlesAtOnce pins that "减弱动效" really removes motion.
// A zero-valued Transition is not an off switch: MyGo reads one with no
// property flags as "animate everything", so reduced motion used to make the
// shell slower, not instant.
func TestReducedMotionSettlesAtOnce(t *testing.T) {
	a := newTestApp(t)
	a.Settings.UIReducedMotion = true
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	open, ok := tt.Access("工具卡")
	if !ok {
		t.Fatal("no rail")
	}
	if open.Bounds.W < railWidth-4 {
		t.Fatalf("open rail width = %v, want %v", open.Bounds.W, railWidth)
	}

	a.railCollapsed = true
	tt.Frame()
	collapsed, ok := tt.Access("工具卡")
	if !ok {
		t.Fatal("rail disappeared")
	}
	if collapsed.Bounds.W > railCollapsedWidth+1 {
		t.Errorf("reduced motion still animated the rail: width = %v, want %v",
			collapsed.Bounds.W, railCollapsedWidth)
	}
	if tt.FramePending() {
		t.Error("reduced motion asked for further frames")
	}
}

// TestSettingsHighlightFades pins that the settings row's highlight appears and
// disappears with the page handover. It used to be applied in one frame while
// the tool indicator faded out over 220 ms, so for a frame the rail showed two
// blue rows at once — the "sidebar refreshed" flicker.
func TestSettingsHighlightFades(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	// A point on the right edge of the settings row, away from its icon and text.
	pixel := func() (uint8, uint8, uint8) {
		info, ok := tt.Access("全局设置")
		if !ok {
			t.Fatal("no settings button")
		}
		img := tt.Image()
		c := img.RGBAAt(int(info.Bounds.X+info.Bounds.W-30),
			int(info.Bounds.Y+info.Bounds.H/2))
		return c.R, c.G, c.B
	}
	blue := func() bool {
		r, _, b := pixel()
		return b > 180 && r < 80
	}
	if blue() {
		t.Fatal("settings row is highlighted on a tool page")
	}

	settle := func() {
		for i := 0; i < 40; i++ {
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
		}
	}
	a.ShowSettings = true
	settle()
	if !blue() {
		t.Error("settings row never reached its highlighted state")
	}
	a.ShowSettings = false
	settle()
	if blue() {
		t.Error("settings row stayed highlighted after returning to a tool")
	}
}

// TestSettingsSwitchKeepsRailIdentity pins that opening the settings page does
// not rebuild the rail. The selection pill used to be built only for a tool
// page: hiding it took one child out of the list, which renumbered every row
// after it (unkeyed children are identified by their position), so all thirteen
// rows were treated as new elements and the whole sidebar refreshed.
func TestSettingsSwitchKeepsRailIdentity(t *testing.T) {
	a := newTestApp(t)
	a.Active = 4
	tt := myui.NewTester(a.View, 1440, 900)
	settle := func(n int) {
		for i := 0; i < n; i++ {
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
		}
	}
	settle(30)

	before, ok := tt.Access("选中指示")
	if !ok {
		t.Fatal("no selection pill on the tool page")
	}

	a.ShowSettings = true
	tt.Frame()
	if _, ok := tt.Access("选中指示"); !ok {
		t.Error("the selection pill was torn down when the settings page opened: the whole rail was rebuilt")
	}

	// Coming back to a different tool, the pill must travel from where it was,
	// not restart from the top row. Its animation channel only survives if the
	// element kept its identity across the settings page.
	a.ShowSettings = false
	a.Active = 1
	tt.Advance(16 * time.Millisecond)
	tt.Frame()
	mid, ok := tt.Access("选中指示")
	if !ok {
		t.Fatal("selection pill missing after returning to a tool")
	}
	if mid.Bounds.Y < before.Bounds.Y-10 {
		t.Errorf("pill restarted at y=%v instead of travelling from y=%v", mid.Bounds.Y, before.Bounds.Y)
	}
}

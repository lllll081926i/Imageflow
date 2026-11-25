package ui

// rail_key_test.go pins that the sidebar keeps its element across a page switch.
//
// The rail is a sibling of the page container in the same row. Without a key of its
// own it was matched positionally against the page, whose key changes on every
// switch, so the rail was rebuilt along with the content and its own animation
// state (the width it animates, the pill's travel) restarted every time. That is
// what read as the sidebar "refreshing" while the page changed.

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestRailKeepsItsWidthAcrossPageSwitches pins the sidebar's geometry.
//
// The rail's width is an animated value owned by the parent row. Rebuilding the
// rail drops that value, so the width restarts from wherever the new element is
// first measured and the whole column visibly jumps. Sampling the width on every
// frame of a switch catches that: a stable rail reports railWidth throughout.
//
// The selection highlight and the 全局设置 row do change, on purpose, so the check
// is geometry rather than pixels.
func TestRailKeepsItsWidthAcrossPageSwitches(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1200, 800)
	tt.Frame()
	for i := 0; i < 30; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}

	check := func(label string) {
		t.Helper()
		for i := 0; i < 30; i++ {
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
			info, ok := tt.Access("工具卡")
			if !ok {
				t.Fatalf("%s frame %d: the rail vanished", label, i)
			}
			if info.Bounds.W < railWidth-2 || info.Bounds.W > railWidth+2 {
				t.Fatalf("%s frame %d: rail width = %v, want %v; it was re-measured",
					label, i, info.Bounds.W, railWidth)
			}
			// The shell pads 16 DIP on each side; the rail keeps that x and its own
			// width, which together are what a re-measured rail would lose.
			if info.Bounds.X < 15 || info.Bounds.X > 17 {
				t.Fatalf("%s frame %d: rail x = %v, want the shell's 16 DIP margin",
					label, i, info.Bounds.X)
			}
		}
	}

	a.Active = 6
	check("tool switch")
	a.ShowSettings = true
	check("open settings")
	a.ShowSettings = false
	a.Active = 2
	check("close settings")
}

// TestPageSwitchFadesWithoutTravelling pins that the handover never animates
// layout geometry (position or size), so the row's layout stays in step with the
// rail while the directional enter transition plays.
func TestPageSwitchFadesWithoutTravelling(t *testing.T) {
	if pageSwap.Position || pageSwap.Size || pageSwap.Width || pageSwap.Height {
		t.Error("pageSwap animates geometry; a handover must not touch the layout")
	}
	if pageSwap.Exit != nil {
		t.Error("pageSwap has an Exit copy: two composited layouts flicker as their alphas cross")
	}
	if pageSwap.Enter == nil {
		t.Fatal("pageSwap has no Enter motion")
	}
	if pageSwap.Enter.Opacity <= 0 || pageSwap.Enter.Opacity >= 1 {
		t.Errorf("pageSwap Enter opacity = %v, want a partial fade in (0, 1)", pageSwap.Enter.Opacity)
	}
}

package ui

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestRailCollapseIsAnimated pins that collapsing the rail is a spring, not a
// jump: the row keeps asking for frames while the width travels, and the cards
// beside it see the intermediate widths because the number is animated in the
// layout, not painted on the rail afterwards.
func TestRailCollapseIsAnimated(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	open, ok := tt.Access("工具卡")
	if !ok {
		t.Fatal("no rail")
	}
	if open.Bounds.W < railWidth-4 || open.Bounds.W > railWidth+4 {
		t.Fatalf("open rail width = %v, want %v", open.Bounds.W, railWidth)
	}

	a.railCollapsed = true
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("collapsing asked for no further frames: the width jumped")
	}

	// Walk the spring and collect the widths it passes through.
	var seen []float32
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if info, ok := tt.Access("工具卡"); ok {
			seen = append(seen, info.Bounds.W)
		}
	}
	if len(seen) < 5 {
		t.Fatalf("only %d frames observed", len(seen))
	}
	intermediate := 0
	for _, w := range seen {
		if w > railCollapsedWidth+8 && w < railWidth-8 {
			intermediate++
		}
	}
	if intermediate < 3 {
		t.Errorf("rail jumped between %v and %v; intermediate widths seen: %v", railWidth, railCollapsedWidth, seen)
	}
	// The end state is the collapsed strip, and the labels are gone with it.
	tt.Advance(time.Second)
	tt.Frame()
	collapsed, ok := tt.Access("工具卡")
	if !ok {
		t.Fatal("rail disappeared")
	}
	if collapsed.Bounds.W > railCollapsedWidth+8 {
		t.Errorf("settled rail width = %v, want %v", collapsed.Bounds.W, railCollapsedWidth)
	}
	if !tt.HasText("展开侧栏") {
		t.Errorf("expand action missing after collapse: %v", tt.Texts())
	}
}

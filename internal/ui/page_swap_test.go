package ui

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestPageSwitchAnimates pins that moving between tools and the settings page is
// a handover, not a swap: the container is keyed by what it shows, so the new
// page enters with a motion and asks for frames while it settles.
func TestPageSwitchAnimates(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	a.Active = 3
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("switching tools asked for no further frames: the page swapped in place")
	}

	// Opening the settings is the same kind of change.
	for i := 0; i < 30; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	a.ShowSettings = true
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("opening the settings asked for no further frames")
	}
	for i := 0; i < 30; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.Advance(time.Second)
	tt.Frame()

	if !tt.HasText("全局设置") {
		t.Errorf("settings page did not settle: %v", tt.Texts())
	}
}

// TestPageSwitchSettlesToFullWidth pins that the page entering does not share the
// row with a departing copy: after the switch the cards are back at their full
// width, not squeezed by a stale element in the same row.
func TestPageSwitchSettlesToFullWidth(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	a.Active = 5
	for i := 0; i < 60; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.Advance(time.Second)
	tt.Frame()

	in, ok := tt.Access("输入卡")
	if !ok {
		t.Fatalf("input card missing after the switch: %v", tt.Texts())
	}
	// 1440 - 16 - 216 - 16 - 420 - 16 = 756.
	if in.Bounds.W < 730 {
		t.Errorf("input card width = %v after the switch, want about 740", in.Bounds.W)
	}
}

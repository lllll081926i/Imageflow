package ui

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestSegmentedPillTravels pins the switch's one job: the selection pill moves
// over several frames instead of jumping, and it lands without covering the
// labels.
//
// A jump is what MyGo's built-in Segmented does, and the reason this widget
// exists: the panel repaints the raised cell where it now is, with nothing in
// between.
func TestSegmentedPillTravels(t *testing.T) {
	selected := 0
	view := func(c *myui.Context) {
		myui.Column(c).Fill().Padding(40).Children(func() {
			segmented(c, &selected, []string{"一", "二", "三", "四"})
		})
	}
	tt := myui.NewTester(view, 600, 200)
	tt.Frame()

	selected = 3
	tt.Frame()
	// The spring keeps asking for frames while the pill is on its way; a jump
	// would leave nothing pending.
	if !tt.FramePending() {
		t.Fatal("the selection changed but no frame was requested: the pill jumped")
	}
	for i := 0; i < 30; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.Advance(time.Second)
	tt.Frame()

	// The pill is painted under the labels; covering them would hide the text.
	for _, want := range []string{"一", "二", "三", "四"} {
		if !tt.HasText(want) {
			t.Errorf("label %q missing after the switch: %v", want, tt.Texts())
		}
	}
}

// TestSegmentedReducedMotionLandsAtOnce pins the reduced-motion path: with the
// setting on, the pill is at its new place in the frame that changed it.
func TestSegmentedReducedMotionLandsAtOnce(t *testing.T) {
	old := motionReduced
	motionReduced = true
	t.Cleanup(func() { motionReduced = old })

	selected := 0
	view := func(c *myui.Context) {
		myui.Column(c).Fill().Padding(40).Children(func() {
			segmented(c, &selected, []string{"甲", "乙"})
		})
	}
	tt := myui.NewTester(view, 400, 160)
	tt.Frame()
	selected = 1
	tt.Frame()
	if tt.FramePending() {
		t.Error("reduced motion must not keep animating the pill")
	}
	if !tt.HasText("甲") || !tt.HasText("乙") {
		t.Errorf("labels missing: %v", tt.Texts())
	}
}

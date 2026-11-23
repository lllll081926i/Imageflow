package ui

// press_motion_test.go pins the press feedback as a pose.
//
// docs/ui/motion.md describes AnimatePose as the cheapest motion there is: it
// moves the element in frames that only paint, without running the view. A
// static Scale transform, which the shell used to set in the press branch,
// snaps between two sizes in a single frame. The tests below check both halves:
// the press asks for frames while it settles, and the release comes back.

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestPressPoseAnimates pins that a ghost button's press is a movement, not a
// jump: pressing it asks for further frames, and releasing it settles back.
func TestPressPoseAnimates(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	info, ok := tt.Access("选择输出位置")
	if !ok {
		// The landing page is off, so the header's output button carries the label.
		t.Fatalf("no output button: %v", tt.Texts())
	}
	x := info.Bounds.X + info.Bounds.W/2
	y := info.Bounds.Y + info.Bounds.H/2

	tt.Press(x, y)
	if !tt.FramePending() {
		t.Fatal("pressing the button asked for no frames: it jumped to its pressed size")
	}
	for i := 0; i < 20; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.Release(x, y)
	if !tt.FramePending() {
		t.Fatal("releasing the button asked for no frames: the pose never came back")
	}
	for i := 0; i < 20; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	if tt.FramePending() {
		t.Error("the pose is still moving after a full settle")
	}
}

// TestPressPoseIsAbsentWithReducedMotion pins that 减弱动效 stops the press
// feedback too: the button is built neutral and asks for no frames.
func TestPressPoseIsAbsentWithReducedMotion(t *testing.T) {
	a := newTestApp(t)
	a.Settings.UIReducedMotion = true
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	info, ok := tt.Access("选择输出位置")
	if !ok {
		t.Fatalf("no output button: %v", tt.Texts())
	}
	x := info.Bounds.X + info.Bounds.W/2
	y := info.Bounds.Y + info.Bounds.H/2

	tt.Press(x, y)
	tt.Frame()
	if tt.FramePending() {
		t.Error("reduced motion still posed the button")
	}
	tt.Release(x, y)
	tt.Frame()
	if tt.FramePending() {
		t.Error("reduced motion asked for frames after the release")
	}
}

package ui

// rail_drag_test.go pins the sidebar's long-press reorder: a hold lifts a row,
// the rows it passes over glide out of the way, and the drop writes the new order.

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// railRowTopY is a row's resting top for a position, in window DIPs.
func railRowTopY(pos int) float32 {
	// The rail's own top padding is 10, and each row is railRowHeight tall with
	// railRowGap between them.
	return float32(10 + pos*(railRowHeight+railRowGap))
}

// rowCentreY reads a row's real centre from the rendered frame, so a test does not
// duplicate the rail's padding arithmetic.
func rowCentreY(t *testing.T, tt *myui.Tester, name string) float32 {
	t.Helper()
	info, ok := tt.Access(name)
	if !ok {
		t.Fatalf("%q missing: %v", name, tt.Texts())
	}
	return info.Bounds.Y + info.Bounds.H/2
}

// holdRow presses a row and waits out the long press without moving.
func holdRow(t *testing.T, tt *myui.Tester, a *App, x, y float32) {
	t.Helper()
	tt.Press(x, y)
	tt.Frame()
	// The lift is noticed on the frame the press scheduled, so the clock has to
	// pass railLongPress before it can fire.
	for i := 0; i < 40 && a.railDragTool < 0; i++ {
		tt.Advance(40 * time.Millisecond)
		tt.Frame()
	}
}

// TestRailLongPressLiftsTheRow pins the first half of the gesture: a hold lifts
// the row, and a short click never does.
func TestRailLongPressLiftsTheRow(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	info, ok := tt.Access("批量水印")
	if !ok {
		t.Fatalf("row missing: %v", tt.Texts())
	}
	x := info.Bounds.X + 60
	y := rowCentreY(t, tt, "批量水印")

	// A quick click must not lift anything.
	tt.Press(x, y)
	tt.Frame()
	tt.Release(x, y)
	tt.Frame()
	if a.railDragTool >= 0 {
		t.Fatal("a click lifted the row")
	}
	if a.ActiveTool().Name() != "批量水印" {
		t.Fatalf("a click did not select the tool: active = %q", a.ActiveTool().Name())
	}

	// A hold lifts it.
	holdRow(t, tt, a, x, rowCentreY(t, tt, "批量水印"))
	if a.railDragTool < 0 {
		t.Fatal("the long press never lifted the row")
	}
	if got := a.railOrderSnapshot()[a.railDragFrom]; a.Tools[got].Name() != "批量水印" {
		t.Fatalf("long press lifted %q, want 批量水印", a.Tools[got].Name())
	}
	if !tt.FramePending() {
		t.Error("the lift asked for no further frames")
	}
	tt.Release(x, rowCentreY(t, tt, "批量水印"))
	tt.Frame()
}

// TestRailDragReordersAndPersists pins the whole gesture: holding a row and
// dragging it over the first moves it there, animates the rows in between, and
// writes the new order to the settings file.
func TestRailDragReordersAndPersists(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	tt.Frame()

	first := a.Tools[0].Name()
	info, _ := tt.Access("批量水印")
	x := info.Bounds.X + 60
	holdRow(t, tt, a, x, rowCentreY(t, tt, "批量水印"))
	if a.railDragTool < 0 {
		t.Fatal("the long press never lifted the row")
	}

	// Drag up past the first row.
	tt.Move(x, rowCentreY(t, tt, first))
	tt.Frame()
	if a.railDragTo != 0 {
		t.Fatalf("drag target = %d, want 0", a.railDragTo)
	}

	// The rows between glide: at least one frame has the first row below its
	// resting place, which is the gap the carried row would fill.
	glided := false
	for i := 0; i < 12; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if row, ok := tt.Access(first); ok {
			if row.Bounds.Y-railRowTopY(0) > 4 {
				glided = true
			}
		}
	}
	if !glided {
		t.Error("the displaced row never moved: the neighbours did not glide")
	}

	// The drop point is where the carried row was last moved to; releasing
	// anywhere else would also move the pointer first, which no real hand does
	// without the engine reporting the move.
	drop, ok := tt.Access("批量水印")
	if !ok {
		t.Fatal("the carried row vanished mid-drag")
	}
	lastX, lastY := drop.Bounds.X+drop.Bounds.W/3, drop.Bounds.Y+drop.Bounds.H/2
	tt.Move(lastX, lastY)
	tt.Frame()
	tt.Release(lastX, lastY)
	tt.Frame()

	order := a.railOrderSnapshot()
	if a.Tools[order[0]].Name() != "批量水印" {
		t.Fatalf("order = %v, want 批量水印 first", order)
	}
	// A reorder is not navigation: the page that was open stays open.
	if a.ActiveTool().Name() != first {
		t.Errorf("dropping the row switched the page to %q, want %q", a.ActiveTool().Name(), first)
	}

	// Animated settle: the rows land on their new resting places.
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	for pos, toolIndex := range a.railOrderSnapshot() {
		name := a.Tools[toolIndex].Name()
		row, ok := tt.Access(name)
		if !ok {
			t.Fatalf("%q vanished after the reorder", name)
		}
		want := railRowTopY(pos)
		if diff := row.Bounds.Y - want; diff > 2 || diff < -2 {
			t.Errorf("%q sits at y=%v, want %v (position %d)", name, row.Bounds.Y, want, pos)
		}
	}

	// Persisted.
	loaded := settings.Load()
	if len(loaded.RailOrder) != len(a.Tools) || a.Tools[loaded.RailOrder[0]].Name() != "批量水印" {
		t.Errorf("persisted order = %v, want 批量水印 first", loaded.RailOrder)
	}
}

// TestRailOrderSurvivesRestart pins the other half of persistence: a new App
// reads the stored permutation and draws the rail in it. The selected tool is a
// separate fact from the row order, so it stays where it was.
func TestRailOrderSurvivesRestart(t *testing.T) {
	a := newTestApp(t)
	declared := append([]int{3}, 0, 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13)
	if len(declared) != len(a.Tools) {
		t.Fatalf("the fixture declares %d tools, want %d", len(declared), len(a.Tools))
	}
	a.railOrder = declared
	a.persistRailOrder()

	restored := NewApp(settings.Load())
	order := restored.railOrderSnapshot()
	if len(order) != len(restored.Tools) || order[0] != 3 {
		t.Fatalf("restored order = %v, want 3 first", order)
	}
	// The page a fresh app shows is still the first tool, not the first row.
	if restored.ActiveTool().Name() != restored.Tools[0].Name() {
		t.Errorf("the restored order changed the selected tool to %q", restored.ActiveTool().Name())
	}
}

// TestRailOrderRepairsAStaleFile pins the guard: a stored permutation that no
// longer matches the tool list falls back to the natural order instead of
// pointing a row at a tool that does not exist.
func TestRailOrderRepairsAStaleFile(t *testing.T) {
	a := newTestApp(t)
	natural := make([]int, len(a.Tools))
	for i := range natural {
		natural[i] = i
	}
	duplicate := append([]int(nil), natural...)
	duplicate[len(duplicate)-1] = 0
	outOfRange := append([]int(nil), natural...)
	outOfRange[0] = 99
	for _, stale := range [][]int{
		{0, 1, 2},  // too short
		duplicate,  // a duplicate
		outOfRange, // out of range
	} {
		a.railOrder = append([]int(nil), stale...)
		a.ensureRailOrder()
		if len(a.railOrder) != len(a.Tools) {
			t.Fatalf("%v was not repaired: %v", stale, a.railOrder)
		}
		for i, idx := range a.railOrder {
			if idx != i {
				t.Fatalf("%v was not repaired to the natural order: %v", stale, a.railOrder)
			}
		}
	}
}

// TestRailReorderKeepsThePlateAxis pins that a reorder does not break the column's
// alignment: every row still has its plate on the same x after a drag.
func TestRailReorderKeepsThePlateAxis(t *testing.T) {
	a := newTestApp(t)
	a.ShowSettings = true // every row inactive, so the plate is the plain tint
	a.railOrder = []int{5, 4, 3, 2, 1, 0, 6, 7, 8, 9, 10, 11, 12, 13}
	tt := myui.NewTester(a.View, 1440, 900)
	for i := 0; i < 20; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	xs := map[float32]bool{}
	for _, tool := range a.Tools {
		info, ok := tt.Access(tool.Name())
		if !ok {
			t.Fatalf("%q missing", tool.Name())
		}
		xs[info.Bounds.X] = true
	}
	if len(xs) != 1 {
		t.Errorf("rows start at %v, want one shared x", xs)
	}
}

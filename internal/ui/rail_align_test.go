package ui

import (
	"image/color"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestRailRowsAlignOnOneAxis pins the sidebar's shared left axis.
//
// ButtonBase centers its children by default, and a rail row that leaves that
// default in place aligns by its own label width: every icon landed at a
// different x and the whole column read as misaligned. The plates are drawn,
// not exposed to the accessibility tree, so the check is pixel-based: on each
// inactive row's centre line (the active row is a gradient that fills the row
// and hides its plate), the coloured run at the rail's leading edge must sit
// at the same centre x on every row, open or collapsed.
func TestRailRowsAlignOnOneAxis(t *testing.T) {
	a := newTestApp(t)
	// The settings page showing leaves every tool row inactive: its plate is
	// the plain tint the scan expects, where the active row's gradient is not.
	a.ShowSettings = true
	tt := myui.NewTester(a.View, 1440, 900)
	for i := 0; i < 20; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}

	// The scan window stops before the labels begin (plate at 36, label at 79),
	// so the coloured run it finds is the plate or the footer icon alone.
	const scanFrom, scanTo = 30, 74
	centre := func(name string) int {
		t.Helper()
		info, ok := tt.Access(name)
		if !ok {
			t.Fatalf("%q has no accessible name", name)
		}
		img := tt.Image()
		if img == nil {
			t.Fatal("no frame")
		}
		y := int(info.Bounds.Y + info.Bounds.H/2)
		bg := img.RGBAAt(200, y) // past the labels: the row's own background
		first, last := -1, -1
		for x := scanFrom; x < scanTo; x++ {
			if diffRGBA(img.RGBAAt(x, y), bg) > 10 {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first < 0 {
			t.Fatalf("%q: no plate found on the row's centre line", name)
		}
		return first + last // 2x the centre; rows compare equal either way
	}

	// Every inactive row: the tools, and the collapse action (never active).
	rows := make([]string, 0, len(a.Tools)+1)
	for _, tool := range a.Tools {
		rows = append(rows, tool.Name())
	}
	rows = append(rows, "收起侧栏")
	want := centre(rows[0])
	for _, name := range rows[1:] {
		if got := centre(name); got > want+4 || got < want-4 {
			t.Errorf("%q plate centre = %v, want %v (row misaligned)", name, got/2.0, want/2.0)
		}
	}

	// Collapsed: the labels are gone and the plates centre in the strip, one
	// centre for every row. The collapse action renames itself on the way.
	a.railCollapsed = true
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	rows[len(rows)-1] = "展开侧栏"
	want = centre(rows[0])
	for _, name := range rows[1:] {
		if got := centre(name); got > want+4 || got < want-4 {
			t.Errorf("collapsed %q plate centre = %v, want %v", name, got/2.0, want/2.0)
		}
	}
}

// diffRGBA is the largest channel difference of two pixels.
func diffRGBA(a, b color.RGBA) int {
	d := abs8(a.R, b.R)
	if e := abs8(a.G, b.G); e > d {
		d = e
	}
	if e := abs8(a.B, b.B); e > d {
		d = e
	}
	return d
}

func abs8(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

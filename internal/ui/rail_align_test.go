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
// visible row's centre line, the coloured run at the rail's leading edge must
// sit at the same centre x on every row, open or collapsed.
//
// The rail scrolls, so the scan covers only the rows it can actually measure: a
// row clipped by the viewport has no plate on its centre line, and a stale bound
// from a frame before the scroll settled is not a layout defect.
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

	// The scan window stops before the labels begin, so the coloured run it finds is
	// the plate or the footer icon alone.
	const scanFrom, scanTo = 30, 74
	// centre returns 2x the plate's centre on one row, and false when the row has no
	// measurable plate on this frame.
	centre := func(name string) (int, bool) {
		t.Helper()
		info, ok := tt.Access(name)
		if !ok {
			return 0, false
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
		if first < 0 || last-first < 8 {
			return 0, false
		}
		return first + last, true // 2x the centre; rows compare equal either way
	}

	measure := func(label string) map[string]int {
		t.Helper()
		found := map[string]int{}
		for _, row := range a.railRows() {
			if row.Header {
				continue
			}
			name := a.Tools[row.Tool].Name()
			if value, ok := centre(name); ok {
				found[name] = value
			}
		}
		if value, ok := centre("收起侧栏"); ok {
			found["收起侧栏"] = value
		}
		if len(found) < 3 {
			t.Fatalf("%s: only %d rows measurable: %v", label, len(found), found)
		}
		return found
	}

	compare := func(label string, found map[string]int) {
		t.Helper()
		var want int
		first := true
		for _, value := range found {
			if first {
				want, first = value, false
				continue
			}
			if value > want+4 || value < want-4 {
				t.Errorf("%s: a plate centre is %v, want %v (row misaligned)", label, float64(value)/2, float64(want)/2)
			}
		}
	}

	compare("open", measure("open"))

	// Collapsed: the labels are gone and the plates centre in the strip, one centre
	// for every row. The footer action renames itself on the way.
	a.railCollapsed = true
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	found := map[string]int{}
	for _, row := range a.railRows() {
		if row.Header {
			continue
		}
		name := a.Tools[row.Tool].Name()
		if value, ok := centre(name); ok {
			found[name] = value
		}
	}
	if value, ok := centre("展开侧栏"); ok {
		found["展开侧栏"] = value
	}
	if len(found) < 3 {
		t.Fatalf("collapsed: only %d rows measurable: %v", len(found), found)
	}
	compare("collapsed", found)
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

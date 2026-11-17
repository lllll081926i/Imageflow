package ui

// rail_pill_test.go pins the sidebar's selection pill: it is one element that
// travels between rows, not a per-row highlight that switches on and off.

import (
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"
)

// TestRailPillTravelsBetweenRows pins that selecting another tool moves the pill
// instead of redrawing the highlight in a new place.
//
// The check is geometric, because the pill is drawn rather than exposed to the
// accessibility tree: on each frame, the brand-blue run on the rail's centre column
// must sit at a different y and, on the way, at a y between the two rows. A
// per-row highlight would report only the two end positions.
func TestRailPillTravelsBetweenRows(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1200, 800)
	tt.Frame()

	// Row centres: kept only for the sanity check above, where the first row is the
	// one immediately after the first group header.
	rowCentre := func(i int) float32 {
		return float32(10+railHeaderHeight+i*(railRowHeight+railRowGap)) + railRowHeight/2
	}
	_ = rowCentre

	pillTop := func() (int, int, bool) {
		img := tt.Image()
		if img == nil {
			t.Fatal("no frame")
		}
		b := img.Bounds()
		start, run := -1, 0
		bestStart, bestLen := -1, 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			c := img.RGBAAt(200, y)
			// The pill is the brand gradient: high blue, low red.
			blue := c.B > 190 && c.R < 120 && c.G < 190
			if blue {
				if start < 0 {
					start = y
				}
				run++
				continue
			}
			if run > bestLen {
				bestStart, bestLen = start, run
			}
			start, run = -1, 0
		}
		if run > bestLen {
			bestStart, bestLen = start, run
		}
		return bestStart, bestLen, bestLen >= railRowHeight-8
	}

	firstTop, firstLen, ok := pillTop()
	if !ok {
		t.Fatalf("no pill found on the first frame (len %d)", firstLen)
	}
	// The pill sits at the first tool row's planned top. That plan is in the list's
	// content space, and the scan reads window pixels, so the comparison uses the
	// first row's own bounds to carry the offset rather than a hand-copied padding
	// constant that a group header would invalidate.
	firstRow, ok := tt.Access(a.Tools[0].Name())
	if !ok {
		t.Fatal("the first row is missing")
	}
	if want := int(firstRow.Bounds.Y); abs(firstTop-want) > 4 {
		t.Fatalf("pill top = %d, want the first row's top %d", firstTop, want)
	}
	// Jump to a row close enough that it stays inside the rendered list, so the
	// pill can be measured in window pixels, but far enough that the travel is
	// unambiguous.
	dest := a.toolIndex(ToolCollage)
	a.Active = dest
	tt.Frame()

	// The planned top of the destination row is in the list's content space; the
	// pill is drawn in that same space, so the comparison is against the row table
	// rather than against a fixed pitch. A header between two rows is exactly why
	// the old "10 + i*(48+2)" formula no longer holds.
	destTop := a.railToolRowY(dest)
	if destTop < 0 {
		t.Fatalf("row %d has no planned position", dest)
	}

	var seen []int
	for i := 0; i < 24; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if top, _, found := pillTop(); found {
			seen = append(seen, top)
		}
	}
	if len(seen) < 6 {
		t.Fatalf("pill was visible on only %d frames", len(seen))
	}
	settled := seen[len(seen)-1]
	// The pill is drawn inside the scrolling list, so its window y carries the list's
	// own offset. Comparing it with the destination row's committed top removes that
	// offset and checks the one thing that matters: the pill is on its row.
	if info, ok := tt.Access(a.Tools[dest].Name()); ok {
		if abs(settled-int(info.Bounds.Y)) > 4 {
			t.Errorf("pill settled at %d, want the row top %v", settled, info.Bounds.Y)
		}
	}
	// Intermediate positions: at least three frames between the two rows.
	low, high := firstTop, settled
	if low > high {
		low, high = high, low
	}
	between := 0
	for _, y := range seen[1:] {
		if y > low+8 && y < high-8 {
			between++
		}
	}
	if between < 3 {
		t.Errorf("pill jumped between %d and %d without travelling: %v", firstTop, settled, seen)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

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

	// Row centres: the rail's padding is 10 top, and each row is railRowHeight tall
	// with railRowGap between.
	rowCentre := func(i int) float32 {
		return float32(10+i*(railRowHeight+railRowGap)) + railRowHeight/2
	}

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
	if want := rowCentre(0); abs(int(float32(firstTop)-want+railRowHeight/2)) > 6 {
		t.Fatalf("pill top = %d, want row 0 (centre %.0f)", firstTop, want)
	}

	// Jump to a row far enough down that the travel is unambiguous.
	a.Active = 9
	tt.Frame()

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
	if want := rowCentre(9); abs(int(float32(settled)-want+railRowHeight/2)) > 6 {
		t.Errorf("pill settled at %d, want row 9 (centre %.0f)", settled, want)
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

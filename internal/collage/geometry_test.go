package collage

import "testing"

func TestCanvasSizeFollowsTheLongEdge(t *testing.T) {
	w, h := CanvasSize(16, 9, 1600)
	if w != 1600 || h != 900 {
		t.Fatalf("16:9 at 1600 = %dx%d, want 1600x900", w, h)
	}
	w, h = CanvasSize(9, 16, 1600)
	if h != 1600 || w != 900 {
		t.Fatalf("9:16 at 1600 = %dx%d, want 900x1600", w, h)
	}
	// A tiny long edge is raised to the floor instead of producing a 1 pixel
	// canvas that the encoder would reject.
	w, h = CanvasSize(1, 1, 1)
	if w < 64 || h < 64 {
		t.Fatalf("floor = %dx%d, want at least 64", w, h)
	}
}

// TestLayoutKeepsTheGapBetweenCellsOnly asserts the two rules a collage margin
// has to obey: the gap separates neighbours, and it never eats the canvas
// margin of a cell that touches the edge.
func TestLayoutKeepsTheGapBetweenCellsOnly(t *testing.T) {
	cells := []Rect{{0, 0, 0.5, 1}, {0.5, 0, 0.5, 1}}
	px := Layout(cells, 1000, 500, Uniform(20), 12)
	if len(px) != 2 {
		t.Fatalf("cells = %d", len(px))
	}
	if px[0].X != 20 {
		t.Errorf("left cell x = %d, want the 20 px margin", px[0].X)
	}
	if px[0].Y != 20 || px[0].Bottom() != 480 {
		t.Errorf("left cell y = %d..%d, want 20..480", px[0].Y, px[0].Bottom())
	}
	if px[1].Right() != 980 {
		t.Errorf("right cell right edge = %d, want 980", px[1].Right())
	}
	// The gap between the two cells is 12 px, centred on the split.
	if gap := px[1].X - px[0].Right(); gap != 12 {
		t.Errorf("gap = %d, want 12", gap)
	}
	if px[0].W+px[1].W+12+40 != 1000 {
		t.Errorf("widths do not add up: %d + %d + 12 + 40", px[0].W, px[1].W)
	}
}

func TestLayoutHitTesting(t *testing.T) {
	px := Layout([]Rect{{0, 0, 0.5, 0.5}, {0.5, 0, 0.5, 0.5}, {0, 0.5, 1, 0.5}},
		400, 400, Uniform(0), 0)
	if got := Hit(px, 50, 50); got != 0 {
		t.Errorf("top-left hit = %d, want 0", got)
	}
	if got := Hit(px, 350, 50); got != 1 {
		t.Errorf("top-right hit = %d, want 1", got)
	}
	if got := Hit(px, 200, 350); got != 2 {
		t.Errorf("bottom hit = %d, want 2", got)
	}
	// Margins and gaps belong to no cell.
	if got := Hit(px, -1, 10); got != -1 {
		t.Errorf("outside hit = %d, want -1", got)
	}
	withGap := Layout([]Rect{{0, 0, 0.5, 1}, {0.5, 0, 0.5, 1}}, 400, 400, Uniform(0), 20)
	if got := Hit(withGap, 200, 200); got != -1 {
		t.Errorf("gap hit = %d, want -1", got)
	}
}

// TestMarginsShrinkTheContentArea covers a canvas whose margins exceed its size:
// the content area must stay positive so the renderer never allocates a
// negative rectangle.
func TestMarginsShrinkTheContentArea(t *testing.T) {
	x, y, w, h := Uniform(500).Content(100, 100)
	if w != 1 || h != 1 {
		t.Fatalf("content = %dx%d at %d,%d, want 1x1", w, h, x, y)
	}
}

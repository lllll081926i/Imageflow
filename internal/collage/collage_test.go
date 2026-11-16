package collage

import "testing"

// TestPresetTableIsComplete pins the property the panel relies on: every count
// from 1 to MaxPresetImages has at least one template, and every template has
// exactly as many cells as its identifier claims.
func TestPresetTableIsComplete(t *testing.T) {
	for n := MinPresetImages; n <= MaxPresetImages; n++ {
		list := PresetsFor(n)
		if len(list) == 0 {
			t.Errorf("%d pictures: no template", n)
			continue
		}
		for _, p := range list {
			if p.Count() != n {
				t.Errorf("%s: %d cells, want %d", p.ID, p.Count(), n)
			}
		}
	}
}

// TestPresetCellsFillTheCanvas asserts the cell areas add up to one and stay
// inside the canvas: a template with a hole would render a band of background
// nobody asked for.
func TestPresetCellsFillTheCanvas(t *testing.T) {
	for _, p := range Presets {
		sum := 0.0
		for i, cell := range p.Cells {
			if cell.X < -1e-9 || cell.Y < -1e-9 ||
				cell.Right() > 1+1e-9 || cell.Bottom() > 1+1e-9 {
				t.Errorf("%s cell %d escapes the canvas: %+v", p.ID, i, cell)
			}
			if cell.W <= 0 || cell.H <= 0 {
				t.Errorf("%s cell %d is empty: %+v", p.ID, i, cell)
			}
			sum += cell.Area()
		}
		if diff := sum - 1; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("%s covers %.4f of the canvas, want 1", p.ID, sum)
		}
	}
}

// TestPresetIDsAreUnique guards the panel's stored selection: a duplicate id
// would restore the wrong template.
func TestPresetIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Presets {
		if seen[p.ID] {
			t.Errorf("duplicate preset id %q", p.ID)
		}
		seen[p.ID] = true
		if p.Name == "" {
			t.Errorf("%s has no label", p.ID)
		}
	}
}

func TestMergeGridMergesABoundingBox(t *testing.T) {
	g := NewMergeGrid(3, 3)
	if len(g.Blocks()) != 9 {
		t.Fatalf("fresh 3x3 has %d blocks", len(g.Blocks()))
	}
	// Merge the top two columns of the top row into one wide block.
	if !g.Merge(0, 0, 1, 0) {
		t.Fatal("merge reported no change")
	}
	blocks := g.Blocks()
	if len(blocks) != 8 {
		t.Fatalf("after merge: %d blocks, want 8", len(blocks))
	}
	top := blocks[0]
	if top.X != 0 || top.Y != 0 || top.W < 0.66 || top.W > 0.67 || top.H < 0.33 || top.H > 0.34 {
		t.Fatalf("merged block = %+v", top)
	}
	// Every block must still be a rectangle: the grid must split back into
	// single cells of the same count.
	if !g.SplitAt(1, 0) {
		t.Fatal("split reported no change")
	}
	if len(g.Blocks()) != 9 {
		t.Fatalf("after split: %d blocks, want 9", len(g.Blocks()))
	}
}

// TestMergeGridGrowsPastACrossingBlock covers the case that breaks naive merge
// editors: merging two blocks has to grow the selection until no block straddles
// its edge, which is what keeps every block a rectangle.
func TestMergeGridGrowsPastACrossingBlock(t *testing.T) {
	g := NewMergeGrid(3, 3)
	// A tall block in the middle column, rows 0..1.
	if !g.Merge(1, 0, 1, 1) {
		t.Fatal("tall merge failed")
	}
	// A wide selection across row 0 cuts through it, so the merge must grow
	// to the bounding box: the whole top two rows become one block.
	if !g.Merge(0, 0, 2, 0) {
		t.Fatal("wide merge failed")
	}
	blocks := g.Blocks()
	if len(blocks) != 4 {
		t.Fatalf("blocks = %d (%+v), want 4", len(blocks), blocks)
	}
	if b := blocks[0]; b.W < 0.99 || b.H < 0.66 {
		t.Fatalf("grown block = %+v, want the top two rows", b)
	}
	for i, b := range blocks {
		colsN := float64(g.Cols) * b.W
		rowsN := float64(g.Rows) * b.H
		if absFloat(colsN-float64(int(colsN+0.5))) > 1e-6 || absFloat(rowsN-float64(int(rowsN+0.5))) > 1e-6 {
			t.Errorf("block %d is not a whole number of cells: %+v", i, b)
		}
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestMergeGridSizeChangeResets(t *testing.T) {
	g := NewMergeGrid(2, 2)
	g.Merge(0, 0, 1, 0)
	g.SetSize(4, 3)
	if len(g.Blocks()) != 12 {
		t.Fatalf("resized grid has %d blocks, want 12", len(g.Blocks()))
	}
	// Clamping keeps the grid inside what the editor offers.
	g.SetSize(99, 0)
	if g.Cols != MaxCustomCols || g.Rows != MinCustomRows {
		t.Fatalf("clamped size = %dx%d", g.Cols, g.Rows)
	}
}

func TestMirrorKeepsOrderAndFlipsX(t *testing.T) {
	in := []Rect{{0, 0, 0.5, 0.5}, {0.5, 0, 0.5, 1}}
	out := Mirror(in)
	if out[0].X != 0.5 || out[1].X != 0 {
		t.Fatalf("mirrored = %+v", out)
	}
	if out[0].Y != in[0].Y || out[0].W != in[0].W {
		t.Fatalf("mirror changed more than x: %+v", out[0])
	}
}

func TestMergeGridMirrorKeepsRectangles(t *testing.T) {
	g := NewMergeGrid(3, 2)
	g.Merge(0, 0, 1, 0)
	before := g.Blocks()
	g.Mirror()
	after := g.Blocks()
	if len(before) != len(after) {
		t.Fatalf("mirror changed the block count: %d → %d", len(before), len(after))
	}
	// The wide block moved from the left two columns to the right two.
	var wide Rect
	found := false
	for _, b := range after {
		if b.W > 0.6 {
			wide, found = b, true
		}
	}
	if !found || wide.X < 0.3 {
		t.Fatalf("wide block = %+v in %+v, want it on the right", wide, after)
	}
	for i, b := range after {
		cols := float64(g.Cols) * b.W
		if absFloat(cols-float64(int(cols+0.5))) > 1e-6 {
			t.Errorf("block %d is no longer a whole number of cells: %+v", i, b)
		}
	}
}

func TestParseAspectAndID(t *testing.T) {
	if a, ok := ParseAspect("16:9"); !ok || a.W != 16 || a.H != 9 {
		t.Fatalf("ParseAspect(16:9) = %+v %v", a, ok)
	}
	if a, ok := ParseAspect("3x4"); !ok || a.W != 3 || a.H != 4 {
		t.Fatalf("ParseAspect(3x4) = %+v %v", a, ok)
	}
	for _, bad := range []string{"", "16", "0:9", "-1:2", "a:b"} {
		if _, ok := ParseAspect(bad); ok {
			t.Errorf("ParseAspect(%q) accepted", bad)
		}
	}
	if n, ok := ParseID("4-grid"); !ok || n != 4 {
		t.Fatalf("ParseID = %d %v", n, ok)
	}
	if _, ok := ParseID("grid"); ok {
		t.Error("ParseID accepted a template without a count")
	}
	if a, ok := AspectByID("a4"); !ok || a.W != 210 {
		t.Fatalf("AspectByID(a4) = %+v %v", a, ok)
	}
}

package collage

// mergegrid.go is the custom layout editor's model: a cols × rows grid of cells
// that can be merged into rectangles, which is how a free-form collage template
// is drawn without asking the user for coordinates.
//
// Every block is a rectangle. Merging keeps that invariant by growing the
// selection to the bounding box of the touched blocks, so a layout can never
// contain an L shape that the renderer would then have to guess about.

import (
	"fmt"
)

// MaxCustomCols and MaxCustomRows bound the editor grid.
const (
	MaxCustomCols = 8
	MaxCustomRows = 8
	MinCustomCols = 1
	MinCustomRows = 1
)

// MergeGrid is a rectangular grid whose cells belong to blocks.
type MergeGrid struct {
	Cols int
	Rows int
	// Parent has one entry per cell, row-major, holding the cell's block id.
	// Block ids are the index of the cell that started them.
	Parent []int
	// next is the next block id to hand out.
	next int
}

// NewMergeGrid returns a grid of single-cell blocks.
func NewMergeGrid(cols, rows int) *MergeGrid {
	cols = clamp(cols, MinCustomCols, MaxCustomCols)
	rows = clamp(rows, MinCustomRows, MaxCustomRows)
	g := &MergeGrid{Cols: cols, Rows: rows}
	g.reset()
	return g
}

func (g *MergeGrid) reset() {
	g.Parent = make([]int, g.Cols*g.Rows)
	for i := range g.Parent {
		g.Parent[i] = i
	}
	g.next = g.Cols * g.Rows
}

// SetSize rebuilds the grid at a new size, dropping every merge.
func (g *MergeGrid) SetSize(cols, rows int) {
	cols = clamp(cols, MinCustomCols, MaxCustomCols)
	rows = clamp(rows, MinCustomRows, MaxCustomRows)
	if cols == g.Cols && rows == g.Rows && len(g.Parent) == cols*rows {
		return
	}
	g.Cols, g.Rows = cols, rows
	g.reset()
}

// Reset splits every block back into single cells, keeping the size.
func (g *MergeGrid) Reset() { g.reset() }

func (g *MergeGrid) index(col, row int) int {
	if col < 0 || col >= g.Cols || row < 0 || row >= g.Rows {
		return -1
	}
	return row*g.Cols + col
}

// BlockAt returns the block id at a cell, or -1 outside the grid.
func (g *MergeGrid) BlockAt(col, row int) int {
	i := g.index(col, row)
	if i < 0 {
		return -1
	}
	return g.Parent[i]
}

// CellRect is one grid cell as a fraction of the canvas.
func (g *MergeGrid) CellRect(col, row int) Rect {
	w := 1 / float64(g.Cols)
	h := 1 / float64(g.Rows)
	return Rect{X: float64(col) * w, Y: float64(row) * h, W: w, H: h}
}

// BlockRect is the block's rectangle, or the zero Rect for an unknown id.
func (g *MergeGrid) BlockRect(id int) (Rect, bool) {
	minCol, minRow := -1, -1
	maxCol, maxRow := -1, -1
	for i, p := range g.Parent {
		if p != id {
			continue
		}
		col, row := i%g.Cols, i/g.Cols
		if minCol < 0 || col < minCol {
			minCol = col
		}
		if maxCol < 0 || col > maxCol {
			maxCol = col
		}
		if minRow < 0 || row < minRow {
			minRow = row
		}
		if maxRow < 0 || row > maxRow {
			maxRow = row
		}
	}
	if minCol < 0 {
		return Rect{}, false
	}
	w := 1 / float64(g.Cols)
	h := 1 / float64(g.Rows)
	return Rect{
		X: float64(minCol) * w,
		Y: float64(minRow) * h,
		W: float64(maxCol-minCol+1) * w,
		H: float64(maxRow-minRow+1) * h,
	}, true
}

// Merge joins the blocks of the two cells, growing the selection to the
// bounding box of everything it touches, and reports whether anything changed.
//
// Growing is what keeps every block a rectangle: a block that the bounding box
// cuts through is pulled in whole, and the box widens again for what the pull
// reaches, until it is stable.
func (g *MergeGrid) Merge(colA, rowA, colB, rowB int) bool {
	a, b := g.index(colA, rowA), g.index(colB, rowB)
	if a < 0 || b < 0 {
		return false
	}
	if g.Parent[a] == g.Parent[b] {
		return false
	}
	x0, y0 := min(colA, colB), min(rowA, rowB)
	x1, y1 := max(colA, colB), max(rowA, rowB)
	for grown := true; grown; {
		grown = false
		for row := y0; row <= y1; row++ {
			for col := x0; col <= x1; col++ {
				id := g.Parent[g.index(col, row)]
				r, ok := g.BlockRect(id)
				if !ok {
					continue
				}
				// The block's cells in grid units.
				bc0 := intRound(r.X * float64(g.Cols))
				br0 := intRound(r.Y * float64(g.Rows))
				bc1 := bc0 + intRound(r.W*float64(g.Cols)) - 1
				br1 := br0 + intRound(r.H*float64(g.Rows)) - 1
				if bc0 < x0 || bc1 > x1 || br0 < y0 || br1 > y1 {
					x0, y0 = min(x0, bc0), min(y0, br0)
					x1, y1 = max(x1, bc1), max(y1, br1)
					grown = true
				}
			}
		}
	}
	// The merged block keeps the id of its top-left cell, so the ids stay
	// explainable ("the block that starts here").
	id := g.Parent[g.index(x0, y0)]
	for row := y0; row <= y1; row++ {
		for col := x0; col <= x1; col++ {
			g.Parent[g.index(col, row)] = id
		}
	}
	return true
}

// Split turns a merged block back into single cells.
func (g *MergeGrid) Split(id int) bool {
	if r, ok := g.BlockRect(id); !ok {
		return false
	} else if intRound(r.W*float64(g.Cols)) == 1 && intRound(r.H*float64(g.Rows)) == 1 {
		return false
	}
	for i, p := range g.Parent {
		if p == id {
			g.Parent[i] = i
		}
	}
	return true
}

// SplitAt splits the block that owns a cell.
func (g *MergeGrid) SplitAt(col, row int) bool {
	id := g.BlockAt(col, row)
	if id < 0 {
		return false
	}
	return g.Split(id)
}

// Mirror flips the grid left to right: the block that was in column 0 ends up in
// the last column. It is the one-click variation of a hand-drawn layout the panel
// offers, and it works on the grid rather than on the block list so every block
// stays a rectangle.
func (g *MergeGrid) Mirror() {
	if g.Cols < 2 {
		return
	}
	for row := 0; row < g.Rows; row++ {
		for col := 0; col < g.Cols/2; col++ {
			left := g.index(col, row)
			right := g.index(g.Cols-1-col, row)
			g.Parent[left], g.Parent[right] = g.Parent[right], g.Parent[left]
		}
	}
}

// Parents returns a copy of the cell→block map, row-major. It is what the panel
// persists, so a hand-drawn layout survives a restart.
func (g *MergeGrid) Parents() []int {
	return append([]int(nil), g.Parent...)
}

// Restore loads a persisted cell→block map. A map whose length does not match the
// grid is ignored rather than trusted: a truncated file would otherwise index out
// of range on the next merge.
func (g *MergeGrid) Restore(cols, rows int, parents []int) {
	cols = clamp(cols, MinCustomCols, MaxCustomCols)
	rows = clamp(rows, MinCustomRows, MaxCustomRows)
	g.Cols, g.Rows = cols, rows
	if len(parents) != cols*rows {
		g.reset()
		return
	}
	seen := map[int]bool{}
	maxID := -1
	for _, p := range parents {
		if p < 0 || p >= len(parents) {
			g.reset()
			return
		}
		seen[p] = true
		if p > maxID {
			maxID = p
		}
	}
	g.Parent = append(g.Parent[:0], parents...)
	g.next = maxID + 1
	if g.next < len(parents) {
		g.next = len(parents)
	}
}

// Blocks lists the rectangles of the layout, in reading order.
func (g *MergeGrid) Blocks() []Rect {
	seen := map[int]bool{}
	out := make([]Rect, 0, 8)
	for _, id := range g.Parent {
		if seen[id] {
			continue
		}
		seen[id] = true
		if r, ok := g.BlockRect(id); ok {
			out = append(out, r)
		}
	}
	SortCells(out)
	return out
}

// Mirror returns the blocks flipped horizontally, which the panel offers as a
// one-click variation of a hand-drawn layout. The order is kept, so image i
// stays on its own cell and only the cell moves to the other side.
func Mirror(blocks []Rect) []Rect {
	out := make([]Rect, len(blocks))
	for i, r := range blocks {
		out[i] = Rect{X: 1 - r.Right(), Y: r.Y, W: r.W, H: r.H}
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// String renders the grid's blocks as a compact debug form.
func (g *MergeGrid) String() string {
	return fmt.Sprintf("MergeGrid{%dx%d, %d blocks}", g.Cols, g.Rows, len(g.Blocks()))
}

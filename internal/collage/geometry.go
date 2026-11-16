package collage

// geometry.go turns the fractional layout into pixels: margins, the gap between
// neighbouring cells and the canvas size.
//
// The renderer and the interactive preview share this file, so the cell a click
// lands in is the cell that was drawn there.

// RectPx is one cell in canvas pixels.
type RectPx struct {
	X, Y, W, H int
}

// Right and Bottom are the far pixel edges.
func (r RectPx) Right() int  { return r.X + r.W }
func (r RectPx) Bottom() int { return r.Y + r.H }

// Contains reports whether a point is inside the cell.
func (r RectPx) Contains(x, y int) bool {
	return x >= r.X && x < r.Right() && y >= r.Y && y < r.Bottom()
}

// Margins is the canvas padding on each side, in pixels.
type Margins struct {
	Top, Right, Bottom, Left int
}

// Uniform returns the same margin on all four sides.
func Uniform(v int) Margins { return Margins{Top: v, Right: v, Bottom: v, Left: v} }

// Content returns the area inside the margins, never smaller than 1×1.
func (m Margins) Content(w, h int) (x, y, cw, ch int) {
	x = m.Left
	y = m.Top
	cw = w - m.Left - m.Right
	ch = h - m.Top - m.Bottom
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	return
}

// CanvasSize resolves the output pixels for a ratio and a long edge: the longer
// side takes the requested length, the other follows the ratio.
func CanvasSize(aspectW, aspectH float64, longEdge int) (int, int) {
	if aspectW <= 0 || aspectH <= 0 {
		aspectW, aspectH = 1, 1
	}
	if longEdge < 64 {
		longEdge = 64
	}
	if aspectW >= aspectH {
		w := longEdge
		h := int(float64(longEdge) * aspectH / aspectW)
		return w, maxInt(1, h)
	}
	h := longEdge
	w := int(float64(longEdge) * aspectW / aspectH)
	return maxInt(1, w), h
}

// Layout resolves the cells in pixels.
//
// The gap is applied half per side, and only on a side that faces another cell:
// a cell at the canvas edge keeps its full size, so a 40 px margin stays 40 px
// instead of becoming 40 minus half a gap.
func Layout(cells []Rect, w, h int, m Margins, gap int) []RectPx {
	if gap < 0 {
		gap = 0
	}
	_, _, cw, ch := m.Content(w, h)
	out := make([]RectPx, 0, len(cells))
	half := gap / 2
	for _, c := range cells {
		x0 := float64(cw) * c.X
		x1 := float64(cw) * c.Right()
		y0 := float64(ch) * c.Y
		y1 := float64(ch) * c.Bottom()
		left, right := 0, 0
		if c.X > 1e-6 {
			left = half
		}
		if c.Right() < 1-1e-6 {
			right = half
		}
		top, bottom := 0, 0
		if c.Y > 1e-6 {
			top = half
		}
		if c.Bottom() < 1-1e-6 {
			bottom = half
		}
		px := m.Left + intRound(x0) + left
		py := m.Top + intRound(y0) + top
		pw := intRound(x1) - intRound(x0) - left - right
		ph := intRound(y1) - intRound(y0) - top - bottom
		if pw < 1 {
			pw = 1
		}
		if ph < 1 {
			ph = 1
		}
		out = append(out, RectPx{X: px, Y: py, W: pw, H: ph})
	}
	return out
}

// Hit returns the index of the cell at a pixel, or -1 for a point in a margin
// or a gap.
func Hit(cells []RectPx, x, y int) int {
	for i, c := range cells {
		if c.Contains(x, y) {
			return i
		}
	}
	return -1
}

func intRound(v float64) int { return int(v + 0.5) }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

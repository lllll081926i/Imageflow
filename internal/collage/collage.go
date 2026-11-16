// Package collage holds the拼图 layout model: the preset templates, the
// custom grid editor and the geometry that turns either one into rectangles.
//
// The model is pure data and pure functions. The renderer in internal/engine
// and the panel in internal/ui both read it, so a preset thumb, the editor's
// preview and the exported picture always agree on where the cells are.
package collage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Rect is one cell of a layout in fractions of the canvas: X,Y in [0,1), and
// W,H in (0,1].
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Right and Bottom are the far edges of the cell.
func (r Rect) Right() float64  { return r.X + r.W }
func (r Rect) Bottom() float64 { return r.Y + r.H }

// Area returns the cell's share of the canvas.
func (r Rect) Area() float64 { return r.W * r.H }

// Center returns the cell's middle.
func (r Rect) Center() (float64, float64) { return r.X + r.W/2, r.Y + r.H/2 }

// Preset is one template: a named set of cells for a fixed image count.
type Preset struct {
	// ID is stable across releases; the panel remembers it, not its index.
	ID string
	// Name is the short Chinese label shown under the thumbnail.
	Name string
	// Cells holds the cells in fill order, top-left to bottom-right.
	Cells []Rect
}

// Count is how many images the preset takes.
func (p Preset) Count() int { return len(p.Cells) }

// Presets lists every template, grouped by image count in the panel.
var Presets = buildPresets()

// PresetsFor returns the templates that take exactly n images.
func PresetsFor(n int) []Preset {
	out := make([]Preset, 0, 4)
	for _, p := range Presets {
		if p.Count() == n {
			out = append(out, p)
		}
	}
	return out
}

// MaxPresetImages is the largest image count with a built-in template.
const MaxPresetImages = 16

// MinPresetImages is the smallest image count with a built-in template.
const MinPresetImages = 1

// PresetByID looks a template up by its stable identifier.
func PresetByID(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// buildPresets lays out the template table.
//
// The row rules below are the layouts people actually ask for: a single
// picture, the two splits, three-in-a-row and its turns, the 2×2 grid and the
// "one big plus three" variants, then the repeated grids. Every rule is written
// on a unit canvas so a thumbnail and the export share one source of truth.
func buildPresets() []Preset {
	var out []Preset
	add := func(id, name string, cells ...Rect) {
		out = append(out, Preset{ID: id, Name: name, Cells: cells})
	}

	// 1 picture.
	add("1-full", "整张", Rect{0, 0, 1, 1})

	// 2 pictures.
	add("2-h", "左右", Rect{0, 0, 0.5, 1}, Rect{0.5, 0, 0.5, 1})
	add("2-v", "上下", Rect{0, 0, 1, 0.5}, Rect{0, 0.5, 1, 0.5})

	// 3 pictures.
	add("3-v", "三列", Rect{0, 0, 1.0 / 3, 1}, Rect{1.0 / 3, 0, 1.0 / 3, 1}, Rect{2.0 / 3, 0, 1.0 / 3, 1})
	add("3-h", "三行", Rect{0, 0, 1, 1.0 / 3}, Rect{0, 1.0 / 3, 1, 1.0 / 3}, Rect{0, 2.0 / 3, 1, 1.0 / 3})
	add("3-t1b2", "上一下二", Rect{0, 0, 1, 0.5},
		Rect{0, 0.5, 0.5, 0.5}, Rect{0.5, 0.5, 0.5, 0.5})
	add("3-t2b1", "上二下一", Rect{0, 0, 0.5, 0.5}, Rect{0.5, 0, 0.5, 0.5},
		Rect{0, 0.5, 1, 0.5})
	add("3-l1r2", "左一右二", Rect{0, 0, 0.5, 1},
		Rect{0.5, 0, 0.5, 0.5}, Rect{0.5, 0.5, 0.5, 0.5})
	add("3-l2r1", "左二右一", Rect{0, 0, 0.5, 0.5}, Rect{0, 0.5, 0.5, 0.5},
		Rect{0.5, 0, 0.5, 1})

	// 4 pictures.
	add("4-grid", "四宫格", Rect{0, 0, 0.5, 0.5}, Rect{0.5, 0, 0.5, 0.5},
		Rect{0, 0.5, 0.5, 0.5}, Rect{0.5, 0.5, 0.5, 0.5})
	add("4-h", "四行", Rect{0, 0, 1, 0.25}, Rect{0, 0.25, 1, 0.25},
		Rect{0, 0.5, 1, 0.25}, Rect{0, 0.75, 1, 0.25})
	add("4-v", "四列", Rect{0, 0, 0.25, 1}, Rect{0.25, 0, 0.25, 1},
		Rect{0.5, 0, 0.25, 1}, Rect{0.75, 0, 0.25, 1})
	add("4-t1b3", "上一下三", Rect{0, 0, 1, 0.5},
		Rect{0, 0.5, 1.0 / 3, 0.5}, Rect{1.0 / 3, 0.5, 1.0 / 3, 0.5}, Rect{2.0 / 3, 0.5, 1.0 / 3, 0.5})
	add("4-t3b1", "上三下一", Rect{0, 0, 1.0 / 3, 0.5}, Rect{1.0 / 3, 0, 1.0 / 3, 0.5}, Rect{2.0 / 3, 0, 1.0 / 3, 0.5},
		Rect{0, 0.5, 1, 0.5})
	add("4-l1r3", "左一右三", Rect{0, 0, 0.5, 1},
		Rect{0.5, 0, 0.5, 1.0 / 3}, Rect{0.5, 1.0 / 3, 0.5, 1.0 / 3}, Rect{0.5, 2.0 / 3, 0.5, 1.0 / 3})
	add("4-l3r1", "左三右一", Rect{0, 0, 0.5, 1.0 / 3}, Rect{0, 1.0 / 3, 0.5, 1.0 / 3}, Rect{0, 2.0 / 3, 0.5, 1.0 / 3},
		Rect{0.5, 0, 0.5, 1})
	add("4-big-l", "左大右三", Rect{0, 0, 2.0 / 3, 1},
		Rect{2.0 / 3, 0, 1.0 / 3, 1.0 / 3}, Rect{2.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3}, Rect{2.0 / 3, 2.0 / 3, 1.0 / 3, 1.0 / 3})
	add("4-big-t", "上大下三", Rect{0, 0, 1, 2.0 / 3},
		Rect{0, 2.0 / 3, 1.0 / 3, 1.0 / 3}, Rect{1.0 / 3, 2.0 / 3, 1.0 / 3, 1.0 / 3}, Rect{2.0 / 3, 2.0 / 3, 1.0 / 3, 1.0 / 3})

	// 5 pictures.
	add("5-big-left", "左大右四", Rect{0, 0, 0.5, 1},
		Rect{0.5, 0, 0.25, 0.5}, Rect{0.75, 0, 0.25, 0.5}, Rect{0.5, 0.5, 0.25, 0.5}, Rect{0.75, 0.5, 0.25, 0.5})
	add("5-big-top", "上大下四", Rect{0, 0, 1, 0.5},
		Rect{0, 0.5, 0.25, 0.5}, Rect{0.25, 0.5, 0.25, 0.5}, Rect{0.5, 0.5, 0.25, 0.5}, Rect{0.75, 0.5, 0.25, 0.5})
	add("5-t1b4", "上一下四", Rect{0, 0, 1, 0.5},
		Rect{0, 0.5, 0.25, 0.5}, Rect{0.25, 0.5, 0.25, 0.5}, Rect{0.5, 0.5, 0.25, 0.5}, Rect{0.75, 0.5, 0.25, 0.5})
	add("5-l1r4", "左一右四", Rect{0, 0, 0.5, 1},
		Rect{0.5, 0, 0.5, 0.25}, Rect{0.5, 0.25, 0.5, 0.25}, Rect{0.5, 0.5, 0.5, 0.25}, Rect{0.5, 0.75, 0.5, 0.25})
	add("5-rows", "五行", rows(5)...)
	add("5-cols", "五列", cols(5)...)

	// 6 to 16 pictures: the two axis-aligned grids plus one feature layout.
	add("6-grid2x3", "3×2 均分", grid(2, 3)...)
	add("6-grid3x2", "2×3 均分", grid(3, 2)...)
	add("6-big-top", "上大下五", append([]Rect{{0, 0, 1, 0.5}}, band(5, 0.5)...)...)
	add("6-big-left", "左大右五", append([]Rect{{0, 0, 0.5, 1}}, column(5, 0.5)...)...)
	add("7-grid", "3+4 错格", mixed(3, 4)...)
	add("7-big-top", "上大下六", append([]Rect{{0, 0, 1, 0.5}}, band(6, 0.5)...)...)
	add("8-grid2x4", "4×2 均分", grid(2, 4)...)
	add("8-grid4x2", "2×4 均分", grid(4, 2)...)
	add("8-big-top", "上大下七", append([]Rect{{0, 0, 1, 0.5}}, band(7, 0.5)...)...)
	add("9-grid3x3", "九宫格", grid(3, 3)...)
	add("10-grid", "5×2 均分", grid(2, 5)...)
	add("12-grid", "4×3 均分", grid(3, 4)...)
	add("16-grid", "4×4 均分", grid(4, 4)...)
	for n := 6; n <= MaxPresetImages; n++ {
		// Every count keeps a plain single-column split, so a batch with an
		// unusual number of pictures always has a sane template.
		add(strconv.Itoa(n)+"-rows", fmt.Sprintf("%d 行", n), rows(n)...)
	}
	return dedupeByID(out)
}

// dedupeByID drops a repeated identifier, keeping the first definition.
func dedupeByID(in []Preset) []Preset {
	seen := map[string]bool{}
	out := make([]Preset, 0, len(in))
	for _, p := range in {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		out = append(out, p)
	}
	return out
}

func grid(cols, rowsCount int) []Rect {
	out := make([]Rect, 0, cols*rowsCount)
	cw := 1 / float64(cols)
	ch := 1 / float64(rowsCount)
	for r := 0; r < rowsCount; r++ {
		for c := 0; c < cols; c++ {
			out = append(out, Rect{X: float64(c) * cw, Y: float64(r) * ch, W: cw, H: ch})
		}
	}
	return out
}

func rows(n int) []Rect {
	out := make([]Rect, 0, n)
	h := 1 / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, Rect{X: 0, Y: float64(i) * h, W: 1, H: h})
	}
	return out
}

func cols(n int) []Rect {
	out := make([]Rect, 0, n)
	w := 1 / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, Rect{X: float64(i) * w, Y: 0, W: w, H: 1})
	}
	return out
}

// rowOf splits the band at y..y+h into n equal cells across the full width.
func rowOf(n int, y, h float64) []Rect {
	out := make([]Rect, 0, n)
	w := 1 / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, Rect{X: float64(i) * w, Y: y, W: w, H: h})
	}
	return out
}

// band fills the area below y with n cells, using the widest short grid that
// fits them: seven cells become a row of three and a row of four, so a
// "big top" template keeps a regular lower half instead of one long strip.
func band(n int, y float64) []Rect {
	h := 1 - y
	if n <= 1 {
		return []Rect{{X: 0, Y: y, W: 1, H: h}}
	}
	// Prefer a two-row grid with balanced columns, then fall back to one row.
	if n%2 == 0 && n/2 >= 2 {
		colsN := n / 2
		w := 1 / float64(colsN)
		out := make([]Rect, 0, n)
		for r := 0; r < 2; r++ {
			for c := 0; c < colsN; c++ {
				out = append(out, Rect{X: float64(c) * w, Y: y + float64(r)*h/2, W: w, H: h / 2})
			}
		}
		return out
	}
	// An odd count splits into a full row and a shorter one below it.
	top := (n + 1) / 2
	out := make([]Rect, 0, n)
	tw := 1 / float64(top)
	for c := 0; c < top; c++ {
		out = append(out, Rect{X: float64(c) * tw, Y: y, W: tw, H: h / 2})
	}
	rest := n - top
	if rest > 0 {
		rw := 1 / float64(rest)
		for c := 0; c < rest; c++ {
			out = append(out, Rect{X: float64(c) * rw, Y: y + h/2, W: rw, H: h / 2})
		}
	}
	return out
}

// column fills the area right of x with n cells stacked in one column.
func column(n int, x float64) []Rect {
	out := make([]Rect, 0, n)
	h := 1 / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, Rect{X: x, Y: float64(i) * h, W: 1 - x, H: h})
	}
	return out
}

// mixed splits the canvas into a left column of `left` cells and a right
// column of the rest, which is the "3+4" mosaic people use for odd counts.
func mixed(left, right int) []Rect {
	out := make([]Rect, 0, left+right)
	lh := 1 / float64(left)
	for i := 0; i < left; i++ {
		out = append(out, Rect{X: 0, Y: float64(i) * lh, W: 0.5, H: lh})
	}
	rh := 1 / float64(right)
	for i := 0; i < right; i++ {
		out = append(out, Rect{X: 0.5, Y: float64(i) * rh, W: 0.5, H: rh})
	}
	return out
}

// SortCells orders cells the way a reader expects to fill them: top to bottom,
// then left to right, with the row decided by the cell's middle.
func SortCells(cells []Rect) {
	sort.SliceStable(cells, func(i, j int) bool {
		ci := cells[i].Y + cells[i].H/2
		cj := cells[j].Y + cells[j].H/2
		if diff := ci - cj; diff > 0.04 || diff < -0.04 {
			return ci < cj
		}
		return cells[i].X < cells[j].X
	})
}

// Aspect is one canvas ratio.
type Aspect struct {
	// ID is the stable name the panel stores.
	ID string
	// Label is the short form shown on the pill.
	Label string
	W, H  float64
}

// Aspects lists the canvas ratios the page offers: the screen ratios, the print
// sizes and A4. Custom ratios come from the panel's own fields.
var Aspects = []Aspect{
	{ID: "1:1", Label: "1:1", W: 1, H: 1},
	{ID: "16:9", Label: "16:9", W: 16, H: 9},
	{ID: "9:16", Label: "9:16", W: 9, H: 16},
	{ID: "16:10", Label: "16:10", W: 16, H: 10},
	{ID: "4:3", Label: "4:3", W: 4, H: 3},
	{ID: "3:4", Label: "3:4", W: 3, H: 4},
	{ID: "2:3", Label: "6寸", W: 2, H: 3},
	{ID: "3:2", Label: "6寸横", W: 3, H: 2},
	{ID: "5:7", Label: "7寸", W: 5, H: 7},
	{ID: "7:5", Label: "7寸横", W: 7, H: 5},
	{ID: "4:5", Label: "8寸", W: 4, H: 5},
	{ID: "5:4", Label: "8寸横", W: 5, H: 4},
	{ID: "a4", Label: "A4", W: 210, H: 297},
	{ID: "a4l", Label: "A4横", W: 297, H: 210},
}

// AspectByID looks a ratio up by its identifier.
func AspectByID(id string) (Aspect, bool) {
	for _, a := range Aspects {
		if a.ID == id {
			return a, true
		}
	}
	return Aspect{}, false
}

// AspectByLabel looks a ratio up by the label shown on its pill.
func AspectByLabel(label string) (Aspect, bool) {
	for _, a := range Aspects {
		if a.Label == label {
			return a, true
		}
	}
	return Aspect{}, false
}

// CustomAspectID is the identifier of the panel's free-form ratio.
const CustomAspectID = "custom"

// ParseAspect reads "16:9" or "16x9" from a text field.
func ParseAspect(text string) (Aspect, bool) {
	fields := strings.FieldsFunc(strings.TrimSpace(text), func(r rune) bool {
		return r == ':' || r == 'x' || r == 'X' || r == '×' || r == '/' || r == '*'
	})
	if len(fields) != 2 {
		return Aspect{}, false
	}
	w, errW := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
	h, errH := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
	if errW != nil || errH != nil || w <= 0 || h <= 0 || w > 1000 || h > 1000 {
		return Aspect{}, false
	}
	return Aspect{ID: CustomAspectID, W: w, H: h}, true
}

// ParseID splits a preset identifier into its parts, for tests and the batch
// command line ("3-t1b2" → 3 pictures).
func ParseID(id string) (count int, ok bool) {
	head, _, found := strings.Cut(id, "-")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(head)
	if err != nil || n < MinPresetImages || n > MaxPresetImages {
		return 0, false
	}
	return n, true
}

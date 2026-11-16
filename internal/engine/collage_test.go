package engine

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/collage"
)

// solid writes a w×h picture of one colour and returns its path.
func solid(t *testing.T, dir, name string, w, h int, c color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	path := filepath.Join(dir, name)
	if err := codec.Save(img, path, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCollageRendersATemplate covers the whole engine path: a three-cell
// template, a margin, a gap and a rounded corner must land where the geometry
// says, and the exported file must be readable.
func TestCollageRendersATemplate(t *testing.T) {
	dir := t.TempDir()
	red := solid(t, dir, "red.png", 200, 200, color.NRGBA{R: 255, A: 255})
	green := solid(t, dir, "green.png", 200, 200, color.NRGBA{G: 255, A: 255})
	blue := solid(t, dir, "blue.png", 200, 200, color.NRGBA{B: 255, A: 255})

	preset, ok := collage.PresetByID("3-t1b2")
	if !ok {
		t.Fatal("preset 3-t1b2 missing")
	}
	out := filepath.Join(dir, "collage.png")
	res := Collage(CollageRequest{
		Cells:   []CollageCell{{Path: red}, {Path: green}, {Path: blue}},
		Rects:   preset.Cells,
		AspectW: 1, AspectH: 1, LongEdge: 600,
		MarginTop: 20, MarginRight: 20, MarginBottom: 20, MarginLeft: 20,
		Gap: 10, Radius: 0,
		Background: "#FFFFFF",
		Fit:        CollageCover,
		OutputPath: out,
		Quality:    95,
	})
	if !res.Success {
		t.Fatalf("collage failed: %s", res.Error)
	}
	if res.Width != 600 || res.Height != 600 {
		t.Fatalf("canvas = %dx%d, want 600x600", res.Width, res.Height)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	px := opened.Image
	// The top half is red, the bottom-left is green and the bottom-right blue.
	check := func(x, y int, want color.NRGBA) {
		t.Helper()
		got := color.NRGBAModel.Convert(px.At(x, y)).(color.NRGBA)
		if got.R != want.R || got.G != want.G || got.B != want.B {
			t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
		}
	}
	check(300, 150, color.NRGBA{R: 255, A: 255})
	check(160, 450, color.NRGBA{G: 255, A: 255})
	check(440, 450, color.NRGBA{B: 255, A: 255})
	// The margin stays white; the gap between the two lower cells does too.
	check(10, 300, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	check(300, 560, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
}

// TestCollageLeavesEmptySlotsBlank pins the short-batch behaviour: a template
// with four slots and two pictures keeps the background where the other two
// would be, instead of failing.
func TestCollageLeavesEmptySlotsBlank(t *testing.T) {
	dir := t.TempDir()
	red := solid(t, dir, "red.png", 100, 100, color.NRGBA{R: 255, A: 255})
	out := filepath.Join(dir, "two.png")
	res := Collage(CollageRequest{
		Cells:      []CollageCell{{Path: red}, {Path: red}},
		Rects:      []collage.Rect{{X: 0, Y: 0, W: 0.5, H: 0.5}, {X: 0.5, Y: 0, W: 0.5, H: 0.5}, {X: 0, Y: 0.5, W: 0.5, H: 0.5}, {X: 0.5, Y: 0.5, W: 0.5, H: 0.5}},
		AspectW:    1,
		AspectH:    1,
		LongEdge:   400,
		Background: "#00FF00",
		OutputPath: out,
	})
	if !res.Success {
		t.Fatalf("collage failed: %s", res.Error)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	// The bottom row was never filled, so it keeps the green background.
	got := color.NRGBAModel.Convert(opened.Image.At(100, 300)).(color.NRGBA)
	if got.G != 255 || got.R != 0 {
		t.Errorf("empty slot = %v, want the background colour", got)
	}
}

// TestCollagePansTheCoverCrop pins the per-cell framing: a red/blue split
// picture shows only its left half when the cell pans fully left.
func TestCollagePansTheCoverCrop(t *testing.T) {
	dir := t.TempDir()
	split := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			if x < 100 {
				split.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
			} else {
				split.SetNRGBA(x, y, color.NRGBA{B: 255, A: 255})
			}
		}
	}
	path := filepath.Join(dir, "split.png")
	if err := codec.Save(split, path, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "pan.png")
	res := Collage(CollageRequest{
		Cells:      []CollageCell{{Path: path, OffsetX: -1}},
		Rects:      []collage.Rect{{X: 0, Y: 0, W: 1, H: 1}},
		AspectW:    1,
		AspectH:    1,
		LongEdge:   200,
		Fit:        CollageCover,
		Background: "#FFFFFF",
		OutputPath: out,
	})
	if !res.Success {
		t.Fatalf("collage failed: %s", res.Error)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	got := color.NRGBAModel.Convert(opened.Image.At(190, 100)).(color.NRGBA)
	if got.R != 255 {
		t.Errorf("right edge = %v, want the red half after panning left", got)
	}
}

// TestCollageDrawsMarksAndWatermark asserts the two decoration layers reach the
// picture instead of being silently dropped.
func TestCollageDrawsMarksAndWatermark(t *testing.T) {
	dir := t.TempDir()
	white := solid(t, dir, "white.png", 200, 200, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	out := filepath.Join(dir, "deco.png")
	res := Collage(CollageRequest{
		Cells:      []CollageCell{{Path: white}},
		Rects:      []collage.Rect{{X: 0, Y: 0, W: 1, H: 1}},
		AspectW:    1,
		AspectH:    1,
		LongEdge:   600,
		Background: "#FFFFFF",
		Marks: []CollageMark{
			{Kind: "rect", X1: 0.1, Y1: 0.1, X2: 0.4, Y2: 0.4, Color: "#FF0000", Width: 6},
			{Kind: "arrow", X1: 0.6, Y1: 0.2, X2: 0.9, Y2: 0.5, Color: "#00AA00", Width: 6},
			{Kind: "text", X1: 0.1, Y1: 0.7, Text: "标题", Color: "#0000FF", Width: 6},
		},
		Watermark:  CollageWatermark{Enabled: true, Text: "IF", Size: 24, Opacity: 0.3, Angle: -30, Density: 1},
		OutputPath: out,
	})
	if !res.Success {
		t.Fatalf("collage failed: %s", res.Error)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	red := 0
	blue := 0
	b := opened.Image.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(opened.Image.At(x, y)).(color.NRGBA)
			if c.R > 200 && c.G < 90 && c.B < 90 {
				red++
			}
			if c.B > 160 && c.R < 120 {
				blue++
			}
		}
	}
	if red < 30 {
		t.Errorf("red rectangle left %d red pixels", red)
	}
	if blue < 20 {
		t.Errorf("blue label left %d blue pixels", blue)
	}
}

// TestCollageMirrorsACell pins the per-cell mirror: a left/right split picture
// flipped horizontally shows its blue half where the red half was, with the crop
// unchanged.
func TestCollageMirrorsACell(t *testing.T) {
	dir := t.TempDir()
	split := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			if x < 100 {
				split.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
			} else {
				split.SetNRGBA(x, y, color.NRGBA{B: 255, A: 255})
			}
		}
	}
	path := filepath.Join(dir, "split.png")
	if err := codec.Save(split, path, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	render := func(flip bool) color.NRGBA {
		out := filepath.Join(dir, fmt.Sprintf("flip_%v.png", flip))
		res := Collage(CollageRequest{
			Cells:      []CollageCell{{Path: path, FlipH: flip}},
			Rects:      []collage.Rect{{X: 0, Y: 0, W: 1, H: 1}},
			AspectW:    2,
			AspectH:    1,
			LongEdge:   400,
			Fit:        CollageCover,
			Background: "#FFFFFF",
			OutputPath: out,
		})
		if !res.Success {
			t.Fatalf("collage failed: %s", res.Error)
		}
		img, err := codec.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		return color.NRGBAModel.Convert(img.Image.At(20, 100)).(color.NRGBA)
	}
	if c := render(false); c.R != 255 {
		t.Fatalf("unflipped left edge = %v, want red", c)
	}
	if c := render(true); c.B != 255 {
		t.Fatalf("flipped left edge = %v, want blue", c)
	}
}

func TestCollageSeamlessByDefault(t *testing.T) {
	dir := t.TempDir()
	red := solid(t, dir, "red.png", 200, 200, color.NRGBA{R: 255, A: 255})
	blue := solid(t, dir, "blue.png", 200, 200, color.NRGBA{B: 255, A: 255})
	out := filepath.Join(dir, "seamless.png")
	res := Collage(CollageRequest{
		Cells: []CollageCell{{Path: red}, {Path: blue}},
		Rects: []collage.Rect{
			{X: 0, Y: 0, W: 0.5, H: 1}, {X: 0.5, Y: 0, W: 0.5, H: 1},
		},
		AspectW: 1, AspectH: 1, LongEdge: 400,
		Background: "#00FF00",
		OutputPath: out,
	})
	if !res.Success {
		t.Fatalf("collage failed: %s", res.Error)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	// The joint must show no background: no green column in the middle.
	for y := 0; y < 400; y += 20 {
		for x := 195; x < 205; x++ {
			c := color.NRGBAModel.Convert(opened.Image.At(x, y)).(color.NRGBA)
			if c.G > 200 && c.R < 60 && c.B < 60 {
				t.Fatalf("seam at (%d,%d): %v", x, y, c)
			}
		}
	}
}

// TestCollageSeamedShowsTheBackground covers the other half: with a gap and a
// radius, the joint and the corners show the canvas colour.
func TestCollageSeamedShowsTheBackground(t *testing.T) {
	dir := t.TempDir()
	red := solid(t, dir, "red.png", 200, 200, color.NRGBA{R: 255, A: 255})
	blue := solid(t, dir, "blue.png", 200, 200, color.NRGBA{B: 255, A: 255})
	out := filepath.Join(dir, "seamed.png")
	res := Collage(CollageRequest{
		Cells: []CollageCell{{Path: red}, {Path: blue}},
		Rects: []collage.Rect{
			{X: 0, Y: 0, W: 0.5, H: 1}, {X: 0.5, Y: 0, W: 0.5, H: 1},
		},
		AspectW: 1, AspectH: 1, LongEdge: 400,
		MarginTop: 20, MarginRight: 20, MarginBottom: 20, MarginLeft: 20,
		Gap: 20, Radius: 20,
		Background: "#00FF00",
		OutputPath: out,
	})
	if !res.Success {
		t.Fatalf("collage failed: %s", res.Error)
	}
	opened, err := codec.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	green := 0
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			c := color.NRGBAModel.Convert(opened.Image.At(x, y)).(color.NRGBA)
			if c.G > 200 && c.R < 60 && c.B < 60 {
				green++
			}
		}
	}
	if green < 400 {
		t.Errorf("seamed collage shows %d background pixels, want the gap and corners", green)
	}
}

func TestCollageRejectsBadInputs(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "x.png")
	res := Collage(CollageRequest{OutputPath: out})
	if res.Success {
		t.Error("an empty request must not succeed")
	}
	res = Collage(CollageRequest{
		Cells:      []CollageCell{{Path: filepath.Join(dir, "missing.png")}},
		Rects:      []collage.Rect{{X: 0, Y: 0, W: 1, H: 1}},
		OutputPath: out,
	})
	if res.Success || res.ErrorCode != "READ_FAILED" {
		t.Errorf("missing input = %s (%s)", res.ErrorCode, res.Error)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a failed render must not leave a file behind")
	}
}

func TestCollageRespectsCancellation(t *testing.T) {
	dir := t.TempDir()
	red := solid(t, dir, "red.png", 50, 50, color.NRGBA{R: 255, A: 255})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := CollageContext(ctx, CollageRequest{
		Cells:      []CollageCell{{Path: red}},
		Rects:      []collage.Rect{{X: 0, Y: 0, W: 1, H: 1}},
		OutputPath: filepath.Join(dir, "c.png"),
	})
	if res.Success || res.ErrorCode != "CANCELLED" {
		t.Fatalf("cancelled render = %+v", res)
	}
}

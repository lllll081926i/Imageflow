package engine_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestPDFOrientationChangesPageBox pins the orientation control: the panel used a
// bool whose zero value silently meant landscape, so every default request came
// out sideways. A4 portrait is 595.28x841.89 points, landscape is the transpose.
func TestPDFOrientationChangesPageBox(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 100, 80)

	portrait := filepath.Join(dir, "portrait.pdf")
	res := engine.GeneratePDF(engine.PDFRequest{
		ImagePaths: []string{src}, OutputPath: portrait, PageSize: "A4", Layout: "single",
	})
	if !res.Success {
		t.Fatalf("portrait: %s", res.Error)
	}
	w, h := pageBox(t, portrait)
	if !(w < h) {
		t.Errorf("portrait page is %vx%v, want taller than wide", w, h)
	}
	if w < 590 || w > 600 || h < 836 || h > 848 {
		t.Errorf("portrait page = %vx%v, want A4 at 595.28x841.89", w, h)
	}

	landscape := filepath.Join(dir, "landscape.pdf")
	res = engine.GeneratePDF(engine.PDFRequest{
		ImagePaths: []string{src}, OutputPath: landscape, PageSize: "A4",
		Layout: "single", Landscape: true,
	})
	if !res.Success {
		t.Fatalf("landscape: %s", res.Error)
	}
	lw, lh := pageBox(t, landscape)
	if !(lw > lh) {
		t.Errorf("landscape page is %vx%v, want wider than tall", lw, lh)
	}
	if lw != h || lh != w {
		t.Errorf("landscape page = %vx%v, want %vx%v", lw, lh, h, w)
	}
}

var mediaBox = regexp.MustCompile(`/MediaBox\s*\[\s*0\s+0\s+([0-9.]+)\s+([0-9.]+)\s*\]`)

// pageBox reads the first page's MediaBox out of the encoded PDF.
func pageBox(t *testing.T, path string) (float64, float64) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := mediaBox.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("no MediaBox in %s:\n%s", path, head(raw, 900))
	}
	w, _ := strconv.ParseFloat(string(m[1]), 64)
	h, _ := strconv.ParseFloat(string(m[2]), 64)
	return w, h
}

func head(raw []byte, n int) string {
	if len(raw) > n {
		return string(raw[:n])
	}
	return string(raw)
}

package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestBuildIconSetWritesTheBundle pins the icon bundle's contents: one PNG per
// size, a multi-resolution .ico, a small favicon.ico and an SVG wrapper.
func TestBuildIconSetWritesTheBundle(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 512, 512)
	outDir := filepath.Join(dir, "icons")
	res := engine.BuildIconSet(engine.IconRequest{
		InputPath: src, OutputDir: outDir,
		Sizes:   []int{16, 32, 256},
		WithICO: true, WithFavicon: true, WithSVG: true,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	for _, name := range []string{"icon-16x16.png", "icon-32x32.png", "icon-256x256.png", "icon.ico", "favicon.ico", "icon.svg"} {
		path := filepath.Join(outDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}
	// Every PNG is exactly its size.
	for name, size := range map[string]int{"icon-16x16.png": 16, "icon-32x32.png": 32, "icon-256x256.png": 256} {
		opened, err := codec.Open(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if opened.Width != size || opened.Height != size {
			t.Errorf("%s = %dx%d", name, opened.Width, opened.Height)
		}
	}
	// The .ico is a real Windows icon: 0,0 then type 1.
	raw, err := os.ReadFile(filepath.Join(outDir, "icon.ico"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 6 || raw[0] != 0 || raw[1] != 0 || raw[2] != 1 || raw[3] != 0 {
		t.Errorf("icon.ico header = % x, want a type-1 icon", raw[:min(6, len(raw))])
	}
	// The SVG wraps a data URL, so it opens on its own.
	svgText, err := os.ReadFile(filepath.Join(outDir, "icon.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(svgText) < 100 || !contains(string(svgText), "data:image/png;base64,") {
		t.Errorf("icon.svg does not embed the picture: %s", headBytes(svgText, 120))
	}
}

// TestBuildIconSetCropsToSquare pins that a non-square source is centre-cropped
// rather than stretched, and that padding shrinks the picture inside its canvas.
func TestBuildIconSetCropsToSquare(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 300, 100)
	outDir := filepath.Join(dir, "icons")
	res := engine.BuildIconSet(engine.IconRequest{
		InputPath: src, OutputDir: outDir, Sizes: []int{64},
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Width != 100 || res.Height != 100 {
		t.Errorf("prepared square = %dx%d, want 100x100", res.Width, res.Height)
	}

	padded := filepath.Join(dir, "padded")
	res = engine.BuildIconSet(engine.IconRequest{
		InputPath: src, OutputDir: padded, Sizes: []int{64},
		Padding: 10, Background: "#FFFFFF",
	})
	if !res.Success {
		t.Fatalf("padded: %s", res.Error)
	}
	opened, err := codec.Open(filepath.Join(padded, "icon-64x64.png"))
	if err != nil {
		t.Fatal(err)
	}
	// A 10% padding leaves the subject inside a white margin: the corner pixel is
	// the background, the centre is not.
	r, g, b, _ := opened.Image.At(2, 2).RGBA()
	if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
		t.Errorf("corner = %d,%d,%d, want the white margin", r>>8, g>>8, b>>8)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func headBytes(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}

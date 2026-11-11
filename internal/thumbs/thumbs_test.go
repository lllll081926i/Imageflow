package thumbs

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

func sampleFile(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	path := filepath.Join(dir, "thumb-source.png")
	if err := codec.Save(img, path, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGetRendersAndCaches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	src := sampleFile(t, dir, 800, 600)
	cache := New()
	info, err := cache.Get(src, 1, 2)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if info.Width > MaxEdge || info.Height > MaxEdge {
		t.Fatalf("thumbnail %dx%d exceeds %d", info.Width, info.Height, MaxEdge)
	}
	if info.Width == 0 || info.Height == 0 {
		t.Fatal("thumbnail has no size")
	}
	if len(info.Data) == 0 {
		t.Fatal("thumbnail has no bytes")
	}
	if cache.Len() != 1 {
		t.Errorf("memory entries = %d", cache.Len())
	}
	again, err := cache.Get(src, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if again.Width != info.Width || len(again.Data) != len(info.Data) {
		t.Error("cached thumbnail differs")
	}
}

func TestGetRejectsMissingFile(t *testing.T) {
	cache := New()
	if _, err := cache.Get(filepath.Join(t.TempDir(), "nope.png"), 0, 0); err == nil {
		t.Fatal("missing file produced a thumbnail")
	}
}

func TestKeyChangesWithMetadata(t *testing.T) {
	a := keyFor("/x/a.png", 1, 2)
	b := keyFor("/x/a.png", 3, 2)
	c := keyFor("/x/a.png", 1, 4)
	if a == b || a == c {
		t.Error("cache key ignores mtime or size")
	}
	if a != keyFor("/x/a.png", 1, 2) {
		t.Error("cache key is not stable")
	}
}

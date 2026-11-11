package ui

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// TestResizeToolPresetsEndToEnd pins the multi-size mode through the real batch:
// one input, three boxes, three files on disk at exactly those sizes.
func TestResizeToolPresetsEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1) // 120x90 each
	a.Active = 1
	tool := a.ActiveTool().(*ResizeTool)
	tool.mode = 3
	tool.preset = "1024x512, 256x128, 64x16"
	tool.fit = 0
	tool.presetHex = "#FFFFFF"
	tool.format = ""

	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)

	row := a.Session.Snapshot()[0]
	// A tool that writes several files reports their folder, not one path.
	outDir := row.Output
	if filepath.Base(outDir) != "img00_sizes" {
		// The row holds the first file; its folder is the one the tool made.
		outDir = filepath.Dir(outDir)
	}
	if filepath.Base(outDir) != "img00_sizes" {
		t.Fatalf("output dir = %q, want img00_sizes", outDir)
	}
	want := map[string][2]int{
		"img00_1024x512.png": {1024, 512},
		"img00_256x128.png":  {256, 128},
		"img00_64x16.png":    {64, 16},
	}
	for name, size := range want {
		opened, err := codec.Open(filepath.Join(outDir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if opened.Width != size[0] || opened.Height != size[1] {
			t.Errorf("%s = %dx%d, want %dx%d", name, opened.Width, opened.Height, size[0], size[1])
		}
	}
}

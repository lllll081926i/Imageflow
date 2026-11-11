package ui

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// TestCropToolEndToEnd pins the crop page: a 120×90 input cropped to 1:1 comes
// out square, and its filename carries the operation.
func TestCropToolEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 2
	tool := a.ActiveTool().(*CropTool)
	tool.ratio = 0 // 1:1
	tool.anchor = 4
	tool.scale = 0
	tool.format = "png"

	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)

	row := a.Session.Snapshot()[0]
	if filepath.Ext(row.Output) != ".png" {
		t.Fatalf("output = %q", row.Output)
	}
	opened, err := codec.Open(row.Output)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Width != 90 || opened.Height != 90 {
		t.Errorf("crop = %dx%d, want 90x90", opened.Width, opened.Height)
	}
}

// TestIconToolEndToEnd pins the icon page: one input produces the whole bundle in
// a folder next to it.
func TestIconToolEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 8
	tool := a.ActiveTool().(*IconTool)
	tool.withICO = true
	tool.withFav = true
	tool.withSVG = true

	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)

	row := a.Session.Snapshot()[0]
	dir := filepath.Dir(row.Output)
	if filepath.Base(dir) != "img00_icons" {
		t.Fatalf("bundle dir = %q, want img00_icons", dir)
	}
	// 16 PNGs, one .ico, one favicon.ico, one .svg.
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 19 {
		t.Errorf("bundle has %d files, want 19: %v", len(files), files)
	}
	for _, name := range []string{"icon-1024x1024.png", "icon.ico", "favicon.ico", "icon.svg"} {
		if _, err := codec.Open(filepath.Join(dir, name)); err != nil && name == "icon-1024x1024.png" {
			t.Errorf("%s: %v", name, err)
		}
	}
}

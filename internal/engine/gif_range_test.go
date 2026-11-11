package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestGIFFrameRangeExportsSubset pins the frame range the export panel exposes.
// The engine accepted a range but the panel never set one, so a 5-frame
// animation could only ever be exported whole.
func TestGIFFrameRangeExportsSubset(t *testing.T) {
	dir := t.TempDir()
	src := writeGIF(t, filepath.Join(dir, "in.gif"), 40, 40, 5)

	outAll := filepath.Join(dir, "all")
	res := engine.RunGIF(engine.GIFRequest{
		Action: "export_frames", InputPath: src, OutputDir: outAll, OutputFormat: "png",
	})
	if !res.Success {
		t.Fatalf("all: %s", res.Error)
	}
	if res.Count != 5 {
		t.Fatalf("all frames = %d, want 5", res.Count)
	}

	outRange := filepath.Join(dir, "range")
	res = engine.RunGIF(engine.GIFRequest{
		Action: "export_frames", InputPath: src, OutputDir: outRange,
		OutputFormat: "png", FrameStart: 1, FrameEnd: 3,
	})
	if !res.Success {
		t.Fatalf("range: %s", res.Error)
	}
	if res.Count != 2 {
		t.Fatalf("range frames = %d, want 2", res.Count)
	}
	// Names carry the original 1-based index, so a subset keeps its position.
	names := map[string]bool{}
	for _, p := range res.Outputs {
		names[filepath.Base(p)] = true
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s", p)
		}
	}
	for _, want := range []string{"in_frame_0002.png", "in_frame_0003.png"} {
		if !names[want] {
			t.Errorf("missing %s in %v", want, names)
		}
	}
}

// TestGIFFrameRangeRejectsEmptyRange pins that an inverted range is an error
// instead of silently writing nothing.
func TestGIFFrameRangeRejectsEmptyRange(t *testing.T) {
	dir := t.TempDir()
	src := writeGIF(t, filepath.Join(dir, "in.gif"), 20, 20, 3)
	res := engine.RunGIF(engine.GIFRequest{
		Action: "export_frames", InputPath: src, OutputDir: filepath.Join(dir, "x"),
		OutputFormat: "png", FrameStart: 2, FrameEnd: 1,
	})
	if res.Success {
		t.Fatal("an empty frame range must fail")
	}
	if res.ErrorCode != "GIF_BAD_REQUEST" {
		t.Errorf("code = %s", res.ErrorCode)
	}
}

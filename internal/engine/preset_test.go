package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestParsePresetSizes pins the size list people actually type: mixed separators,
// the multiplication sign, a bare number as a square, duplicates dropped.
func TestParsePresetSizes(t *testing.T) {
	got := engine.ParsePresetSizes("1024x512, 256×128 64*16;512x512,256x128")
	want := []engine.PresetSize{
		{Width: 1024, Height: 512},
		{Width: 256, Height: 128},
		{Width: 64, Height: 16},
		{Width: 512, Height: 512},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	if sizes := engine.ParsePresetSizes("abc, 0x10, -4x5, 1024"); len(sizes) != 1 || sizes[0] != (engine.PresetSize{Width: 1024, Height: 1024}) {
		t.Errorf("invalid entries were kept: %v", sizes)
	}
}

// TestResizeToPresetsWritesEverySize pins the feature: one run produces one file
// per box, each exactly that size, letterboxed when the ratios differ.
func TestResizeToPresetsWritesEverySize(t *testing.T) {
	dir := t.TempDir()
	// 300x200 is 3:2, so an 8:1 and a 1:1 box both need letterboxing.
	src := samplePNG(t, dir, 300, 200)
	outDir := filepath.Join(dir, "presets")
	res := engine.ResizeToPresets(engine.PresetRequest{
		InputPath: src, OutputDir: outDir, BaseName: "photo",
		Sizes: engine.ParsePresetSizes("1024x512, 256x128, 64x16"),
		Mode:  "contain", Background: "#FFFFFF", OutputFormat: "png",
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Count != 3 || len(res.Outputs) != 3 {
		t.Fatalf("count = %d, outputs = %v", res.Count, res.Outputs)
	}
	want := []engine.PresetSize{
		{Width: 1024, Height: 512},
		{Width: 256, Height: 128},
		{Width: 64, Height: 16},
	}
	for i, size := range want {
		path := res.Outputs[i]
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if base := filepath.Base(path); base != "photo_"+size.String()+".png" {
			t.Errorf("file %d = %q, want the size in the name", i, base)
		}
		opened, err := codec.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if opened.Width != size.Width || opened.Height != size.Height {
			t.Errorf("%s = %dx%d, want %dx%d", size, opened.Width, opened.Height, size.Width, size.Height)
		}
	}

	// A second run must not overwrite the first: the names collide.
	res2 := engine.ResizeToPresets(engine.PresetRequest{
		InputPath: src, OutputDir: outDir, BaseName: "photo",
		Sizes: engine.ParsePresetSizes("256x128"), OutputFormat: "png",
	})
	if !res2.Success {
		t.Fatalf("%s", res2.Error)
	}
	if res2.Outputs[0] == res.Outputs[1] {
		t.Error("the second run overwrote the first run's file")
	}
}

// TestResizeToPresetsModes pins the three ways the picture can meet its box.
func TestResizeToPresetsModes(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 300, 200)
	for _, mode := range []string{"contain", "cover", "stretch"} {
		res := engine.ResizeToPresets(engine.PresetRequest{
			InputPath: src, OutputDir: filepath.Join(dir, mode), BaseName: "p",
			Sizes: []engine.PresetSize{{Width: 64, Height: 64}},
			Mode:  mode, Background: "#000000", OutputFormat: "png",
		})
		if !res.Success {
			t.Fatalf("%s: %s", mode, res.Error)
		}
		opened, err := codec.Open(res.Outputs[0])
		if err != nil {
			t.Fatal(err)
		}
		if opened.Width != 64 || opened.Height != 64 {
			t.Errorf("%s = %dx%d, want 64x64", mode, opened.Width, opened.Height)
		}
	}
}

// TestResizeToPresetsRejectsEmptyList pins the guard the panel relies on.
func TestResizeToPresetsRejectsEmptyList(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 40, 40)
	res := engine.ResizeToPresets(engine.PresetRequest{
		InputPath: src, OutputDir: dir, OutputFormat: "png",
	})
	if res.Success || res.ErrorCode != "PRESET_NO_SIZES" {
		t.Fatalf("got %+v", res)
	}
}

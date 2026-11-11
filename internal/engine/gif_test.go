package engine_test

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// distinctFrames builds n visibly different frames: libwebp collapses
// identical frames into a single still image, which would make an animation
// fixture degenerate.
func distinctFrames(t *testing.T, w, h, n int) []engine.Frame {
	t.Helper()
	out := make([]engine.Frame, 0, n)
	for i := 0; i < n; i++ {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		base := uint8(30 + i*70)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetNRGBA(x, y, color.NRGBA{R: base, G: uint8(x * 4), B: uint8(y * 4), A: 255})
			}
		}
		out = append(out, engine.Frame{Image: img, Delay: 120})
	}
	return out
}

func writeGIF(t *testing.T, path string, w, h, n int) string {
	t.Helper()
	if err := engine.WriteGIFForTest(&engine.Animation{
		Frames: distinctFrames(t, w, h, n), Width: w, Height: h,
	}, path); err != nil {
		t.Fatalf("write gif: %v", err)
	}
	return path
}

func TestGIFProbeAndExport(t *testing.T) {
	dir := t.TempDir()
	in := writeGIF(t, filepath.Join(dir, "anim.gif"), 48, 32, 3)
	mustExist(t, in)

	res := engine.RunGIF(engine.GIFRequest{Action: "get_frame_count", InputPath: in})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Count != 3 {
		t.Fatalf("frames = %d, want 3", res.Count)
	}
	if res.Width != 48 || res.Height != 32 {
		t.Errorf("canvas %dx%d", res.Width, res.Height)
	}

	outDir := filepath.Join(dir, "frames")
	exported := engine.RunGIF(engine.GIFRequest{
		Action: "export_frames", InputPath: in, OutputDir: outDir, OutputFormat: "png",
	})
	if !exported.Success {
		t.Fatalf("%s", exported.Error)
	}
	if exported.Count != 3 {
		t.Fatalf("exported = %d", exported.Count)
	}
	for _, p := range exported.Outputs {
		mustExist(t, p)
	}
}

func TestGIFReverseAndSpeed(t *testing.T) {
	dir := t.TempDir()
	in := writeGIF(t, filepath.Join(dir, "in.gif"), 40, 40, 2)

	rev := filepath.Join(dir, "rev.gif")
	res := engine.RunGIF(engine.GIFRequest{Action: "reverse", InputPath: in, OutputPath: rev, OutputFormat: "gif"})
	if !res.Success {
		t.Fatalf("reverse: %s", res.Error)
	}
	loaded, err := engine.LoadAnimation(rev)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Frames) != 2 {
		t.Fatalf("reversed frames = %d", len(loaded.Frames))
	}

	fast := filepath.Join(dir, "fast.gif")
	res = engine.RunGIF(engine.GIFRequest{
		Action: "change_speed", InputPath: in, OutputPath: fast,
		OutputFormat: "gif", SpeedFactor: 2,
	})
	if !res.Success {
		t.Fatalf("speed: %s", res.Error)
	}
	loaded, err = engine.LoadAnimation(fast)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Frames[0].Delay > 100 {
		t.Errorf("speed change did not shorten delays: %d", loaded.Frames[0].Delay)
	}
}

func TestGIFAnimatedWebPAndAPNG(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 48, 48)
	frames := distinctFrames(t, 48, 48, 3)
	in := filepath.Join(dir, "in.gif")
	if err := engine.WriteGIFForTest(&engine.Animation{Frames: frames, Width: 48, Height: 48}, in); err != nil {
		t.Fatal(err)
	}
	_ = src

	for _, format := range []string{"webp", "apng"} {
		out := filepath.Join(dir, "anim."+format)
		res := engine.RunGIF(engine.GIFRequest{
			Action: "reverse", InputPath: in, OutputPath: out, OutputFormat: format, Quality: 80,
		})
		if !res.Success {
			t.Fatalf("%s: %s", format, res.Error)
		}
		mustExist(t, out)
		loaded, err := engine.LoadAnimation(out)
		if err != nil {
			t.Fatalf("%s reload: %v", format, err)
		}
		if len(loaded.Frames) != 3 {
			t.Errorf("%s frames = %d, want 3", format, len(loaded.Frames))
		}
	}
}

func TestGIFResize(t *testing.T) {
	dir := t.TempDir()
	in := writeGIF(t, filepath.Join(dir, "in.gif"), 80, 40, 2)
	out := filepath.Join(dir, "small.gif")
	res := engine.RunGIF(engine.GIFRequest{
		Action: "resize", InputPath: in, OutputPath: out, OutputFormat: "gif",
		Width: 40, MaintainAR: true,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	if res.Width != 40 {
		t.Errorf("width = %d", res.Width)
	}
}

func TestGIFRejectsStillImages(t *testing.T) {
	dir := t.TempDir()
	still := samplePNG(t, dir, 20, 20)
	res := engine.RunGIF(engine.GIFRequest{
		Action: "reverse", InputPath: still, OutputPath: filepath.Join(dir, "x.gif"),
	})
	if res.Success {
		t.Fatal("a still PNG must not be treated as an animation")
	}
	if res.ErrorCode != "GIF_DECODE_FAILED" {
		t.Errorf("code = %s", res.ErrorCode)
	}
}

func TestGIFUnknownAction(t *testing.T) {
	res := engine.RunGIF(engine.GIFRequest{Action: "nope"})
	if res.Success || res.ErrorCode != "GIF_BAD_REQUEST" {
		t.Fatalf("got %+v", res)
	}
}

// TestGIFCompressShrinksFile pins that the compress action actually changes the
// output: a lower quality must quantise to fewer colours and write fewer bytes,
// because a no-op mutation would silently export the input unchanged.
func TestGIFCompressShrinksFile(t *testing.T) {
	dir := t.TempDir()
	// A gradient-heavy frame makes the palette size visible in the byte count.
	frames := distinctFrames(t, 120, 120, 3)
	in := filepath.Join(dir, "in.gif")
	if err := engine.WriteGIFForTest(&engine.Animation{Frames: frames, Width: 120, Height: 120}, in); err != nil {
		t.Fatal(err)
	}

	low := filepath.Join(dir, "low.gif")
	res := engine.RunGIF(engine.GIFRequest{
		Action: "compress", InputPath: in, OutputPath: low, OutputFormat: "gif", Quality: 10,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}
	high := filepath.Join(dir, "high.gif")
	res = engine.RunGIF(engine.GIFRequest{
		Action: "compress", InputPath: in, OutputPath: high, OutputFormat: "gif", Quality: 100,
	})
	if !res.Success {
		t.Fatalf("%s", res.Error)
	}

	lowInfo, err := os.Stat(low)
	if err != nil {
		t.Fatal(err)
	}
	highInfo, err := os.Stat(high)
	if err != nil {
		t.Fatal(err)
	}
	if lowInfo.Size() >= highInfo.Size() {
		t.Errorf("quality 10 wrote %d bytes, quality 100 wrote %d", lowInfo.Size(), highInfo.Size())
	}
	lowAnim, err := engine.LoadAnimation(low)
	if err != nil {
		t.Fatal(err)
	}
	highAnim, err := engine.LoadAnimation(high)
	if err != nil {
		t.Fatal(err)
	}
	if len(lowAnim.Frames) != len(highAnim.Frames) {
		t.Errorf("frame count changed: %d vs %d", len(lowAnim.Frames), len(highAnim.Frames))
	}
	// The palette lives in the file, not in the composed frames, so read it back
	// with the container decoder.
	if n := gifPaletteSize(t, low); n > 60 {
		t.Errorf("quality 10 palette has %d entries", n)
	}
	if n := gifPaletteSize(t, high); n < 100 {
		t.Errorf("quality 100 palette has %d entries", n)
	}
}

// gifPaletteSize reports how many colours the first frame of a GIF carries.
func gifPaletteSize(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decoded, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Image) == 0 {
		t.Fatal("no frames")
	}
	return len(decoded.Image[0].Palette)
}

var _ = codec.Open

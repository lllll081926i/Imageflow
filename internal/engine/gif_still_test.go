package engine_test

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestStillGIFIsNotAnAnimation pins that a one-frame GIF is rejected by the
// animation tool. The container is a valid GIF, so only the frame count can tell
// it apart from an animation, and re-encoding it as "reverse" or "speed" would
// silently do nothing.
func TestStillGIFIsNotAnAnimation(t *testing.T) {
	dir := t.TempDir()
	frames := distinctFrames(t, 20, 20, 1)
	src := filepath.Join(dir, "still.gif")
	if err := engine.WriteGIFForTest(&engine.Animation{
		Frames: frames, Width: 20, Height: 20, Loop: 1,
	}, src); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := engine.ProbeAnimation(src); err == nil {
		t.Fatal("a one-frame GIF must not probe as an animation")
	}
	res := engine.RunGIF(engine.GIFRequest{
		Action: "reverse", InputPath: src, OutputPath: filepath.Join(dir, "out.gif"),
	})
	if res.Success {
		t.Fatal("a one-frame GIF must not be processed as an animation")
	}
	if res.ErrorCode != "GIF_DECODE_FAILED" {
		t.Errorf("code = %s", res.ErrorCode)
	}
}

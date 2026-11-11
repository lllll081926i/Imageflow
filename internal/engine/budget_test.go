package engine_test

import (
	"strings"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestRichTextRejectsOversizedCanvas pins the shared canvas budget on the path
// that has no fixed height: a very long document must fail with a clear error
// instead of asking the runtime for a multi-gigabyte image.
func TestRichTextRejectsOversizedCanvas(t *testing.T) {
	blocks := []engine.RichTextBlock{{
		Type: "paragraph",
		Text: strings.Repeat("这是一段用来撑高画布的测试文字。", 4000),
	}}
	_, err := engine.RenderRichTextImage(engine.RichTextRequest{
		Blocks: blocks, Width: 800, Padding: 48, FontSize: 18, LineHeight: 1.6,
	})
	if err == nil {
		t.Fatal("a canvas taller than the budget must be rejected")
	}
	if !strings.Contains(err.Error(), "超出") {
		t.Errorf("error = %v, want a budget message", err)
	}
}

// TestWatermarkRejectsOversizedStamp pins the same budget on the stamp box.
func TestWatermarkRejectsOversizedStamp(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 64, 64)
	res := engine.Watermark(engine.WatermarkRequest{
		InputPath:  src,
		OutputPath: dir + "/out.png",
		Text:       strings.Repeat("W", 4000),
		FontSize:   512,
	})
	if res.Success {
		t.Fatal("an oversized watermark stamp must be rejected")
	}
	if res.ErrorCode != "OUTPUT_TOO_LARGE" {
		t.Errorf("code = %s, message = %s", res.ErrorCode, res.Error)
	}
}

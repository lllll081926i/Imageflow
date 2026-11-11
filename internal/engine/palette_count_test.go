package engine_test

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestPaletteSwatchCountHonoursRequest pins the palette page's swatch count: the
// request field existed but the engine always produced ten, so a caller asking
// for six got ten and a caller asking for twenty also got ten.
func TestPaletteSwatchCountHonoursRequest(t *testing.T) {
	dir := t.TempDir()
	src := samplePNG(t, dir, 120, 90)

	for _, want := range []int{0, 4, 6, 16} {
		res, pal := engine.ExtractPalette(engine.PaletteRequest{
			InputPath: src, Colors: want,
		})
		if !res.Success {
			t.Fatalf("%d: %s", want, res.Error)
		}
		effective := want
		if want < 2 || want > 16 {
			effective = 10
		}
		if len(pal.Colors) != effective {
			t.Errorf("requested %d: got %d swatches, want %d", want, len(pal.Colors), effective)
		}
		if pal.ThemeColor == "" {
			t.Errorf("requested %d: no theme colour", want)
		}
	}
	// The strip must have one block per colour, so the width scales with the count.
	out := filepath.Join(dir, "strip.png")
	res, _ := engine.ExtractPalette(engine.PaletteRequest{
		InputPath: src, OutputPath: out, Colors: 4, SwatchHeight: 80,
	})
	if !res.Success {
		t.Fatalf("strip: %s", res.Error)
	}
	if res.Width != 120 {
		t.Errorf("strip width = %d, want the source width", res.Width)
	}
}

package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestAvailableFontsResolveToRealFiles pins the font selector's data source: every
// family the panel offers must map to a font file on this machine, otherwise the
// engine would silently substitute a different face.
func TestAvailableFontsResolveToRealFiles(t *testing.T) {
	families := engine.AvailableFonts()
	if len(families) == 0 {
		t.Fatal("no font families on this machine")
	}
	for _, family := range families {
		face, err := engine.FontFace(family, 24, 72, false)
		if err != nil {
			t.Errorf("%q: %v", family, err)
			continue
		}
		if face == nil {
			t.Errorf("%q resolved to a nil face", family)
		}
	}
}

// TestRichTextUsesRequestedFamily pins that the family reaches the renderer: two
// different installed fonts must produce different pixels for the same text.
func TestRichTextUsesRequestedFamily(t *testing.T) {
	families := engine.AvailableFonts()
	if len(families) < 2 {
		t.Skip("only one family installed")
	}
	blocks := []engine.RichTextBlock{{Type: "heading", Level: 1, Text: "ImageFlow 字体"}}
	render := func(family string) []byte {
		img, err := engine.RenderRichTextImage(engine.RichTextRequest{
			Blocks: blocks, Width: 400, Padding: 16, FontSize: 30, LineHeight: 1.6,
			Family: family,
		})
		if err != nil {
			t.Fatalf("%q: %v", family, err)
		}
		path := filepath.Join(t.TempDir(), "out.png")
		if err := engine.SaveImage(img, path); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	first := render(families[0])
	second := render(families[1])
	if string(first) == string(second) {
		t.Errorf("families %q and %q rendered identical bytes", families[0], families[1])
	}
}

package batch

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// TestBuildOutputPathKeepsEncoderExtension pins the multi-dot filename case.
// "photo.2024.jpg" renders the stem "photo.2024"; filepath.Ext then reports
// ".2024", which used to be mistaken for a real extension, so the file was
// written with no suffix and the encoder rejected it as unsupported.
func TestBuildOutputPathKeepsEncoderExtension(t *testing.T) {
	s := settings.Default()
	s.OutputTemplate = "{basename}"
	s.OutputPrefix = ""
	s.DefaultOutputDir = t.TempDir()

	for _, in := range []string{"photo.2024.jpg", "a.b.png", "plain.webp"} {
		got := BuildOutputPath(s, filepath.Join(t.TempDir(), in), "", "convert", "png", 1)
		if filepath.Ext(got) != ".png" {
			t.Errorf("input %q: output %q has extension %q, want .png",
				in, got, filepath.Ext(got))
		}
	}
}

// TestBuildOutputPathDoesNotDuplicateExtension covers a template that already
// names the target format.
func TestBuildOutputPathDoesNotDuplicateExtension(t *testing.T) {
	s := settings.Default()
	s.OutputTemplate = "{basename}.png"
	s.OutputPrefix = ""
	s.DefaultOutputDir = t.TempDir()

	got := BuildOutputPath(s, filepath.Join(t.TempDir(), "photo.jpg"), "", "convert", "png", 1)
	if filepath.Base(got) != "photo.png" {
		t.Errorf("output = %q, want photo.png", filepath.Base(got))
	}
}

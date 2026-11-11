package batch_test

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// TestPreserveFolderStructureRebuildsTree pins the setting the settings page
// exposes: with it on, a file found in a sub-folder of a dropped folder lands in
// the same sub-folder under the output directory; with it off, every result sits
// flat in the output directory.
func TestPreserveFolderStructureRebuildsTree(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	s := settings.Default()
	s.DefaultOutputDir = out
	input := filepath.Join(t.TempDir(), "trip", "day2", "photo.png")
	rel := filepath.Join("day2", "photo.png")

	s.PreserveFolderStructure = true
	if got, want := batch.OutputDirFor(s, input, rel), filepath.Join(out, "day2"); got != want {
		t.Errorf("preserved dir = %q, want %q", got, want)
	}
	got := batch.BuildOutputPath(s, input, rel, "convert", "png", 1)
	if want := filepath.Join(out, "day2", "IFphoto.png"); got != want {
		t.Errorf("preserved path = %q, want %q", got, want)
	}

	s.PreserveFolderStructure = false
	if got := batch.OutputDirFor(s, input, rel); got != out {
		t.Errorf("flat dir = %q, want %q", got, out)
	}
}

// TestPreserveFallsBackToSourceFolder pins that without a configured output
// directory the results still land next to the sources, in the same subtree.
func TestPreserveFallsBackToSourceFolder(t *testing.T) {
	src := filepath.Join(t.TempDir(), "trip")
	s := settings.Default()
	input := filepath.Join(src, "day2", "photo.png")
	got := batch.OutputDirFor(s, input, filepath.Join("day2", "photo.png"))
	if want := filepath.Join(src, "day2"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

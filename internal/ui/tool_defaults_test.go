package ui

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// TestConvertToolKeepsAspectByDefault pins the switch semantics of the format
// page: the aspect lock starts on (as the original app had it), and unlocking it
// really stretches the image instead of being a dead control.
func TestConvertToolKeepsAspectByDefault(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 0
	tool := a.ActiveTool().(*ConvertTool)

	if !tool.maintain {
		t.Fatal("aspect lock must default to on after the first frame")
	}

	tool.resize = 2
	tool.width = 60
	tool.height = 60
	tool.maintain = true
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	row := a.Session.Snapshot()[0]
	locked, err := codec.Open(row.Output)
	if err != nil {
		t.Fatal(err)
	}
	// 120x90 fitted into 60x60 keeps the ratio: 60x45.
	if locked.Width != 60 || locked.Height != 45 {
		t.Fatalf("locked resize = %dx%d, want 60x45", locked.Width, locked.Height)
	}

	a = newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 0
	tool = a.ActiveTool().(*ConvertTool)
	tool.resize = 2
	tool.width = 60
	tool.height = 60
	tool.maintain = false
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	row = a.Session.Snapshot()[0]
	stretched, err := codec.Open(row.Output)
	if err != nil {
		t.Fatal(err)
	}
	if stretched.Width != 60 || stretched.Height != 60 {
		t.Fatalf("unlocked resize = %dx%d, want 60x60", stretched.Width, stretched.Height)
	}
}

// TestToolDefaultsMatchOriginalPanels pins the three switches whose zero value
// differed from the original app's initial state.
func TestToolDefaultsMatchOriginalPanels(t *testing.T) {
	a := newTestApp(t)

	gif := a.Tools[10].(*GIFTool)
	gif.ensure()
	if !gif.maintain {
		t.Error("GIF resize must keep the aspect ratio by default")
	}

	pal := a.Tools[7].(*PaletteTool)
	pal.ensure()
	if !pal.includeText {
		t.Error("the palette strip must label its HEX values by default")
	}

	pdf := a.Tools[9].(*PDFTool)
	pdf.ensure()
	if pdf.orientation != 0 {
		t.Error("the PDF page must start portrait")
	}
}

// TestPreserveFolderStructureOutput pins the settings switch end to end: a file
// found in a sub-folder lands in the same sub-folder under the output directory.
func TestPreserveFolderStructureOutput(t *testing.T) {
	a := newTestApp(t)
	out := t.TempDir()
	s := a.Session.Settings
	s.DefaultOutputDir = out
	s.OutputTemplate = "{basename}"
	s.PreserveFolderStructure = true
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	a.resetPlan()
	item := batch.Item{
		Path:    filepath.Join(t.TempDir(), "trip", "day2", "photo.png"),
		RelPath: filepath.Join("day2", "photo.png"),
	}
	got, skipped := a.outputPathFor(item, "convert", "png", 1)
	want := filepath.Join(out, "day2", "photo.png")
	if skipped || got != want {
		t.Fatalf("got %q skipped=%v, want %q", got, skipped, want)
	}

	s.PreserveFolderStructure = false
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	a.resetPlan()
	if flat, skipped := a.outputPathFor(item, "convert", "png", 1); skipped || flat != filepath.Join(out, "photo.png") {
		t.Fatalf("flat output = %q skipped=%v", flat, skipped)
	}
	_ = settings.Default()
}

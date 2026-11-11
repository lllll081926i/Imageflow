package ui

import (
	"path/filepath"
	"testing"
)

// TestPendingSettingsPicksUpFolderHistory pins that a folder recorded by a
// background callback (a drop, or the output-directory dialog) reaches the
// editable settings on the next frame, without the callback writing the struct
// the view reads.
func TestPendingSettingsPicksUpFolderHistory(t *testing.T) {
	a := newTestApp(t)
	in := filepath.Join(t.TempDir(), "trip")
	a.rememberInputDir(in)

	if len(a.Settings.RecentInputDirs) != 0 {
		t.Fatalf("the background write must not touch the editable settings: %v",
			a.Settings.RecentInputDirs)
	}
	a.applyPendingSettings()
	if len(a.Settings.RecentInputDirs) != 1 || a.Settings.RecentInputDirs[0] != in {
		t.Fatalf("recent inputs = %v, want [%s]", a.Settings.RecentInputDirs, in)
	}

	// An unsaved edit to another field must survive the merge.
	a.Settings.OutputPrefix = "EDITED"
	out := filepath.Join(t.TempDir(), "out")
	next := a.Session.Settings
	next.DefaultOutputDir = out
	if err := a.Session.SetSettings(next); err != nil {
		t.Fatal(err)
	}
	a.applyPendingSettings()
	if a.Settings.DefaultOutputDir != out {
		t.Errorf("output dir = %q, want %q", a.Settings.DefaultOutputDir, out)
	}
	if a.Settings.OutputPrefix != "EDITED" {
		t.Errorf("unsaved edit was lost: prefix = %q", a.Settings.OutputPrefix)
	}
}

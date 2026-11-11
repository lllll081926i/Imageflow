package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", filepath.Join(dir, "settings.json"))
	got := Load()
	if got.OutputPrefix != "IF" || got.ConflictStrategy != ConflictRename {
		t.Fatalf("defaults = %+v", got)
	}
	if got.MaxConcurrency < 1 || got.MaxConcurrency > 32 {
		t.Errorf("concurrency = %d", got.MaxConcurrency)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "settings.json")
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", path)
	in := Default()
	in.OutputPrefix = "IMG"
	in.OutputTemplate = "{prefix}_{basename}_{seq:3}"
	in.ConflictStrategy = ConflictOverwrite
	in.MaxConcurrency = 7
	in.DefaultOutputDir = filepath.Join(dir, "out")
	if err := Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := Load()
	if got.OutputPrefix != "IMG" || got.MaxConcurrency != 7 || got.ConflictStrategy != ConflictOverwrite {
		t.Fatalf("round trip = %+v", got)
	}
	if got.OutputTemplate != "{prefix}_{basename}_{seq:3}" {
		t.Errorf("template = %q", got.OutputTemplate)
	}
}

func TestLoadNormalizesOutOfRangeValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", path)
	raw := `{"max_concurrency": 999, "conflict_strategy": "wat", "output_prefix": "", "output_template": ""}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.MaxConcurrency < 1 || got.MaxConcurrency > 32 {
		t.Errorf("concurrency not clamped: %d", got.MaxConcurrency)
	}
	if got.ConflictStrategy != ConflictRename {
		t.Errorf("strategy = %q", got.ConflictStrategy)
	}
	if got.OutputPrefix != "IF" {
		t.Errorf("prefix = %q", got.OutputPrefix)
	}
	if got.OutputTemplate == "" {
		t.Error("template was left empty")
	}
}

func TestLoadSurvivesCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", path)
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.OutputPrefix != "IF" {
		t.Fatalf("corrupt file did not fall back to defaults: %+v", got)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", path)
	if err := Save(Default()); err != nil {
		t.Fatal(err)
	}
	// No temporary file may survive a successful save.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "settings.json" {
			t.Errorf("leftover file: %s", e.Name())
		}
	}
}

func TestPushRecentDedupesAndCaps(t *testing.T) {
	got := []string{}
	for _, v := range []string{"/a", "/b", "/c", "/d", "/e"} {
		got = PushRecent(got, v)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, %v", len(got), got)
	}
	// filepath.Clean rewrites "/e" to "\\e" on Windows, so compare on the
	// cleaned form rather than the raw input.
	wantFirst := filepath.Clean("/e")
	if got[0] != wantFirst {
		t.Errorf("most recent first: %v, want %q", got, wantFirst)
	}
	again := PushRecent(got, "/c")
	if len(again) != 4 || again[0] != filepath.Clean("/c") {
		t.Errorf("dedupe failed: %v", again)
	}
	if PushRecent(got, "   ") == nil {
		t.Error("blank push must keep the list")
	}
}

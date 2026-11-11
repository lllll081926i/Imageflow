package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	appui "github.com/lllll081926i/imageflow-go/internal/ui"
)

// newTestApp builds a shell that runs without a window.
func newTestApp(t *testing.T) *appui.App {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", filepath.Join(dir, "settings.json"))
	a := appui.NewApp(settings.Default())
	return a
}

func TestShellRendersEveryTool(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	for i := range a.Tools {
		a.Active = i
		a.ShowSettings = false
		tt.Frame()
		if len(tt.Texts()) == 0 {
			t.Fatalf("tool %d rendered no text", i)
		}
		if !tt.HasText(a.Tools[i].Name()) {
			t.Errorf("tool %d: %q missing from %v", i, a.Tools[i].Name(), tt.Texts())
		}
	}
}

func TestToolNavigation(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	if err := tt.Click("批量水印"); err != nil {
		t.Fatalf("click watermark: %v", err)
	}
	if got := a.ActiveTool().Name(); got != "批量水印" {
		t.Errorf("active tool = %q", got)
	}
	if !tt.HasText("水印") {
		t.Errorf("watermark page missing: %v", tt.Texts())
	}
	if err := tt.Click("全局设置"); err != nil {
		t.Fatalf("click settings: %v", err)
	}
	if !a.ShowSettings {
		t.Error("settings page did not open")
	}
}

func TestAddPathsPopulatesSession(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	src := writeSamplePNG(t, filepath.Join(dir, "a.png"))
	added := a.AddPaths([]string{src})
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	rows := a.Session.Snapshot()
	if len(rows) != 1 || rows[0].Path != src {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Width != 600 || rows[0].Height != 40 {
		t.Errorf("probed size = %dx%d, want 600x40", rows[0].Width, rows[0].Height)
	}
	if a.AddPaths([]string{src}) != 0 {
		t.Error("duplicate add should not grow the list")
	}
}

func writeSamplePNG(t *testing.T, path string) string {
	t.Helper()
	img, err := engine.PaletteImage([]string{"#112233", "#445566"}, 40)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveImage(img, path); err != nil {
		t.Fatal(err)
	}
	return path
}

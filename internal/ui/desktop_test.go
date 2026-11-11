package ui

import (
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/batch"
)

func TestOutputFolderFallsBackToFirstInput(t *testing.T) {
	a := newTestApp(t)
	s := a.Session.Settings
	s.DefaultOutputDir = ""
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	a.Session.Mu.Lock()
	a.Session.Items = []*batch.Item{{Path: filepath.Join("in", "dir", "a.png")}}
	a.Session.Mu.Unlock()

	if got := a.outputFolder(); got != filepath.Join("in", "dir") {
		t.Fatalf("outputFolder = %q, want the first input's directory", got)
	}
}

func TestOutputFolderPrefersConfigured(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	s := a.Session.Settings
	s.DefaultOutputDir = dir
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	if got := a.outputFolder(); got != dir {
		t.Fatalf("outputFolder = %q, want %q", got, dir)
	}
}

func TestQuitFlagsAreGuarded(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	quitting := a.quitting
	a.mu.Unlock()
	if quitting {
		t.Error("a fresh app must not be quitting")
	}
	a.SetQuitWithoutTray()
	a.mu.Lock()
	without := a.quitWithoutTray
	a.mu.Unlock()
	if !without {
		t.Error("SetQuitWithoutTray did not stick")
	}
}

// TestCloseHidesOnlyWhenBackgroundRunning pins the close behaviour: with the
// setting off the window closes and the app quits, with it on the window hides.
func TestCloseHidesOnlyWhenBackgroundRunning(t *testing.T) {
	a := newTestApp(t)
	if a.ShouldHideOnClose() {
		t.Error("background running is off by default, so close must quit")
	}

	on := a.Session.Settings
	on.BackgroundRunning = true
	if err := a.Session.SetSettings(on); err != nil {
		t.Fatal(err)
	}
	if !a.ShouldHideOnClose() {
		t.Error("background running is on, so close must hide")
	}

	// A desktop without a tray must never hide the window.
	a.SetQuitWithoutTray()
	if a.ShouldHideOnClose() {
		t.Error("without a tray, close must quit")
	}
}

func TestTrayIconIsAPNG(t *testing.T) {
	png := trayIconPNG()
	if len(png) < 100 {
		t.Fatalf("tray icon is %d bytes", len(png))
	}
	if string(png[1:4]) != "PNG" {
		t.Errorf("tray icon is not a PNG: % x", png[:8])
	}
}

func TestUpdateBadge(t *testing.T) {
	a := newTestApp(t)
	if a.updateBadge() != "" {
		t.Error("badge shown before any check")
	}
	a.mu.Lock()
	a.update.Version = "1.2.3"
	a.mu.Unlock()
	if a.updateBadge() != "v1.2.3" {
		t.Errorf("badge = %q", a.updateBadge())
	}
}

package ui

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// seedInputs writes n distinct PNGs into dir/out and points the default output
// directory at dir/out, so each tool writes where the test can find it.
func seedInputs(t *testing.T, a *App, n int) []string {
	t.Helper()
	inDir := t.TempDir()
	outDir := t.TempDir()
	var paths []string
	for i := 0; i < n; i++ {
		img := image.NewNRGBA(image.Rect(0, 0, 120, 90))
		for y := 0; y < 90; y++ {
			for x := 0; x < 120; x++ {
				img.SetNRGBA(x, y, color.NRGBA{
					R: uint8(x + i*20), G: uint8(y + i*10), B: uint8(60 + i*30), A: 255,
				})
			}
		}
		p := filepath.Join(inDir, fmt.Sprintf("img%02d.png", i))
		if err := codec.Save(img, p, codec.SaveOptions{}); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	next := a.Session.Settings
	next.DefaultOutputDir = outDir
	next.MaxConcurrency = 2
	if err := a.Session.SetSettings(next); err != nil {
		t.Fatal(err)
	}
	a.applyPendingSettings()
	if added := a.AddPaths(paths); added != n {
		t.Fatalf("added %d of %d inputs", added, n)
	}
	return paths
}

// waitForRun blocks until the asynchronous batch finishes.
func waitForRun(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		a.Session.Mu.Lock()
		running := a.Session.Running
		a.Session.Mu.Unlock()
		if !running && !a.IsRunning() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("batch did not finish in time")
}

func statuses(a *App) []string {
	rows := a.Session.Snapshot()
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Status
	}
	return out
}

func assertAllOK(t *testing.T, a *App) {
	t.Helper()
	for i, s := range statuses(a) {
		if s != "ok" {
			rows := a.Session.Snapshot()
			t.Fatalf("row %d status = %q, message = %q", i, s, rows[i].Message)
		}
	}
}

// countFiles counts the regular files under dir.
func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunConvertEndToEnd(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 3)
	a.Active = 0
	tool := a.ActiveTool().(*ConvertTool)
	tool.format = "jpg"
	tool.quality = 88
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	outDir := a.Session.Settings.DefaultOutputDir
	if countFiles(t, outDir) != 3 {
		t.Fatalf("wrote %d files", countFiles(t, outDir))
	}
	for _, row := range a.Session.Snapshot() {
		if row.Output == "" {
			t.Error("row has no output path")
			continue
		}
		if filepath.Ext(row.Output) != ".jpg" {
			t.Errorf("output %q has the wrong extension", row.Output)
		}
		if _, err := os.Stat(row.Output); err != nil {
			t.Errorf("output missing: %v", err)
		}
	}
	if p, _ := a.ProgressState(); p < 99 {
		t.Errorf("progress = %v", p)
	}
}

func TestRunConvertWithResize(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 0
	tool := a.ActiveTool().(*ConvertTool)
	tool.format = "png"
	tool.resize = 1
	tool.percent = 50
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	row := a.Session.Snapshot()[0]
	opened, err := codec.Open(row.Output)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Width != 60 || opened.Height != 45 {
		t.Fatalf("resized to %dx%d, want 60x45", opened.Width, opened.Height)
	}
}

func TestRunResizeTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	a.Active = 1
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	for _, row := range a.Session.Snapshot() {
		opened, err := codec.Open(row.Output)
		if err != nil {
			t.Fatal(err)
		}
		if opened.Width != 120 {
			t.Errorf("width = %d", opened.Width)
		}
	}
}

func TestRunGridTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 3
	tool := a.ActiveTool().(*GridTool)
	tool.grid = 2
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	outDir := a.Session.Settings.DefaultOutputDir
	if n := countFiles(t, outDir); n != 4 {
		t.Fatalf("expected 4 tiles, found %d files", n)
	}
}

func TestRunLayoutTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 4
	tool := a.ActiveTool().(*LayoutTool)
	tool.rows, tool.cols = 1, 3
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	if n := countFiles(t, a.Session.Settings.DefaultOutputDir); n != 3 {
		t.Fatalf("expected 3 pieces, found %d", n)
	}
}

func TestRunStitchTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 3)
	a.Active = 5
	a.Start()
	waitForRun(t, a)
	rows := a.Session.Snapshot()
	for i, r := range rows {
		if r.Status != "ok" {
			t.Fatalf("row %d = %s (%s)", i, r.Status, r.Message)
		}
	}
	outputs := map[string]bool{}
	for _, r := range rows {
		outputs[r.Output] = true
	}
	if len(outputs) != 1 {
		t.Fatalf("stitch produced %d outputs, want 1: %v", len(outputs), outputs)
	}
	for p := range outputs {
		opened, err := codec.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		// Three 120x90 images stacked vertically with no gap.
		if opened.Height != 270 {
			t.Errorf("stitched height = %d, want 270", opened.Height)
		}
	}
}

func TestRunWatermarkTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	a.Active = 6
	tool := a.ActiveTool().(*WatermarkTool)
	tool.text = "TEST"
	tool.opacity = 100
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	for _, row := range a.Session.Snapshot() {
		if _, err := os.Stat(row.Output); err != nil {
			t.Errorf("watermark output missing: %v", err)
		}
	}
}

func TestRunPaletteTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 7
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	row := a.Session.Snapshot()[0]
	if row.Output == "" {
		t.Fatal("no palette image")
	}
	if _, err := os.Stat(row.Output); err != nil {
		t.Fatal(err)
	}
}

func TestRunPDFTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 2)
	a.Active = 9
	a.Start()
	waitForRun(t, a)
	rows := a.Session.Snapshot()
	for i, r := range rows {
		if r.Status != "ok" {
			t.Fatalf("row %d = %s (%s)", i, r.Status, r.Message)
		}
	}
	// A PDF is one document, so every row reports the same file.
	pdf := rows[0].Output
	if filepath.Ext(pdf) != ".pdf" {
		t.Fatalf("output = %q", pdf)
	}
	raw, err := os.ReadFile(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 1000 || string(raw[:5]) != "%PDF-" {
		t.Fatalf("not a valid pdf (%d bytes)", len(raw))
	}
}

func TestRunRichTextTool(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 11
	a.Start()
	waitForRun(t, a)
	assertAllOK(t, a)
	row := a.Session.Snapshot()[0]
	opened, err := codec.Open(row.Output)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Width != 800 {
		t.Errorf("canvas width = %d", opened.Width)
	}
}

func TestRunGIFToolRejectsStills(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 1)
	a.Active = 10
	a.Start()
	waitForRun(t, a)
	rows := a.Session.Snapshot()
	if rows[0].Status != "failed" {
		t.Fatalf("a still PNG should fail the GIF tool, got %q", rows[0].Status)
	}
	if rows[0].Message == "" {
		t.Error("failure has no message")
	}
}

func TestCancelStopsUnstartedRows(t *testing.T) {
	a := newTestApp(t)
	seedInputs(t, a, 8)
	a.Active = 0
	tool := a.ActiveTool().(*ConvertTool)
	tool.format = "png"
	a.Start()
	a.Cancel()
	waitForRun(t, a)
	var cancelled, ok int
	for _, s := range statuses(a) {
		switch s {
		case "cancelled":
			cancelled++
		case "ok":
			ok++
		}
	}
	// A cancelled row is never handed to a job, so it must be reported as such
	// rather than left blank. An AND of "no cancelled" and "empty status" used
	// to pass whenever the status line was merely non-empty.
	if cancelled == 0 {
		t.Fatal("cancel left no row marked 已取消")
	}
	if cancelled+ok != 8 {
		t.Errorf("cancelled %d + completed %d = %d, want 8 rows accounted for",
			cancelled, ok, cancelled+ok)
	}
	if !strings.Contains(a.StatusLine(), "取消") {
		t.Errorf("status line %q must report the cancellation", a.StatusLine())
	}
}

func TestSettingsSaveRoundTripThroughSession(t *testing.T) {
	a := newTestApp(t)
	next := a.Session.Settings
	next.OutputPrefix = "RT"
	next.ConflictStrategy = settings.ConflictSkip
	if err := a.Session.SetSettings(next); err != nil {
		t.Fatal(err)
	}
	if got := settings.Load(); got.OutputPrefix != "RT" || got.ConflictStrategy != settings.ConflictSkip {
		t.Fatalf("persisted = %+v", got)
	}
}

var _ = engine.OK

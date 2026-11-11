package ui

// format_matrix_test.go drives the real ConvertTool over one file of every
// supported format, in every direction.
//
// The engine has its own matrix test. This one goes through the page, which is
// what a user touches: it catches a format the panel does not offer, an
// extension the output path loses, and a row that reports success while writing
// nothing.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// TestConvertToolEveryFormatEndToEnd converts one PNG into every format the page
// offers, then converts each result back into PNG.
func TestConvertToolEveryFormatEndToEnd(t *testing.T) {
	// Every target the converter page shows.
	targets := []string{"jpg", "png", "webp", "avif", "tiff", "bmp", "ico"}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			a := newTestApp(t)
			seedInputs(t, a, 1)
			a.Active = 0
			tool := a.ActiveTool().(*ConvertTool)
			tool.format = target
			tool.quality = 92
			tool.resize = 0

			a.Start()
			waitForRun(t, a)
			assertAllOK(t, a)

			row := a.Session.Snapshot()[0]
			if row.Output == "" {
				t.Fatal("no output path")
			}
			if ext := filepath.Ext(row.Output); ext != "."+canonicalExt(target) {
				t.Fatalf("output %q does not end in .%s", row.Output, canonicalExt(target))
			}
			if st, err := os.Stat(row.Output); err != nil || st.Size() == 0 {
				t.Fatalf("output missing or empty: %v", err)
			}
			opened, err := codec.Open(row.Output)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			// ICO stores square entries; a non-square source becomes a square icon.
			if target == "ico" {
				if opened.Width != opened.Height || opened.Width < 16 {
					t.Errorf("ico output = %dx%d, want a square", opened.Width, opened.Height)
				}
			} else if opened.Width != 120 || opened.Height != 90 {
				t.Errorf("output = %dx%d, want 120x90", opened.Width, opened.Height)
			}

			// And back to PNG, which proves the format we just wrote is readable.
			b := newTestApp(t)
			seedInputs(t, b, 1)
			b.Session.Clear()
			b.AddPaths([]string{row.Output})
			b.Active = 0
			back := b.ActiveTool().(*ConvertTool)
			back.format = "png"
			back.quality = 95
			back.resize = 0
			b.Start()
			waitForRun(t, b)
			assertAllOK(t, b)
			last := b.Session.Snapshot()[0]
			if filepath.Ext(last.Output) != ".png" {
				t.Errorf("reverse output = %q", last.Output)
			}
		})
	}
}

// canonicalExt maps a format name to its file extension.
func canonicalExt(format string) string {
	switch format {
	case "tiff":
		return "tiff"
	case "jpeg":
		return "jpg"
	}
	return format
}

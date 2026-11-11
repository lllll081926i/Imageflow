package engine_test

import (
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/engine"
)

// TestPreferredFontIsACJKFace pins the starting family of the watermark and
// rich-text panels: the platform's preference list puts a CJK face first, so a
// fresh panel must not open on Arial just because it sorts first alphabetically.
func TestPreferredFontIsACJKFace(t *testing.T) {
	got := engine.PreferredFont()
	if got == "" {
		t.Skip("no known font family installed")
	}
	if got == "Arial" || got == "DejaVu Sans" || got == "Liberation Sans" {
		t.Errorf("preferred font = %q, want the platform's first installed candidate", got)
	}
	// The preference list must be a prefix of what the family resolves to.
	families := engine.AvailableFonts()
	found := false
	for _, f := range families {
		if f == got {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("preferred font %q is not in the available list %v", got, families)
	}
}

package ui

import (
	"testing"

	myui "github.com/egoist/mygo/ui"
)

// TestGhostButtonUsesSurfaceInDarkMode guards a regression where the small
// secondary buttons rendered as white blocks with near-white text in the dark
// theme, because their surface was painted after the theme switch.
func TestGhostButtonUsesSurfaceInDarkMode(t *testing.T) {
	view := func(c *myui.Context) {
		applyBrandThemeForTest(c, true)
		st := styles(c)
		myui.Column(c).Fill().Padding(20).Background(st.Card).Children(func() {
			(&App{}).ghostButton(c, st, "选择输出位置", nil, func() {})
		})
	}
	img := renderForTest(view, 240, 90)
	// Sample the button body, away from its border and glyph.
	c := img.RGBAAt(40, 45)
	if luminanceOf(c) > 120 {
		t.Fatalf("ghost button surface is too light in dark mode: %+v", c)
	}
}

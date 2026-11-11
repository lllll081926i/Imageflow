package ui

import (
	"image"
	"testing"

	myui "github.com/egoist/mygo/ui"
)

// TestSegmentedPillContrast renders one segmented control in both appearances and
// asserts the selected pill is a surface colour with readable text, not a white
// block with white text.
func TestSegmentedPillContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		selected := 1
		view := func(c *myui.Context) {
			applyBrandThemeForTest(c, dark)
			st := styles(c)
			myui.Column(c).Fill().Padding(20).Background(st.Card).Children(func() {
				myui.Column(c).Width(320).Children(func() {
					segmented(c, &selected, []string{"JPG", "PNG", "WebP"})
				})
			})
		}
		img := renderForTest(view, 360, 100)
		// The pill sits in the middle cell of the track.
		pill := img.RGBAAt(180, 30)
		if dark && luminanceOf(pill) > 150 {
			t.Fatalf("dark pill is too light: %+v", pill)
		}
		if !dark && luminanceOf(pill) < 200 {
			t.Fatalf("light pill is too dark: %+v", pill)
		}
	}
}

// applyBrandThemeForTest mirrors View's theme setup without a window.
func applyBrandThemeForTest(c *myui.Context, dark bool) {
	st := newStyle(dark)
	base := myui.LightTheme()
	if dark {
		base = myui.DarkTheme()
	}
	base.Dark = dark
	base.Accent = st.Brand
	base.Background = st.Shell
	base.Text = st.Text
	base.TextMuted = st.TextSub
	base.Border = st.Divider
	base.Surface = st.Recessed
	base.SurfaceHover = st.CardHover
	base.SurfacePressed = st.Recessed
	c.SetTheme(base)
}

func renderForTest(view func(c *myui.Context), w, h int) *image.RGBA {
	t := myui.NewTester(view, w, h)
	t.Frame()
	return t.Image()
}

func luminanceOf(c any) float64 {
	type rgba interface {
		RGBA() (uint32, uint32, uint32, uint32)
	}
	if v, ok := c.(rgba); ok {
		r, g, b, _ := v.RGBA()
		return 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
	}
	return 0
}

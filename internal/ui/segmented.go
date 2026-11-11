package ui

// segmented.go is the design system's switch: a recessed track with a raised
// pill that slides between segments.
//
// MyGo's built-in Segmented repaints the pill at its new place with no travel
// in between, so a switch reads as a jump. This one keeps its own selection
// position and animates it with a spring, which is what the original's
// `.spring-indicator` did.

import (
	"time"

	myui "github.com/egoist/mygo/ui"
)

// segmentSlide is the curve the indicator rides between segments.
//
// A spring overshoots a narrow track and makes the pill look like it is
// overshooting its cell; a short eased slide reads as one control moving.
var segmentSlide = myui.Animation{
	Duration: 180 * time.Millisecond,
	Ease:     myui.EaseOut,
}

// segmentedSwitch draws the switch and reports the chosen index.
//
// The indicator is painted under the labels: the track's own background is the
// recessed surface, and one raised pill carries the selection from segment to
// segment instead of each cell painting and unpainting itself.
func segmentedSwitch(c *myui.Context, selected *int, options []string) {
	st := styles(c)
	if len(options) == 0 {
		return
	}
	// Clamp before anything is measured, so a stale index cannot place the pill
	// outside the track.
	if *selected < 0 || *selected >= len(options) {
		*selected = 0
	}

	track := myui.Row(c).FillWidth().Padding(3).Radius(st.RadiusControl).
		Background(st.Recessed).Label(options[*selected])
	// The travel is measured in segment units, not pixels: the pill's width is only
	// known when the track is painted, so the animated value is the position along
	// the track and the painter turns it into pixels. AnimateTo must be called
	// during the build, and its value drives this frame's paint.
	pos := float32(*selected)
	if !motionReduced {
		// The travel is measured in segment units, not pixels: the pill's width is
		// only known when the track is painted, so the animated value is the
		// position along the track and the painter turns it into pixels. AnimateTo
		// must be called during the build, and its value drives this frame's paint.
		// With reduced motion the value is taken as final, so nothing keeps
		// asking for frames.
		pos = track.AnimateTo("segmented-pos", float32(*selected), segmentSlide)
	}
	// Draw paints before the children, so the pill sits under the labels; DrawOver
	// would cover them.
	track.Draw(func(p *myui.Painter, r myui.Rect) {
		n := float32(len(options))
		pill := (r.W - 6) / n
		if pill <= 0 {
			return
		}
		x := r.X + 3 + pos*pill
		p.Fill(myui.Rect{X: x, Y: r.Y + 3, W: pill, H: r.H - 6}, st.Card, st.RadiusPill)
		p.Stroke(myui.Rect{X: x, Y: r.Y + 3, W: pill, H: r.H - 6},
			st.Divider, st.RadiusPill, 1)
	})

	track.Children(func() {
		for i, option := range options {
			index := i
			active := *selected == index
			cell := myui.ButtonBase(c).Grow(1).Height(30).Radius(st.RadiusPill).
				AlignItems(myui.Center).Justify(myui.Center).
				Cursor(myui.CursorPointer).Label(option).
				FocusRingColor(st.Brand).FocusRingOffset(1)
			cell.Children(func() {
				text := myui.Text(c, option).FontSize(12)
				if active {
					text.FontWeight(600).TextColor(st.Text)
				} else {
					text.TextColor(st.TextSub)
				}
			})
			if !active && cell.Pressed() {
				cell.Opacity(0.65)
			} else if !active && cell.Hovered() {
				cell.Opacity(0.85)
			}
			if cell.Clicked() {
				*selected = index
			}
		}
	})
}

package ui

// dashed.go paints the drop target's dashed frame.
//
// MyGo's BorderDashed draws dashes about three times as long as the gaps and
// leaves the rounded corners sparse, which on a 900 DIP tall panel reads as a
// broken line. This draws its own frame: even dashes around the whole perimeter,
// corners included, so the target looks deliberate at any size.

import (
	"math"

	myui "github.com/egoist/mygo/ui"
)

// dashedFrame paints a dashed rounded rectangle inside r.
//
// The dashes are laid along the perimeter, so their count grows with the panel
// instead of the pattern stretching: a 200 DIP box and a 900 DIP one show the same
// dash.
func dashedFrame(p *myui.Painter, r myui.Rect, radius, width float32, c myui.Color) {
	const dash, gap = 5, 4
	if r.W <= 2*radius+1 || r.H <= 2*radius+1 {
		p.Stroke(r, c, radius, width)
		return
	}
	arc := math.Pi * float64(radius) / 2
	// Segment length along the four straight edges plus the four corner arcs.
	perimeter := 2*(float64(r.W)-2*float64(radius)) + 2*(float64(r.H)-2*float64(radius)) + 4*arc
	step := float64(dash + gap)
	count := int(perimeter / step)
	if count < 8 {
		count = 8
	}
	step = perimeter / float64(count)

	// pointAt walks the perimeter from the top-left corner's end, clockwise.
	pointAt := func(s float64) (float32, float32) {
		straightX := float64(r.W) - 2*float64(radius)
		straightY := float64(r.H) - 2*float64(radius)
		// top edge
		if s < straightX {
			return r.X + radius + float32(s), r.Y
		}
		s -= straightX
		// top-right arc
		if s < arc {
			a := s / arc * (math.Pi / 2)
			return r.X + r.W - radius + float32(math.Sin(a))*radius,
				r.Y + radius - float32(math.Cos(a))*radius
		}
		s -= arc
		// right edge
		if s < straightY {
			return r.X + r.W, r.Y + radius + float32(s)
		}
		s -= straightY
		// bottom-right arc
		if s < arc {
			a := s / arc * (math.Pi / 2)
			return r.X + r.W - radius + float32(math.Cos(a))*radius,
				r.Y + r.H - radius + float32(math.Sin(a))*radius
		}
		s -= arc
		// bottom edge
		if s < straightX {
			return r.X + r.W - radius - float32(s), r.Y + r.H
		}
		s -= straightX
		// bottom-left arc
		if s < arc {
			a := s / arc * (math.Pi / 2)
			return r.X + radius - float32(math.Sin(a))*radius,
				r.Y + r.H - radius + float32(math.Cos(a))*radius
		}
		s -= arc
		// left edge
		if s < straightY {
			return r.X, r.Y + r.H - radius - float32(s)
		}
		s -= straightY
		// top-left arc
		a := s / arc * (math.Pi / 2)
		return r.X + radius - float32(math.Cos(a))*radius,
			r.Y + radius - float32(math.Sin(a))*radius
	}

	// Each dash is a short polyline; sampling every 1.5 DIP keeps the corner
	// arcs round at any radius.
	for i := 0; i < count; i++ {
		from := float64(i) * step
		to := from + float64(dash)
		segments := int(math.Ceil(float64(dash) / 1.5))
		if segments < 2 {
			segments = 2
		}
		pts := make([][2]float32, 0, segments+1)
		for k := 0; k <= segments; k++ {
			x, y := pointAt(from + (to-from)*float64(k)/float64(segments))
			pts = append(pts, [2]float32{x, y})
		}
		p.StrokePolyline(pts, width, c)
	}
}

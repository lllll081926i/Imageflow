package ui

import (
	"fmt"
	"strings"
	"time"

	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

// controls.go holds the small building blocks every page reuses. All of them
// read docs/DESIGN.md's tokens: one brand blue, the neutral surface ladder, and
// the spring motion presets.

// A MyGo Transition with no property flags set animates *all* of them:
// position, size, colours and shadows. That is the opposite of "just a fade".
// Setting Shadows alone is how this shell spells "animate nothing about the
// layout box": the framework then follows the layout on the frame it changes
// instead of easing it a second time. Enter/Exit still play, because those are
// driven by the Motion, not by the property flags.
//
// This matters most while the rail's width is animated. The parent Row feeds
// the animated width into the layout every frame; if a child Transition also
// animates its own x and w, the child trails the rail with a different curve.
// The cards used to lag by more than 150 DIP mid-collapse.
var (
	// springTap is the press feedback every clickable surface uses. The press
	// itself is an immediate Scale transform; the Transition adds a soft shadow
	// fade on hover/press, and deliberately animates nothing about the layout
	// box. Size/Position used to be set here, which made every button ease its
	// own width while the rail was collapsing.
	springTap = myui.ElementTransition{
		Animation: &snappySpring,
		Shadows:   true,
	}
	// springSoft is the quieter curve for content that swaps inside a card.
	// An eased curve, not a spring: a spring leaves rest at full speed, which is
	// what made a panel that mostly fades start with a jerk.
	springSoft = myui.ElementTransition{
		Animation: &smoothCurve,
		Shadows:   true,
		Enter:     &myui.Motion{Y: 4, Opacity: 0, Scale: 0.995},
		Exit:      &myui.Motion{Opacity: 0},
	}
)

// instantTransition is what "减弱动效" must return. A zero Transition is not
// "no animation": with no flags it animates every property over 200 ms. A
// negative Duration is MyGo's documented "show every change, Enter and Exit at
// once", which is exactly a reduced-motion frame.
var instantTransition = myui.ElementTransition{Duration: -1, Shadows: true}

// snappySpring keeps the press bounce; smoothCurve is the eased fade used for
// content that swaps in place.
var (
	snappySpring = myui.Animation{
		Spring: myui.SpringOptions{Duration: 180 * time.Millisecond, Bounce: 0.08},
	}
	smoothCurve = myui.Animation{
		Duration: 200 * time.Millisecond,
		Ease:     myui.EaseOut,
	}
	controlCurve = myui.Animation{
		Duration: 140 * time.Millisecond,
		Ease:     myui.EaseOut,
	}
	// shellCurve is the one curve the rail's width, the page handover and the
	// selection pill's fade share. The rail feeds its animated width into the
	// layout every frame, so the cards beside it move on that frame; anything
	// that fades during the same gesture must use the same duration and ease or
	// the two halves of the window settle at different times.
	shellCurve = myui.Animation{
		Duration: 220 * time.Millisecond,
		Ease:     myui.EaseInOut,
	}
)

// motionReduced mirrors the 减弱动效 setting for the current frame. The design
// contract turns every animation off when it is on, but the transitions are
// built while the view is composed, before the frame knows which page it is
// drawing, so the flag is set once by the root view. It is only read and written
// from the render pass, which runs on the single UI thread.
var motionReduced bool

// motion returns a transition preset, or an instant one when the user asked for
// reduced motion.
func motion(preset myui.ElementTransition) myui.ElementTransition {
	if motionReduced {
		return instantTransition
	}
	return preset
}

func label(c *myui.Context, text string) {
	st := styles(c)
	myui.Text(c, text).FontSize(12).FontWeight(500).TextColor(st.TextSub)
}

func textField(c *myui.Context, value *string, placeholder string) {
	textFieldState(c, value, placeholder, false)
}

// textFieldState adds the same error treatment to every text input without
// changing its binding or validation rules.
func textFieldState(c *myui.Context, value *string, placeholder string, invalid bool) {
	input := myui.TextInput(c, value).FillWidth().Height(38).PaddingX(12).
		Radius(styles(c).RadiusControl).FontSize(13).Placeholder(placeholder)
	styleFieldInput(c, input, invalid)
}

// styleFieldInput is shared by text, number, multiline, select and colour
// fields. Lobe's visual order is retained: neutral fill, stronger hover border,
// accent focus ring, and a semantic error ring. Disabled also blocks input at
// the framework level rather than merely painting the control grey.
func styleFieldInput(c *myui.Context, input myui.Element, invalid bool) {
	st := styles(c)
	input.FocusRing(false).PlaceholderColor(st.TextDim).Invalid(invalid)
	switch {
	case input.IsDisabled():
		input.Cursor(myui.CursorNotAllowed).Background(st.Recessed).
			Border(1, st.Divider).TextColor(st.TextDim)
	case invalid:
		input.Background(st.Card).Border(1, st.Danger)
		if input.Focused() {
			input.Shadow(0, 0, 0, 2, myui.RGBA(st.Danger.R, st.Danger.G, st.Danger.B, 0.16))
		}
	case input.Focused():
		input.Background(st.Card).Border(1, st.Brand).
			Shadow(0, 0, 0, 2, myui.RGBA(st.Brand.R, st.Brand.G, st.Brand.B, 0.14))
	case input.Hovered():
		input.Background(st.Card).Border(1, st.TextDim)
	default:
		input.Background(st.Recessed).Border(1, st.Divider)
	}
}

func numberField(c *myui.Context, value *float64, lo, hi, step float64, placeholder string) {
	input := myui.NumberInput(c, value, lo, hi, step).FillWidth().Height(38).PaddingX(12).
		Radius(styles(c).RadiusControl).FontSize(13).Placeholder(placeholder)
	styleFieldInput(c, input, false)
}

func multiLine(c *myui.Context, value *string, minRows, maxRows int, placeholder string) {
	input := myui.TextArea(c, value).FillWidth().MinHeight(float32(minRows) * 20).
		Padding(10).Radius(styles(c).RadiusControl).FontSize(13).Placeholder(placeholder)
	styleFieldInput(c, input, false)
}

// slider is the design system's continuous slider.
//
// It is built on SliderBase rather than Slider so the track is drawn here: the
// framework's track takes the theme border colour, which on this surface reads
// as a grey bar that only brightens on hover. The filled part is brand blue in
// every state, and the track is the recessed step of the surface ladder.
func slider(c *myui.Context, value *float64, lo, hi, step float64) {
	st := styles(c)
	if step > 0 {
		// Snap first, so the knob and the number agree on the first frame.
		*value = lo + roundTo(*value-lo, step)
	}
	track := myui.SliderBase(c, value, lo, hi).FillWidth().Height(20).
		PaddingX(7).Cursor(myui.CursorPointer).FocusRing(true).
		FocusRingColor(st.Brand).FocusRingOffset(2)
	track.Step(step)
	emphasis := float32(0)
	if track.Hovered() || track.Pressed() {
		emphasis = 1
	}
	if !motionReduced {
		emphasis = track.AnimateTo("knob-emphasis", emphasis, controlCurve)
	}
	track.Draw(func(p *myui.Painter, r myui.Rect) {
		knob := float32(14)
		frac := float32(0)
		if hi > lo {
			frac = float32((*value - lo) / (hi - lo))
		}
		if frac < 0 {
			frac = 0
		} else if frac > 1 {
			frac = 1
		}
		trackW := r.W - knob
		h := float32(4)
		cy := r.Y + r.H/2
		p.Fill(myui.Rect{X: r.X + knob/2, Y: cy - h/2, W: trackW, H: h},
			st.Divider, h/2)
		p.Fill(myui.Rect{X: r.X + knob/2, Y: cy - h/2, W: trackW * frac, H: h},
			st.Brand, h/2)
		kx := r.X + knob/2 + trackW*frac
		if emphasis > 0 {
			ring := 3 * emphasis
			p.Fill(myui.Rect{X: kx - knob/2 - ring, Y: cy - knob/2 - ring,
				W: knob + 2*ring, H: knob + 2*ring},
				myui.RGBA(st.Brand.R, st.Brand.G, st.Brand.B, 0.16), knob/2+ring)
		}
		p.Fill(myui.Rect{X: kx - knob/2, Y: cy - knob/2, W: knob, H: knob},
			myui.RGB(255, 255, 255), knob/2)
		p.Stroke(myui.Rect{X: kx - knob/2, Y: cy - knob/2, W: knob, H: knob},
			st.Brand, knob/2, 1)
	})
}

func roundTo(v, step float64) float64 {
	return float64(int(v/step+0.5)) * step
}

// sliderRow draws a labelled slider with its value on the right, the pattern
// the original settings panels used everywhere.
func sliderRow(c *myui.Context, text string, value *float64, lo, hi, step float64, format string) {
	st := styles(c)
	myui.Column(c).FillWidth().Gap(6).Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Children(func() {
			myui.Text(c, text).FontSize(12).FontWeight(500).TextColor(st.TextSub)
			myui.Spacer(c).Grow(1)
			readout := fmt.Sprintf(format, *value)
			myui.Row(c).PaddingX(7).PaddingY(2).Radius(6).Background(st.Recessed).Children(func() {
				myui.Text(c, readout).FontSize(11).FontWeight(600).TextColor(st.Text)
			})
		})
		slider(c, value, lo, hi, step)
	})
}

// segmented is the design system's switch: a recessed track whose raised pill
// slides between segments with a spring, as the original's indicator did.
func segmented(c *myui.Context, selected *int, options []string) {
	segmentedSwitch(c, selected, options)
}

func selectField(c *myui.Context, selected *string, options []string) {
	selectFieldState(c, selected, options, false)
}

func selectFieldState(c *myui.Context, selected *string, options []string, disabled bool) {
	st := styles(c)
	input := myui.Select(c, selected, options).FillWidth().Height(38).PaddingX(12).
		Radius(st.RadiusControl).FontSize(13).Disabled(disabled)
	if disabled {
		input.Cursor(myui.CursorNotAllowed)
	}
	styleFieldInput(c, input, false)
}

func switchField(c *myui.Context, on *bool, text string) {
	st := styles(c)
	row := myui.Row(c).FillWidth().Height(34).Gap(10).PaddingX(6).
		AlignItems(myui.Center).Radius(st.RadiusPill)
	if row.Hovered() {
		row.Background(st.Recessed)
	}
	row.Children(func() {
		sw := myui.Switch(c, on).Label(text)
		myui.Text(c, text).FontSize(12).TextColor(st.TextSub).LabelledBy(sw)
	})
}

// colorField is a swatch and a hex field, side by side and the same height.
//
// The framework's ColorWell is 36×20 by default while a text field here is 38
// tall, so the chip sat small and off-centre inside whatever wrapped it. Both
// controls now take the row's height: the chip keeps its own chrome (recessed
// fill, hairline border) and the input keeps its own, with no third frame around
// the pair.
func colorField(c *myui.Context, value *myui.Color) {
	st := styles(c)
	hex := colorTextFor(value)
	myui.Row(c).FillWidth().Height(38).AlignItems(myui.Center).Gap(8).Children(func() {
		myui.ColorWell(c, value).Size(58, 38).Justify(myui.Center).
			AlignItems(myui.Center).Radius(st.RadiusControl).
			Background(st.Recessed).Border(1, st.Divider)
		input := myui.TextInput(c, &hex.value).Grow(1).Height(38).
			PaddingX(10).Radius(st.RadiusControl).FontSize(12).
			Placeholder("#RRGGBB").Tooltip("支持 #RGB 或 #RRGGBB")
		_, valid := parseHex(hex.value)
		styleFieldInput(c, input, strings.TrimSpace(hex.value) != "" && !valid)
		hex.sync()
	})
}

// colorText binds a text field to a colour: it renders the colour as #RRGGBB and
// parses what the user types back into it.
type colorText struct {
	dst   *myui.Color
	value string
	last  myui.Color
	ready bool
}

// colorFields keeps one binding per colour, keyed by the colour's own address.
//
// A fresh binding per frame lost the text immediately: every frame started with an
// empty string, so the field always showed the placeholder instead of the current
// colour, and a half-typed code could never survive to the next frame. The tools
// keep their colours as struct fields, so the address is stable for the life of
// the page. Only the render pass touches this map.
var colorFields = map[*myui.Color]*colorText{}

func colorTextFor(value *myui.Color) *colorText {
	if field, ok := colorFields[value]; ok {
		return field
	}
	field := &colorText{dst: value}
	colorFields[value] = field
	return field
}

// sync reconciles the text and the colour, in this order: a colour changed from
// outside (the picker) wins over the text, a valid code typed by the user wins over
// a stale colour, and an incomplete code leaves both alone so the user can finish
// it.
func (t *colorText) sync() {
	if !t.ready {
		t.value = hexOf(*t.dst)
		t.last = *t.dst
		t.ready = true
		return
	}
	if *t.dst != t.last {
		t.value = hexOf(*t.dst)
		t.last = *t.dst
		return
	}
	if parsed, ok := parseHex(t.value); ok {
		if parsed != *t.dst {
			*t.dst = parsed
			t.last = parsed
		}
		t.value = hexOf(*t.dst)
	}
}

// parseHex accepts #RGB, #RRGGBB and the same without the hash.
func parseHex(text string) (myui.Color, bool) {
	v := strings.TrimSpace(text)
	v = strings.TrimPrefix(v, "#")
	switch len(v) {
	case 3:
		v = string([]byte{v[0], v[0], v[1], v[1], v[2], v[2]})
	case 6:
	default:
		return myui.Color{}, false
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(v, "%02x%02x%02x", &r, &g, &b); err != nil {
		return myui.Color{}, false
	}
	return myui.RGB(r, g, b), true
}

func grid2(c *myui.Context, body func()) {
	myui.Grid(c).FillWidth().Columns(2).GapX(12).GapY(12).Children(body)
}

// sectionCard is the settings panel's grouped surface. The title sits on a
// hairline header row: a group of controls without a rule above it reads as one
// long form, which is what made the parameter column feel unfinished.
func sectionCard(c *myui.Context, title string, body func()) {
	st := styles(c)
	card := myui.Column(c).FillWidth().Gap(12).Padding(14).Radius(st.RadiusPanel).
		Border(1, st.Divider).Background(st.Card)
	card.Children(func() {
		if title != "" {
			myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(7).Children(func() {
				myui.Row(c).Width(3).Height(11).Radius(2).
					Background(st.Brand).Opacity(0.85)
				myui.Text(c, title).FontSize(12).FontWeight(600).
					LetterSpacing(0.1).TextColor(st.Text)
			})
		}
		body()
	})
}

// statusPill is the small state badge in the header. Its tint fades between
// states, which is what makes "就绪 → 处理中 42%" read as one badge changing
// rather than two badges swapped.
func statusPill(c *myui.Context, text string, tint myui.Color) {
	myui.Row(c).PaddingX(8).PaddingY(3).Radius(999).Background(
		myui.Color{R: tint.R, G: tint.G, B: tint.B, A: 26},
	).Transition(motion(springSoft)).Children(func() {
		myui.Text(c, text).FontSize(11).FontWeight(600).TextColor(tint)
	})
}

// stateAlert is the compact Lobe-style inline feedback used for validation and
// asynchronous failures. The icon and text carry the meaning as well as colour.
func stateAlert(c *myui.Context, text string, tint myui.Color, icon *myui.SVG) {
	st := styles(c)
	myui.Row(c).FillWidth().Padding(10).Radius(st.RadiusPill).Gap(8).
		AlignItems(myui.Center).Background(myui.Color{R: tint.R, G: tint.G, B: tint.B, A: 20}).
		Border(1, myui.Color{R: tint.R, G: tint.G, B: tint.B, A: 42}).
		Role(myui.RoleStatus).Label(text).Children(func() {
		myui.Icon(c, icon).Width(14).Height(14).TextColor(tint)
		myui.Text(c, text).FontSize(11.5).TextColor(tint)
	})
}

func warnAlert(c *myui.Context, text string) {
	stateAlert(c, text, styles(c).Warning, lucide.TriangleAlert())
}

// loadingIcon uses the same restrained loader idiom as Lobe, but follows the
// app's brand token and reduced-motion setting.
func loadingIcon(c *myui.Context, size float32, tint myui.Color) {
	icon := myui.Icon(c, lucide.LoaderCircle()).Width(size).Height(size).TextColor(tint)
	if !motionReduced {
		icon.Rotate(icon.Loop("spin", 900*time.Millisecond, myui.Linear) * 360)
	}
}

// primaryButton is the shared filled action. ButtonBase supplies keyboard
// focus and activation; disabled and loading are real input states, not just a
// grey coat of paint.
func primaryButton(c *myui.Context, st style, text string, icon *myui.SVG, disabled, loading bool, onClick func()) myui.Element {
	unavailable := disabled || loading
	btn := myui.ButtonBase(c).Height(42).PaddingX(20).Radius(st.RadiusControl).Gap(7).
		Justify(myui.Center).AlignItems(myui.Center).Cursor(myui.CursorPointer).
		Label(text).Transition(motion(springTap))
	switch {
	case unavailable:
		btn.Disabled(true).Cursor(myui.CursorNotAllowed).Background(st.Recessed).
			Border(1, st.Divider)
	case btn.Pressed():
		btn.Gradient(st.BrandDeep, st.BrandGradientTo, 90).Scale(0.985, 0.985)
	case btn.Hovered():
		btn.Gradient(st.BrandHover, st.BrandGradientHov, 90).
			Shadow(0, 4, 12, 0, st.BrandGlow)
	default:
		btn.Gradient(st.Brand, st.BrandGradientTo, 90).
			Shadow(0, 4, 12, 0, st.BrandGlow)
	}
	foreground := myui.RGB(255, 255, 255)
	if unavailable {
		foreground = st.TextDim
	}
	btn.Children(func() {
		if loading {
			loadingIcon(c, 15, foreground)
		} else if icon != nil {
			myui.Icon(c, icon).Width(15).Height(15).TextColor(foreground)
		}
		myui.Text(c, text).FontSize(13.5).FontWeight(600).TextColor(foreground).NoWrap()
	})
	if !unavailable && btn.Clicked() && onClick != nil {
		onClick()
	}
	return btn
}

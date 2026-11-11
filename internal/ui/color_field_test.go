package ui

import (
	"testing"

	myui "github.com/egoist/mygo/ui"
)

// TestColorFieldAcceptsTypedHex pins the hex input: both #RRGGBB and #RGB are
// parsed, and a half-typed value is left alone so the user can finish it.
func TestColorFieldAcceptsTypedHex(t *testing.T) {
	cases := []struct {
		text string
		want myui.Color
		ok   bool
	}{
		{"#FFFFFF", myui.RGB(255, 255, 255), true},
		{"#000000", myui.RGB(0, 0, 0), true},
		{"007AFF", myui.RGB(0, 122, 255), true},
		{"#f0a", myui.RGB(255, 0, 170), true},
		{"#12", myui.Color{}, false},
		{"zzzzzz", myui.Color{}, false},
		{"", myui.Color{}, false},
	}
	for _, c := range cases {
		got, ok := parseHex(c.text)
		if ok != c.ok {
			t.Errorf("%q: ok = %v, want %v", c.text, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%q = %+v, want %+v", c.text, got, c.want)
		}
	}
}

// TestColorTextRoundTrips pins that the field shows the canonical hex and writes a
// typed colour back into the model.
func TestColorTextRoundTrips(t *testing.T) {
	value := myui.RGB(0, 122, 255)
	field := colorTextFor(&value)
	field.sync()
	if field.value != "#007AFF" {
		t.Fatalf("initial text = %q, want #007AFF", field.value)
	}

	field.value = "#5856D6"
	field.sync()
	if value != myui.RGB(88, 86, 214) {
		t.Errorf("typed colour = %+v, want #5856D6", value)
	}
	if field.value != "#5856D6" {
		t.Errorf("text was rewritten to %q", field.value)
	}

	// Half-typed input must survive the round trip.
	field.value = "#58"
	field.sync()
	if field.value != "#58" {
		t.Errorf("half-typed text became %q", field.value)
	}
	if value != myui.RGB(88, 86, 214) {
		t.Errorf("half-typed text changed the colour to %+v", value)
	}
}

// TestColorFieldRemembersItsText pins the per-colour binding.
//
// The field used to build a new binding every frame, so the text started empty each
// time: the input showed the placeholder instead of the current colour, and a typed
// code could not survive the next repaint. The binding is now keyed by the colour's
// address, which the tools keep stable.
func TestColorFieldRemembersItsText(t *testing.T) {
	value := myui.RGB(255, 128, 0)
	first := colorTextFor(&value)
	first.sync()
	if first.value != "#FF8000" {
		t.Fatalf("first frame text = %q", first.value)
	}
	// Half-typed on the next frame.
	first.value = "#FF80"
	first.sync()
	second := colorTextFor(&value)
	if second.value != "#FF80" {
		t.Errorf("the next frame started at %q, want the half-typed text kept", second.value)
	}
	// A change from the picker wins over the stale text.
	value = myui.RGB(0, 0, 0)
	second.sync()
	if second.value != "#000000" {
		t.Errorf("picker change did not reach the field: %q", second.value)
	}
}

// TestRailHasNoHeading pins that the rail starts straight at its first tool: the
// 工具箱 caption only pushed the list down.
func TestRailHasNoHeading(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	for _, text := range tt.Texts() {
		if text == "工具箱" || text == "工具" {
			t.Errorf("rail still draws the %q heading", text)
		}
	}
	// The first tool sits near the top of the card.
	first, ok := tt.Access("格式转换")
	if !ok {
		t.Fatal("first tool missing")
	}
	if first.Bounds.Y > 80 {
		t.Errorf("first tool y = %v, want it near the top of the rail", first.Bounds.Y)
	}
}

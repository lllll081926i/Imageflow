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
	field := &colorText{dst: &value}
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

func TestColorFieldRemembersItsText(t *testing.T) {
	value := myui.RGB(255, 128, 0)
	tester := myui.NewTester(func(c *myui.Context) { colorField(c, &value) }, 400, 100)
	for _, field := range tester.AccessAll() {
		if field.Placeholder == "#RRGGBB" {
			tester.ClickAt(field.Bounds.X+10, field.Bounds.Y+10)
			tester.Key(myui.Ctrl, myui.KeyA)
			tester.Type("#FF80")
		}
	}
	tester.Frame()
	if value != myui.RGB(255, 128, 0) {
		t.Fatal("未完成的色值修改了颜色")
	}
	found := false
	for _, field := range tester.AccessAll() {
		if field.Placeholder == "#RRGGBB" && field.Value == "#FF80" {
			found = true
		}
	}
	if !found {
		t.Fatal("跨帧后未完成的色值丢失")
	}
	value = myui.RGB(0, 0, 0)
	tester.Frame()
	for _, field := range tester.AccessAll() {
		if field.Placeholder == "#RRGGBB" && field.Value == "#000000" {
			return
		}
	}
	t.Fatal("外部颜色修改未同步到输入框")
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

package svg

import (
	"strings"
	"testing"
)

const safeSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100" viewBox="0 0 200 100"><rect width="200" height="100" fill="#0af"/></svg>`

func TestValidateAcceptsSafeDocument(t *testing.T) {
	if err := Validate([]byte(safeSVG)); err != nil {
		t.Fatalf("safe svg rejected: %v", err)
	}
}

func TestValidateAcceptsXMLDeclaration(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-8"?>` + safeSVG
	if err := Validate([]byte(doc)); err != nil {
		t.Fatalf("xml declaration rejected: %v", err)
	}
}

func TestValidateRejectsActiveContent(t *testing.T) {
	cases := map[string]string{
		"script":        `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><script>alert(1)</script></svg>`,
		"foreignObject": `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><foreignObject></foreignObject></svg>`,
		"doctype":       `<!DOCTYPE svg [<!ENTITY x "y">]><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`,
		"entity":        `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><!ENTITY a "b"/></svg>`,
		"event handler": `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" onload="x()"/>`,
		"processing":    `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><?php echo 1 ?></svg>`,
	}
	for name, doc := range cases {
		if err := Validate([]byte(doc)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestValidateRejectsExternalReferences(t *testing.T) {
	cases := []string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><image href="https://evil/x.png"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><image href="file:///etc/passwd"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><image xlink:href="D:/secret.png"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><image href="//server/share/x.png"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><image href="..\..\x.png"/></svg>`,
	}
	for _, doc := range cases {
		if err := Validate([]byte(doc)); err == nil {
			t.Errorf("reference accepted: %s", doc)
		}
	}
}

func TestValidateAcceptsInternalAndEmbeddedImages(t *testing.T) {
	cases := []string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><use href="#shape"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><image href="data:image/png;base64,iVBORw0KGgo="/></svg>`,
	}
	for _, doc := range cases {
		if err := Validate([]byte(doc)); err != nil {
			t.Errorf("safe reference rejected: %v (%s)", err, doc)
		}
	}
}

func TestValidateRejectsUndeterminedSize(t *testing.T) {
	cases := []string{
		`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="50%"><rect width="10" height="10"/></svg>`,
	}
	for _, doc := range cases {
		if err := Validate([]byte(doc)); err == nil {
			t.Errorf("undetermined size accepted: %s", doc)
		}
	}
}

func TestValidateAcceptsPercentageWithViewBox(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 64 32"/>`
	if err := Validate([]byte(doc)); err != nil {
		t.Fatalf("percentage with viewBox rejected: %v", err)
	}
}

func TestValidateRejectsNonSVGRoot(t *testing.T) {
	doc := `<html xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`
	if err := Validate([]byte(doc)); err == nil {
		t.Fatal("non-svg root accepted")
	}
}

func TestValidateRejectsOversizeSource(t *testing.T) {
	doc := safeSVG + strings.Repeat(" ", MaxSourceBytes)
	if err := Validate([]byte(doc)); err == nil {
		t.Fatal("oversized source accepted")
	}
}

func TestParseAbsoluteUnits(t *testing.T) {
	cases := map[string]float64{
		"96":     96,
		"96px":   96,
		"1in":    96,
		"2.54cm": 96,
		"25.4mm": 96,
		"72pt":   96,
		"6pc":    96,
	}
	for input, want := range cases {
		got, ok := parseAbsolute(input)
		if !ok {
			t.Errorf("%q not parsed", input)
			continue
		}
		if got < want-0.01 || got > want+0.01 {
			t.Errorf("%q = %v, want %v", input, got, want)
		}
	}
	for _, bad := range []string{"", "abc", "50%", "-5", "0"} {
		if _, ok := parseAbsolute(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestClampSizeEnforcesCanvasBudget(t *testing.T) {
	if _, err := clampSize(100, 100); err != nil {
		t.Fatalf("valid canvas rejected: %v", err)
	}
	if _, err := clampSize(MaxCanvasSide+1, 10); err == nil {
		t.Error("over-wide canvas accepted")
	}
	if _, err := clampSize(0, 10); err == nil {
		t.Error("zero canvas accepted")
	}
}

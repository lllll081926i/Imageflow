package engine

import "testing"

// TestPDFMarginKeepsExplicitZero pins the margin convention: a negative value is
// the only "not set" sentinel. 0 is a real flush-to-edge layout the panel
// offers, and it used to be replaced with the 36 pt default.
func TestPDFMarginKeepsExplicitZero(t *testing.T) {
	a4 := paperSizes["A4"]
	cases := []struct {
		name   string
		margin float64
		page   paperSize
		want   float64
	}{
		{"explicit zero stays zero", 0, a4, 0},
		{"negative means default", -1, a4, 36},
		{"a normal value passes", 24, a4, 24},
		{"capped at a quarter of the short side", 200, paperSizes["A6"], 297.64 / 4},
	}
	for _, tc := range cases {
		if got := pdfMargin(tc.margin, tc.page); got != tc.want {
			t.Errorf("%s: pdfMargin(%v) = %v, want %v", tc.name, tc.margin, got, tc.want)
		}
	}
}

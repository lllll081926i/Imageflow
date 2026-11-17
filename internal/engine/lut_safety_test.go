package engine

import (
	"strings"
	"testing"
)

func TestLUTRejectUnsafeSamples(t *testing.T) {
	for _, document := range []string{
		"LUT_3D_SIZE 2\n1e300 0 0\n",
		"LUT_3D_SIZE 2\nLUT_3D_SIZE 3\n",
		"LUT_3D_SIZE 2\n" + strings.Repeat("0 0 0\n", 9),
	} {
		if _, err := ParseLUT(strings.NewReader(document)); err == nil {
			t.Fatal("必须拒绝溢出、重复声明或过量样本")
		}
	}
}

func FuzzParseLUT(f *testing.F) {
	f.Add("LUT_3D_SIZE 2\n" + strings.Repeat("0 0 0\n", 8))
	f.Add("LUT_3D_SIZE 2\n1e300 0 0\n")
	f.Fuzz(func(t *testing.T, document string) {
		if len(document) > 65536 {
			t.Skip()
		}
		lut, err := ParseLUT(strings.NewReader(document))
		if err == nil {
			lut.Apply(0, 127, 255)
		}
	})
}

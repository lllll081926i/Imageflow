package colorprofile

import (
	_ "embed"
	"image"
	"image/color"
	"testing"
)

//go:embed profiles/DisplayP3-v2-magic.icc
var displayP3 []byte

func TestDisplayP3ConversionAndAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(7, 9, 10, 10))
	for x := 7; x < 10; x++ {
		img.SetNRGBA(x, 9, color.NRGBA{128, 96, 64, uint8((x - 7) * 100)})
	}
	converted, err := ToSRGB(img, displayP3)
	if err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 3; x++ {
		pixel := color.NRGBA64Model.Convert(converted.At(x, 0)).(color.NRGBA64)
		c := color.NRGBA{uint8(pixel.R >> 8), uint8(pixel.G >> 8), uint8(pixel.B >> 8), uint8(pixel.A >> 8)}
		// 独立的 Display P3 → XYZ D65 → sRGB 计算约为 (134, 94, 58)。
		if abs(int(c.R)-134) > 3 || abs(int(c.G)-94) > 3 || abs(int(c.B)-58) > 3 || c.A != uint8(x*100) {
			t.Fatalf("ICC 转换或透明度错误：%+v", c)
		}
	}
}

func TestSRGBIdentity(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{40, 100, 180, 173})
	converted, err := ToSRGB(img, SRGB())
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(converted.At(0, 0)).(color.NRGBA); got != img.NRGBAAt(0, 0) {
		t.Fatalf("sRGB 恒等转换改变像素：%v", got)
	}
}

func TestConcurrentICCInstances(t *testing.T) {
	for range 8 {
		t.Run("独立实例", func(t *testing.T) {
			t.Parallel()
			TestDisplayP3ConversionAndAlpha(t)
		})
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

package colorprofile

import (
	"image"
	"image/color"
	"math"
	"os"
	"testing"
)

func TestCMYKProfileConvertsOriginalChannels(t *testing.T) {
	p, err := os.ReadFile("profiles/CGATS001Compat-v2-micro.icc")
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewCMYK(image.Rect(0, 0, 3, 1))
	img.SetCMYK(0, 0, color.CMYK{})
	img.SetCMYK(1, 0, color.CMYK{K: 255})
	img.SetCMYK(2, 0, color.CMYK{C: 255})
	out, err := ToSRGB(img, p)
	if err != nil {
		t.Fatal(err)
	}
	// 原版 lcms-wasm JavaScript 包装器的独立 8 位转换参考值。
	for x, want := range [][3]int{{255, 255, 255}, {35, 31, 32}, {0, 174, 239}} {
		c := straightPixel(out, x, 0)
		for n, v := range []uint16{c.R, c.G, c.B} {
			if delta := int(v>>8) - want[n]; delta < -1 || delta > 1 {
				t.Fatalf("CMYK 参考值不符：像素=%d 通道=%d 实际=%d 预期=%d", x, n, v>>8, want[n])
			}
		}
	}
}

func TestPQAndHLGReferenceValues(t *testing.T) {
	pq, _ := CICP(9, 16)
	hlg, _ := CICP(9, 18)
	if math.Abs(pq.linear(.5080784215)-100) > .001 {
		t.Fatalf("ST 2084 的 100 nit 参考点错误：%f", pq.linear(.5080784215))
	}
	if math.Abs(pq.linear(1)-10000) > .001 || math.Abs(hlg.linear(.5)-1.0/12) > 1e-10 {
		t.Fatal("HDR 传递函数参考点错误")
	}
}

func TestInvalidChromaticitiesFail(t *testing.T) {
	e := Encoding{Primaries: Chromaticities{}, Transfer: 13}
	if _, err := DecodeEncoding(image.NewNRGBA(image.Rect(0, 0, 1, 1)), e); err == nil {
		t.Fatal("无效原色未拒绝")
	}
}

func TestHDRNeutralGrayAndAlpha(t *testing.T) {
	for _, transfer := range []uint16{16, 18} {
		e, _ := CICP(9, transfer)
		img := image.NewNRGBA64(image.Rect(0, 0, 4, 1))
		for i, v := range []uint16{0, 32768, 49152, 65535} {
			img.SetNRGBA64(i, 0, color.NRGBA64{v, v, v, 32768})
		}
		out, err := DecodeEncoding(img, e)
		if err != nil {
			t.Fatal(err)
		}
		previous := uint16(0)
		for x := range 4 {
			c := straightPixel(out, x, 0)
			if max(c.R, c.G, c.B)-min(c.R, c.G, c.B) > 2 || c.R < previous || c.A != 32768 {
				t.Fatalf("HDR 中性灰或透明度错误：%+v", c)
			}
			previous = c.R
		}
	}
}

package colorprofile

import (
	"image"
	"testing"
)

func TestInvalidProfileRejected(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	for _, p := range [][]byte{[]byte("invalid"), make([]byte, 132)} {
		if _, err := ToSRGB(img, p); err == nil {
			t.Fatal("无效 ICC 被当作 sRGB")
		}
	}
	p := SRGB()
	copy(p[16:20], "CMYK")
	if _, err := ToSRGB(img, p); err == nil {
		t.Fatal("CMYK ICC 被当作 RGB")
	}
}

func TestSRGBBytesAreIndependent(t *testing.T) {
	p := SRGB()
	p[0] ^= 255
	if p[0] == SRGB()[0] {
		t.Fatal("标准 ICC 暴露可变共享内存")
	}
}

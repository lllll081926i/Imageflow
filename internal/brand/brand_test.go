package brand

import (
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/svg"
)

func TestLogoOpaqueInterior(t *testing.T) {
	for _, test := range []struct {
		name     string
		document string
		whiteY   int
	}{
		{"logo", LogoSVG, 220},
		{"icon", IconSVG, 153},
	} {
		t.Run(test.name, func(t *testing.T) {
			img, err := svg.RenderBytes([]byte(test.document), 1024, 1024)
			if err != nil {
				t.Fatal(err)
			}
			if pixel := color.NRGBAModel.Convert(img.At(512, test.whiteY)).(color.NRGBA); pixel != (color.NRGBA{255, 255, 255, 255}) {
				t.Fatalf("内部底色必须为不透明白色，实际为 %v", pixel)
			}
			if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
				t.Fatal("圆环外部必须保持透明")
			}
		})
	}
}

func TestPackagedIconOpaqueInterior(t *testing.T) {
	file, err := os.Open("../../resources/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if pixel := color.NRGBAModel.Convert(img.At(512, 153)).(color.NRGBA); pixel != (color.NRGBA{255, 255, 255, 255}) {
		t.Fatalf("打包图标内部底色必须为不透明白色，实际为 %v", pixel)
	}
	if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("打包图标外部必须保持透明")
	}
}

package codec

import (
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/colorprofile"
)

func TestSRGBOutputReplacesSourceProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "srgb.png")
	if err := Save(image.NewNRGBA(image.Rect(0, 0, 2, 2)), path, SaveOptions{SRGB: true, MetadataSource: "missing.png"}); err != nil {
		t.Fatal(err)
	}
	m, err := readMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(m.icc) != string(colorprofile.SRGB()) {
		t.Fatal("归一化输出未写入标准 sRGB")
	}
}

func TestPNGLinearGammaConverted(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{128, 128, 128, 255})
	raw, err := EncodeBytes(img, "png", SaveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	gamma := make([]byte, 4)
	binary.BigEndian.PutUint32(gamma, 100000)
	withGamma := append(append(append([]byte(nil), raw[:33]...), metadataPNGChunk("gAMA", gamma)...), raw[33:]...)
	path := filepath.Join(t.TempDir(), "linear.png")
	if err := os.WriteFile(path, withGamma, 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := OpenSRGB(path)
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, _ := opened.Image.At(0, 0).RGBA()
	if r>>8 < 187 || r>>8 > 189 {
		t.Fatalf("线性中灰应转换为约 188，而不是 128：%d", r>>8)
	}
	if !opened.Normalized {
		t.Fatal("未记录色彩归一化")
	}
}

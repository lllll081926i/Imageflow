package engine_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

func p3Fixture(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{128, 96, 64, 255})
		}
	}
	raw, err := codec.EncodeBytes(img, "png", codec.SaveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := os.ReadFile("../colorprofile/profiles/DisplayP3-v2-magic.icc")
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	if _, err := z.Write(profile); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte("Display P3\x00\x00"), compressed.Bytes()...)
	chunk := make([]byte, len(payload)+12)
	binary.BigEndian.PutUint32(chunk, uint32(len(payload)))
	copy(chunk[4:], "iCCP")
	copy(chunk[8:], payload)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	raw = append(append(append([]byte(nil), raw[:33]...), chunk...), raw[33:]...)
	path := filepath.Join(t.TempDir(), "p3.png")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertSRGBPixel(t *testing.T, path string, x, y int) {
	t.Helper()
	opened, err := codec.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := color.NRGBAModel.Convert(opened.Image.At(x, y)).(color.NRGBA)
	for i, v := range []int{int(c.R) - 134, int(c.G) - 94, int(c.B) - 58} {
		if v < -3 || v > 3 {
			t.Fatalf("%s 的 sRGB 通道 %d 错误：%+v", path, i, c)
		}
	}
}

func TestCompositorsConvertP3(t *testing.T) {
	source := p3Fixture(t)
	dir := t.TempDir()
	second := filepath.Join(dir, "srgb.png")
	reference := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			reference.SetNRGBA(x, y, color.NRGBA{134, 94, 58, 255})
		}
	}
	if err := codec.Save(reference, second, codec.SaveOptions{SRGB: true}); err != nil {
		t.Fatal(err)
	}
	stitch := engine.Stitch(engine.StitchRequest{InputPaths: []string{source, second}, OutputPath: filepath.Join(dir, "stitch.png"), TargetWidth: 16})
	if !stitch.Success {
		t.Fatal(stitch.Error)
	}
	assertSRGBPixel(t, stitch.OutputPath, 4, 4)
	assertSRGBPixel(t, stitch.OutputPath, 4, 20)
	icons := engine.BuildIconSet(engine.IconRequest{InputPath: source, OutputDir: filepath.Join(dir, "icons"), Sizes: []int{16}, WithICO: true, WithFavicon: true, WithSVG: true})
	if !icons.Success {
		t.Fatal(icons.Error)
	}
	assertSRGBPixel(t, filepath.Join(dir, "icons", "icon-16x16.png"), 4, 4)
	assertSRGBPixel(t, filepath.Join(dir, "icons", "icon.ico"), 4, 4)
	rich := engine.RenderRichText(engine.RichTextRequest{OutputPath: filepath.Join(dir, "rich.png"), Width: 200, Padding: 16, Blocks: []engine.RichTextBlock{{Type: "image", Path: source}}})
	if !rich.Success {
		t.Fatal(rich.Error)
	}
	assertSRGBPixel(t, rich.OutputPath, 64, 64)
}

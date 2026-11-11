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

func TestCMYKICCInputAndExport(t *testing.T) {
	raw, err := os.ReadFile("testdata/color/cmyk.jpeg")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := os.ReadFile("../colorprofile/profiles/CGATS001Compat-v2-micro.icc")
	if err != nil {
		t.Fatal(err)
	}
	raw, err = jpegMetadata(raw, imageMetadata{icc: profile})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "press.jpg")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	if !opened.Normalized {
		t.Fatal("CMYK 未在原始通道解码后转换")
	}
	target := filepath.Join(t.TempDir(), "rgb.png")
	if err := Save(opened.Image, target, SaveOptions{MetadataSource: source}); err != nil {
		t.Fatal(err)
	}
	m, err := readMetadata(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(m.icc) != string(colorprofile.SRGB()) {
		t.Fatal("转换后的 RGB 像素仍附带 CMYK 配置")
	}
	if _, err := OpenSRGB(target); err != nil {
		t.Fatal(err)
	}
	checkNormalizedExports(t, opened, source)
}

func TestRGBICCExportWithoutProfileContainer(t *testing.T) {
	p, err := os.ReadFile("../colorprofile/profiles/DisplayP3-v2-magic.icc")
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, color.NRGBA{128, 96, 64, 255})
		}
	}
	data, err := EncodeBytes(img, "png", SaveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, err = pngMetadata(data, imageMetadata{icc: p})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "p3.png")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"bmp", "ico"} {
		target := filepath.Join(t.TempDir(), "out."+format)
		if err := Save(opened.Image, target, SaveOptions{MetadataSource: source, IcoSizes: []int{16}}); err != nil {
			t.Fatal(err)
		}
		converted, err := OpenSRGB(target)
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := converted.Image.At(0, 0).RGBA()
		if r>>8 < 132 || r>>8 > 136 || g>>8 < 92 || g>>8 > 96 || b>>8 < 56 || b>>8 > 60 {
			t.Fatalf("%s 未转换 P3 像素：%d %d %d", format, r>>8, g>>8, b>>8)
		}
	}
}

func checkNormalizedExports(t *testing.T, opened *OpenResult, source string) {
	t.Helper()
	for _, format := range []string{"jpg", "webp", "avif", "tiff", "bmp", "ico", "gif"} {
		t.Run(format, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "converted."+format)
			if err := Save(opened.Image, target, SaveOptions{MetadataSource: source, IcoSizes: []int{16}}); err != nil {
				t.Fatal(err)
			}
			result, err := OpenSRGB(target)
			if err != nil {
				t.Fatal(err)
			}
			want := opened.Image.Bounds().Size()
			if format == "ico" {
				want = image.Pt(16, 16)
			}
			if result.Image.Bounds().Size() != want {
				t.Fatal("导出尺寸发生变化")
			}
		})
	}
}

func TestHDR10AVIFToneMapping(t *testing.T) {
	for _, name := range []string{"pq10.avif", "hlg10.avif"} {
		t.Run(name, func(t *testing.T) {
			source := filepath.Join("testdata", "color", name)
			opened, err := OpenSRGB(source)
			if err != nil {
				t.Fatal(err)
			}
			if !opened.Normalized {
				t.Fatal("HDR 未执行色调映射")
			}
			if _, ok := opened.Image.(*image.NRGBA64); !ok {
				t.Fatalf("10 位输入未保留 16 位处理精度：%T", opened.Image)
			}
			previous := uint32(0)
			for x := 0; x < 64; x++ {
				r, g, b, a := opened.Image.At(x, 0).RGBA()
				if r+2 < previous || a != 65535 {
					t.Fatalf("灰阶或透明度错误：x=%d r=%d previous=%d a=%d", x, r, previous, a)
				}
				// 10 位 YUV 色度量化经过 HDR 曲线后允许最多 4 个 8 位灰阶的误差。
				if max(r, g, b)-min(r, g, b) > 1024 {
					t.Fatalf("中性灰发生偏色：%d %d %d", r, g, b)
				}
				previous = r
			}
			if previous < 64000 {
				t.Fatalf("高光未映射至 SDR 白点：%d", previous)
			}
			target := filepath.Join(t.TempDir(), "sdr.png")
			if err := Save(opened.Image, target, SaveOptions{MetadataSource: source}); err != nil {
				t.Fatal(err)
			}
			if m, err := readMetadata(target); err != nil || string(m.icc) != string(colorprofile.SRGB()) {
				t.Fatalf("SDR 输出未重写色彩描述：%v", err)
			}
			checkNormalizedExports(t, opened, source)
		})
	}
}

func TestPNGChromaticitiesWithoutICC(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{128, 96, 64, 255})
	raw, err := EncodeBytes(img, "png", SaveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	chrm := make([]byte, 32)
	for i, v := range []uint32{31270, 32900, 68000, 32000, 26500, 69000, 15000, 6000} {
		binary.BigEndian.PutUint32(chrm[i*4:], v)
	}
	raw = append(append(append([]byte(nil), raw[:33]...), metadataPNGChunk("cHRM", chrm)...), raw[33:]...)
	source := filepath.Join(t.TempDir(), "p3.png")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := OpenSRGB(source)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := opened.Image.At(0, 0).RGBA()
	if r>>8 < 132 || r>>8 > 136 || g>>8 < 92 || g>>8 > 96 || b>>8 < 56 || b>>8 > 60 {
		t.Fatalf("PNG 原色转换错误：%d %d %d", r>>8, g>>8, b>>8)
	}
}

func TestPNGHDRCICP(t *testing.T) {
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	img.SetNRGBA64(0, 0, color.NRGBA64{32768, 32768, 32768, 32768})
	img.SetNRGBA64(1, 0, color.NRGBA64{65535, 65535, 65535, 65535})
	raw, err := EncodeBytes(img, "png", SaveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	raw = append(append(append([]byte(nil), raw[:33]...), metadataPNGChunk("cICP", []byte{9, 16, 0, 1})...), raw[33:]...)
	source := filepath.Join(t.TempDir(), "pq.png")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := OpenSRGB(source)
	if err != nil {
		t.Fatal(err)
	}
	out, ok := opened.Image.(*image.NRGBA64)
	if !ok || !opened.Normalized {
		t.Fatal("PNG cICP 未执行 16 位 HDR 转换")
	}
	gray, white := out.NRGBA64At(0, 0), out.NRGBA64At(1, 0)
	if gray.A != 32768 || gray.R <= 32768 || white.R != 65535 || gray.R >= white.R {
		t.Fatalf("PNG PQ 灰阶或透明度错误：%+v %+v", gray, white)
	}
}

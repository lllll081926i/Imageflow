package engine

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

func pngChunk(kind string, data []byte) []byte {
	chunk := make([]byte, len(data)+12)
	binary.BigEndian.PutUint32(chunk, uint32(len(data)))
	copy(chunk[4:], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
	return chunk
}

// straightColor 保留全透明像素的 RGB，避免 Go 的预乘转换丢失隐藏通道。
func straightColor(c color.Color) color.NRGBA64 {
	if c, ok := c.(color.NRGBA); ok {
		return color.NRGBA64{R: uint16(c.R) * 257, G: uint16(c.G) * 257, B: uint16(c.B) * 257, A: uint16(c.A) * 257}
	}
	return color.NRGBA64Model.Convert(c).(color.NRGBA64)
}

func writeCompressionPNG(t *testing.T, img image.Image) string {
	t.Helper()
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	// 插入元数据，验证直接优化 PNG 不会丢弃源文件的附加数据。
	data = append(append(append([]byte{}, data[:33]...), pngChunk("tEXt", []byte("Author\x00ImageFlow"))...), data[33:]...)
	path := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompressPNGLossless(t *testing.T) {
	for _, depth := range []int{8, 16} {
		t.Run(strconv.Itoa(depth), func(t *testing.T) {
			img := image.NewNRGBA64(image.Rect(0, 0, 96, 64))
			for y := 0; y < 64; y++ {
				for x := 0; x < 96; x++ {
					c := color.NRGBA64{R: uint16(x%4) * 0x3333, G: 0x7777, B: 0x9999, A: uint16(y%4) * 0x5555}
					if depth == 16 {
						c.R += 17
						c.A += 1
					}
					img.SetNRGBA64(x, y, c)
				}
			}
			input := writeCompressionPNG(t, img)
			original, _ := os.ReadFile(input)
			out := filepath.Join(t.TempDir(), "nested", "compressed.png")
			r := Compress(CompressRequest{InputPath: input, OutputPath: out})
			if !r.Success {
				t.Fatalf("压缩失败：%+v", r)
			}
			if r.OutputSize >= r.InputSize || r.Width != 96 || r.Height != 64 {
				t.Fatalf("尺寸或大小异常：%+v", r)
			}
			result, err := os.ReadFile(r.OutputPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(result, []byte("Author\x00ImageFlow")) {
				t.Fatal("PNG 元数据丢失")
			}
			before, err := png.Decode(bytes.NewReader(original))
			if err != nil {
				t.Fatal(err)
			}
			after, err := png.Decode(bytes.NewReader(result))
			if err != nil {
				t.Fatal(err)
			}
			for y := 0; y < 64; y++ {
				for x := 0; x < 96; x++ {
					if straightColor(before.At(x, y)) != straightColor(after.At(x, y)) {
						t.Fatalf("像素变化：%d,%d，%T %#v → %T %#v", x, y, before.At(x, y), before.At(x, y), after.At(x, y), after.At(x, y))
					}
				}
			}
			next := Compress(CompressRequest{InputPath: out, OutputPath: filepath.Join(t.TempDir(), "again.png")})
			if !next.Success || next.OutputSize > next.InputSize {
				t.Fatalf("重复压缩膨胀：%+v", next)
			}
			unchanged, _ := os.ReadFile(input)
			if !bytes.Equal(original, unchanged) {
				t.Fatal("原图被修改")
			}
		})
	}
}

func TestCompressFormats(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 80, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 80; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 3), B: 80, A: 255})
		}
	}
	for _, format := range []string{"jpg", "webp", "avif"} {
		t.Run(format, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "input."+format)
			if err := codec.Save(img, input, codec.SaveOptions{Quality: 95}); err != nil {
				t.Fatal(err)
			}
			r := Compress(CompressRequest{InputPath: input, OutputPath: filepath.Join(t.TempDir(), "out."+format), Quality: 65})
			if !r.Success || r.OutputSize > r.InputSize || r.Width != 80 || r.Height != 64 {
				t.Fatalf("压缩异常：%+v", r)
			}
			decoded, err := codec.Open(r.OutputPath)
			if err != nil || decoded.Width != 80 {
				t.Fatalf("无法解码：%v", err)
			}
			if format == "webp" {
				r = Compress(CompressRequest{InputPath: input, OutputPath: filepath.Join(t.TempDir(), "out.png"), Format: "png"})
				if !r.Success {
					t.Fatalf("PNG 标准输入压缩失败：%+v", r)
				}
			}
		})
	}
	input := writeCompressionPNG(t, img)
	r := Compress(CompressRequest{InputPath: input, OutputPath: filepath.Join(t.TempDir(), "out.webp"), Format: "webp", Lossless: true})
	if !r.Success {
		t.Fatalf("WebP 无损失败：%+v", r)
	}
}

func TestCompressRejectsUnsafeRequests(t *testing.T) {
	input := writeCompressionPNG(t, image.NewNRGBA(image.Rect(0, 0, 4, 4)))
	data, _ := os.ReadFile(input)
	for _, req := range []CompressRequest{
		{InputPath: input, OutputPath: input},
		{InputPath: input, OutputPath: input, Format: "gif"},
		{InputPath: input, OutputPath: input, Quality: 101},
		{InputPath: input, OutputPath: input, Format: "jpg", Lossless: true},
	} {
		if r := Compress(req); r.Success {
			t.Fatalf("错误请求成功：%+v", req)
		}
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(data, after) {
		t.Fatal("原图被修改")
	}
	anim := append(append(append([]byte{}, data[:33]...), pngChunk("acTL", make([]byte, 8))...), data[33:]...)
	animPath := filepath.Join(t.TempDir(), "animation.png")
	os.WriteFile(animPath, anim, 0600)
	out := filepath.Join(t.TempDir(), "protected.png")
	os.WriteFile(out, []byte("existing"), 0600)
	r := Compress(CompressRequest{InputPath: animPath, OutputPath: out})
	if r.Success || !strings.Contains(r.Error, "动图") {
		t.Fatalf("动图未被拒绝：%+v", r)
	}
	kept, _ := os.ReadFile(out)
	if string(kept) != "existing" {
		t.Fatal("失败覆盖已有结果")
	}
	// 头部尺寸必须在解码分配像素前被拒绝。
	huge := append([]byte{}, data...)
	binary.BigEndian.PutUint32(huge[16:20], 16001)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	hugePath := filepath.Join(t.TempDir(), "huge.png")
	os.WriteFile(hugePath, huge, 0600)
	r = Compress(CompressRequest{InputPath: hugePath, OutputPath: out})
	if r.ErrorCode != "OUTPUT_TOO_LARGE" {
		t.Fatalf("尺寸预算未生效：%+v", r)
	}
}

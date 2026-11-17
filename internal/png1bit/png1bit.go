// Package png1bit writes one-bit PNG files.
//
// A "strictly black and white" PNG and a "one-bit deep" PNG are two different
// claims: an RGB PNG whose pixels happen to be only black or white is still an
// 8-bit image, and the PRD forbids reporting it as a 1-bit encoding. This
// package writes the real thing — IHDR bitDepth=1 with colorType 0 (greyscale)
// or 3 (indexed) — so the pixel-art page can support both claims separately.
package png1bit

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"io"
)

// ColorType is the PNG colour type of the written file.
type ColorType uint8

const (
	// Gray writes colorType 0: one bit per pixel, 0 = black, 1 = white.
	Gray ColorType = 0
	// Palette writes colorType 3 with a two-entry PLTE.
	Palette ColorType = 3
)

// Options configures one write.
type Options struct {
	// ColorType selects greyscale or indexed output. Both are bitDepth 1.
	ColorType ColorType
	// Black and White are the two colours written to the palette. They are
	// required to be opaque; a halftone area cannot be expressed at bitDepth 1.
	Black, White color.NRGBA
}

// Mask is a bilevel bitmap: one bit per pixel, row-major, MSB first. A set bit
// means "white" (or the second palette entry), a clear bit "black".
//
// Keeping the mask separate from the thresholding lets the caller report exactly
// which two values the file will contain, and lets the tests assert on the bits
// before any encoder is involved.
type Mask struct {
	W, H int
	// Pix holds the packed bits. Stride is (W+7)/8 bytes per row.
	Pix []byte
}

// NewMask returns a mask with every pixel clear.
func NewMask(w, h int) *Mask {
	if w < 1 || h < 1 {
		return &Mask{}
	}
	stride := (w + 7) / 8
	return &Mask{W: w, H: h, Pix: make([]byte, stride*h)}
}

// Stride is the packed row length in bytes.
func (m *Mask) Stride() int {
	if m.W < 1 {
		return 0
	}
	return (m.W + 7) / 8
}

// Set writes one pixel. Out-of-range coordinates are ignored, so a caller that
// walks an image with a shifted origin cannot corrupt a neighbouring row.
func (m *Mask) Set(x, y int, white bool) {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return
	}
	i := y*m.Stride() + x/8
	bit := uint(7 - uint(x%8))
	if white {
		m.Pix[i] |= 1 << bit
	} else {
		m.Pix[i] &^= 1 << bit
	}
}

// At reports one pixel.
func (m *Mask) At(x, y int) bool {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return false
	}
	return m.Pix[y*m.Stride()+x/8]&(1<<uint(7-uint(x%8))) != 0
}

// Raw returns the packed bits, one bit per pixel, MSB first, rows padded to whole
// bytes. It is the bitmap independent of any container, which is what the
// black-and-white acceptance check compares against.
func (m *Mask) Raw() []byte {
	if m == nil {
		return nil
	}
	stride := m.Stride()
	out := make([]byte, stride*m.H)
	copy(out, m.Pix)
	// Padding bits past the last pixel are cleared so two masks of the same
	// pixels compare equal whatever produced them.
	if rem := m.W % 8; rem != 0 {
		mask := byte(0xFF) << uint(8-rem)
		for y := 0; y < m.H; y++ {
			out[y*stride+stride-1] &= mask
		}
	}
	return out
}

// Encode writes the mask as a one-bit PNG.
//
// The returned byte slice is a complete file, so a caller can write it through
// the same atomic output path every other tool uses.
func Encode(m *Mask, opts Options) ([]byte, error) {
	if m == nil || m.W < 1 || m.H < 1 {
		return nil, fmt.Errorf("1-bit PNG 尺寸无效")
	}
	if opts.ColorType != Gray && opts.ColorType != Palette {
		return nil, fmt.Errorf("不支持的 1-bit PNG 颜色类型: %d", opts.ColorType)
	}
	if opts.ColorType == Palette && (!opaque(opts.Black) || !opaque(opts.White)) {
		return nil, fmt.Errorf("索引模式的调色板必须是两种不透明颜色")
	}
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(m.W))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(m.H))
	ihdr[8] = 1 // bit depth
	ihdr[9] = byte(opts.ColorType)
	ihdr[10] = 0 // deflate
	ihdr[11] = 0 // adaptive filtering
	ihdr[12] = 0 // no interlace
	if err := writeChunk(&buf, "IHDR", ihdr); err != nil {
		return nil, err
	}
	if opts.ColorType == Palette {
		plte := []byte{opts.Black.R, opts.Black.G, opts.Black.B, opts.White.R, opts.White.G, opts.White.B}
		if err := writeChunk(&buf, "PLTE", plte); err != nil {
			return nil, err
		}
	}
	// sRGB keeps viewers from applying an unknown transfer curve to a picture
	// whose whole point is two exact values.
	if err := writeChunk(&buf, "sRGB", []byte{0}); err != nil {
		return nil, err
	}

	stride := m.Stride()
	raw := make([]byte, 0, (stride+1)*m.H)
	for y := 0; y < m.H; y++ {
		// Filter type 0 for every row: a 1-bit row has two values, and a filter
		// predictor would cost more than it saves.
		//
		// Both colour types share the packing: PLTE is always written as
		// [Black, White], so a set bit is index 1 = White and an index-0 pixel
		// reads back as Black. One bit layout therefore serves greyscale and
		// indexed output, and the two files differ only in the header.
		raw = append(raw, 0)
		raw = append(raw, m.Pix[y*stride:(y+1)*stride]...)
	}
	var packed bytes.Buffer
	zw, err := zlib.NewWriterLevel(&packed, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := writeChunk(&buf, "IDAT", packed.Bytes()); err != nil {
		return nil, err
	}
	if err := writeChunk(&buf, "IEND", nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FromImage thresholds img into a mask. A pixel whose luminance is at least
// threshold becomes white; everything else becomes black. Transparent pixels are
// composited onto background first, so a transparent logo does not turn black by
// accident.
func FromImage(img image.Image, threshold int, invert bool, background color.NRGBA) *Mask {
	if img == nil {
		return NewMask(1, 1)
	}
	b := img.Bounds()
	m := NewMask(b.Dx(), b.Dy())
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 255 {
		threshold = 255
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			if c.A < 255 {
				c = composite(c, background)
			}
			lum := luminance(c)
			white := lum >= float64(threshold)
			if invert {
				white = !white
			}
			m.Set(x, y, white)
		}
	}
	return m
}

// composite lays a translucent pixel over an opaque background.
func composite(c, bg color.NRGBA) color.NRGBA {
	a := float64(c.A) / 255
	mix := func(s, d uint8) uint8 {
		return uint8(float64(s)*a + float64(d)*(1-a) + 0.5)
	}
	base := bg
	if base.A == 0 {
		base = color.NRGBA{A: 255}
	}
	return color.NRGBA{R: mix(c.R, base.R), G: mix(c.G, base.G), B: mix(c.B, base.B), A: 255}
}

// luminance is the perceptual weight used by every threshold in the app.
func luminance(c color.NRGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}

func opaque(c color.NRGBA) bool { return c.A == 255 }

// writeChunk appends one length/type/data/CRC chunk.
func writeChunk(w io.Writer, kind string, data []byte) error {
	var head [8]byte
	binary.BigEndian.PutUint32(head[0:], uint32(len(data)))
	copy(head[4:], kind)
	if _, err := w.Write(head[:]); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	crc := crc32.NewIEEE()
	crc.Write([]byte(kind))
	crc.Write(data)
	var tail [4]byte
	binary.BigEndian.PutUint32(tail[:], crc.Sum32())
	_, err := w.Write(tail[:])
	return err
}

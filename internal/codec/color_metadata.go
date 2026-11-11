package codec

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/colorprofile"
)

const maxMetadataBytes = 4 << 20

type imageMetadata struct {
	icc, exif, xmp        []byte
	gamma, chromaticities []byte
	srgb                  bool
	nclx, cicp            []byte
}

// PreserveMetadata 保留色彩配置；只有显式开启时才复制 EXIF 和 XMP。
func PreserveMetadata(data []byte, format, source string, keep bool) ([]byte, error) {
	if source == "" {
		return data, nil
	}
	m, err := readMetadata(source)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizesSource(m)
	if err != nil {
		return nil, err
	} else if normalized {
		m.icc = colorprofile.SRGB()
	}
	if !keep {
		m.exif = nil
		m.xmp = nil
	}
	if len(m.icc)+len(m.exif)+len(m.xmp) == 0 {
		return data, nil
	}
	switch strings.TrimPrefix(strings.ToLower(format), ".") {
	case "jpg", "jpeg":
		return jpegMetadata(data, m)
	case "png":
		return pngMetadata(data, m)
	case "webp":
		return webpMetadata(data, m)
	case "avif":
		if len(m.exif)+len(m.xmp) > 0 {
			return nil, fmt.Errorf("AVIF 输出暂不支持保留 EXIF / XMP，请关闭保留元数据或选择 PNG / WebP / JPG")
		}
		return avifICC(data, m.icc)
	case "tif", "tiff":
		if len(m.exif)+len(m.xmp) > 0 {
			return nil, fmt.Errorf("TIFF 输出暂不支持保留 EXIF / XMP，请选择 PNG / WebP / JPG")
		}
		return tiffICC(data, m.icc)
	case "bmp", "ico", "gif":
		if normalized && len(m.exif)+len(m.xmp) == 0 {
			return data, nil
		}
	}
	return nil, fmt.Errorf("%s 输出不能保留源图片色彩配置或元数据，请选择 PNG / WebP / JPG / AVIF", format)
}

func readMetadata(path string) (imageMetadata, error) {
	var m imageMetadata
	f, err := os.Open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return m, err
	}
	read := func(pos, size int64) ([]byte, error) {
		if size < 0 || size > maxMetadataBytes || pos < 0 || pos > info.Size()-size {
			return nil, fmt.Errorf("元数据块超出安全限制")
		}
		b := make([]byte, size)
		_, err := f.ReadAt(b, pos)
		return b, err
	}
	switch extOf(path) {
	case ".jpg", ".jpeg":
		sequence, total := 0, 0
		for pos := int64(2); pos+4 <= info.Size(); {
			h, err := read(pos, 4)
			if err != nil {
				return m, err
			}
			if h[0] != 255 || h[1] == 0xda || h[1] == 0xd9 {
				break
			}
			if h[1] == 255 || h[1] == 0x01 || h[1] >= 0xd0 && h[1] <= 0xd8 {
				pos += 2
				continue
			}
			n := int64(binary.BigEndian.Uint16(h[2:]))
			if n < 2 {
				return m, fmt.Errorf("无效的 JPEG 元数据块")
			}
			if h[1] == 0xe1 || h[1] == 0xe2 {
				b, err := read(pos+4, n-2)
				if err != nil {
					return m, err
				}
				switch {
				case bytes.HasPrefix(b, []byte("Exif\x00\x00")):
					m.exif = append([]byte(nil), b[6:]...)
				case bytes.HasPrefix(b, []byte("ICC_PROFILE\x00")) && len(b) >= 14:
					sequence++
					if sequence == 1 {
						total = int(b[13])
					}
					if int(b[12]) != sequence || int(b[13]) != total || total == 0 {
						return m, fmt.Errorf("ICC 分段顺序无效")
					}
					m.icc = append(m.icc, b[14:]...)
				case bytes.HasPrefix(b, []byte("http://ns.adobe.com/xap/1.0/\x00")):
					m.xmp = append([]byte(nil), b[29:]...)
				}
			}
			if len(m.icc) > maxMetadataBytes {
				return m, fmt.Errorf("ICC 配置超过安全限制")
			}
			pos += 2 + n
		}
		if sequence != total {
			return m, fmt.Errorf("ICC 分段缺失")
		}
	case ".png", ".apng", ".webp":
		png := extOf(path) != ".webp"
		start := int64(12)
		if png {
			start = 8
		}
		for pos := start; pos+8 <= info.Size(); {
			h, err := read(pos, 8)
			if err != nil {
				return m, err
			}
			kind := string(h[:4])
			n := int64(binary.LittleEndian.Uint32(h[4:]))
			step := 8 + n + n%2
			if png {
				kind = string(h[4:])
				n = int64(binary.BigEndian.Uint32(h))
				step = n + 12
			}
			if step > info.Size()-pos {
				return m, fmt.Errorf("截断的图片元数据容器")
			}
			if kind == "iCCP" || kind == "ICCP" || kind == "eXIf" || kind == "EXIF" || kind == "XMP " || kind == "iTXt" || (png && (kind == "gAMA" || kind == "cHRM" || kind == "sRGB" || kind == "cICP")) {
				b, err := read(pos+8, n)
				if err != nil {
					return m, err
				}
				switch kind {
				case "cICP":
					if len(b) != 4 {
						return m, fmt.Errorf("无效的 PNG cICP")
					}
					m.cicp = b
				case "gAMA":
					if len(b) != 4 {
						return m, fmt.Errorf("无效的 PNG gAMA")
					}
					m.gamma = b
				case "cHRM":
					if len(b) != 32 {
						return m, fmt.Errorf("无效的 PNG cHRM")
					}
					m.chromaticities = b
				case "sRGB":
					if len(b) != 1 || b[0] > 3 {
						return m, fmt.Errorf("无效的 PNG sRGB")
					}
					m.srgb = true
				case "ICCP":
					m.icc = b
				case "iCCP":
					i := bytes.IndexByte(b, 0)
					if i < 0 || i+2 > len(b) || b[i+1] != 0 {
						return m, fmt.Errorf("无效的 PNG ICC 配置")
					}
					z, err := zlib.NewReader(bytes.NewReader(b[i+2:]))
					if err != nil {
						return m, err
					}
					m.icc, err = io.ReadAll(io.LimitReader(z, maxMetadataBytes+1))
					z.Close()
					if err != nil || len(m.icc) > maxMetadataBytes {
						return m, fmt.Errorf("PNG ICC 配置超过安全限制")
					}
				case "eXIf", "EXIF":
					m.exif = bytes.TrimPrefix(b, []byte("Exif\x00\x00"))
				case "XMP ":
					m.xmp = b
				case "iTXt":
					prefix := []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00")
					if bytes.HasPrefix(b, prefix) {
						m.xmp = b[len(prefix):]
					}
				}
			}
			pos += step
		}
	case ".avif", ".heic", ".heif":
		m.icc, m.nclx, err = readBMFFColor(f, info.Size())
		if err != nil {
			return m, err
		}
	case ".tif", ".tiff":
		m.icc, err = readTIFFICC(f, info.Size())
		if err != nil {
			return m, err
		}
	}
	return m, nil
}

func jpegMetadata(data []byte, m imageMetadata) ([]byte, error) {
	var segments [][]byte
	add := func(marker byte, payload []byte) error {
		if len(payload) > 65533 {
			return fmt.Errorf("JPEG 元数据块超过 65533 字节")
		}
		b := []byte{255, marker, 0, 0}
		binary.BigEndian.PutUint16(b[2:], uint16(len(payload)+2))
		b = append(b, payload...)
		segments = append(segments, b)
		return nil
	}
	if len(m.exif) > 0 {
		if err := add(0xe1, append([]byte("Exif\x00\x00"), m.exif...)); err != nil {
			return nil, err
		}
		segments[len(segments)-1] = withOrientationReset(segments[len(segments)-1])
	}
	if len(m.xmp) > 0 {
		if err := add(0xe1, append([]byte("http://ns.adobe.com/xap/1.0/\x00"), m.xmp...)); err != nil {
			return nil, err
		}
	}
	const part = 65519
	count := (len(m.icc) + part - 1) / part
	for i := 0; i < count; i++ {
		payload := append([]byte("ICC_PROFILE\x00"), byte(i+1), byte(count))
		payload = append(payload, m.icc[i*part:min((i+1)*part, len(m.icc))]...)
		if err := add(0xe2, payload); err != nil {
			return nil, err
		}
	}
	return spliceJPEGMetadata(data, segments), nil
}

func metadataPNGChunk(kind string, payload []byte) []byte {
	b := make([]byte, len(payload)+12)
	binary.BigEndian.PutUint32(b, uint32(len(payload)))
	copy(b[4:], kind)
	copy(b[8:], payload)
	binary.BigEndian.PutUint32(b[len(b)-4:], crc32.ChecksumIEEE(b[4:len(b)-4]))
	return b
}

func uprightEXIF(data []byte) []byte {
	segment := append([]byte{255, 0xe1, 0, 0, 'E', 'x', 'i', 'f', 0, 0}, data...)
	reset := withOrientationReset(segment)
	if len(reset) < 10 {
		return nil
	}
	return reset[10:]
}

func pngMetadata(data []byte, m imageMetadata) ([]byte, error) {
	if len(data) < 33 {
		return nil, fmt.Errorf("无效的 PNG 输出")
	}
	var chunks []byte
	if len(m.icc) > 0 {
		var buf bytes.Buffer
		z := zlib.NewWriter(&buf)
		if _, err := z.Write(m.icc); err != nil {
			return nil, err
		}
		if err := z.Close(); err != nil {
			return nil, err
		}
		chunks = append(chunks, metadataPNGChunk("iCCP", append([]byte("ICC\x00\x00"), buf.Bytes()...))...)
	}
	if len(m.exif) > 0 {
		chunks = append(chunks, metadataPNGChunk("eXIf", uprightEXIF(m.exif))...)
	}
	if len(m.xmp) > 0 {
		chunks = append(chunks, metadataPNGChunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"), m.xmp...))...)
	}
	result := append([]byte(nil), data[:33]...)
	result = append(result, chunks...)
	return append(result, data[33:]...), nil
}

func riffMetadataChunk(kind string, payload []byte) []byte {
	b := make([]byte, 8+len(payload)+len(payload)%2)
	copy(b, kind)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(payload)))
	copy(b[8:], payload)
	return b
}

func webpMetadata(data []byte, m imageMetadata) ([]byte, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("无效的 WebP 输出")
	}
	cfg, err := webpConfig(data)
	if err != nil {
		return nil, err
	}
	flags := byte(0)
	if len(m.icc) > 0 {
		flags |= 0x20
	}
	if len(m.exif) > 0 {
		flags |= 8
	}
	if len(m.xmp) > 0 {
		flags |= 4
	}
	var body []byte
	if string(data[12:16]) == "VP8X" {
		body = append([]byte(nil), data[12:]...)
		body[8] |= flags
	} else {
		vp8x := make([]byte, 10)
		vp8x[0] = flags
		if string(data[12:16]) == "VP8L" && len(data) > 24 && data[24]&0x10 != 0 {
			vp8x[0] |= 0x10
		}
		for i := 0; i < 3; i++ {
			vp8x[4+i] = byte((cfg.Width - 1) >> (8 * i))
			vp8x[7+i] = byte((cfg.Height - 1) >> (8 * i))
		}
		body = riffMetadataChunk("VP8X", vp8x)
		body = append(body, data[12:]...)
	}
	if len(m.icc) > 0 {
		body = append(body[:18], append(riffMetadataChunk("ICCP", m.icc), body[18:]...)...)
	}
	if len(m.exif) > 0 {
		body = append(body, riffMetadataChunk("EXIF", uprightEXIF(m.exif))...)
	}
	if len(m.xmp) > 0 {
		body = append(body, riffMetadataChunk("XMP ", m.xmp)...)
	}
	out := append([]byte("RIFF\x00\x00\x00\x00WEBP"), body...)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return out, nil
}

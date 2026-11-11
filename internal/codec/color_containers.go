package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"

	"github.com/gen2brain/webp"
)

func webpConfig(data []byte) (image.Config, error) { return webp.DecodeConfig(bytes.NewReader(data)) }

type bmffBox struct {
	kind                string
	start, header, size int64
}

func walkBMFF(r io.ReaderAt, start, end int64, visit func(bmffBox) error) error {
	for pos := start; pos < end; {
		var h [16]byte
		if end-pos < 8 {
			return fmt.Errorf("截断的图像容器块")
		}
		if _, err := r.ReadAt(h[:8], pos); err != nil {
			return err
		}
		size, header := int64(binary.BigEndian.Uint32(h[:4])), int64(8)
		if size == 1 {
			if _, err := r.ReadAt(h[8:], pos+8); err != nil {
				return err
			}
			n := binary.BigEndian.Uint64(h[8:])
			if n > uint64(end-pos) {
				return fmt.Errorf("图像容器块超出文件范围")
			}
			size, header = int64(n), 16
		} else if size == 0 {
			size = end - pos
		}
		if size < header || size > end-pos {
			return fmt.Errorf("无效的图像容器块大小")
		}
		if err := visit(bmffBox{string(h[4:8]), pos, header, size}); err != nil {
			return err
		}
		pos += size
	}
	return nil
}

func readBMFFICC(r io.ReaderAt, size int64) ([]byte, error) {
	profile, _, err := readBMFFColor(r, size)
	return profile, err
}

func readColorProperty(r io.ReaderAt, size int64) ([]byte, []byte, error) {
	var profile []byte
	var nclx []byte
	var scan func(int64, int64, int) error
	scan = func(start, end int64, depth int) error {
		if depth > 8 {
			return fmt.Errorf("图像容器嵌套过深")
		}
		return walkBMFF(r, start, end, func(b bmffBox) error {
			payload := b.start + b.header
			switch b.kind {
			case "meta":
				return scan(payload+4, b.start+b.size, depth+1)
			case "iprp", "ipco":
				return scan(payload, b.start+b.size, depth+1)
			case "colr":
				if b.size-b.header < 4 {
					return fmt.Errorf("无效的色彩属性")
				}
				var kind [4]byte
				if _, err := r.ReadAt(kind[:], payload); err != nil {
					return err
				}
				if string(kind[:]) == "prof" || string(kind[:]) == "rICC" {
					n := b.size - b.header - 4
					if n > maxMetadataBytes {
						return fmt.Errorf("ICC 配置超过安全限制")
					}
					profile = make([]byte, n)
					_, err := r.ReadAt(profile, payload+4)
					return err
				}
				if string(kind[:]) == "nclx" {
					var c [7]byte
					if b.size-b.header < 11 {
						return fmt.Errorf("截断的 NCLX 色彩属性")
					}
					if _, err := r.ReadAt(c[:], payload+4); err != nil {
						return err
					}
					nclx = append([]byte(nil), c[:]...)
				}
			}
			return nil
		})
	}
	err := scan(0, size, 0)
	return profile, nclx, err
}

func makeBMFFBox(kind string, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b, uint32(len(b)))
	copy(b[4:], kind)
	copy(b[8:], payload)
	return b
}

// avifICC 替换已有 colr 属性，并同步修正 meta 大小和 mdat 的绝对偏移。
func avifICC(data, profile []byte) ([]byte, error) {
	if len(profile) == 0 {
		return data, nil
	}
	replaced := false
	var rewrite func([]byte) ([]byte, error)
	rewrite = func(src []byte) ([]byte, error) {
		var result []byte
		err := walkBMFF(bytes.NewReader(src), 0, int64(len(src)), func(b bmffBox) error {
			payload := src[b.start+b.header : b.start+b.size]
			switch b.kind {
			case "meta", "iprp", "ipco":
				prefix := 0
				if b.kind == "meta" {
					prefix = 4
				}
				if len(payload) < prefix {
					return fmt.Errorf("无效的 AVIF 属性")
				}
				children, err := rewrite(payload[prefix:])
				if err != nil {
					return err
				}
				replacement := append(append([]byte(nil), payload[:prefix]...), children...)
				result = append(result, makeBMFFBox(b.kind, replacement)...)
			case "colr":
				replaced = true
				result = append(result, makeBMFFBox("colr", append([]byte("prof"), profile...))...)
			default:
				result = append(result, src[b.start:b.start+b.size]...)
			}
			return nil
		})
		return result, err
	}
	// 仅改写 meta；其他块直接复制，随后按 meta 的增量修正 iloc。
	var out []byte
	err := walkBMFF(bytes.NewReader(data), 0, int64(len(data)), func(b bmffBox) error {
		if b.kind != "meta" {
			out = append(out, data[b.start:b.start+b.size]...)
			return nil
		}
		meta, err := rewrite(data[b.start : b.start+b.size])
		if err != nil {
			return err
		}
		delta := int64(len(meta)) - b.size
		if err := shiftItemOffsets(meta, b.start+b.size, delta); err != nil {
			return err
		}
		out = append(out, meta...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !replaced {
		return nil, fmt.Errorf("AVIF 编码结果缺少可替换的色彩属性")
	}
	return out, nil
}

func shiftItemOffsets(meta []byte, threshold, delta int64) error {
	return walkBMFF(bytes.NewReader(meta), 12, int64(len(meta)), func(b bmffBox) error {
		if b.kind != "iloc" {
			return nil
		}
		p := meta[b.start+b.header : b.start+b.size]
		if len(p) < 8 || p[0] > 2 {
			return fmt.Errorf("无效的 AVIF iloc")
		}
		version := p[0]
		offsetSize, lengthSize, baseSize, indexSize := int(p[4]>>4), int(p[4]&15), int(p[5]>>4), int(p[5]&15)
		pos := 6
		read := func(n int) (uint64, error) {
			if n > 8 || n < 0 || pos+n > len(p) {
				return 0, fmt.Errorf("无效的 AVIF 位置字段")
			}
			var v uint64
			for _, c := range p[pos : pos+n] {
				v = v<<8 | uint64(c)
			}
			pos += n
			return v, nil
		}
		shift := func(at, n int, v uint64) error {
			if v < uint64(threshold) {
				return nil
			}
			next := int64(v) + delta
			if next < 0 || n == 0 || n < 8 && uint64(next) > uint64(1)<<(n*8)-1 {
				return fmt.Errorf("AVIF 偏移溢出")
			}
			for i := n - 1; i >= 0; i-- {
				p[at+i] = byte(next)
				next >>= 8
			}
			return nil
		}
		countSize := 2
		if version == 2 {
			countSize = 4
		}
		count, err := read(countSize)
		if err != nil {
			return err
		}
		for i := uint64(0); i < count; i++ {
			idSize := 2
			if version == 2 {
				idSize = 4
			}
			if _, err = read(idSize); err != nil {
				return err
			}
			method := uint64(0)
			if version > 0 {
				method, err = read(2)
				if err != nil {
					return err
				}
				method &= 15
			}
			reference, err := read(2)
			if err != nil {
				return err
			}
			basePos := pos
			base, err := read(baseSize)
			if err != nil {
				return err
			}
			if method == 0 && reference == 0 {
				if err = shift(basePos, baseSize, base); err != nil {
					return err
				}
			}
			extents, err := read(2)
			if err != nil {
				return err
			}
			for j := uint64(0); j < extents; j++ {
				if version > 0 {
					if _, err = read(indexSize); err != nil {
						return err
					}
				}
				at := pos
				offset, err := read(offsetSize)
				if err != nil {
					return err
				}
				if method == 0 && reference == 0 && base == 0 {
					if err = shift(at, offsetSize, offset); err != nil {
						return err
					}
				}
				if _, err = read(lengthSize); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func tiffOrder(h []byte) (binary.ByteOrder, error) {
	if len(h) < 8 {
		return nil, fmt.Errorf("截断的 TIFF 头")
	}
	if string(h[:2]) == "II" {
		return binary.LittleEndian, nil
	}
	if string(h[:2]) == "MM" {
		return binary.BigEndian, nil
	}
	return nil, fmt.Errorf("无效的 TIFF 字节序")
}

func readTIFFICC(r io.ReaderAt, size int64) ([]byte, error) {
	var header [8]byte
	if _, err := r.ReadAt(header[:], 0); err != nil {
		return nil, err
	}
	order, err := tiffOrder(header[:])
	if err != nil {
		return nil, err
	}
	pos := int64(order.Uint32(header[4:]))
	if pos < 8 || pos > size-2 {
		return nil, fmt.Errorf("无效的 TIFF IFD")
	}
	var count [2]byte
	if _, err = r.ReadAt(count[:], pos); err != nil {
		return nil, err
	}
	for i := 0; i < int(order.Uint16(count[:])); i++ {
		var entry [12]byte
		if _, err = r.ReadAt(entry[:], pos+2+int64(i)*12); err != nil {
			return nil, err
		}
		if order.Uint16(entry[:2]) != 34675 {
			continue
		}
		n := int64(order.Uint32(entry[4:8]))
		offset := int64(order.Uint32(entry[8:]))
		if n > maxMetadataBytes || offset > size-n {
			return nil, fmt.Errorf("TIFF ICC 配置超过安全限制")
		}
		profile := make([]byte, n)
		_, err = r.ReadAt(profile, offset)
		return profile, err
	}
	return nil, nil
}

func tiffICC(data, profile []byte) ([]byte, error) {
	if len(profile) == 0 {
		return data, nil
	}
	order, err := tiffOrder(data)
	if err != nil {
		return nil, err
	}
	pos := int(order.Uint32(data[4:8]))
	if pos < 8 || pos > len(data)-2 {
		return nil, fmt.Errorf("无效的 TIFF IFD")
	}
	count := int(order.Uint16(data[pos:]))
	end := pos + 2 + count*12
	if end+4 > len(data) || count == 65535 {
		return nil, fmt.Errorf("无效的 TIFF IFD 长度")
	}
	out := append([]byte(nil), data...)
	if len(out)%2 != 0 {
		out = append(out, 0)
	}
	profileOffset := len(out)
	out = append(out, profile...)
	if len(out)%2 != 0 {
		out = append(out, 0)
	}
	ifdOffset := len(out)
	ifd := make([]byte, 2+(count+1)*12+4)
	order.PutUint16(ifd, uint16(count+1))
	copy(ifd[2:], data[pos+2:end])
	entry := ifd[2+count*12:]
	order.PutUint16(entry, 34675)
	order.PutUint16(entry[2:], 7)
	order.PutUint32(entry[4:], uint32(len(profile)))
	order.PutUint32(entry[8:], uint32(profileOffset))
	copy(ifd[len(ifd)-4:], data[end:end+4])
	out = append(out, ifd...)
	order.PutUint32(out[4:], uint32(ifdOffset))
	return out, nil
}

package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// readBMFFColor 仅使用主图关联的属性；网格图像可从关联的图块继承色彩。
func readBMFFColor(r io.ReaderAt, size int64) ([]byte, []byte, error) {
	var primary uint32
	var properties []bmffBox
	associations := map[uint32][]int{}
	derived := map[uint32][]uint32{}
	read := func(b bmffBox) ([]byte, error) {
		n := b.size - b.header
		if n > maxMetadataBytes {
			return nil, fmt.Errorf("图像属性超过安全限制")
		}
		data := make([]byte, n)
		_, err := r.ReadAt(data, b.start+b.header)
		return data, err
	}
	var scan func(int64, int64, int) error
	scan = func(start, end int64, depth int) error {
		if depth > 8 {
			return fmt.Errorf("图像容器嵌套过深")
		}
		return walkBMFF(r, start, end, func(b bmffBox) error {
			payload := b.start + b.header
			switch b.kind {
			case "meta":
				if b.size-b.header < 4 {
					return fmt.Errorf("截断的 meta")
				}
				return scan(payload+4, b.start+b.size, depth+1)
			case "iprp":
				return scan(payload, b.start+b.size, depth+1)
			case "ipco":
				return walkBMFF(r, payload, b.start+b.size, func(p bmffBox) error { properties = append(properties, p); return nil })
			case "pitm":
				data, err := read(b)
				if err != nil {
					return err
				}
				if len(data) < 6 || data[0] > 1 {
					return fmt.Errorf("无效的主图属性")
				}
				id := uint32(binary.BigEndian.Uint16(data[4:]))
				if data[0] == 1 {
					if len(data) < 8 {
						return io.ErrUnexpectedEOF
					}
					id = binary.BigEndian.Uint32(data[4:])
				}
				if primary != 0 && primary != id {
					return fmt.Errorf("主图标识冲突")
				}
				primary = id
			case "ipma":
				data, err := read(b)
				if err != nil {
					return err
				}
				if len(data) < 8 || data[0] > 1 {
					return fmt.Errorf("无效的属性关联表")
				}
				idSize, associationSize := 2, 1
				if data[0] == 1 {
					idSize = 4
				}
				if data[3]&1 != 0 {
					associationSize = 2
				}
				count := binary.BigEndian.Uint32(data[4:])
				pos := 8
				for i := uint32(0); i < count; i++ {
					if len(data)-pos < idSize+1 {
						return io.ErrUnexpectedEOF
					}
					id := uint32(binary.BigEndian.Uint16(data[pos:]))
					if idSize == 4 {
						id = binary.BigEndian.Uint32(data[pos:])
					}
					pos += idSize
					n := int(data[pos])
					pos++
					if len(data)-pos < n*associationSize {
						return io.ErrUnexpectedEOF
					}
					for j := 0; j < n; j++ {
						index := int(data[pos] & 0x7f)
						if associationSize == 2 {
							index = int(binary.BigEndian.Uint16(data[pos:]) & 0x7fff)
						}
						pos += associationSize
						if index != 0 {
							associations[id] = append(associations[id], index)
						}
					}
				}
			case "iref":
				var header [4]byte
				if b.size-b.header < 4 {
					return io.ErrUnexpectedEOF
				}
				if _, err := r.ReadAt(header[:], payload); err != nil {
					return err
				}
				if header[0] > 1 {
					return fmt.Errorf("无效的图像引用版本")
				}
				return walkBMFF(r, payload+4, b.start+b.size, func(ref bmffBox) error {
					if ref.kind != "dimg" {
						return nil
					}
					data, err := read(ref)
					if err != nil {
						return err
					}
					width := 2
					if header[0] == 1 {
						width = 4
					}
					if len(data) < width+2 {
						return io.ErrUnexpectedEOF
					}
					idAt := func(p int) uint32 {
						if width == 4 {
							return binary.BigEndian.Uint32(data[p:])
						}
						return uint32(binary.BigEndian.Uint16(data[p:]))
					}
					id := idAt(0)
					n := int(binary.BigEndian.Uint16(data[width:]))
					if len(data)-width-2 < n*width {
						return io.ErrUnexpectedEOF
					}
					for i := 0; i < n; i++ {
						derived[id] = append(derived[id], idAt(width+2+i*width))
					}
					return nil
				})
			}
			return nil
		})
	}
	if err := scan(0, size, 0); err != nil {
		return nil, nil, err
	}
	type colors struct{ icc, nclx []byte }
	merge := func(dst *colors, src colors) error {
		if len(dst.icc) > 0 && len(src.icc) > 0 && !bytes.Equal(dst.icc, src.icc) || len(dst.nclx) > 0 && len(src.nclx) > 0 && !bytes.Equal(dst.nclx, src.nclx) {
			return fmt.Errorf("主图关联的色彩属性冲突")
		}
		if len(src.icc) > 0 {
			dst.icc = src.icc
		}
		if len(src.nclx) > 0 {
			dst.nclx = src.nclx
		}
		return nil
	}
	property := func(p bmffBox) (colors, error) {
		if p.kind != "colr" {
			return colors{}, nil
		}
		icc, nclx, err := readColorProperty(io.NewSectionReader(r, p.start, p.size), p.size)
		return colors{icc, nclx}, err
	}
	var resolve func(uint32, int) (colors, error)
	resolved := map[uint32]colors{}
	visiting := map[uint32]bool{}
	resolve = func(id uint32, depth int) (colors, error) {
		if result, ok := resolved[id]; ok {
			return result, nil
		}
		var result colors
		if depth > 8 || visiting[id] {
			return result, fmt.Errorf("图像引用循环或嵌套过深")
		}
		visiting[id] = true
		defer delete(visiting, id)
		for _, index := range associations[id] {
			if index > len(properties) {
				return result, fmt.Errorf("属性关联索引越界")
			}
			c, err := property(properties[index-1])
			if err != nil {
				return result, err
			}
			if err = merge(&result, c); err != nil {
				return result, err
			}
		}
		if len(result.icc)+len(result.nclx) == 0 {
			for _, child := range derived[id] {
				c, err := resolve(child, depth+1)
				if err != nil {
					return result, err
				}
				if err = merge(&result, c); err != nil {
					return result, err
				}
			}
		}
		resolved[id] = result
		return result, nil
	}
	var result colors
	var err error
	if primary != 0 {
		result, err = resolve(primary, 0)
	} else {
		// 兼容无主图标记的单属性容器；冲突配置不可猜测。
		for _, p := range properties {
			var c colors
			c, err = property(p)
			if err != nil {
				break
			}
			err = merge(&result, c)
			if err != nil {
				break
			}
		}
	}
	return result.icc, result.nclx, err
}

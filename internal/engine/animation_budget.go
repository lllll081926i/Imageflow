package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

const (
	MaxAnimationBytes  = 64 << 20
	MaxAnimationFrames = 500
	MaxAnimationPixels = 32_000_000
)

// readAnimation 在解码前核验容器、帧数及累计完整画布像素。
func readAnimation(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxAnimationBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxAnimationBytes {
		return nil, fmt.Errorf("动图源文件超过 %d MiB 限制", MaxAnimationBytes>>20)
	}
	if err := checkAnimation(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func animationSize(width, height, frames int) error {
	if err := output.CheckSize(width, height); err != nil {
		return err
	}
	if frames < 1 || frames > MaxAnimationFrames || int64(width)*int64(height)*int64(frames) > MaxAnimationPixels {
		return fmt.Errorf("动图超过帧数或累计像素限制（最多 %d 帧、%d 像素）", MaxAnimationFrames, MaxAnimationPixels)
	}
	return nil
}

func checkAnimation(raw []byte) error {
	bad := fmt.Errorf("无效或截断的动图容器")
	width, height, frames := 0, 0, 0
	add := func(w, h int) error {
		if w < 1 || h < 1 || w > width || h > height {
			return bad
		}
		frames++
		return animationSize(width, height, frames)
	}
	switch {
	case len(raw) >= 13 && (string(raw[:6]) == "GIF89a" || string(raw[:6]) == "GIF87a"):
		width, height = int(binary.LittleEndian.Uint16(raw[6:8])), int(binary.LittleEndian.Uint16(raw[8:10]))
		if err := animationSize(width, height, 1); err != nil {
			return err
		}
		pos := 13
		if raw[10]&128 != 0 {
			pos += 3 * (1 << ((raw[10] & 7) + 1))
		}
		skipBlocks := func() bool {
			for pos < len(raw) {
				n := int(raw[pos])
				pos++
				if n == 0 {
					return true
				}
				if n > len(raw)-pos {
					return false
				}
				pos += n
			}
			return false
		}
		for pos < len(raw) {
			kind := raw[pos]
			pos++
			switch kind {
			case 0x3b:
				return animationSize(width, height, frames)
			case 0x21:
				if pos >= len(raw) {
					return bad
				}
				pos++
				if !skipBlocks() {
					return bad
				}
			case 0x2c:
				if pos+9 > len(raw) {
					return bad
				}
				x, y := int(binary.LittleEndian.Uint16(raw[pos:pos+2])), int(binary.LittleEndian.Uint16(raw[pos+2:pos+4]))
				w, h := int(binary.LittleEndian.Uint16(raw[pos+4:pos+6])), int(binary.LittleEndian.Uint16(raw[pos+6:pos+8]))
				if x+w > width || y+h > height {
					return bad
				}
				if err := add(w, h); err != nil {
					return err
				}
				flags := raw[pos+8]
				pos += 9
				if flags&128 != 0 {
					pos += 3 * (1 << ((flags & 7) + 1))
				}
				if pos >= len(raw) {
					return bad
				}
				pos++
				if !skipBlocks() {
					return bad
				}
			default:
				return bad
			}
		}
		return bad
	case len(raw) >= 8 && bytes.Equal(raw[:8], []byte("\x89PNG\r\n\x1a\n")):
		declared := 0
		for pos := 8; pos+12 <= len(raw); {
			n := int64(binary.BigEndian.Uint32(raw[pos : pos+4]))
			if n > int64(len(raw)-pos-12) {
				return bad
			}
			kind := string(raw[pos+4 : pos+8])
			data := raw[pos+8 : pos+8+int(n)]
			switch kind {
			case "IHDR":
				if len(data) != 13 {
					return bad
				}
				width, height = int(binary.BigEndian.Uint32(data)), int(binary.BigEndian.Uint32(data[4:]))
				if err := animationSize(width, height, 1); err != nil {
					return err
				}
			case "acTL":
				if len(data) != 8 {
					return bad
				}
				declared = int(binary.BigEndian.Uint32(data))
				if err := animationSize(width, height, declared); err != nil {
					return err
				}
			case "fcTL":
				if len(data) != 26 {
					return bad
				}
				w, h := int(binary.BigEndian.Uint32(data[4:])), int(binary.BigEndian.Uint32(data[8:]))
				x, y := int64(binary.BigEndian.Uint32(data[12:])), int64(binary.BigEndian.Uint32(data[16:]))
				if x+int64(w) > int64(width) || y+int64(h) > int64(height) {
					return bad
				}
				if err := add(w, h); err != nil {
					return err
				}
			case "IEND":
				if declared == 0 {
					return fmt.Errorf("不是动图: PNG 缺少 acTL 块")
				}
				if declared != frames {
					return bad
				}
				return nil
			}
			pos += int(n) + 12
		}
		return bad
	case len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP":
		u24 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
		for pos := 12; pos+8 <= len(raw); {
			n := int64(binary.LittleEndian.Uint32(raw[pos+4 : pos+8]))
			if n > int64(len(raw)-pos-8) {
				return bad
			}
			data := raw[pos+8 : pos+8+int(n)]
			switch string(raw[pos : pos+4]) {
			case "VP8X":
				if len(data) != 10 {
					return bad
				}
				width, height = u24(data[4:])+1, u24(data[7:])+1
				if err := animationSize(width, height, 1); err != nil {
					return err
				}
			case "ANMF":
				if len(data) < 16 {
					return bad
				}
				w, h := u24(data[6:])+1, u24(data[9:])+1
				if u24(data)*2+w > width || u24(data[3:])*2+h > height {
					return bad
				}
				if err := add(w, h); err != nil {
					return err
				}
			}
			pos += 8 + int(n) + int(n%2)
		}
		if frames == 0 {
			return fmt.Errorf("不是动图: WebP 缺少动画帧")
		}
		return animationSize(width, height, frames)
	}
	return fmt.Errorf("不是支持的动图格式")
}

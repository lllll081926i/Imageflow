package codec

// meta.go 提供 JPEG 元数据和 EXIF 方向处理。
// 跨格式 ICC / EXIF / XMP 的保留策略见 color_metadata.go。

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
)

// exifOrientationTag is the TIFF tag that rotates the pixels on display.
const exifOrientationTag = 0x0112

// metadataSegments returns the APP1/APP2 segments worth copying from a JPEG.
func metadataSegments(path string) [][]byte {
	if ext := extOf(path); ext != ".jpg" && ext != ".jpeg" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return nil
	}
	var out [][]byte
	for i := 2; i+4 <= len(raw); {
		if raw[i] != 0xFF {
			break
		}
		marker := raw[i+1]
		// Start of scan: the frame data follows, stop walking.
		if marker == 0xDA || marker == 0xD9 {
			break
		}
		// Standalone markers carry no length.
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
			i += 2
			continue
		}
		if i+4 > len(raw) {
			break
		}
		length := int(binary.BigEndian.Uint16(raw[i+2 : i+4]))
		if length < 2 || i+2+length > len(raw) {
			break
		}
		payload := raw[i+4 : i+2+length]
		segment := raw[i : i+2+length]
		switch {
		case marker == 0xE1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")):
			out = append(out, withOrientationReset(segment))
		case marker == 0xE2 && bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00")):
			out = append(out, segment)
		case marker == 0xE1 && bytes.HasPrefix(payload, []byte("http://ns.adobe.com/xap/1.0/\x00")):
			out = append(out, segment)
		}
		i += 2 + length
	}
	return out
}

// withOrientationReset rewrites the EXIF orientation tag to 1 (upright).
//
// The decoder already rotated the pixels, so copying the source's orientation
// value would rotate the result a second time in every viewer. When the tag
// cannot be located the segment is dropped rather than copied unsafe.
func withOrientationReset(segment []byte) []byte {
	if len(segment) < 10 {
		return nil
	}
	// segment = FF E1 LL LL + payload; payload starts with "Exif\0\0".
	tiff := segment[10:]
	if len(tiff) < 8 {
		return nil
	}
	var order binary.ByteOrder
	switch {
	case bytes.HasPrefix(tiff, []byte("II")):
		order = binary.LittleEndian
	case bytes.HasPrefix(tiff, []byte("MM")):
		order = binary.BigEndian
	default:
		return nil
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return nil
	}
	ifdOffset := int(order.Uint32(tiff[4:8]))
	if ifdOffset < 8 || ifdOffset+2 > len(tiff) {
		return nil
	}
	count := int(order.Uint16(tiff[ifdOffset : ifdOffset+2]))
	found := false
	for i := 0; i < count; i++ {
		entry := ifdOffset + 2 + i*12
		if entry+12 > len(tiff) {
			return nil
		}
		if order.Uint16(tiff[entry:entry+2]) != exifOrientationTag {
			continue
		}
		// SHORT value 1 in the first two bytes of the value field.
		order.PutUint16(tiff[entry+8:entry+10], 1)
		found = true
		break
	}
	if !found {
		return append([]byte(nil), segment...)
	}
	out := make([]byte, len(segment))
	copy(out, segment)
	return out
}

// spliceJPEGMetadata inserts segments right after the SOI marker of an
// already encoded JPEG.
func spliceJPEGMetadata(encoded []byte, segments [][]byte) []byte {
	if len(segments) == 0 || len(encoded) < 2 || encoded[0] != 0xFF || encoded[1] != 0xD8 {
		return encoded
	}
	total := 2
	for _, s := range segments {
		total += len(s)
	}
	out := make([]byte, 0, len(encoded)+total)
	out = append(out, encoded[:2]...)
	for _, s := range segments {
		out = append(out, s...)
	}
	out = append(out, encoded[2:]...)
	return out
}

// extOf returns the lower-case extension including the dot.
func extOf(path string) string {
	i := strings.LastIndexByte(path, '.')
	if i < 0 {
		return ""
	}
	return strings.ToLower(path[i:])
}

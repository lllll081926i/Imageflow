package codec

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/colorprofile"
)

// OpenSRGB 在合成、图标和 PDF 输出前将源像素统一到 sRGB。
// 普通单图转换仍通过 Open 保留原色彩空间。
func OpenSRGB(path string) (*OpenResult, error) {
	if _, _, _, err := Probe(path); err != nil {
		return nil, err
	}
	m, err := readMetadata(path)
	if err != nil {
		return nil, err
	}
	opened, err := Open(path)
	if err != nil {
		return nil, err
	}
	if opened.Normalized {
		return opened, nil
	}
	img, err := colorprofile.ToSRGB(opened.Image, m.icc)
	if err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	opened.Image = img
	opened.Normalized = true
	return opened, nil
}

func normalizesSource(m imageMetadata) (bool, error) {
	if len(m.icc) >= 20 {
		return string(m.icc[16:20]) == "CMYK" || string(m.icc[16:20]) == "GRAY", nil
	}
	e, err := workingEncoding(m)
	return e != nil, err
}

func workingEncoding(m imageMetadata) (*colorprofile.Encoding, error) {
	if len(m.icc) > 0 {
		return nil, nil
	}
	if len(m.nclx) > 0 || len(m.cicp) > 0 {
		var p, t uint16
		if len(m.nclx) > 0 {
			p, t = binary.BigEndian.Uint16(m.nclx), binary.BigEndian.Uint16(m.nclx[2:])
		} else {
			if m.cicp[2] != 0 || m.cicp[3] != 1 {
				return nil, fmt.Errorf("PNG cICP 必须描述全范围 RGB 像素")
			}
			p, t = uint16(m.cicp[0]), uint16(m.cicp[1])
		}
		e, err := colorprofile.CICP(p, t)
		if err != nil {
			return nil, err
		}
		if (p == 1 || p == 2) && (t == 2 || t == 13) {
			return nil, nil
		}
		return &e, nil
	}
	if m.srgb || len(m.gamma)+len(m.chromaticities) == 0 {
		return nil, nil
	}
	e := colorprofile.Encoding{Primaries: colorprofile.SRGBPrimaries, Transfer: 13}
	if len(m.gamma) > 0 {
		g := binary.BigEndian.Uint32(m.gamma)
		if g == 0 {
			return nil, fmt.Errorf("PNG gAMA 不能为零")
		}
		e.Gamma = float64(g) / 100000
	}
	if len(m.chromaticities) > 0 {
		xy := func(i int) [2]float64 {
			return [2]float64{float64(binary.BigEndian.Uint32(m.chromaticities[i*8:])) / 100000, float64(binary.BigEndian.Uint32(m.chromaticities[i*8+4:])) / 100000}
		}
		e.Primaries = colorprofile.Chromaticities{White: xy(0), Red: xy(1), Green: xy(2), Blue: xy(3)}
	}
	return &e, nil
}

func srgbMetadata(data []byte, format string) ([]byte, error) {
	m := imageMetadata{icc: colorprofile.SRGB()}
	switch strings.TrimPrefix(strings.ToLower(format), ".") {
	case "jpg", "jpeg":
		return jpegMetadata(data, m)
	case "png":
		return pngMetadata(data, m)
	case "webp":
		return webpMetadata(data, m)
	case "avif":
		return avifICC(data, m.icc)
	case "tiff", "tif":
		return tiffICC(data, m.icc)
	default:
		return data, nil
	}
}

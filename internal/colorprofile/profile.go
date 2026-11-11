// Package colorprofile 将 ICC、PNG 和 HDR 色彩转换到统一的 sRGB 工作空间。
package colorprofile

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
)

//go:embed profiles/sRGB-v2-magic.icc
var srgb []byte

// SRGB 返回独立的标准 sRGB ICC 数据。
func SRGB() []byte { return append([]byte(nil), srgb...) }

// ToSRGB 通过 Little CMS 转换 RGB、CMYK 和灰度 ICC。
func ToSRGB(img image.Image, profile []byte) (image.Image, error) {
	if len(profile) == 0 {
		return img, nil
	}
	if len(profile) < 132 || len(profile) > 4<<20 || int(binary.BigEndian.Uint32(profile)) != len(profile) || string(profile[36:40]) != "acsp" {
		return nil, fmt.Errorf("ICC 色彩配置无效")
	}
	switch string(profile[16:20]) {
	case "RGB ", "CMYK", "GRAY":
	default:
		return nil, fmt.Errorf("无法解释 ICC 通道空间 %q", profile[16:20])
	}
	if bytes.Equal(profile, srgb) {
		return img, nil
	}
	return convert(img, profile)
}

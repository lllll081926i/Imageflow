package colorprofile

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"github.com/tetratelabs/wazero/api"
)

func cmsCall(ctx context.Context, m api.Module, name string, args ...uint64) (uint64, error) {
	f := m.ExportedFunction(name)
	if f == nil {
		return 0, fmt.Errorf("Little CMS 缺少函数 %s", name)
	}
	result, err := f.Call(ctx, args...)
	if err != nil {
		return 0, fmt.Errorf("Little CMS %s：%w", name, err)
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0], nil
}

func convert(img image.Image, profile []byte) (image.Image, error) {
	ctx, m, cleanup, err := newLCMS()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	alloc := func(n int) (uint64, error) {
		p, err := cmsCall(ctx, m, "malloc", uint64(n))
		if err == nil && p == 0 {
			err = fmt.Errorf("Little CMS 内存不足")
		}
		return p, err
	}
	ptr, err := alloc(len(profile))
	if err != nil {
		return nil, err
	}
	if !m.Memory().Write(uint32(ptr), profile) {
		return nil, fmt.Errorf("ICC 写入超出 WASM 内存")
	}
	src, err := cmsCall(ctx, m, "cmsOpenProfileFromMem", ptr, uint64(len(profile)))
	if err != nil {
		return nil, err
	}
	if src == 0 {
		return nil, fmt.Errorf("Little CMS 无法读取源 ICC")
	}
	dst, err := cmsCall(ctx, m, "cmsCreate_sRGBProfile")
	if err != nil {
		return nil, err
	}
	if dst == 0 {
		return nil, fmt.Errorf("无法创建 sRGB 配置")
	}
	// TYPE_RGB_16 / TYPE_GRAY_16 / TYPE_CMYK_16；使用非预乘的原始通道。
	channels, pixelType := 3, uint64(4<<16|3<<3|2)
	switch string(profile[16:20]) {
	case "CMYK":
		channels, pixelType = 4, 6<<16|4<<3|2
	case "GRAY":
		channels, pixelType = 1, 3<<16|1<<3|2
	}
	if channels == 4 {
		if _, ok := img.(*image.CMYK); !ok {
			return nil, fmt.Errorf("CMYK ICC 必须与原始 CMYK 像素匹配")
		}
	}
	// 感知意图可使用只有 A2B0 的印刷配置；RGB 使用相对色度意图。
	intent := uint64(1)
	if channels == 4 {
		intent = 0
	}
	transform, err := cmsCall(ctx, m, "cmsCreateTransform", src, pixelType, dst, 4<<16|3<<3|2, intent, 0x2000)
	if err != nil {
		return nil, err
	}
	if transform == 0 {
		return nil, fmt.Errorf("Little CMS 无法创建色彩转换")
	}
	b := img.Bounds()
	out := image.NewNRGBA64(image.Rect(0, 0, b.Dx(), b.Dy()))
	if b.Empty() {
		return out, nil
	}
	input, output := make([]byte, b.Dx()*channels*2), make([]byte, b.Dx()*6)
	inPtr, err := alloc(len(input))
	if err != nil {
		return nil, err
	}
	outPtr, err := alloc(len(output))
	if err != nil {
		return nil, err
	}
	for y := 0; y < b.Dy(); y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < b.Dx(); x++ {
			c := straightPixel(img, b.Min.X+x, b.Min.Y+y)
			values := [4]uint16{c.R, c.G, c.B}
			if channels == 4 {
				p := img.(*image.CMYK).CMYKAt(b.Min.X+x, b.Min.Y+y)
				values = [4]uint16{uint16(p.C) * 257, uint16(p.M) * 257, uint16(p.Y) * 257, uint16(p.K) * 257}
			}
			if channels == 1 {
				values[0] = color.Gray16Model.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray16).Y
			}
			for n := 0; n < channels; n++ {
				binary.LittleEndian.PutUint16(input[(x*channels+n)*2:], values[n])
			}
			binary.BigEndian.PutUint16(out.Pix[y*out.Stride+x*8+6:], c.A)
		}
		if !m.Memory().Write(uint32(inPtr), input) {
			return nil, fmt.Errorf("WASM 输入缓冲区越界")
		}
		if _, err := cmsCall(ctx, m, "cmsDoTransform", transform, inPtr, outPtr, uint64(b.Dx())); err != nil {
			return nil, err
		}
		data, ok := m.Memory().Read(uint32(outPtr), uint32(len(output)))
		if !ok {
			return nil, fmt.Errorf("WASM 输出缓冲区越界")
		}
		copy(output, data)
		for x := 0; x < b.Dx(); x++ {
			for n := 0; n < 3; n++ {
				binary.BigEndian.PutUint16(out.Pix[y*out.Stride+x*8+n*2:], binary.LittleEndian.Uint16(output[x*6+n*2:]))
			}
		}
	}
	return out, nil
}

func straightPixel(img image.Image, x, y int) color.NRGBA64 {
	switch src := img.(type) {
	case *image.NRGBA:
		c := src.NRGBAAt(x, y)
		return color.NRGBA64{uint16(c.R) * 257, uint16(c.G) * 257, uint16(c.B) * 257, uint16(c.A) * 257}
	case *image.NRGBA64:
		return src.NRGBA64At(x, y)
	default:
		return color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64)
	}
}

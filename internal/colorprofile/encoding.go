package colorprofile

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

type Chromaticities struct{ White, Red, Green, Blue [2]float64 }
type Encoding struct {
	Primaries Chromaticities
	Transfer  uint16
	Gamma     float64
}

var SRGBPrimaries = Chromaticities{[2]float64{.3127, .3290}, [2]float64{.64, .33}, [2]float64{.30, .60}, [2]float64{.15, .06}}

// CICP 使用 ITU-T H.273 的原色及传递函数编号。
func CICP(primaries, transfer uint16) (Encoding, error) {
	e := Encoding{Primaries: SRGBPrimaries, Transfer: transfer}
	switch primaries {
	case 1, 2:
	case 9:
		e.Primaries = Chromaticities{[2]float64{.3127, .329}, [2]float64{.708, .292}, [2]float64{.170, .797}, [2]float64{.131, .046}}
	case 12:
		e.Primaries = Chromaticities{[2]float64{.3127, .329}, [2]float64{.68, .32}, [2]float64{.265, .69}, [2]float64{.15, .06}}
	case 11:
		e.Primaries = Chromaticities{[2]float64{.314, .351}, [2]float64{.68, .32}, [2]float64{.265, .69}, [2]float64{.15, .06}}
	case 5:
		e.Primaries = Chromaticities{[2]float64{.3127, .329}, [2]float64{.64, .33}, [2]float64{.29, .60}, [2]float64{.15, .06}}
	case 6, 7:
		e.Primaries = Chromaticities{[2]float64{.3127, .329}, [2]float64{.630, .340}, [2]float64{.310, .595}, [2]float64{.155, .070}}
	default:
		return e, fmt.Errorf("无效或保留的 CICP 原色编号 %d", primaries)
	}
	switch transfer {
	case 1, 2, 4, 5, 6, 8, 13, 14, 15, 16, 17, 18:
	default:
		return e, fmt.Errorf("无效或保留的 CICP 传递函数 %d", transfer)
	}
	return e, nil
}

type matrix [3][3]float64

func (a matrix) vec(v [3]float64) (r [3]float64) {
	for i := range 3 {
		for j := range 3 {
			r[i] += a[i][j] * v[j]
		}
	}
	return
}
func (a matrix) mul(b matrix) (r matrix) {
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				r[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return
}
func (a matrix) inverse() (matrix, error) {
	d := a[0][0]*(a[1][1]*a[2][2]-a[1][2]*a[2][1]) - a[0][1]*(a[1][0]*a[2][2]-a[1][2]*a[2][0]) + a[0][2]*(a[1][0]*a[2][1]-a[1][1]*a[2][0])
	if math.Abs(d) < 1e-10 || math.IsNaN(d) || math.IsInf(d, 0) {
		return matrix{}, fmt.Errorf("色彩原色矩阵不可逆")
	}
	var r matrix
	for i := range 3 {
		for j := range 3 {
			r[j][i] = (a[(i+1)%3][(j+1)%3]*a[(i+2)%3][(j+2)%3] - a[(i+1)%3][(j+2)%3]*a[(i+2)%3][(j+1)%3]) / d
		}
	}
	return r, nil
}
func xyz(xy [2]float64) ([3]float64, error) {
	if xy[0] < 0 || xy[1] <= 0 || xy[0]+xy[1] > 1.00001 || math.IsNaN(xy[0]+xy[1]) {
		return [3]float64{}, fmt.Errorf("无效的色彩色度坐标")
	}
	return [3]float64{xy[0] / xy[1], 1, (1 - xy[0] - xy[1]) / xy[1]}, nil
}
func rgbMatrix(c Chromaticities) (matrix, error) {
	w, err := xyz(c.White)
	if err != nil {
		return matrix{}, err
	}
	var m matrix
	for j, xy := range [][2]float64{c.Red, c.Green, c.Blue} {
		v, err := xyz(xy)
		if err != nil {
			return m, err
		}
		for i := range 3 {
			m[i][j] = v[i]
		}
	}
	inv, err := m.inverse()
	if err != nil {
		return m, err
	}
	scale := inv.vec(w)
	for i := range 3 {
		for j := range 3 {
			m[i][j] *= scale[j]
		}
	}
	return m, nil
}
func conversionMatrix(c Chromaticities) (matrix, error) {
	src, err := rgbMatrix(c)
	if err != nil {
		return matrix{}, err
	}
	dst, err := rgbMatrix(SRGBPrimaries)
	if err != nil {
		return matrix{}, err
	}
	inv, err := dst.inverse()
	if err != nil {
		return matrix{}, err
	}
	bradford := matrix{{.8951, .2664, -.1614}, {-.7502, 1.7135, .0367}, {.0389, -.0685, 1.0296}}
	binv, _ := bradford.inverse()
	w1, err := xyz(c.White)
	if err != nil {
		return matrix{}, err
	}
	w2, _ := xyz(SRGBPrimaries.White)
	s, d := bradford.vec(w1), bradford.vec(w2)
	var scale matrix
	for i := range 3 {
		if math.Abs(s[i]) < 1e-9 {
			return matrix{}, fmt.Errorf("无效的色彩白点")
		}
		scale[i][i] = d[i] / s[i]
	}
	return inv.mul(binv.mul(scale.mul(bradford))).mul(src), nil
}

func (e Encoding) linear(v float64) float64 {
	if e.Gamma > 0 {
		return math.Pow(v, 1/e.Gamma)
	}
	switch e.Transfer {
	case 16:
		const m1 = 2610.0 / 16384
		const m2 = 2523.0 / 32
		const c1 = 3424.0 / 4096
		const c2 = 2413.0 / 128
		const c3 = 2392.0 / 128
		p := math.Pow(v, 1/m2)
		return 10000 * math.Pow(math.Max(p-c1, 0)/(c2-c3*p), 1/m1)
	case 18:
		const a = .17883277
		const b = 1 - 4*a
		const c = .55991073
		if v <= .5 {
			return v * v / 3
		}
		return (math.Exp((v-c)/a) + b) / 12
	case 1, 6, 14, 15:
		if v < .081 {
			return v / 4.5
		}
		return math.Pow((v+.099)/1.099, 1/.45)
	case 4:
		return math.Pow(v, 2.2)
	case 5:
		return math.Pow(v, 2.8)
	case 8:
		return v
	case 17:
		return math.Pow(v, 2.6)
	default:
		if v <= .04045 {
			return v / 12.92
		}
		return math.Pow((v+.055)/1.055, 2.4)
	}
}
func srgbEncode(v float64) uint16 {
	v = math.Max(0, math.Min(1, v))
	if v <= .0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - .055
	}
	return uint16(math.Round(v * 65535))
}

// DecodeEncoding 将 PNG / CICP 描述的像素转换到 sRGB。HDR 输出为 SDR。
// PQ 采用 ST 2084 绝对亮度；HLG 采用 BT.2100 的 1000 nit、1.2 系统伽马。
func DecodeEncoding(img image.Image, e Encoding) (image.Image, error) {
	if e.Gamma < 0 || math.IsNaN(e.Gamma) || math.IsInf(e.Gamma, 0) {
		return nil, fmt.Errorf("无效的 PNG 伽马")
	}
	m, err := conversionMatrix(e.Primaries)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	hdr := e.Transfer == 16 || e.Transfer == 18
	var curve [65536]float64
	for i := range curve {
		curve[i] = e.linear(float64(i) / 65535)
	}
	linear := func(c color.NRGBA64) [3]float64 {
		v := m.vec([3]float64{curve[c.R], curve[c.G], curve[c.B]})
		if e.Transfer == 18 {
			y := math.Max(0, .2126*v[0]+.7152*v[1]+.0722*v[2])
			s := 1000 * math.Pow(y, .2)
			for i := range 3 {
				v[i] *= s
			}
		}
		return v
	}
	// 依据实际峰值确定扩展 Reinhard 白点，避免依赖缺失的 HDR 峰值标签。
	peak := 203.0
	if hdr {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := straightPixel(img, x, y)
				if c.A == 0 {
					continue
				}
				v := linear(c)
				peak = math.Max(peak, .2126*v[0]+.7152*v[1]+.0722*v[2])
			}
		}
	}
	white := math.Min(10000, peak) / 203
	out := image.NewNRGBA64(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := straightPixel(img, b.Min.X+x, b.Min.Y+y)
			v := linear(c)
			if hdr {
				lum := math.Max(0, .2126*v[0]+.7152*v[1]+.0722*v[2]) / 203
				s := 0.0
				if lum > 0 {
					s = (1 + lum/(white*white)) / (1 + lum) / 203
				}
				for i := range 3 {
					v[i] *= s
				}
			}
			// 超出 sRGB 时按最大通道压缩，保留高光层次。
			if hdr {
				maxv := math.Max(v[0], math.Max(v[1], v[2]))
				if maxv > 1 {
					for i := range 3 {
						v[i] /= maxv
					}
				}
			}
			out.SetNRGBA64(x, y, color.NRGBA64{srgbEncode(v[0]), srgbEncode(v[1]), srgbEncode(v[2]), c.A})
		}
	}
	return out, nil
}

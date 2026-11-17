package engine

// lut.go parses 3D `.cube` lookup tables and applies them.
//
// The PRD asks for a `.cube` import with a strength control and forbids accepting
// a table whose header and data disagree. The format is plain text: a `.cube`
// header carries the title, the LUT size and the domain, then size³ lines of
// "r g b" triples ordered with red varying fastest.

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// LUTMinSize and LUTMaxSize bound the accepted table. A 64³ table is already
// 262 144 lines; anything larger is either a mistake or an attempt to exhaust
// memory, so it is refused with a readable error.
const (
	LUTMinSize = 2
	LUTMaxSize = 128
)

// LUT3D is one parsed cube table.
type LUT3D struct {
	Title string
	// Size is the edge length of the cube.
	Size int
	// Domain is the input range the table expects, 0..1 by default.
	DomainMin [3]float64
	DomainMax [3]float64
	// Data holds Size³ RGB triples in 0..1, red varying fastest.
	Data []float32
}

// Index returns the offset of one lattice point.
func (l *LUT3D) Index(r, g, b int) int {
	return ((b*l.Size)+g)*l.Size + r
}

// Apply maps one colour through the table with trilinear interpolation.
//
// A nearest-neighbour lookup would show visible steps on a smooth gradient, which
// is exactly the artefact a grading LUT is used to remove.
func (l *LUT3D) Apply(r, g, b float64) (float64, float64, float64) {
	if l == nil || l.Size < LUTMinSize || len(l.Data) < l.Size*l.Size*l.Size*3 {
		return r, g, b
	}
	norm := func(v float64, axis int) float64 {
		lo, hi := l.DomainMin[axis], l.DomainMax[axis]
		if hi <= lo {
			lo, hi = 0, 1
		}
		return clampF((v/255-lo)/(hi-lo), 0, 1)
	}
	fr, fg, fb := norm(r, 0), norm(g, 1), norm(b, 2)
	last := l.Size - 1
	sr, sg, sb := fr*float64(last), fg*float64(last), fb*float64(last)
	r0 := int(math.Floor(sr))
	g0 := int(math.Floor(sg))
	b0 := int(math.Floor(sb))
	r1 := minIntG(r0+1, last)
	g1 := minIntG(g0+1, last)
	b1 := minIntG(b0+1, last)
	tr, tg, tb := sr-float64(r0), sg-float64(g0), sb-float64(b0)

	sample := func(ri, gi, bi int, ch int) float64 {
		return float64(l.Data[l.Index(ri, gi, bi)*3+ch])
	}
	lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
	out := [3]float64{}
	for ch := 0; ch < 3; ch++ {
		c00 := lerp(sample(r0, g0, b0, ch), sample(r0, g0, b1, ch), tb)
		c01 := lerp(sample(r0, g1, b0, ch), sample(r0, g1, b1, ch), tb)
		c10 := lerp(sample(r1, g0, b0, ch), sample(r1, g0, b1, ch), tb)
		c11 := lerp(sample(r1, g1, b0, ch), sample(r1, g1, b1, ch), tb)
		c0 := lerp(c00, c01, tg)
		c1 := lerp(c10, c11, tg)
		out[ch] = lerp(c0, c1, tr) * 255
	}
	return out[0], out[1], out[2]
}

func minIntG(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LoadLUT reads a `.cube` file.
func LoadLUT(path string) (*LUT3D, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseLUT(f)
}

// ParseLUT parses a `.cube` document from r.
//
// The parser is deliberately strict about the two things that make a table
// unusable: the declared size must match the sample count, and no sample may be
// NaN or infinite.
func ParseLUT(r io.Reader) (*LUT3D, error) {
	lut := &LUT3D{DomainMin: [3]float64{0, 0, 0}, DomainMax: [3]float64{1, 1, 1}}
	declared := 0
	scanner := bufio.NewScanner(r)
	// A 128³ table is 2M lines; the default 64 KiB token limit is enough per line
	// but the buffer is raised anyway so an unusually wide header line does not
	// abort the import.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		upper := strings.ToUpper(text)
		switch {
		case strings.HasPrefix(upper, "TITLE"):
			if i := strings.IndexByte(text, '"'); i >= 0 {
				if j := strings.LastIndexByte(text, '"'); j > i {
					lut.Title = text[i+1 : j]
				}
			}
			continue
		case strings.HasPrefix(upper, "LUT_3D_SIZE"):
			if declared != 0 {
				return nil, fmt.Errorf("第 %d 行：重复声明 LUT_3D_SIZE", line)
			}
			fields := strings.Fields(text)
			if len(fields) != 2 {
				return nil, fmt.Errorf("第 %d 行：LUT_3D_SIZE 缺少数值", line)
			}
			size, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("第 %d 行：LUT_3D_SIZE 不是整数：%v", line, err)
			}
			if size < LUTMinSize || size > LUTMaxSize {
				return nil, fmt.Errorf("LUT 尺寸 %d 超出 %d–%d 的支持范围", size, LUTMinSize, LUTMaxSize)
			}
			declared = size
			continue
		case strings.HasPrefix(upper, "LUT_1D_SIZE"):
			return nil, fmt.Errorf("只支持 3D LUT，文件声明的是 1D 表")
		case strings.HasPrefix(upper, "DOMAIN_MIN"):
			if err := parseTriple(text, &lut.DomainMin); err != nil {
				return nil, fmt.Errorf("第 %d 行：%v", line, err)
			}
			continue
		case strings.HasPrefix(upper, "DOMAIN_MAX"):
			if err := parseTriple(text, &lut.DomainMax); err != nil {
				return nil, fmt.Errorf("第 %d 行：%v", line, err)
			}
			continue
		}
		if declared == 0 {
			return nil, fmt.Errorf("第 %d 行：数据出现在 LUT_3D_SIZE 之前", line)
		}
		if len(lut.Data) >= declared*declared*declared*3 {
			return nil, fmt.Errorf("第 %d 行：样本数超过 LUT 声明尺寸", line)
		}
		fields := strings.Fields(text)
		if len(fields) != 3 {
			return nil, fmt.Errorf("第 %d 行：需要 3 个分量，实际 %d 个", line, len(fields))
		}
		for _, field := range fields {
			v, err := strconv.ParseFloat(field, 64)
			if err != nil {
				return nil, fmt.Errorf("第 %d 行：分量 %q 不是数字", line, field)
			}
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > math.MaxFloat32 {
				return nil, fmt.Errorf("第 %d 行：分量 %q 不是有限数值", line, field)
			}
			lut.Data = append(lut.Data, float32(v))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if declared == 0 {
		return nil, fmt.Errorf("缺少 LUT_3D_SIZE")
	}
	want := declared * declared * declared * 3
	if len(lut.Data) != want {
		return nil, fmt.Errorf("LUT 声明 %d³ 但样本数不符：期望 %d 个分量，实际 %d 个", declared, want, len(lut.Data))
	}
	lut.Size = declared
	for i := 0; i < 3; i++ {
		if lut.DomainMax[i] <= lut.DomainMin[i] {
			return nil, fmt.Errorf("DOMAIN_MAX 必须大于 DOMAIN_MIN")
		}
	}
	return lut, nil
}

// parseTriple reads the three numbers of a DOMAIN line.
func parseTriple(text string, dst *[3]float64) error {
	fields := strings.Fields(text)
	if len(fields) != 4 {
		return fmt.Errorf("%s 需要 3 个数值", fields[0])
	}
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(fields[i+1], 64)
		if err != nil {
			return fmt.Errorf("%q 不是数字", fields[i+1])
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("%q 不是有限数值", fields[i+1])
		}
		dst[i] = v
	}
	return nil
}

// IdentityLUT builds the identity table of one size, which the tests use to prove
// that a table round-trips exactly.
func IdentityLUT(size int) *LUT3D {
	lut := &LUT3D{Size: size, DomainMin: [3]float64{0, 0, 0}, DomainMax: [3]float64{1, 1, 1}}
	lut.Data = make([]float32, size*size*size*3)
	last := float64(size - 1)
	for r := 0; r < size; r++ {
		for g := 0; g < size; g++ {
			for b := 0; b < size; b++ {
				i := lut.Index(r, g, b) * 3
				lut.Data[i] = float32(float64(r) / last)
				lut.Data[i+1] = float32(float64(g) / last)
				lut.Data[i+2] = float32(float64(b) / last)
			}
		}
	}
	return lut
}

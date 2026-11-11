package engine

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"sort"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

const paletteDefaultSize = 10

// swatchCount resolves the requested swatch count, clamped to a usable range.
// Zero keeps the default of ten that the original palette engine produced.
func swatchCount(requested int) int {
	if requested < 2 || requested > 16 {
		return paletteDefaultSize
	}
	return requested
}

// ExtractPalette returns the theme colour and the swatch palette.
func ExtractPalette(req PaletteRequest) (Result, PaletteResult) {
	size := swatchCount(req.Colors)
	res := OK()
	if strings.TrimSpace(req.InputPath) == "" {
		res = Fail("BAD_INPUT", "缺少输入路径")
		return res, PaletteResult{}
	}
	opened, err := codec.OpenSRGB(req.InputPath)
	if err != nil {
		res = Fail("READ_FAILED", err.Error())
		return res, PaletteResult{}
	}
	// Work on a downscaled probe: quantisation quality barely changes and it
	// keeps the cost flat for large inputs.
	probe := imaging.Fit(opened.Image, 256, 256, imaging.Linear)
	pal, counts := quantize(probe, size+1)
	type entry struct {
		c color.NRGBA
		n int
	}
	entries := make([]entry, 0, len(pal))
	for i, c := range pal {
		entries = append(entries, entry{c: c, n: counts[i]})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].n > entries[j].n })
	if len(entries) == 0 {
		res = Fail("PALETTE_EMPTY", "无法提取颜色")
		return res, PaletteResult{}
	}
	pr := PaletteResult{ThemeColor: hex(entries[0].c)}
	for i := 1; i < len(entries) && len(pr.Colors) < size; i++ {
		pr.Colors = append(pr.Colors, hex(entries[i].c))
	}
	for len(pr.Colors) < size {
		pr.Colors = append(pr.Colors, pr.ThemeColor)
	}
	res.Count = len(pr.Colors) + 1

	if strings.TrimSpace(req.OutputPath) == "" {
		return res, pr
	}
	swatchH := req.SwatchHeight
	if swatchH == 0 {
		swatchH = 96
	}
	// The default is applied before the clamp for the same reason as the rich-text
	// canvas: clamping first silently turned 0 into the lower bound of 24.
	swatchH = clampInt(swatchH, 24, 600)
	all := append([]string{pr.ThemeColor}, pr.Colors...)
	width := 1200
	if w := opened.Width; w > 0 && w < width {
		width = w
	}
	swatchW := width / len(all)
	if swatchW < 1 {
		swatchW = 1
	}
	canvasW := swatchW * len(all)
	if err := output.CheckSize(canvasW, swatchH); err != nil {
		res = Fail("OUTPUT_TOO_LARGE", err.Error())
		return res, pr
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, canvasW, swatchH))
	for i, h := range all {
		c := hexToColor(h, 255)
		draw.Draw(canvas, image.Rect(i*swatchW, 0, (i+1)*swatchW, swatchH), &image.Uniform{c}, image.Point{}, draw.Src)
		if req.IncludeText {
			label := strings.TrimPrefix(strings.ToUpper(h), "#")
			face, ferr := FontFace("", 14, 72, false)
			if ferr == nil {
				textColor := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
				lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
				if lum > 150 {
					textColor = color.NRGBA{R: 17, G: 24, B: 39, A: 255}
				}
				tw := MeasureText(label, face)
				tx := float32(i*swatchW) + (float32(swatchW)-tw)/2
				ty := float32(swatchH)/2 + 5
				DrawText(canvas, label, tx, ty, TextOptions{Family: "", Size: 14, Color: textColor})
			}
		}
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(filepath.Ext(req.OutputPath), "."))
	}
	if format == "" {
		format = "png"
	}
	target := ensureExtension(req.OutputPath, format)
	if err := codec.Save(canvas, target, codec.SaveOptions{Quality: 95, SRGB: true}); err != nil {
		res = Fail("WRITE_FAILED", err.Error())
		return res, pr
	}
	res.OutputPath = target
	res.Width = canvasW
	res.Height = swatchH
	return res, pr
}

// PaletteImage renders just the swatch strip, used by the UI preview.
func PaletteImage(colors []string, swatchHeight int) (image.Image, error) {
	if len(colors) == 0 {
		return nil, fmt.Errorf("没有颜色")
	}
	if swatchHeight == 0 {
		swatchHeight = 96
	}
	h := clampInt(swatchHeight, 24, 600)
	w := 600 / len(colors)
	if w < 1 {
		w = 1
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, w*len(colors), h))
	for i, c := range colors {
		col := hexToColor(c, 255)
		draw.Draw(canvas, image.Rect(i*w, 0, (i+1)*w, h), &image.Uniform{col}, image.Point{}, draw.Src)
	}
	return canvas, nil
}

// quantize runs a weighted median cut over the probe image.
func quantize(img image.Image, maxColors int) ([]color.NRGBA, []int) {
	type sampleT = sampleItem
	histogram := map[uint32]*sampleT{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a < 0x1000 {
				continue
			}
			key := uint32(r>>9)<<14 | uint32(g>>9)<<7 | uint32(bl>>9)
			if s, ok := histogram[key]; ok {
				s.n++
			} else {
				histogram[key] = &sampleT{
					c: color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: 255},
					n: 1,
				}
			}
		}
	}
	all := make([]sampleItem, 0, len(histogram))
	for _, s := range histogram {
		all = append(all, sampleItem{c: s.c, n: s.n})
	}
	if len(all) == 0 {
		return nil, nil
	}
	boxes := []*box{{items: all}}
	for len(boxes) < maxColors {
		best := -1
		bestRange := 0
		bestChannel := 0
		for i, bx := range boxes {
			if len(bx.items) < 2 {
				continue
			}
			var lo, hi = [3]uint8{255, 255, 255}, [3]uint8{}
			for _, it := range bx.items {
				ch := [3]uint8{it.c.R, it.c.G, it.c.B}
				for k := 0; k < 3; k++ {
					if ch[k] < lo[k] {
						lo[k] = ch[k]
					}
					if ch[k] > hi[k] {
						hi[k] = ch[k]
					}
				}
			}
			for k := 0; k < 3; k++ {
				if r := int(hi[k]) - int(lo[k]); r > bestRange {
					bestRange, best, bestChannel = r, i, k
				}
			}
		}
		if best < 0 {
			break
		}
		bx := boxes[best]
		channel := bestChannel
		sort.SliceStable(bx.items, func(i, j int) bool {
			a, c := bx.items[i].c, bx.items[j].c
			switch channel {
			case 0:
				return a.R < c.R
			case 1:
				return a.G < c.G
			default:
				return a.B < c.B
			}
		})
		// Split by weight so a large flat area is not over-represented.
		total := 0
		for _, it := range bx.items {
			total += it.n
		}
		acc := 0
		split := len(bx.items) / 2
		for i, it := range bx.items {
			acc += it.n
			if acc*2 >= total {
				split = i + 1
				break
			}
		}
		if split <= 0 || split >= len(bx.items) {
			split = len(bx.items) / 2
		}
		left := &box{items: bx.items[:split]}
		right := &box{items: bx.items[split:]}
		boxes[best] = left
		boxes = append(boxes, right)
	}
	pal := make([]color.NRGBA, 0, len(boxes))
	counts := make([]int, 0, len(boxes))
	for _, bx := range boxes {
		if len(bx.items) == 0 {
			continue
		}
		var r, g, bl, n int
		for _, it := range bx.items {
			r += int(it.c.R) * it.n
			g += int(it.c.G) * it.n
			bl += int(it.c.B) * it.n
			n += it.n
		}
		if n == 0 {
			continue
		}
		pal = append(pal, color.NRGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(bl / n), A: 255})
		counts = append(counts, n)
	}
	return pal, counts
}

type sampleItem struct {
	c color.NRGBA
	n int
}

type box struct {
	items []sampleItem
}

func hex(c color.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

package engine

// analysis.go computes the histograms and statistics of the 图像分析 page.
//
// Big pictures are scanned on a stride and the result is labelled as sampled, so
// the page never presents an approximation as an exact whole-image count.

import (
	"image"
	"image/color"
	"math"
)

// Histogram is one channel's 256-bin distribution.
type Histogram [256]int

// Total is the number of samples that fell into the histogram.
func (h Histogram) Total() int {
	n := 0
	for _, v := range h {
		n += v
	}
	return n
}

// AnalysisRequest names what to measure.
type AnalysisRequest struct {
	Image image.Image
	// MaxSamples caps how many pixels are read. 0 means "scan everything".
	MaxSamples int
}

// AnalysisResult carries every measurement the page shows.
type AnalysisResult struct {
	Width, Height int
	// SampledPixels is how many pixels were actually read.
	SampledPixels int
	// Sampled is true when the scan skipped pixels, so the UI can label the
	// numbers as 采样统计 instead of claiming an exact whole-image count.
	Sampled bool

	Red, Green, Blue, Luma, Alpha Histogram

	// OverexposedPixels and UnderexposedPixels count fully clipped luminance.
	OverexposedPixels  int
	UnderexposedPixels int
	// TransparentPixels counts A < 255; FullyTransparent counts A == 0.
	TransparentPixels      int
	FullyTransparentPixels int
	// MeanLuma and MeanAlpha are the averages over the sampled pixels.
	MeanLuma  float64
	MeanAlpha float64
	// UniqueColors is the number of distinct 24-bit colours seen while sampling.
	UniqueColors int
	// Opaque reports whether every sampled pixel was fully opaque.
	Opaque bool
	// HasAlpha reports whether any sampled pixel was translucent.
	HasAlpha bool
}

// Analyze measures one image.
func Analyze(req AnalysisRequest) AnalysisResult {
	if req.Image == nil {
		return AnalysisResult{}
	}
	b := req.Image.Bounds()
	w, h := b.Dx(), b.Dy()
	out := AnalysisResult{Width: w, Height: h, Opaque: true}
	total := w * h
	if total == 0 {
		return out
	}
	// The stride keeps the scan bounded. A prime-ish step avoids landing on the
	// same column of every row, which would bias a checkerboard pattern.
	stride := 1
	if req.MaxSamples > 0 && total > req.MaxSamples {
		stride = int(math.Ceil(math.Sqrt(float64(total) / float64(req.MaxSamples))))
		if stride < 1 {
			stride = 1
		}
		out.Sampled = true
	}
	colors := make(map[uint32]struct{}, 4096)
	uniqueOverflow := false
	var lumaSum, alphaSum float64
	for y := 0; y < h; y += stride {
		for x := 0; x < w; x += stride {
			c := color.NRGBAModel.Convert(req.Image.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			out.Red[c.R]++
			out.Green[c.G]++
			out.Blue[c.B]++
			out.Alpha[c.A]++
			luma := int(lumaOf(c) + 0.5)
			if luma < 0 {
				luma = 0
			}
			if luma > 255 {
				luma = 255
			}
			out.Luma[luma]++
			lumaSum += float64(luma)
			alphaSum += float64(c.A)
			out.SampledPixels++
			if luma == 255 && c.R == 255 && c.G == 255 && c.B == 255 {
				out.OverexposedPixels++
			}
			if c.R == 0 && c.G == 0 && c.B == 0 {
				out.UnderexposedPixels++
			}
			if c.A < 255 {
				out.TransparentPixels++
				out.Opaque = false
				out.HasAlpha = true
				if c.A == 0 {
					out.FullyTransparentPixels++
				}
			}
			if !uniqueOverflow {
				key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
				if _, ok := colors[key]; !ok {
					colors[key] = struct{}{}
					// 1 048 576 distinct colours is already beyond what a caption
					// can show meaningfully; the count becomes "100 万+".
					if len(colors) >= 1<<20 {
						uniqueOverflow = true
					}
				}
			}
		}
	}
	if out.SampledPixels > 0 {
		out.MeanLuma = lumaSum / float64(out.SampledPixels)
		out.MeanAlpha = alphaSum / float64(out.SampledPixels)
	}
	if uniqueOverflow {
		out.UniqueColors = -1
	} else {
		out.UniqueColors = len(colors)
	}
	return out
}

// HistogramPeak returns the tallest bin, which the page uses to scale the chart.
func (h Histogram) HistogramPeak() int {
	peak := 0
	for _, v := range h {
		if v > peak {
			peak = v
		}
	}
	return peak
}

// LogScaled returns the bin heights on a logarithmic axis, so a distribution with
// one enormous spike still shows the rest.
func (h Histogram) LogScaled() Histogram {
	var out Histogram
	for i, v := range h {
		if v <= 0 {
			continue
		}
		out[i] = int(math.Log10(float64(v)+1) * 1000)
	}
	return out
}

// SamplePixel reports the colour under one probe, which the analysis page shows
// next to the histogram. It returns false when the point is outside the image.
func SamplePixel(img image.Image, x, y int) (color.NRGBA, bool) {
	if img == nil {
		return color.NRGBA{}, false
	}
	b := img.Bounds()
	if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
		return color.NRGBA{}, false
	}
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA), true
}

// ContrastRatio is the WCAG contrast ratio between two colours, used by the
// colour page as a readability reference.
func ContrastRatio(a, b color.NRGBA) float64 {
	la := relativeLuminance(a)
	lb := relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// relativeLuminance is the WCAG 2.1 definition, not the perceptual luma.
func relativeLuminance(c color.NRGBA) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

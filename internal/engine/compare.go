package engine

// compare.go is the shared image-comparison and analysis core.
//
// Three operations live here because they read the same pixels and must agree on
// how they normalise them: a difference map, the PSNR/SSIM metrics, and the
// histograms of the analysis page. The metrics refuse to compute when the two
// images cannot be put into correspondence, instead of returning a number that
// looks authoritative.

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// AlignMode names how two images of different sizes are put into correspondence.
type AlignMode string

const (
	// AlignNone means "as is": metrics are only computable for equal sizes.
	AlignNone AlignMode = "none"
	// AlignStretch resamples the result onto the reference size, ignoring the
	// aspect difference.
	AlignStretch AlignMode = "stretch"
	// AlignFitLetterboxes the result into the reference size, keeping the aspect
	// ratio, and compares only the overlapping area.
	AlignFit AlignMode = "fit"
)

// CompareRequest describes one comparison.
type CompareRequest struct {
	Reference image.Image
	Result    image.Image
	// Align is how a size mismatch is resolved. Empty means AlignNone.
	Align AlignMode
}

// CompareResult carries the metrics and the reason a metric is unavailable.
type CompareResult struct {
	Width, Height int
	Align         AlignMode
	// Computable is false when the images cannot be compared under the requested
	// alignment; the UI then shows "不可计算" with Reason.
	Computable bool
	Reason     string
	// PSNR in dB; +Inf when the images are identical.
	PSNR float64
	// SSIM in -1..1, 1 meaning identical.
	SSIM float64
	// MaxChannelDelta is the largest per-channel absolute difference found.
	MaxChannelDelta int
	// MeanDelta is the mean per-channel absolute difference.
	MeanDelta float64
}

// Compare computes the difference metrics of two images.
func Compare(req CompareRequest) CompareResult {
	align := req.Align
	if align == "" {
		align = AlignNone
	}
	if req.Reference == nil || req.Result == nil {
		return CompareResult{Align: align, Reason: "缺少参考图或结果图"}
	}
	ref := toNRGBA(req.Reference)
	res := toNRGBA(req.Result)
	rb, sb := ref.Bounds(), res.Bounds()
	if rb.Empty() || sb.Empty() {
		return CompareResult{Align: align, Reason: "图像为空"}
	}
	if rb.Dx() != sb.Dx() || rb.Dy() != sb.Dy() {
		switch align {
		case AlignNone:
			return CompareResult{
				Width: ref.Bounds().Dx(), Height: ref.Bounds().Dy(), Align: align,
				Reason: fmt.Sprintf("尺寸不同（%d×%d 与 %d×%d），未指定对齐方式时不计算指标",
					rb.Dx(), rb.Dy(), sb.Dx(), sb.Dy()),
			}
		case AlignStretch, AlignFit:
			res = resampleLike(res, rb.Dx(), rb.Dy(), align)
			if align == AlignFit {
				scale := math.Min(float64(rb.Dx())/float64(sb.Dx()), float64(rb.Dy())/float64(sb.Dy()))
				width := maxInt(1, int(float64(sb.Dx())*scale+0.5))
				height := maxInt(1, int(float64(sb.Dy())*scale+0.5))
				left, top := (rb.Dx()-width)/2, (rb.Dy()-height)/2
				region := image.Rect(left, top, left+width, top+height)
				ref = toNRGBA(ref.SubImage(region))
				res = toNRGBA(res.SubImage(region))
				rb = ref.Bounds()
			}
		default:
			return CompareResult{Align: align, Reason: "未知的对齐方式"}
		}
	}
	out := CompareResult{Width: rb.Dx(), Height: rb.Dy(), Align: align, Computable: true}
	w, h := rb.Dx(), rb.Dy()
	if w == 0 || h == 0 {
		out.Computable = false
		out.Reason = "图像为空"
		return out
	}
	var sumSq float64
	var sumAbs float64
	maxDelta := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := ref.NRGBAAt(x, y)
			b := res.NRGBAAt(x, y)
			for _, pair := range [3][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
				d := int(pair[0]) - int(pair[1])
				ad := d
				if ad < 0 {
					ad = -ad
				}
				if ad > maxDelta {
					maxDelta = ad
				}
				sumAbs += float64(ad)
				sumSq += float64(d) * float64(d)
			}
		}
	}
	n := float64(w * h * 3)
	mse := sumSq / n
	out.MaxChannelDelta = maxDelta
	out.MeanDelta = sumAbs / n
	if mse == 0 {
		out.PSNR = math.Inf(1)
	} else {
		out.PSNR = 10 * math.Log10(255*255/mse)
	}
	out.SSIM = ssim(ref, res)
	return out
}

// SSIMUnavailable explains why a metric is missing, for the UI's hint line.
func (r CompareResult) SSIMUnavailable() string {
	if r.Computable {
		return ""
	}
	return r.Reason
}

// ssim computes the mean structural similarity over a sliding 8×8 window with
// the standard constants for 8-bit data.
func ssim(a, b *image.NRGBA) float64 {
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	const win = 8
	const c1 = 6.5025  // (0.01*255)^2
	const c2 = 58.5225 // (0.03*255)^2
	var total float64
	var windows int
	for oy := 0; oy+win <= h; oy += win {
		for ox := 0; ox+win <= w; ox += win {
			var sumA, sumB, sumAA, sumBB, sumAB float64
			for y := oy; y < oy+win; y++ {
				for x := ox; x < ox+win; x++ {
					va := lumaOf(a.NRGBAAt(x, y))
					vb := lumaOf(b.NRGBAAt(x, y))
					sumA += va
					sumB += vb
					sumAA += va * va
					sumBB += vb * vb
					sumAB += va * vb
				}
			}
			n := float64(win * win)
			meanA := sumA / n
			meanB := sumB / n
			varA := sumAA/n - meanA*meanA
			varB := sumBB/n - meanB*meanB
			cov := sumAB/n - meanA*meanB
			num := (2*meanA*meanB + c1) * (2*cov + c2)
			den := (meanA*meanA + meanB*meanB + c1) * (varA + varB + c2)
			if den != 0 {
				total += num / den
				windows++
			}
		}
	}
	if windows == 0 {
		// A picture smaller than one window: fall back to the whole-image form so
		// the page still gets a meaningful number for a tiny preview.
		return ssimWhole(a, b)
	}
	return total / float64(windows)
}

// ssimWhole is the single-window form used for images smaller than 8×8.
func ssimWhole(a, b *image.NRGBA) float64 {
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	if w == 0 || h == 0 {
		return 0
	}
	const c1 = 6.5025
	const c2 = 58.5225
	var sumA, sumB, sumAA, sumBB, sumAB float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			va := lumaOf(a.NRGBAAt(x, y))
			vb := lumaOf(b.NRGBAAt(x, y))
			sumA += va
			sumB += vb
			sumAA += va * va
			sumBB += vb * vb
			sumAB += va * vb
		}
	}
	n := float64(w * h)
	meanA, meanB := sumA/n, sumB/n
	varA := sumAA/n - meanA*meanA
	varB := sumBB/n - meanB*meanB
	cov := sumAB/n - meanA*meanB
	num := (2*meanA*meanB + c1) * (2*cov + c2)
	den := (meanA*meanA + meanB*meanB + c1) * (varA + varB + c2)
	if den == 0 {
		return 1
	}
	return num / den
}

// lumaOf is the BT.601 luma the metrics use.
func lumaOf(c color.NRGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}

// DiffImage renders the absolute difference, amplified by Gain so a small loss is
// visible. A gain of 1 shows the raw difference.
func DiffImage(req CompareRequest, gain float64) (image.Image, CompareResult) {
	res := Compare(req)
	if !res.Computable {
		return nil, res
	}
	ref := toNRGBA(req.Reference)
	other := toNRGBA(req.Result)
	if ref.Bounds().Dx() != other.Bounds().Dx() || ref.Bounds().Dy() != other.Bounds().Dy() {
		other = resampleLike(other, ref.Bounds().Dx(), ref.Bounds().Dy(), res.Align)
	}
	if gain <= 0 {
		gain = 1
	}
	w, h := ref.Bounds().Dx(), ref.Bounds().Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := ref.NRGBAAt(x, y)
			b := other.NRGBAAt(x, y)
			diff := func(p, q uint8) uint8 {
				d := float64(p) - float64(q)
				if d < 0 {
					d = -d
				}
				return clampTo8(d * gain)
			}
			out.SetNRGBA(x, y, color.NRGBA{
				R: diff(a.R, b.R), G: diff(a.G, b.G), B: diff(a.B, b.B), A: 255,
			})
		}
	}
	return out, res
}

// HeatmapImage renders the difference as a heat map: black where the images agree,
// red where they differ most.
func HeatmapImage(req CompareRequest) (image.Image, CompareResult) {
	res := Compare(req)
	if !res.Computable {
		return nil, res
	}
	ref := toNRGBA(req.Reference)
	other := toNRGBA(req.Result)
	if ref.Bounds().Dx() != other.Bounds().Dx() || ref.Bounds().Dy() != other.Bounds().Dy() {
		other = resampleLike(other, ref.Bounds().Dx(), ref.Bounds().Dy(), res.Align)
	}
	w, h := ref.Bounds().Dx(), ref.Bounds().Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Normalise against the largest difference found, so a barely-visible change
	// is still visible; the caller is told the scale through the result's
	// MaxChannelDelta.
	scale := float64(res.MaxChannelDelta)
	if scale <= 0 {
		scale = 1
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := ref.NRGBAAt(x, y)
			b := other.NRGBAAt(x, y)
			var sum float64
			for _, pair := range [3][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
				d := float64(pair[0]) - float64(pair[1])
				if d < 0 {
					d = -d
				}
				sum += d
			}
			t := sum / 3 / scale
			t = clampF(t, 0, 1)
			out.SetNRGBA(x, y, color.NRGBA{
				R: clampTo8(255 * math.Min(1, t*2)),
				G: clampTo8(255 * math.Max(0, t*2-1)),
				B: 0,
				A: 255,
			})
		}
	}
	return out, res
}

// toNRGBA normalises any image into a copy at the origin. Working on a copy keeps
// the metrics independent of the caller's image type and of its origin.
func toNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok && n.Bounds().Min == (image.Point{}) {
		return n
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA))
		}
	}
	return dst
}

// resampleLike scales src to the requested size under the given alignment.
func resampleLike(src *image.NRGBA, w, h int, align AlignMode) *image.NRGBA {
	if align == AlignFit {
		// Keep the aspect ratio and centre the result inside the target box.
		sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
		scale := math.Min(float64(w)/float64(sw), float64(h)/float64(sh))
		tw, th := maxInt(1, int(float64(sw)*scale+0.5)), maxInt(1, int(float64(sh)*scale+0.5))
		fitted := resizeNearest(src, tw, th)
		out := image.NewNRGBA(image.Rect(0, 0, w, h))
		dx, dy := (w-tw)/2, (h-th)/2
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				out.SetNRGBA(dx+x, dy+y, fitted.NRGBAAt(x, y))
			}
		}
		return out
	}
	return resizeNearest(src, w, h)
}

// resizeNearest is a nearest-neighbour resample: the comparison must not add its
// own smoothing to the numbers it reports.
func resizeNearest(src *image.NRGBA, w, h int) *image.NRGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 || w < 1 || h < 1 {
		return image.NewNRGBA(image.Rect(0, 0, maxInt(w, 1), maxInt(h, 1)))
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := y * sh / h
		for x := 0; x < w; x++ {
			sx := x * sw / w
			out.SetNRGBA(x, y, src.NRGBAAt(sx, sy))
		}
	}
	return out
}

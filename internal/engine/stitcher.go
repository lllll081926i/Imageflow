package engine

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// StitchRequest combines several images into one long image.
type StitchRequest struct {
	InputPaths   []string `json:"input_paths"`
	OutputPath   string   `json:"output_path"`
	Direction    string   `json:"direction"`
	TargetWidth  int      `json:"target_width"`
	TargetHeight int      `json:"target_height"`
	Gap          int      `json:"gap"`
	Background   string   `json:"background"`
	BorderMode   bool     `json:"border_mode"`
	Border       int      `json:"border"`
	OutputFormat string   `json:"output_format"`
	Quality      int      `json:"quality"`
}

// Stitch puts the inputs side by side or one under the other.
func Stitch(req StitchRequest) Result {
	return StitchContext(context.Background(), req)
}

// StitchContext 在各张图片处理前及输出前响应取消。
func StitchContext(ctx context.Context, req StitchRequest) Result {
	if ctx.Err() != nil {
		return Fail("CANCELLED", "已取消")
	}
	paths := cleanPaths(req.InputPaths)
	if len(paths) == 0 {
		return Fail("STITCH_NO_INPUT", "没有输入图像")
	}
	if strings.TrimSpace(req.OutputPath) == "" {
		return Fail("STITCH_BAD_REQUEST", "缺少输出路径")
	}
	direction := strings.ToLower(req.Direction)
	if direction != "horizontal" {
		direction = "vertical"
	}
	gap := clampInt(req.Gap, 0, 500)
	bg := hexToColor(req.Background, 255)
	if strings.TrimSpace(req.Background) == "" {
		bg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	format := normalizeFormat(req.OutputFormat)
	if format == "" {
		format = normalizeFormat(strings.TrimPrefix(filepath.Ext(req.OutputPath), "."))
	}
	if format == "" {
		format = "png"
	}
	target := ensureExtension(req.OutputPath, format)
	quality := req.Quality
	if quality <= 0 {
		quality = 95
	}

	// Frame mode: a single input is padded into a background frame.
	if req.BorderMode || len(paths) == 1 {
		opened, err := codec.OpenSRGB(paths[0])
		if err != nil {
			return Fail("READ_FAILED", err.Error())
		}
		border := req.Border
		if border < 0 {
			border = 0
		}
		// A negative width means "not set": 0 is a legitimate choice (a tight frame
		// with no visible border), and the panel's slider offers it.
		if border > 1000 {
			border = 1000
		}
		base := opened.Image
		w := base.Bounds().Dx() + border*2
		h := base.Bounds().Dy() + border*2
		if err := output.CheckSize(w, h); err != nil {
			return Fail("STITCH_TOO_LARGE", err.Error())
		}
		final := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(final, final.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		draw.Draw(final, image.Rect(border, border, border+base.Bounds().Dx(), border+base.Bounds().Dy()), base, base.Bounds().Min, draw.Over)
		if err := codec.SaveContext(ctx, final, target, codec.SaveOptions{Quality: quality, SRGB: true}); err != nil {
			if ctx.Err() != nil {
				return Fail("CANCELLED", "已取消")
			}
			return Fail("WRITE_FAILED", err.Error())
		}
		out := OK()
		out.OutputPath = target
		out.Width = w
		out.Height = h
		out.Count = 1
		out.Message = "frame"
		return out
	}

	type item struct {
		img image.Image
		w   int
		h   int
	}
	items := make([]item, 0, len(paths))
	for _, p := range paths {
		if ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		if _, err := os.Stat(p); err != nil {
			return Fail("STITCH_INPUT_NOT_FOUND", "输入不存在: "+p)
		}
		opened, err := codec.OpenSRGB(p)
		if err != nil {
			return Fail("READ_FAILED", err.Error())
		}
		b := opened.Image.Bounds()
		items = append(items, item{img: opened.Image, w: b.Dx(), h: b.Dy()})
	}

	// The target length defaults to the median of the inputs so nothing is
	// upscaled harder than necessary.
	targetLen := req.TargetHeight
	if direction == "vertical" {
		targetLen = req.TargetWidth
	}
	if targetLen <= 0 {
		lengths := make([]int, 0, len(items))
		for _, it := range items {
			if direction == "vertical" {
				lengths = append(lengths, it.w)
			} else {
				lengths = append(lengths, it.h)
			}
		}
		targetLen = median(lengths)
	}
	targetLen = clampInt(targetLen, 1, output.MaxOutputSide)

	type scaled struct {
		img image.Image
		w   int
		h   int
	}
	sized := make([]scaled, 0, len(items))
	total := 0
	maxCross := 0
	for _, it := range items {
		var sw, sh int
		if direction == "vertical" {
			scale := float64(targetLen) / float64(it.w)
			sw = targetLen
			sh = maxInt(1, int(float64(it.h)*scale))
		} else {
			scale := float64(targetLen) / float64(it.h)
			sh = targetLen
			sw = maxInt(1, int(float64(it.w)*scale))
		}
		total += map[bool]int{true: sh, false: sw}[direction == "vertical"]
		if direction == "vertical" {
			if sw > maxCross {
				maxCross = sw
			}
		} else {
			if sh > maxCross {
				maxCross = sh
			}
		}
		sized = append(sized, scaled{img: it.img, w: sw, h: sh})
	}
	along := total + gap*(len(sized)-1)

	var canvasW, canvasH int
	if direction == "vertical" {
		canvasW, canvasH = maxCross, along
	} else {
		canvasW, canvasH = along, maxCross
	}
	if err := output.CheckSize(canvasW, canvasH); err != nil {
		return Fail("STITCH_TOO_LARGE", err.Error())
	}
	for i := range sized {
		if ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		sized[i].img = imaging.Resize(sized[i].img, sized[i].w, sized[i].h, imaging.Lanczos)
	}
	final := image.NewNRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(final, final.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	cursor := 0
	for _, s := range sized {
		var dst image.Rectangle
		if direction == "vertical" {
			dst = image.Rect(0, cursor, s.w, cursor+s.h)
		} else {
			dst = image.Rect(cursor, 0, cursor+s.w, s.h)
		}
		draw.Draw(final, dst, s.img, s.img.Bounds().Min, draw.Over)
		cursor += map[bool]int{true: s.h, false: s.w}[direction == "vertical"] + gap
	}
	if err := codec.SaveContext(ctx, final, target, codec.SaveOptions{Quality: quality, SRGB: true}); err != nil {
		if ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		return Fail("WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Width = canvasW
	out.Height = canvasH
	out.Count = len(sized)
	out.Message = direction
	return out
}

func cleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func median(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted[len(sorted)/2]
}

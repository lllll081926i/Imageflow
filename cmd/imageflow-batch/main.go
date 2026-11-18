// Command imageflow-batch runs one engine over a set of inputs from the
// command line. It exists so the same code paths the UI calls can be verified
// on real files, in CI and from a script, without a window.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/collage"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/paths"
)

type report struct {
	Tool    string          `json:"tool"`
	Results []engine.Result `json:"results"`
	OK      int             `json:"ok"`
	Failed  int             `json:"failed"`
}

var toolNames = []string{
	"convert", "resize", "grid", "layout", "stitch",
	"watermark", "palette", "pdf", "gif", "richtext",
	"compress", "collage",
}

func validTool(name string) bool {
	for _, n := range toolNames {
		if name == n {
			return true
		}
	}
	return false
}

func main() {
	tool := flag.String("tool", "convert", "convert|compress|resize|grid|layout|collage|stitch|watermark|palette|pdf|gif|richtext")
	outDir := flag.String("out", "", "output directory (default: a temp dir)")
	format := flag.String("format", "png", "output format for convert/resize/grid/layout/collage")
	quality := flag.Int("quality", 90, "quality for lossy formats")
	lossless := flag.Bool("lossless", false, "无损压缩（仅 PNG / WebP 输出）")
	grid := flag.Int("grid", 3, "grid size for the grid cutter")
	rows := flag.Int("rows", 2, "rows for the layout tool")
	cols := flag.Int("cols", 2, "cols for the layout tool")
	preset := flag.String("preset", "", "collage template id (3-t1b2); empty takes the first one matching the input count")
	aspect := flag.String("aspect", "1:1", "collage canvas ratio, e.g. 16:9 or 210:297")
	longEdge := flag.Int("long-edge", 1600, "collage canvas long edge in pixels")
	gap := flag.Int("gap", 0, "collage gap between cells in pixels (0 = seamless)")
	margin := flag.Int("margin", 0, "collage margin around the canvas in pixels")
	radius := flag.Int("radius", 0, "collage cell corner radius in pixels (0 = seamless)")
	seam := flag.Bool("seam", false, "collage with a visible seam: sets -gap 10 and -radius 8 unless given")
	background := flag.String("background", "#FFFFFF", "collage background colour")
	watermark := flag.String("watermark", "", "collage tiled watermark text")
	text := flag.String("text", "ImageFlow", "watermark or rich-text heading")
	theme := flag.String("theme", "", "rich-text theme: \"\", dark, sepia or ocean")
	image := flag.String("image", "", "rich-text image block path")
	action := flag.String("action", "probe", "GIF action: probe|reverse|change_speed|compress|resize|export_frames")
	speed := flag.Float64("speed", 1.5, "speed factor for the change_speed GIF action")
	frameStart := flag.Int("frame-start", 0, "first frame to export (0-based)")
	frameEnd := flag.Int("frame-end", 0, "frame to stop exporting at (0 = all)")
	jsonOut := flag.Bool("json", false, "print the report as JSON")
	flag.Parse()
	if *tool == "compress" {
		formatSet := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "format" {
				formatSet = true
			}
		})
		qualitySet := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "quality" {
				qualitySet = true
			}
		})
		if !formatSet {
			*format = "auto"
		}
		if !qualitySet {
			*quality = 80
		}
	}

	// Validate the tool before creating any output directory. An unknown name
	// used to fall through to the per-file switch, so it created (and leaked) a
	// temp directory before finally reporting the error.
	if !validTool(*tool) {
		fmt.Fprintf(os.Stderr, "unknown tool %q\n", *tool)
		flag.Usage()
		os.Exit(2)
	}

	inputs := flag.Args()
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: imageflow-batch [flags] <input>...")
		os.Exit(2)
	}

	expanded := paths.Expand(inputs)
	if len(expanded.Files) == 0 {
		fmt.Fprintln(os.Stderr, "no supported inputs found")
		os.Exit(2)
	}
	files := make([]string, 0, len(expanded.Files))
	for _, f := range expanded.Files {
		files = append(files, f.Path)
	}
	sort.Strings(files)

	dir := *outDir
	if dir == "" {
		tmp, err := os.MkdirTemp("", "imageflow-batch-*")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		dir = tmp
		// The files are the result, so the directory is intentionally kept.
		// It must be printed: in -json mode no output path reveals its root.
		fmt.Fprintln(os.Stderr, "output directory:", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rep := report{Tool: *tool}
	reserved := []string{}
	reserve := func(name string) (string, error) {
		return plan(filepath.Join(dir, name), &reserved)
	}

	// Stitch, PDF and collage consume the whole list in one pass, so they run once
	// and the per-file loop must not repeat them.
	wholeList := *tool == "stitch" || *tool == "pdf" || *tool == "collage"
	if wholeList {
		var res engine.Result
		switch *tool {
		case "stitch":
			target, err := reserve("stitched." + staticExtension(*format))
			if err != nil {
				res = engine.Fail("OUTPUT_PATH", err.Error())
				break
			}
			res = engine.Stitch(engine.StitchRequest{
				InputPaths:   files,
				OutputPath:   target,
				Direction:    "vertical",
				Background:   "#FFFFFF",
				OutputFormat: *format,
				Quality:      *quality,
			})
		case "pdf":
			target, err := reserve("document.pdf")
			if err != nil {
				res = engine.Fail("OUTPUT_PATH", err.Error())
				break
			}
			res = engine.GeneratePDF(engine.PDFRequest{
				ImagePaths: files,
				OutputPath: target,
				Layout:     "2x2",
				PageSize:   "A4",
				Landscape:  false,
				FitMode:    "contain",
				Margin:     36,
				Title:      *text,
			})
		case "collage":
			if *seam {
				// The seamless collage is the default, as it is in the panel: a seamed
				// one only asks for the gap and the corners when -seam is given.
				if *gap == 0 {
					*gap = 10
				}
				if *radius == 0 {
					*radius = 8
				}
			}
			res = collageFromFlags(files, reserve, *format, *quality, *preset, *aspect,
				*longEdge, *gap, *margin, *radius, *background, *watermark)
		}
		rep.Results = append(rep.Results, res)
		if res.Success {
			rep.OK++
		} else {
			rep.Failed++
		}
		files = nil
	}

	for _, in := range files {
		base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
		candidate, gifFormat := batchDestination(*tool, *format, *action, in, base)
		var target string
		if candidate != "" {
			var err error
			target, err = reserve(candidate)
			if err != nil {
				rep.Results = append(rep.Results, engine.Fail("OUTPUT_PATH", err.Error()))
				rep.Failed++
				continue
			}
		}
		var res engine.Result
		switch *tool {
		case "compress":
			if target == "" {
				res = engine.Fail("BAD_INPUT", "图片压缩支持 JPG、PNG、WebP、AVIF")
				break
			}
			res = engine.Compress(engine.CompressRequest{
				InputPath:  in,
				OutputPath: target,
				Format:     *format, Quality: *quality, Lossless: *lossless,
			})
		case "convert":
			if engine.OutputExtension(*format) == "" {
				res = engine.Fail("BAD_INPUT", "未知的输出格式")
				break
			}
			res = engine.Convert(engine.ConvertRequest{
				InputPath:  in,
				OutputPath: target,
				Format:     *format,
				Quality:    *quality,
			})
		case "resize":
			res = engine.Resize(engine.ResizeRequest{
				InputPath:    in,
				OutputPath:   target,
				Mode:         "long_edge",
				Value:        640,
				OutputFormat: *format,
				Quality:      *quality,
			})
		case "grid":
			res = engine.CutGrid(engine.GridRequest{
				InputPath:    in,
				OutputDir:    target,
				GridSize:     *grid,
				OutputFormat: *format,
				Quality:      *quality,
				BaseName:     base,
			})
		case "layout":
			res = engine.Split(engine.SplitRequest{
				InputPath:    in,
				OutputDir:    target,
				Rows:         *rows,
				Cols:         *cols,
				Mode:         "cover",
				OutputFormat: *format,
				Quality:      *quality,
				BaseName:     base,
			})
		case "watermark":
			res = engine.Watermark(engine.WatermarkRequest{
				InputPath:  in,
				OutputPath: target,
				Text:       *text,
				FontSize:   48,
				Color:      "#FFFFFF",
				Opacity:    80,
				Position:   "br",
				Quality:    *quality,
			})
		case "palette":
			var pr engine.PaletteResult
			res, pr = engine.ExtractPalette(engine.PaletteRequest{
				InputPath:    in,
				OutputPath:   target,
				SwatchHeight: 96,
				IncludeText:  true,
			})
			if res.Success {
				res.Message = pr.ThemeColor
			}
		case "gif":
			request := engine.GIFRequest{
				Action:      *action,
				InputPath:   in,
				SpeedFactor: *speed,
				Quality:     *quality,
				FrameStart:  *frameStart,
				FrameEnd:    *frameEnd,
			}
			if *action == "export_frames" {
				request.OutputDir = target
				request.OutputFormat = "png"
			} else if *action != "probe" {
				request.OutputPath = target
				request.OutputFormat = gifFormat
			}
			res = engine.RunGIF(request)
		case "richtext":
			blocks := []engine.RichTextBlock{
				{Type: "heading", Level: 1, Text: *text},
				{Type: "paragraph", Text: "在 Go 与 Lobe 组件上重写的原生图像工作台。"},
				{Type: "list", Items: []string{"格式转换", "批量水印", "转 PDF"}},
			}
			if *image != "" {
				blocks = append(blocks, engine.RichTextBlock{
					Type: "image", Path: *image, WidthPercent: 60,
				})
			}
			res = engine.RenderRichText(engine.RichTextRequest{
				Blocks:     blocks,
				OutputPath: target,
				Width:      800,
				Padding:    48,
				FontSize:   18,
				LineHeight: 1.6,
				Theme:      *theme,
			})
		default:
			// validTool already rejected this; keep the branch as a guard for
			// a tool added to the whitelist but not implemented here.
			fmt.Fprintf(os.Stderr, "tool %q has no per-file job\n", *tool)
			os.Exit(2)
		}
		rep.Results = append(rep.Results, res)
		if res.Success {
			rep.OK++
		} else {
			rep.Failed++
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		for _, r := range rep.Results {
			if r.Success {
				target := r.OutputPath
				if target == "" {
					target = r.OutputDir
				}
				fmt.Printf("ok   %-40s %s\n", target, describe(r))
			} else {
				fmt.Printf("FAIL [%s] %s\n", r.ErrorCode, r.Error)
			}
		}
		fmt.Printf("\n%d ok, %d failed\n", rep.OK, rep.Failed)
	}
	if rep.Failed > 0 {
		os.Exit(1)
	}
}

// collageFromFlags composes a collage from command-line flags.
//
// The template defaults to the first preset that takes exactly as many pictures
// as the command was given: a batch script should not have to name a layout, and
// the alternative — one full-canvas slot with the rest wrapped — is rarely what
// the caller wanted.
func collageFromFlags(files []string, reserve func(string) (string, error), format string, quality int, presetID, aspect string,
	longEdge, gap, margin, radius int, background, watermark string) engine.Result {
	rects, id := collageSkeleton(len(files), presetID)
	cells := make([]engine.CollageCell, len(rects))
	for i := 0; i < len(files) && i < len(cells); i++ {
		cells[i] = engine.CollageCell{Path: files[i]}
	}
	ratio, ok := collage.ParseAspect(aspect)
	if !ok {
		if known, found := collage.AspectByID(aspect); found {
			ratio = known
		} else {
			ratio = collage.Aspect{ID: "1:1", W: 1, H: 1}
		}
	}
	target, err := reserve("collage_" + id + "." + staticExtension(format))
	if err != nil {
		return engine.Fail("OUTPUT_PATH", err.Error())
	}
	req := engine.CollageRequest{
		Cells:        cells,
		Rects:        rects,
		OutputPath:   target,
		AspectW:      ratio.W,
		AspectH:      ratio.H,
		LongEdge:     longEdge,
		MarginTop:    margin,
		MarginRight:  margin,
		MarginBottom: margin,
		MarginLeft:   margin,
		Gap:          gap,
		Radius:       radius,
		Background:   background,
		Fit:          engine.CollageCover,
		Quality:      quality,
		Format:       format,
	}
	if watermark != "" {
		req.Watermark = engine.CollageWatermark{
			Enabled: true, Text: watermark, Size: 28, Opacity: 0.25, Angle: -30, Density: 1,
		}
	}
	res := engine.Collage(req)
	if res.Success {
		res.Message = fmt.Sprintf("%s · %d 格", id, len(rects))
	}
	return res
}

// collageSkeleton resolves the template and its cells; it falls back to the
// largest template when no named one matches.
func collageSkeleton(count int, presetID string) ([]collage.Rect, string) {
	if presetID != "" {
		if preset, ok := collage.PresetByID(presetID); ok && len(preset.Cells) > 0 {
			return preset.Cells, preset.ID
		}
	}
	if list := collage.PresetsFor(count); len(list) > 0 {
		return list[0].Cells, list[0].ID
	}
	best := collage.Presets[0]
	for _, p := range collage.Presets {
		if p.Count() > best.Count() {
			best = p
		}
	}
	return best.Cells, best.ID
}

func describe(r engine.Result) string {
	parts := []string{}
	if r.Width > 0 && r.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dx%d", r.Width, r.Height))
	}
	if r.Count > 0 {
		parts = append(parts, fmt.Sprintf("%d items", r.Count))
	}
	if r.OutputSize > 0 {
		parts = append(parts, fmt.Sprintf("%d B", r.OutputSize))
	}
	if r.Message != "" {
		parts = append(parts, r.Message)
	}
	return strings.Join(parts, "  ")
}

// plan resolves a candidate output path against the outputs already planned in
// this run, so two inputs sharing a base name never write the same file.
func plan(candidate string, reserved *[]string) (string, error) {
	resolved, _, err := paths.ResolveOutput(candidate, *reserved, "rename")
	if err != nil {
		return "", err
	}
	*reserved = append(*reserved, resolved)
	return resolved, nil
}

func staticExtension(format string) string {
	if ext := engine.OutputExtension(format); ext != "" {
		return ext
	}
	return "png"
}

func batchDestination(tool, format, action, input, base string) (string, string) {
	switch tool {
	case "convert", "resize":
		return base + "." + staticExtension(format), ""
	case "compress":
		if ext := engine.CompressionFormat(input, format); ext != "" {
			return base + "_compressed." + ext, ""
		}
	case "grid", "layout":
		return base + "_" + tool, ""
	case "watermark":
		return base + "_wm.png", ""
	case "palette":
		return base + "_palette.png", ""
	case "richtext":
		return base + "_rich.png", ""
	case "gif":
		switch action {
		case "export_frames":
			return base + "_frames", ""
		case "probe":
			return "", ""
		}
		switch strings.ToLower(format) {
		case "png", "apng":
			return base + "_anim.apng", "apng"
		case "webp":
			return base + "_anim.webp", "webp"
		default:
			return base + "_anim.gif", "gif"
		}
	}
	return "", ""
}

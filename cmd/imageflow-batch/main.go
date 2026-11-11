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
	"compress",
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
	tool := flag.String("tool", "convert", "convert|compress|resize|grid|layout|stitch|watermark|palette|pdf|gif|richtext")
	outDir := flag.String("out", "", "output directory (default: a temp dir)")
	format := flag.String("format", "png", "output format for convert/resize/grid/layout")
	quality := flag.Int("quality", 90, "quality for lossy formats")
	lossless := flag.Bool("lossless", false, "无损压缩（仅 PNG / WebP 输出）")
	grid := flag.Int("grid", 3, "grid size for the grid cutter")
	rows := flag.Int("rows", 2, "rows for the layout tool")
	cols := flag.Int("cols", 2, "cols for the layout tool")
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

	// Stitch and PDF consume the whole list in one pass, so they run once and
	// the per-file loop must not repeat them.
	wholeList := *tool == "stitch" || *tool == "pdf"
	if wholeList {
		var res engine.Result
		switch *tool {
		case "stitch":
			res = engine.Stitch(engine.StitchRequest{
				InputPaths:   files,
				OutputPath:   filepath.Join(dir, "stitched."+*format),
				Direction:    "vertical",
				Background:   "#FFFFFF",
				OutputFormat: *format,
				Quality:      *quality,
			})
		case "pdf":
			res = engine.GeneratePDF(engine.PDFRequest{
				ImagePaths: files,
				OutputPath: filepath.Join(dir, "document.pdf"),
				Layout:     "2x2",
				PageSize:   "A4",
				Landscape:  false,
				FitMode:    "contain",
				Margin:     36,
				Title:      *text,
			})
		}
		rep.Results = append(rep.Results, res)
		if res.Success {
			rep.OK++
		} else {
			rep.Failed++
		}
		files = nil
	}

	// reserved holds the outputs already planned, so two inputs with the same
	// base name do not overwrite each other.
	reserved := []string{}
	for _, in := range files {
		base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
		var res engine.Result
		switch *tool {
		case "compress":
			ext := engine.CompressionFormat(in, *format)
			if ext == "" {
				res = engine.Fail("BAD_INPUT", "图片压缩支持 JPG、PNG、WebP、AVIF")
				break
			}
			res = engine.Compress(engine.CompressRequest{
				InputPath:  in,
				OutputPath: plan(filepath.Join(dir, base+"_compressed."+ext), &reserved),
				Format:     *format, Quality: *quality, Lossless: *lossless,
			})
		case "convert":
			res = engine.Convert(engine.ConvertRequest{
				InputPath:  in,
				OutputPath: plan(filepath.Join(dir, base+"."+*format), &reserved),
				Format:     *format,
				Quality:    *quality,
			})
		case "resize":
			res = engine.Resize(engine.ResizeRequest{
				InputPath:    in,
				OutputPath:   plan(filepath.Join(dir, base+"."+*format), &reserved),
				Mode:         "long_edge",
				Value:        640,
				OutputFormat: *format,
				Quality:      *quality,
			})
		case "grid":
			res = engine.CutGrid(engine.GridRequest{
				InputPath:    in,
				OutputDir:    filepath.Join(dir, base+"_grid"),
				GridSize:     *grid,
				OutputFormat: *format,
				Quality:      *quality,
				BaseName:     base,
			})
		case "layout":
			res = engine.Split(engine.SplitRequest{
				InputPath:    in,
				OutputDir:    filepath.Join(dir, base+"_layout"),
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
				OutputPath: plan(filepath.Join(dir, base+"_wm.png"), &reserved),
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
				OutputPath:   plan(filepath.Join(dir, base+"_palette.png"), &reserved),
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
				request.OutputDir = filepath.Join(dir, base+"_frames")
				request.OutputFormat = "png"
			} else if *action != "probe" {
				// -format decides the container the animation is written into, so the
				// CLI can exercise the same three targets the page offers.
				format := strings.ToLower(*format)
				if format == "" || format == "png" {
					if format == "png" && *action != "export_frames" {
						format = "apng"
					}
				}
				switch format {
				case "webp", "apng", "gif":
				default:
					format = "gif"
				}
				ext := format
				if ext == "apng" {
					ext = "png"
				}
				request.OutputPath = plan(filepath.Join(dir, base+"_anim."+ext), &reserved)
				request.OutputFormat = format
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
				OutputPath: plan(filepath.Join(dir, base+"_rich.png"), &reserved),
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
			if res.OutputPath != "" {
				reserved = append(reserved, res.OutputPath)
			}
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
func plan(candidate string, reserved *[]string) string {
	resolved, _, err := paths.ResolveOutput(candidate, *reserved, "rename")
	if err != nil {
		return candidate
	}
	*reserved = append(*reserved, resolved)
	return resolved
}

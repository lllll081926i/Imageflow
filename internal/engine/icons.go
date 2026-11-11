package engine

// icons.go builds an application icon set from one square source.
//
// The ladder and the three extra files follow what a desktop or web project
// actually consumes: a PNG per size, one multi-resolution .ico, a small
// favicon.ico, and an SVG that wraps a PNG so a browser tab and a packaging
// script can share one source.

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// IconLadder is every PNG size an icon bundle carries.
var IconLadder = []int{16, 24, 32, 48, 64, 96, 128, 144, 152, 167, 180, 192, 256, 384, 512, 1024}

// IconICOSizes are the entries of the multi-resolution icon.ico; FaviconSizes
// those of favicon.ico. A Windows shortcut and a browser tab want different
// ladders.
var (
	IconICOSizes = []int{16, 24, 32, 48, 64, 128, 256}
	FaviconSizes = []int{16, 32, 48}
)

// iconMaxPixels caps the source, matching the rest of the app's input budget.
const iconMaxPixels = 20_000_000

// IconRequest is the icon tool payload.
type IconRequest struct {
	InputPath string `json:"input_path"`
	OutputDir string `json:"output_dir"`
	// Background fills the margin left when the source is not square. Empty
	// keeps it transparent, which is what a dock icon wants.
	Background  string `json:"background"`
	Padding     int    `json:"padding"`
	Radius      int    `json:"radius"`
	Sizes       []int  `json:"sizes"`
	WithICO     bool   `json:"with_ico"`
	WithFavicon bool   `json:"with_favicon"`
	WithSVG     bool   `json:"with_svg"`
}

// BuildIconSet writes every requested file into OutputDir.
func BuildIconSet(req IconRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputDir) == "" {
		return Fail("BAD_INPUT", "缺少输入路径或输出目录")
	}
	opened, err := codec.OpenSRGB(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	b := opened.Image.Bounds()
	if b.Dx()*b.Dy() > iconMaxPixels {
		return Fail("ICON_TOO_LARGE", "源图像素超过 2000 万")
	}
	sizes := req.Sizes
	if len(sizes) == 0 {
		sizes = IconLadder
	}
	sizes = clampIconSizes(sizes)
	if len(sizes) == 0 {
		return Fail("BAD_INPUT", "没有可用的图标尺寸")
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}

	// One prepared square serves every size: padding and rounding belong to the
	// canvas, not to each resize, so the small entries stay crisp.
	square, err := iconSquare(opened.Image, req)
	if err != nil {
		return Fail("ICON_RENDER_FAILED", err.Error())
	}

	var written []string
	for _, size := range sizes {
		if err := output.CheckSize(size, size); err != nil {
			return Fail("OUTPUT_TOO_LARGE", err.Error())
		}
		thumb := imaging.Resize(square, size, size, imaging.Lanczos)
		target := filepath.Join(req.OutputDir, fmt.Sprintf("icon-%dx%d.png", size, size))
		if err := codec.Save(thumb, target, codec.SaveOptions{SRGB: true}); err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		written = append(written, target)
	}
	if req.WithICO {
		target := filepath.Join(req.OutputDir, "icon.ico")
		if err := codec.SaveICO(iconEntries(square, IconICOSizes), target); err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		written = append(written, target)
	}
	if req.WithFavicon {
		target := filepath.Join(req.OutputDir, "favicon.ico")
		if err := codec.SaveICO(iconEntries(square, FaviconSizes), target); err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		written = append(written, target)
	}
	if req.WithSVG {
		target := filepath.Join(req.OutputDir, "icon.svg")
		if err := writeIconSVG(square, target); err != nil {
			return Fail("WRITE_FAILED", err.Error())
		}
		written = append(written, target)
	}

	out := OK()
	out.OutputDir = req.OutputDir
	out.Outputs = written
	out.Count = len(written)
	out.Width = square.Bounds().Dx()
	out.Height = square.Bounds().Dy()
	return out
}

// clampIconSizes keeps only sizes an icon can be, deduplicated and ascending.
func clampIconSizes(in []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(in))
	for _, size := range in {
		if size < 8 || size > 1024 || seen[size] {
			continue
		}
		seen[size] = true
		out = append(out, size)
	}
	sort.Ints(out)
	return out
}

// iconEntries renders the square at each size of an .ico ladder.
func iconEntries(square image.Image, sizes []int) []image.Image {
	out := make([]image.Image, 0, len(sizes))
	for _, size := range sizes {
		if size < 8 || size > 256 {
			continue
		}
		out = append(out, imaging.Resize(square, size, size, imaging.Lanczos))
	}
	return out
}

// iconSquare crops the source to its centre square, then applies the requested
// padding and corner radius.
func iconSquare(src image.Image, req IconRequest) (image.Image, error) {
	b := src.Bounds()
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	square := imaging.Crop(src, image.Rect(x0, y0, x0+side, y0+side))

	padding := clampInt(req.Padding, 0, 40)
	radius := clampInt(req.Radius, 0, 50)
	if padding == 0 && radius == 0 {
		return square, nil
	}

	inner := square
	if padding > 0 {
		innerSide := side * (100 - padding*2) / 100
		if innerSide < 1 {
			return nil, fmt.Errorf("内边距过大")
		}
		inner = imaging.Resize(square, innerSide, innerSide, imaging.Lanczos)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, side, side))
	// A transparent canvas needs no fill; a chosen one is painted first.
	if strings.TrimSpace(req.Background) != "" {
		fill := hexToColor(req.Background, 255)
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{fill}, image.Point{}, draw.Src)
	}
	offset := (side - inner.Bounds().Dx()) / 2
	drawRoundedSquare(canvas, offset, offset, inner, radius)
	return canvas, nil
}

// drawRoundedSquare blits src at (x,y), masked to a square rounded by radius
// percent of its side. A zero radius copies every pixel.
func drawRoundedSquare(dst *image.NRGBA, x, y int, src image.Image, radius int) {
	sb := src.Bounds()
	side := sb.Dx()
	r := float64(side) * float64(radius) / 100
	for dy := 0; dy < side; dy++ {
		for dx := 0; dx < side; dx++ {
			if r > 0 && outsideRoundedCorner(dx, dy, side, r) {
				continue
			}
			dst.Set(x+dx, y+dy, src.At(sb.Min.X+dx, sb.Min.Y+dy))
		}
	}
}

// outsideRoundedCorner reports whether a pixel lies beyond a square of the given
// side rounded by r.
func outsideRoundedCorner(x, y, side int, r float64) bool {
	cx, cy := float64(x)+0.5, float64(y)+0.5
	limit := float64(side) - r
	var ox, oy float64
	switch {
	case cx < r && cy < r:
		ox, oy = cx-r, cy-r
	case cx > limit && cy < r:
		ox, oy = cx-limit, cy-r
	case cx < r && cy > limit:
		ox, oy = cx-r, cy-limit
	case cx > limit && cy > limit:
		ox, oy = cx-limit, cy-limit
	default:
		return false
	}
	return ox*ox+oy*oy > r*r
}

// writeIconSVG wraps the square in an SVG with the PNG embedded, which is what a
// browser tab and a packaging script can both consume.
func writeIconSVG(square image.Image, target string) error {
	data, err := codec.EncodeBytes(square, "png", codec.SaveOptions{SRGB: true})
	if err != nil {
		return err
	}
	b := square.Bounds()
	doc := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" `+
			`width="%d" height="%d" viewBox="0 0 %d %d">`+
			`<image width="%d" height="%d" xlink:href="data:image/png;base64,%s"/></svg>`,
		b.Dx(), b.Dy(), b.Dx(), b.Dy(), b.Dx(), b.Dy(), base64.StdEncoding.EncodeToString(data))
	return output.Commit(target, func(f *os.File) error {
		_, werr := f.Write([]byte(doc))
		return werr
	})
}

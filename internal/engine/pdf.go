package engine

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/signintech/gopdf"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// PDFRequest is the PDF tool payload.
type PDFRequest struct {
	ImagePaths []string `json:"images"`
	OutputPath string   `json:"output_path"`
	Layout     string   `json:"layout"`
	CustomRows int      `json:"custom_rows"`
	CustomCols int      `json:"custom_cols"`
	PageSize   string   `json:"page_size"`
	Margin     float64  `json:"margin"`
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	// Landscape turns the paper sideways. It is the opt-in field rather than a
	// Portrait one so a zero-value request means portrait, which is the default
	// the original tool documents.
	Landscape  bool   `json:"landscape"`
	FitMode    string `json:"fit_mode"`
	UserPasswd string `json:"user_password"`
	Quality    int    `json:"quality"`
}

type paperSize struct {
	w, h float64 // points
}

var paperSizes = map[string]paperSize{
	"A0":        {2383.94, 3370.39},
	"A1":        {1683.78, 2383.94},
	"A2":        {1190.55, 1683.78},
	"A3":        {841.89, 1190.55},
	"A4":        {595.28, 841.89},
	"A5":        {419.53, 595.28},
	"A6":        {297.64, 419.53},
	"Letter":    {612, 792},
	"Legal":     {612, 1008},
	"Tabloid":   {792, 1224},
	"Ledger":    {1224, 792},
	"B4":        {708.66, 1002.52},
	"B5":        {498.9, 708.66},
	"Executive": {521.86, 756},
	"Statement": {396, 612},
}

// GeneratePDF lays images out on pages and writes a PDF.
func GeneratePDF(req PDFRequest) Result {
	return GeneratePDFContext(context.Background(), req)
}

// GeneratePDFContext 在图片处理边界和输出提交前响应取消。
func GeneratePDFContext(ctx context.Context, req PDFRequest) Result {
	if ctx.Err() != nil {
		return Fail("CANCELLED", "已取消")
	}
	paths := cleanPaths(req.ImagePaths)
	if len(paths) == 0 {
		return Fail("PDF_NO_IMAGES", "没有输入图像")
	}
	if strings.TrimSpace(req.OutputPath) == "" {
		return Fail("PDF_BAD_REQUEST", "缺少输出路径")
	}
	page, ok := paperSizes[strings.TrimSpace(req.PageSize)]
	if !ok {
		page = paperSizes["A4"]
	}
	if req.Landscape {
		page.w, page.h = page.h, page.w
	}
	rows, cols := gridFor(req.Layout, req.CustomRows, req.CustomCols)
	margin := pdfMargin(req.Margin, page)

	doc := &gopdf.GoPdf{}
	doc.Start(gopdf.Config{PageSize: gopdf.Rect{W: page.w, H: page.h}})
	// SetInfo replaces the whole info dictionary, so the two metadata fields must
	// be written in one call: a second SetInfo discarded the first, which silently
	// dropped the title whenever an author was also given.
	if req.Title != "" || req.Author != "" {
		doc.SetInfo(gopdf.PdfInfo{Title: req.Title, Author: req.Author})
	}

	perPage := rows * cols
	if perPage < 1 {
		perPage = 1
	}
	usableW := page.w - margin*2
	usableH := page.h - margin*2
	cellW := usableW / float64(cols)
	cellH := usableH / float64(rows)
	gap := 6.0

	tmpDir, err := os.MkdirTemp("", "imageflow-pdf-*")
	if err != nil {
		return Fail("PDF_TEMP_FAILED", err.Error())
	}
	defer os.RemoveAll(tmpDir)

	quality := req.Quality
	if quality <= 0 {
		quality = 90
	}

	written := 0
	for i, p := range paths {
		if ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		opened, oerr := codec.OpenSRGB(p)
		if oerr != nil {
			return Fail("READ_FAILED", fmt.Sprintf("%s: %v", filepath.Base(p), oerr))
		}
		// gopdf embeds a raster file; normalise each input to a JPEG temp. The
		// extension has to match what codec.Save encodes, which it does because
		// Save dispatches on the extension.
		tmp := filepath.Join(tmpDir, fmt.Sprintf("p%04d.jpg", i))
		// JPEG 无透明通道；先按 PDF 的白色纸面合成，避免透明区域变黑。
		paper := image.NewNRGBA(opened.Image.Bounds())
		draw.Draw(paper, paper.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
		draw.Draw(paper, paper.Bounds(), opened.Image, opened.Image.Bounds().Min, draw.Over)
		if err := codec.Save(paper, tmp, codec.SaveOptions{Quality: quality}); err != nil {
			return Fail("PDF_ENCODE_FAILED", err.Error())
		}
		if i%perPage == 0 {
			doc.AddPage()
		}
		slot := i % perPage
		row := slot / cols
		col := slot % cols
		boxX := margin + float64(col)*cellW + gap/2
		boxY := margin + float64(row)*cellH + gap/2
		boxW := cellW - gap
		boxH := cellH - gap

		iw := float64(opened.Width)
		ih := float64(opened.Height)
		if iw <= 0 || ih <= 0 {
			continue
		}
		scale := boxW / iw
		if strings.EqualFold(req.FitMode, "contain") || req.FitMode == "" {
			if s := boxH / ih; s < scale {
				scale = s
			}
		} else {
			if s := boxH / ih; s > scale {
				scale = s
			}
		}
		dw := iw * scale
		dh := ih * scale
		dx := boxX + (boxW-dw)/2
		dy := boxY + (boxH-dh)/2
		rect := &gopdf.Rect{W: dw, H: dh}
		doc.SaveGraphicsState()
		doc.ClipPolygon([]gopdf.Point{
			{X: boxX, Y: boxY},
			{X: boxX + boxW, Y: boxY},
			{X: boxX + boxW, Y: boxY + boxH},
			{X: boxX, Y: boxY + boxH},
		})
		if err := doc.Image(tmp, dx, dy, rect); err != nil {
			return Fail("PDF_DRAW_FAILED", err.Error())
		}
		doc.RestoreGraphicsState()
		written++
	}
	if written == 0 {
		return Fail("PDF_NO_PAGES", "没有可写入的图像")
	}
	target := ensureExtension(req.OutputPath, "pdf")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	plain := filepath.Join(tmpDir, "plain.pdf")
	if err := doc.WritePdf(plain); err != nil {
		return Fail("PDF_WRITE_FAILED", err.Error())
	}
	prepared := filepath.Join(tmpDir, "srgb.pdf")
	if err := tagPDFSRGB(plain, prepared); err != nil {
		return Fail("PDF_COLOR_FAILED", err.Error())
	}
	if ctx.Err() != nil {
		return Fail("CANCELLED", "已取消")
	}
	if err := output.Atomic(target, func(dst *os.File) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		src, err := os.Open(prepared)
		if err != nil {
			return err
		}
		defer src.Close()
		if req.UserPasswd == "" {
			_, err = dst.ReadFrom(src)
			if err != nil {
				return err
			}
			return ctx.Err()
		}
		conf := pdfmodel.NewDefaultConfiguration()
		conf.UserPW = req.UserPasswd
		conf.OwnerPW = req.UserPasswd + "-owner"
		conf.EncryptUsingAES = true
		conf.EncryptKeyLength = 256
		if err := api.Encrypt(ctx, src, dst, conf); err != nil {
			return err
		}
		return ctx.Err()
	}); err != nil {
		if ctx.Err() != nil {
			return Fail("CANCELLED", "已取消")
		}
		return Fail("PDF_WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Count = written
	if st, err := os.Stat(target); err == nil {
		out.OutputSize = st.Size()
	}
	return out
}

func gridFor(layout string, rows, cols int) (int, int) {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "1", "single":
		return 1, 1
	case "2", "two":
		return 1, 2
	case "2x2", "four":
		return 2, 2
	case "3x3", "nine":
		return 3, 3
	case "4x4", "sixteen":
		return 4, 4
	case "custom":
		return clampInt(rows, 1, 10), clampInt(cols, 1, 10)
	}
	return 1, 1
}

// pdfMargin resolves the page margin. A negative value means "not set": the
// panel's number field offers 0 as a flush-to-edge layout, so treating 0 as
// unset silently replaced the user's choice with 36 — the same bug the
// watermark margin had. 0 is a real value. The margin is capped at a quarter of
// the page's short side so the usable area stays positive on every paper.
func pdfMargin(margin float64, page paperSize) float64 {
	if margin < 0 {
		margin = 36
	}
	if limit := math.Min(page.w, page.h) / 4; margin > limit {
		margin = limit
	}
	return margin
}

// RenderRichText draws structured blocks onto a canvas.
func RenderRichText(req RichTextRequest) Result {
	if strings.TrimSpace(req.OutputPath) == "" {
		return Fail("RT_BAD_REQUEST", "缺少输出路径")
	}
	if len(req.Blocks) == 0 {
		return Fail("RT_NO_BLOCKS", "内容为空")
	}
	canvas, err := RenderRichTextImage(req)
	if err != nil {
		return Fail("RT_RENDER_FAILED", err.Error())
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
		return Fail("WRITE_FAILED", err.Error())
	}
	out := OK()
	out.OutputPath = target
	out.Width = canvas.Bounds().Dx()
	out.Height = canvas.Bounds().Dy()
	return out
}

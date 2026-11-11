package codec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	"github.com/gen2brain/heic"
	"github.com/gen2brain/jpegli"
	"github.com/gen2brain/webp"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/sergeymakinen/go-ico"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"github.com/lllll081926i/imageflow-go/internal/colorprofile"
	"github.com/lllll081926i/imageflow-go/internal/output"
	"github.com/lllll081926i/imageflow-go/internal/svg"
)

// ErrUnsupported reports a format the app cannot read or write.
var ErrUnsupported = errors.New("不支持的图像格式")

// OutputFormats lists the formats the converter offers, in the order the page
// shows them. The UI reads this list, so the labels and the engine cannot drift
// apart.
var OutputFormats = []string{"jpg", "png", "webp", "avif", "tiff", "bmp", "ico"}

// QualityFormats are the formats that honour a quality setting.
var QualityFormats = map[string]bool{"jpg": true, "jpeg": true, "webp": true, "avif": true}

func init() {
	// Register the decoders that follow the standard image registry so that
	// image.Decode understands them when a caller sniffs the format.
	image.RegisterFormat("webp", "RIFF????WEBPVP8", func(r io.Reader) (image.Image, error) { return webp.Decode(r) }, func(r io.Reader) (image.Config, error) { return webp.DecodeConfig(r) })
	image.RegisterFormat("avif", "\x00\x00\x00 ftypavif", func(r io.Reader) (image.Image, error) { return avif.Decode(r) }, func(r io.Reader) (image.Config, error) { return avif.DecodeConfig(r) })
	image.RegisterFormat("heic", "\x00\x00\x00 ftypheic", func(r io.Reader) (image.Image, error) { return heic.Decode(r) }, func(r io.Reader) (image.Config, error) { return heic.DecodeConfig(r) })
}

// IsSVG reports whether path names an SVG document.
func IsSVG(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".svg")
}

// OpenResult is a decoded image together with the metadata callers need.
type OpenResult struct {
	Image      image.Image
	Format     string
	Width      int
	Height     int
	Normalized bool
}

// Open decodes path, applying EXIF orientation for raster formats and
// rasterising SVG through the shared safe reader.
func Open(path string) (*OpenResult, error) {
	if _, _, _, err := Probe(path); err != nil {
		return nil, err
	}
	if IsSVG(path) {
		img, err := svg.Render(path, 0, 0)
		if err != nil {
			return nil, err
		}
		return &OpenResult{Image: img, Format: "SVG", Width: img.Bounds().Dx(), Height: img.Bounds().Dy()}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	format := strings.ToUpper(strings.TrimPrefix(filepath.Ext(path), "."))
	var img image.Image
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		img, err = jpeg.Decode(f)
		format = "JPEG"
	case "png":
		img, err = png.Decode(f)
	case "apng":
		// An APNG decodes as a PNG; only its first frame is the still image, and
		// the GIF tool reads the animation separately.
		img, err = png.Decode(f)
		format = "APNG"
	case "gif":
		img, err = gif.Decode(f)
	case "bmp":
		img, err = bmp.Decode(f)
	case "tif", "tiff":
		img, err = tiff.Decode(f)
		format = "TIFF"
	case "webp":
		img, err = webp.Decode(f, webp.Options{AutoRotate: true})
	case "avif":
		img, err = avif.Decode(f, avif.Options{AutoRotate: true})
		format = "AVIF"
	case "heic", "heif":
		img, err = heic.Decode(f)
		format = "HEIF"
	case "ico":
		img, err = ico.Decode(f)
		format = "ICO"
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, format)
	}
	if err != nil {
		return nil, err
	}
	m, err := readMetadata(path)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizesSource(m)
	if err != nil {
		return nil, err
	}
	if normalized {
		if len(m.icc) > 0 {
			img, err = colorprofile.ToSRGB(img, m.icc)
		} else {
			encoding, encErr := workingEncoding(m)
			if encErr != nil {
				return nil, encErr
			}
			img, err = colorprofile.DecodeEncoding(img, *encoding)
		}
		if err != nil {
			return nil, err
		}
	}
	if format == "JPEG" || format == "PNG" || format == "APNG" {
		img = applyOrientation(path, img)
	}
	b := img.Bounds()
	return &OpenResult{Image: img, Format: format, Width: b.Dx(), Height: b.Dy(), Normalized: normalized}, nil
}

// Probe returns only the pixel dimensions, without decoding the pixels fully.
func Probe(path string) (width, height int, format string, err error) {
	if IsSVG(path) {
		size, serr := svg.SizeOf(path)
		if serr != nil {
			return 0, 0, "SVG", serr
		}
		return size.W, size.H, "SVG", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, 0, "", err
	}
	if info.Size() > MaxSourceBytes {
		return 0, 0, "", fmt.Errorf("图片源文件超过 %d MiB 限制", MaxSourceBytes>>20)
	}
	cfg, err := decodeConfig(f, path)
	if err != nil {
		return 0, 0, "", err
	}
	if err := output.CheckSize(cfg.Width, cfg.Height); err != nil {
		return 0, 0, "", err
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".apng" || ext == ".webp" {
		if o := orientation(path); o >= 5 && o <= 8 {
			cfg.Width, cfg.Height = cfg.Height, cfg.Width
		}
	}
	// image.DecodeConfig reports whichever decoder registered that magic number
	// first, and github.com/kettek/apng sorts before image/png, so a still PNG
	// comes back as "apng". The extension is the authoritative label here: the
	// app dispatches by extension everywhere else.
	return cfg.Width, cfg.Height, formatName(path, ""), nil
}

// formatName labels a file for the UI: the extension's canonical name, falling
// back to the decoder registry's when the extension is unfamiliar.
func formatName(path, decoded string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "jpg", "jpeg":
		return "JPEG"
	case "png":
		return "PNG"
	case "apng":
		return "APNG"
	case "gif":
		return "GIF"
	case "bmp":
		return "BMP"
	case "tif", "tiff":
		return "TIFF"
	case "webp":
		return "WEBP"
	case "avif":
		return "AVIF"
	case "heic", "heif":
		return "HEIF"
	case "ico":
		return "ICO"
	case "svg":
		return "SVG"
	}
	return strings.ToUpper(decoded)
}

func applyOrientation(path string, img image.Image) image.Image {
	o := orientation(path)
	if o >= 2 && o <= 8 && highDepth(img) {
		return orientHighDepth(img, o)
	}
	switch o {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	}
	return img
}

func orientation(path string) int {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".png" || ext == ".apng" || ext == ".webp" {
		m, err := readMetadata(path)
		if err != nil || len(m.exif) == 0 {
			return 1
		}
		meta, err := exif.Decode(bytes.NewReader(m.exif))
		if err != nil {
			return 1
		}
		tag, err := meta.Get(exif.Orientation)
		if err != nil {
			return 1
		}
		value, err := tag.Int(0)
		if err != nil {
			return 1
		}
		return value
	}
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()
	meta, err := exif.Decode(f)
	if err != nil {
		return 1
	}
	tag, err := meta.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	value, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return value
}

// SaveOptions configures one encode.
type SaveOptions struct {
	Quality   int
	IcoSizes  []int
	Lossless  bool
	StripMeta bool
	// SRGB 表示像素已归一化，输出时标记标准 sRGB，替代源 ICC。
	SRGB bool

	// MetadataSource 指定元数据来源；默认保留 ICC。
	// KeepMetadata 额外保留 EXIF / XMP，不支持的输出格式明确报错。
	KeepMetadata   bool
	MetadataSource string
}

// Save encodes img to path in the format named by the extension.
func Save(img image.Image, path string, opts SaveOptions) error {
	return SaveContext(context.Background(), img, path, opts)
}

// SaveContext 在编码前和原子提交前响应取消，不改变已有输出。
func SaveContext(ctx context.Context, img image.Image, path string, opts SaveOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return fmt.Errorf("%w: 缺少输出扩展名", ErrUnsupported)
	}
	if opts.MetadataSource != "" || opts.SRGB {
		data, err := EncodeBytes(img, ext, opts)
		if err != nil {
			return err
		}
		return writeFile(path, func(f *os.File) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if _, err := f.Write(data); err != nil {
				return err
			}
			return ctx.Err()
		})
	}
	if err := writeFile(path, func(f *os.File) error {
		if err := encode(f, img, ext, opts); err != nil {
			return err
		}
		return ctx.Err()
	}); err != nil {
		return err
	}
	return nil
}

// EncodeBytes encodes img to memory, used for previews.
func EncodeBytes(img image.Image, ext string, opts SaveOptions) ([]byte, error) {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if (ext == "bmp" || ext == "ico" || ext == "gif") && opts.MetadataSource != "" && !opts.SRGB {
		m, err := readMetadata(opts.MetadataSource)
		if err != nil {
			return nil, err
		}
		if opts.KeepMetadata && !opts.StripMeta && len(m.exif)+len(m.xmp) > 0 {
			return nil, fmt.Errorf("%s 输出不能保留 EXIF / XMP，请关闭保留元数据", ext)
		}
		normalized, err := normalizesSource(m)
		if err != nil {
			return nil, err
		}
		if !normalized {
			img, err = colorprofile.ToSRGB(img, m.icc)
			if err != nil {
				return nil, err
			}
		}
		opts.SRGB = true
	}
	buf := &memWriter{}
	if err := encode(buf, img, strings.ToLower(strings.TrimPrefix(ext, ".")), opts); err != nil {
		return nil, err
	}
	if opts.StripMeta {
		return buf.data, nil
	}
	if opts.SRGB {
		return srgbMetadata(buf.data, ext)
	}
	return PreserveMetadata(buf.data, ext, opts.MetadataSource, opts.KeepMetadata)
}

func encode(w writer, img image.Image, ext string, opts SaveOptions) error {
	quality := opts.Quality
	if quality <= 0 || quality > 100 {
		quality = 90
	}
	switch ext {
	case "jpg", "jpeg":
		o := &jpegli.EncodingOptions{
			Quality:              quality,
			ProgressiveLevel:     2,
			OptimizeCoding:       true,
			AdaptiveQuantization: true,
		}
		jpegli.Init()
		return jpegli.Encode(w, flatten(img), o)
	case "png":
		e := png.Encoder{CompressionLevel: png.BestCompression}
		return e.Encode(w, img)
	case "webp":
		if opts.Lossless {
			return webp.Encode(w, img, webp.Options{Lossless: true, Method: 4})
		}
		return webp.Encode(w, img, webp.Options{Quality: quality, Method: 4})
	case "avif":
		return avif.Encode(w, img, avif.Options{Quality: quality, QualityAlpha: quality, Speed: 8})
	case "bmp":
		return bmp.Encode(w, img)
	case "tif", "tiff":
		return tiff.Encode(w, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
	case "gif":
		return gif.Encode(w, img, nil)
	case "ico":
		sizes := opts.IcoSizes
		if len(sizes) == 0 {
			sizes = []int{16, 32, 48, 256}
		}
		images := make([]image.Image, 0, len(sizes))
		for _, s := range sizes {
			// An ICO directory stores one byte per side and its encoders reject an
			// entry beyond 256, so a size outside that range is dropped rather than
			// failing the whole conversion.
			if s < 1 || s > 256 {
				continue
			}
			images = append(images, resizeSquare(img, s))
		}
		if len(images) == 0 {
			return fmt.Errorf("%w: ICO 尺寸需在 1–256 之间", ErrUnsupported)
		}
		return ico.EncodeAll(w, images)
	}
	return fmt.Errorf("%w: %s", ErrUnsupported, ext)
}

// flatten drops alpha for formats that cannot carry it.
func flatten(img image.Image) image.Image {
	if _, ok := img.(*image.NRGBA); ok {
		if opaque(img) {
			return img
		}
	}
	if opaque(img) {
		return img
	}
	dst := image.NewNRGBA(img.Bounds())
	bg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			// RGBA() returns alpha-premultiplied channels. Converting through
			// NRGBAModel first recovers the straight values, so alpha is applied
			// exactly once. Multiplying the premultiplied channels by alpha again
			// made translucent pixels markedly darker than their source.
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			alpha := float64(c.A) / 255
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(float64(c.R)*alpha + float64(bg.R)*(1-alpha)),
				G: uint8(float64(c.G)*alpha + float64(bg.G)*(1-alpha)),
				B: uint8(float64(c.B)*alpha + float64(bg.B)*(1-alpha)),
				A: 255,
			})
		}
	}
	return dst
}

func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

func resizeSquare(img image.Image, size int) image.Image {
	// An ICO entry is square: the format's directory records a single side per
	// entry and Windows draws it as a square. imaging.Fit kept the source ratio, so
	// a 16:9 image produced 16×9 entries — a malformed icon set that some readers
	// reject. Fill centre-crops instead, which is what an icon wants.
	return imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)
}

type writer interface {
	Write(p []byte) (int, error)
}

type memWriter struct{ data []byte }

func (m *memWriter) Write(p []byte) (int, error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

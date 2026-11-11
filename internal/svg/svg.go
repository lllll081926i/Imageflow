package svg

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/boolean-maybe/go-resvg"
)

// Limits mirror the security budget of the Python backend.
const (
	MaxSourceBytes  = 8 << 20
	MaxCanvasSide   = 8192
	MaxCanvasPixels = 16_000_000
	defaultDPI      = 96.0
)

// ErrUnsafe reports an SVG that contains a construct the renderer must not see.
var ErrUnsafe = errors.New("SVG 包含被禁止的构造")

var (
	reUnsafe   = regexp.MustCompile(`(?i)<!DOCTYPE|<!ENTITY|<!ELEMENT|<!ATTLIST`)
	reScript   = regexp.MustCompile(`(?i)<\s*script\b|<\s*foreignObject\b|<\s*iframe\b`)
	rePI       = regexp.MustCompile(`(?i)<\?[a-z]`)
	reXMLDecl  = regexp.MustCompile(`(?i)<\?xml(?:\s|\?>)`)
	reExternal = regexp.MustCompile(`(?i)(?:xlink:)?href\s*=\s*["']\s*(?:file:|https?:|ftp:|//|[a-zA-Z]:[\\/]|\\\\)`)
	reEvent    = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
)

var (
	rendererOnce sync.Once
	renderer     *resvg.Renderer
	rendererErr  error
)

func shared() (*resvg.Renderer, error) {
	rendererOnce.Do(func() {
		renderer, rendererErr = resvg.New()
	})
	return renderer, rendererErr
}

// Size is an SVG's intrinsic pixel size at 96 DPI.
type Size struct {
	W int
	H int
}

// readSafe reads the document under the customisation-point security budget and
// rejects active content, external references and entities.
func readSafe(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxSourceBytes {
		return nil, fmt.Errorf("SVG 源文件超过 %d MiB 限制", MaxSourceBytes>>20)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := Validate(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// Validate rejects a document that may read local files, run code or expand
// entities. It runs on the raw bytes before the renderer parses them.
func Validate(raw []byte) error {
	text := string(raw)
	if len(raw) > MaxSourceBytes {
		return fmt.Errorf("SVG 源文件超过 %d MiB 限制", MaxSourceBytes>>20)
	}
	switch {
	case reUnsafe.MatchString(text):
		return fmt.Errorf("%w: DOCTYPE/ENTITY 声明", ErrUnsafe)
	case reScript.MatchString(text):
		return fmt.Errorf("%w: script/foreignObject/iframe", ErrUnsafe)
	case rePI.MatchString(stripXMLDecl(text)):
		return fmt.Errorf("%w: 处理指令", ErrUnsafe)
	case reExternal.MatchString(text):
		return fmt.Errorf("%w: 外部资源引用", ErrUnsafe)
	case reEvent.MatchString(text):
		return fmt.Errorf("%w: 事件处理属性", ErrUnsafe)
	}
	if err := checkHrefAttributes(raw); err != nil {
		return err
	}
	root, err := rootElement(raw)
	if err != nil {
		return err
	}
	if !strings.EqualFold(localName(root.Name.Local), "svg") {
		return errors.New("SVG 缺少 svg 根元素")
	}
	if _, err := resolveIntrinsicSize(root); err != nil {
		return err
	}
	return nil
}

// stripXMLDecl removes the XML declaration so the processing-instruction
// check does not flag it: Go's regexp has no lookahead.
func stripXMLDecl(text string) string {
	if loc := reXMLDecl.FindStringIndex(text); loc != nil && loc[0] == 0 {
		return text[loc[1]:]
	}
	return text
}

// checkHrefAttributes walks the parsed tree so encoded and namespaced hrefs are
// checked after decoding, which the raw regex cannot do.
func checkHrefAttributes(raw []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				return nil
			}
			// A parse error is not a clean end of document. Returning nil here
			// used to skip the rest of the attribute walk, so a malformed SVG
			// with a file:// href after the bad markup passed validation.
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range start.Attr {
			name := strings.ToLower(localName(attr.Name.Local))
			if name == "href" || name == "src" || name == "base" || name == "absref" {
				if err := checkReference(attr.Value); err != nil {
					return err
				}
			}
			if strings.HasPrefix(name, "on") {
				return fmt.Errorf("%w: 事件处理属性 %s", ErrUnsafe, name)
			}
		}
	}
}

func checkReference(value string) error {
	v := strings.TrimSpace(value)
	if v == "" || strings.HasPrefix(v, "#") {
		return nil
	}
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "data:image/png;base64,") ||
		strings.HasPrefix(lower, "data:image/jpeg;base64,") ||
		strings.HasPrefix(lower, "data:image/jpg;base64,") ||
		strings.HasPrefix(lower, "data:image/gif;base64,") ||
		strings.HasPrefix(lower, "data:image/webp;base64,") {
		return nil
	}
	return fmt.Errorf("%w: 引用 %q", ErrUnsafe, v)
}

func rootElement(raw []byte) (xml.StartElement, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	for {
		tok, err := decoder.Token()
		if err != nil {
			return xml.StartElement{}, fmt.Errorf("SVG XML 无效: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start, nil
		}
	}
}

func localName(name string) string {
	if i := strings.LastIndex(name, ":"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// resolveIntrinsicSize implements the root viewport rules: absolute lengths or
// a viewBox decide the canvas, percentages need a viewBox, and anything else
// fails instead of being guessed.
func resolveIntrinsicSize(root xml.StartElement) (Size, error) {
	var width, height float64
	var hasW, hasH bool
	var viewBox *[4]float64
	for _, attr := range root.Attr {
		switch strings.ToLower(localName(attr.Name.Local)) {
		case "width":
			if v, ok := parseAbsolute(attr.Value); ok {
				width, hasW = v, true
			} else if strings.HasSuffix(strings.TrimSpace(attr.Value), "%") {
				hasW = false
			} else {
				return Size{}, fmt.Errorf("SVG 宽度无法确定: %q", attr.Value)
			}
		case "height":
			if v, ok := parseAbsolute(attr.Value); ok {
				height, hasH = v, true
			} else if strings.HasSuffix(strings.TrimSpace(attr.Value), "%") {
				hasH = false
			} else {
				return Size{}, fmt.Errorf("SVG 高度无法确定: %q", attr.Value)
			}
		case "viewbox":
			if vb, ok := parseViewBox(attr.Value); ok {
				viewBox = &vb
			}
		}
	}
	if hasW && hasH && width > 0 && height > 0 {
		return clampSize(int(width), int(height))
	}
	if viewBox != nil {
		if hasW && width > 0 {
			h := width * viewBox[3] / viewBox[2]
			return clampSize(int(width), int(h))
		}
		if hasH && height > 0 {
			w := height * viewBox[2] / viewBox[3]
			return clampSize(int(w), int(height))
		}
		return clampSize(int(viewBox[2]), int(viewBox[3]))
	}
	return Size{}, errors.New("SVG 根元素缺少可确定的尺寸或 viewBox")
}

func clampSize(w, h int) (Size, error) {
	if w <= 0 || h <= 0 {
		return Size{}, fmt.Errorf("SVG 尺寸无效: %dx%d", w, h)
	}
	if w > MaxCanvasSide || h > MaxCanvasSide || w*h > MaxCanvasPixels {
		return Size{}, fmt.Errorf("SVG 画布 %dx%d 超过 %dpx/%dMP 限制", w, h, MaxCanvasSide, MaxCanvasPixels>>20)
	}
	return Size{W: w, H: h}, nil
}

func parseViewBox(value string) ([4]float64, bool) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(fields) != 4 {
		return [4]float64{}, false
	}
	var out [4]float64
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return [4]float64{}, false
		}
		out[i] = v
	}
	if out[2] <= 0 || out[3] <= 0 {
		return [4]float64{}, false
	}
	return out, true
}

// parseAbsolute reads a root length in px/in/cm/mm/pt/pc or unitless.
func parseAbsolute(value string) (float64, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}
	unit := ""
	for _, u := range []string{"px", "in", "cm", "mm", "pt", "pc"} {
		if strings.HasSuffix(v, u) {
			unit = u
			v = strings.TrimSuffix(v, u)
			break
		}
	}
	if strings.HasSuffix(v, "%") || v == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	switch unit {
	case "in":
		n *= defaultDPI
	case "cm":
		n *= defaultDPI / 2.54
	case "mm":
		n *= defaultDPI / 25.4
	case "pt":
		n *= defaultDPI / 72
	case "pc":
		n *= defaultDPI / 6
	}
	return n, true
}

// SizeOf returns the intrinsic size of the SVG at path.
func SizeOf(path string) (Size, error) {
	raw, err := readSafe(path)
	if err != nil {
		return Size{}, err
	}
	root, err := rootElement(raw)
	if err != nil {
		return Size{}, err
	}
	return resolveIntrinsicSize(root)
}

// RenderBytes rasterises an in-memory SVG document. It is the path the app's own
// logo takes: the mark is compiled in, so it is never read from disk and cannot
// be replaced by a file next to the executable.
func RenderBytes(raw []byte, width, height int) (image.Image, error) {
	if err := Validate(raw); err != nil {
		return nil, err
	}
	root, err := rootElement(raw)
	if err != nil {
		return nil, err
	}
	intrinsic, err := resolveIntrinsicSize(root)
	if err != nil {
		return nil, err
	}
	target := fitTarget(intrinsic, width, height)
	r, err := shared()
	if err != nil {
		return nil, err
	}
	img, err := r.Render(raw, resvg.RenderOptions{Width: target.W, Height: target.H})
	if err != nil {
		return nil, fmt.Errorf("SVG 光栅化失败: %w", err)
	}
	return img, nil
}

// Render rasterises the SVG at path. A zero width or height keeps the
// intrinsic size; a positive pair scales the result.
func Render(path string, width, height int) (image.Image, error) {
	raw, err := readSafe(path)
	if err != nil {
		return nil, err
	}
	intrinsic, err := SizeOf(path)
	if err != nil {
		return nil, err
	}
	target := fitTarget(intrinsic, width, height)
	r, err := shared()
	if err != nil {
		return nil, err
	}
	img, err := r.Render(raw, resvg.RenderOptions{Width: target.W, Height: target.H})
	if err != nil {
		return nil, fmt.Errorf("SVG 光栅化失败: %w", err)
	}
	return img, nil
}

func fitTarget(intrinsic Size, width, height int) Size {
	switch {
	case width > 0 && height > 0:
		return Size{W: width, H: height}
	case width > 0:
		h := int(float64(width) * float64(intrinsic.H) / float64(intrinsic.W))
		if h < 1 {
			h = 1
		}
		return Size{W: width, H: h}
	case height > 0:
		w := int(float64(height) * float64(intrinsic.W) / float64(intrinsic.H))
		if w < 1 {
			w = 1
		}
		return Size{W: w, H: height}
	}
	return intrinsic
}

// Ext returns the lower-case extension without the dot.
func Ext(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

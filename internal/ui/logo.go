package ui

// logo.go draws the application mark.
//
// The SVG itself lives in the brand package, together with the trimmed variant
// the icon generator uses, so the window and the .exe icon are built from one
// document. Everything here is a thin rasterising cache on top of it.

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"github.com/lllll081926i/imageflow-go/internal/brand"
	"github.com/lllll081926i/imageflow-go/internal/svg"
)

// logoKV caches one rasterised mark per (document, edge) pair, so the rail, the
// title bar, the About page and the tray never re-render the SVG per frame.
type logoKV struct {
	doc  int // 0 = full document, 1 = trimmed icon document
	size int
}

var logoCache sync.Map // logoKV -> image.Image

func rasterLogo(doc int, size int) image.Image {
	if size < 8 {
		size = 8
	}
	key := logoKV{doc: doc, size: size}
	if cached, ok := logoCache.Load(key); ok {
		return cached.(image.Image)
	}
	src := brand.LogoSVG
	if doc == 1 {
		src = brand.IconSVG
	}
	img, err := svg.RenderBytes([]byte(src), size, size)
	if err != nil {
		return nil
	}
	logoCache.Store(key, img)
	return img
}

// logoImage returns the full mark rasterised to a sizexsize square. It keeps the
// original document's margin, which is right for the About page where the mark
// sits on its own.
func logoImage(size int) image.Image { return rasterLogo(0, size) }

// iconImage returns the mark trimmed to its own bounds. Every place that shows
// the logo at small sizes uses this: the title bar's badge and the tray icon both
// looked a size too small because the original document leaves a 9% margin on
// every side.
func iconImage(size int) image.Image { return rasterLogo(1, size) }

// iconPNG returns the trimmed mark as PNG bytes.
func iconPNG(size int) []byte { return pngOf(iconImage(size)) }

func pngOf(img image.Image) []byte {
	if img == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

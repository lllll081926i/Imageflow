package ui

// logo.go is the thin cache the interface draws through.
//
// The artwork and the render path live in internal/brand, so the window's mark and
// the packager's icon.png come from one implementation. This file keeps the sizes
// the pages ask for in one place and hands MyGo a bitmap it can draw.

import (
	"image"
	"sync"

	myui "github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/brand"
)

// The sizes the interface asks for. They are named because three of them are
// design decisions, not arbitrary numbers: 36 is the landing page's header tile,
// 40 the About row's, 32 the title-bar badge and the tray glyph.
const (
	logoHomeSize  = 36
	logoAboutSize = 40
	logoBadgeSize = 32
)

// logoImage returns the mark at one size, on a transparent canvas.
//
// Every caller gets the same cached bitmap per size: the mark is a 1.3 MB PNG and
// re-rendering it per frame would be a measurable cost for a picture that never
// changes.
func logoImage(size int) image.Image { return brand.MarkAt(size) }

// iconImage is the mark used where a badge stands alone (the About row, the rail's
// header). It is the same render path, kept as a separate name because the two call
// sites read differently.
func iconImage(size int) image.Image { return brand.MarkAt(size) }

// iconPNG returns the mark at one size as PNG bytes, for the tray and the platform.
func iconPNG(size int) []byte { return brand.PNG(size) }

var logoBitmaps sync.Map

func logoBitmap(size int) *myui.Bitmap {
	if cached, ok := logoBitmaps.Load(size); ok {
		return cached.(*myui.Bitmap)
	}
	mark := iconImage(size)
	if mark == nil {
		return nil
	}
	bitmap, _ := logoBitmaps.LoadOrStore(size, myui.NewBitmap(mark))
	return bitmap.(*myui.Bitmap)
}

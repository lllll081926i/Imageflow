package ui

// logo.go is the thin cache the interface draws through.
//
// The pre-rendered squares and the colour live in internal/brand: the icon
// generator writes those files from the artwork, so the window's mark, the tray
// and the packager's icon.png come from one render. This file keeps the sizes the
// pages ask for in one place and hands MyGo a bitmap it can draw.

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
// Every caller gets the same cached picture per size: internal/brand decodes the
// embedded PNG once, and the bitmap cache below converts it to a GPU-friendly
// bitmap once, for a picture that never changes.
func logoImage(size int) image.Image { return brand.Mark(size) }

// iconImage is the mark used where a badge stands alone (the About row, the rail's
// header). It is the same render path, kept as a separate name because the two call
// sites read differently.
func iconImage(size int) image.Image { return brand.Mark(size) }

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

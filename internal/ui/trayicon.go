package ui

// trayicon.go hands the platform the same mark the window draws.

import (
	"path/filepath"
	"sync"
)

var (
	trayIconOnce sync.Once
	trayIconData []byte
)

// trayIconPNG returns the tray icon: the application mark rasterised to the size
// a menu bar asks for. It is the original project's logo, not a redrawn lookalike,
// so the tray, the taskbar and the window all show one shape.
//
// The trimmed document is used so the mark fills the icon: the full logo carries
// a 9% margin on every side, which at 32 px reads as a smaller picture.
func trayIconPNG() []byte {
	trayIconOnce.Do(func() {
		if data := iconPNG(32); data != nil {
			trayIconData = data
		}
	})
	return trayIconData
}

// parentDir is filepath.Dir with an empty result replaced by ".", which the
// engine layer can always use.
func parentDir(path string) string {
	dir := filepath.Dir(path)
	if dir == "" {
		return "."
	}
	return dir
}

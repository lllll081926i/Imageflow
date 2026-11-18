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
// a menu bar asks for. It is the same artwork the window draws, so the tray, the
// taskbar and the window all show one shape.
//
// 32 px is what a Windows tray wants; the mark is rendered from the artwork's own
// tile crop, so it fills the icon instead of carrying the source canvas's margin.
func trayIconPNG() []byte {
	trayIconOnce.Do(func() {
		if data := iconPNG(logoBadgeSize); data != nil {
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

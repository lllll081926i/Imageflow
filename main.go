package main

import (
	"bytes"
	"flag"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/settings"
	appui "github.com/lllll081926i/imageflow-go/internal/ui"
)

// capturePath is set by -capture: the app opens its window, saves a PNG of what
// the native window actually draws, and quits. It exists so a packaged build can
// be verified in CI without a person looking at the screen.
var capturePath = flag.String("capture", "", "save a PNG of the window and exit")

// openPaths preloads inputs, so a capture can show a populated file list.
var openPaths = flag.String("open", "", "comma-separated files or folders to load at startup")

// toolIndex selects the tool page a capture should show.
var toolIndex = flag.Int("tool", 0, "tool page index to show at startup")

// startHidden opens the app in the tray only, which is what an autostart entry
// uses when the user does not want a window at login.
var startHidden = flag.Bool("hidden", false, "start minimised to the tray")

func main() {
	flag.Parse()

	// Stills are captured by their own short-lived run, so they must not be
	// blocked by, nor take, the single-instance lock.
	capturing := *capturePath != ""
	if !capturing && !mygo.App.RequestSingleInstanceLock() {
		// Another instance owns the app; it brought itself to the front.
		return
	}

	cfg := settings.Load()

	// With window state memory off, the window must not reuse the last geometry.
	// MyGo keys the saved rectangle by StateKey, so an empty key means nothing is
	// remembered.
	stateKey := "main"
	if !cfg.RememberWindowState {
		stateKey = ""
	}

	mygo.App.WhenReady(func() {
		app := appui.NewApp(cfg)
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "ImageFlow",
			Width:     1440,
			Height:    900,
			MinWidth:  960,
			MinHeight: 620,
			StateKey:  stateKey,
			Hidden:    *startHidden && !capturing,
			// The native title bar stays: its corners are the window's, and a
			// drawn one inside a rounded frame shows two radii that never match.
			// The window controls are the platform's own, so they behave as every
			// other Windows app's do.
			Content: ui.View(app.View),
		})
		app.Window = win
		if *toolIndex >= 0 && *toolIndex < len(app.Tools) {
			app.Active = *toolIndex
		}
		if *openPaths != "" {
			app.AddPaths(strings.Split(*openPaths, ","))
		}
		win.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) > 0 {
				app.AddPaths(e.Paths)
				win.Invalidate()
			}
		})

		// Closing the window hides it while the tray keeps the app alive, as the
		// original app did with its background-running setting on.
		win.OnClose(func(e *mygo.CloseEvent) {
			if !app.ShouldHideOnClose() {
				app.CancelImport()
				return
			}
			e.PreventDefault()
			win.Hide()
		})

		if !capturing {
			mygo.App.OnSecondInstance(func(args []string, _ string) {
				win.Show()
				win.Focus()
				if len(args) > 0 {
					app.AddPaths(args)
				}
			})
			if !app.StartTray() {
				// Without a tray entry there is nothing to restore the window
				// from, so closing quits instead of hiding.
				app.SetQuitWithoutTray()
			}
			app.AutoCheckUpdates()
		}
		if capturing {
			go captureAndQuit(win)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func captureAndQuit(win *mygo.Window) {
	// Give the window a moment to paint its first frames.
	time.Sleep(1200 * time.Millisecond)
	data, err := win.CapturePage()
	if err != nil {
		log.Printf("capture failed: %v", err)
		mygo.App.Exit(1)
		return
	}
	// Some platforms hand back an opaque image; normalise to PNG so the file is
	// always a real PNG whatever the backend returned.
	if !bytes.HasPrefix(data, []byte("\x89PNG")) {
		img, derr := png.Decode(bytes.NewReader(data))
		if derr == nil {
			var buf bytes.Buffer
			if encErr := png.Encode(&buf, img); encErr == nil {
				data = buf.Bytes()
			}
		}
	}
	if err := os.WriteFile(*capturePath, data, 0o644); err != nil {
		log.Printf("write capture failed: %v", err)
		mygo.App.Exit(1)
		return
	}
	log.Printf("captured %d bytes to %s", len(data), *capturePath)
	mygo.App.Exit(0)
}

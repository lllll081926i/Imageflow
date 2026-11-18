package main

import (
	"bytes"
	"context"
	"flag"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/appcache"
	"github.com/lllll081926i/imageflow-go/internal/brand"
	"github.com/lllll081926i/imageflow-go/internal/settings"
	appui "github.com/lllll081926i/imageflow-go/internal/ui"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
)

// capturePath is set by -capture: the app opens its window, saves a PNG of what
// the native window actually draws, and quits. It exists so a packaged build can
// be verified in CI without a person looking at the screen.
var capturePath = flag.String("capture", "", "save a PNG of the window and exit")

// openPaths preloads inputs, so a capture can show a populated file list.
var openPaths = flag.String("open", "", "comma-separated files or folders to load at startup")

// toolIndex selects the tool page a capture should show.
var toolIndex = flag.Int("tool", -1, "tool page index to show at startup")

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
	var migrationErrors []string
	if !capturing {
		if err := appcache.PruneExpired(context.Background(), cfg.AIModelsDir, time.Now()); err != nil {
			log.Printf("过期临时图片清理失败：%v", err)
		}
		roots := []string{cfg.AIModelsDir}
		if path, err := settings.Path(); err == nil {
			roots = append(roots, filepath.Join(filepath.Dir(path), "models"))
		}
		for _, root := range roots {
			if err := upscale.MigrateLegacyModels(context.Background(), root); err != nil {
				log.Printf("旧 AI 组件迁移失败：%v", err)
				migrationErrors = append(migrationErrors, err.Error())
			}
		}
	}

	// With window state memory off, the window must not reuse the last geometry.
	// MyGo keys the saved rectangle by StateKey, so an empty key means nothing is
	// remembered.
	stateKey := "main"
	if !cfg.RememberWindowState {
		stateKey = ""
	}

	mygo.App.WhenReady(func() {
		app := appui.NewApp(cfg)
		if len(migrationErrors) > 0 {
			app.SetStatus("旧 AI 组件未完全清理：" + strings.Join(migrationErrors, "；"))
		}
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
		if runtime.GOOS == "windows" {
			if err := win.SetIcon(brand.PNG(32)); err != nil {
				log.Printf("设置窗口图标失败：%v", err)
			}
		}
		mygo.App.OnQuit(func() {
			app.Cancel()
			app.CancelStudioWork()
			app.CancelCollageWork()
		})
		selectStartupTool(app, *toolIndex)
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
				app.Cancel()
				app.CancelStudioWork()
				app.CancelCollageWork()
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
			go captureAndQuit(win, app)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func selectStartupTool(app *appui.App, index int) {
	if index >= 0 && index < len(app.Tools) {
		app.Active = index
		app.ShowHome = false
	}
}

func captureAndQuit(win *mygo.Window, app *appui.App) {
	deadline := time.Now().Add(30 * time.Second)
	for app.IsImporting() {
		if time.Now().After(deadline) {
			log.Print("capture failed: startup import timed out")
			mygo.App.Exit(1)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Give the window a moment to paint its first frames.
	time.Sleep(1200 * time.Millisecond)
	log.Printf("截图准备完成：输入 %d 张，状态 %s", app.Session.Len(), app.StatusLine())
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

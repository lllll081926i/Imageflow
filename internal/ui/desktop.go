package ui

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// desktop.go holds the shell's platform wiring: the tray icon, the quit path and
// the update check. Everything here degrades to "not available" instead of
// failing, so the app runs on a desktop with no tray or a build without updates.

// tray keeps the running tray so the quit path can destroy it.
var trayOnce sync.Once
var trayHolder *mygo.Tray

// ShouldHideOnClose reports whether closing the window should leave the app
// running in the tray. It is false when the user turned background running off,
// or when there is no tray entry to restore the window from.
func (a *App) ShouldHideOnClose() bool {
	a.mu.Lock()
	quitting := a.quitting
	withoutTray := a.quitWithoutTray
	a.mu.Unlock()
	if quitting || withoutTray {
		return false
	}
	// The locked snapshot: SetSettings rewrites this struct under Session's
	// lock from a dialog goroutine, and OnClose runs concurrently with it.
	return a.Session.SettingsSnapshot().BackgroundRunning
}

// SetQuitWithoutTray makes closing the window quit: the app has no tray entry to
// restore it from, and a hidden window with no icon would be unreachable.
func (a *App) SetQuitWithoutTray() {
	a.mu.Lock()
	a.quitWithoutTray = true
	a.mu.Unlock()
}

// quit hides the tray and asks the app to exit.
func (a *App) quit() {
	a.CancelImport()
	a.CancelCollageWork()
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	if trayHolder != nil {
		trayHolder.Destroy()
		trayHolder = nil
	}
	mygo.App.Quit()
}

// StartTray installs the menu-bar / notification-area icon. It returns false when
// the platform has no tray, so the caller can keep closing as quitting.
func (a *App) StartTray() bool {
	var ok bool
	trayOnce.Do(func() {
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{
				Label: "显示主窗口",
				Click: func(*mygo.MenuItem, *mygo.Window) {
					a.ShowWindow()
				},
			},
			mygo.Separator(),
			{
				Label: "打开输出目录",
				Click: func(*mygo.MenuItem, *mygo.Window) {
					a.openOutputFolder()
				},
			},
			mygo.Separator(),
			{
				Label: "退出 ImageFlow",
				Click: func(*mygo.MenuItem, *mygo.Window) {
					a.quit()
				},
			},
		})
		tray, err := mygo.NewTray(mygo.TrayOptions{
			ID:      "imageflow",
			Icon:    trayIconPNG(),
			ToolTip: "ImageFlow",
			Menu:    menu,
		})
		if err != nil || tray == nil {
			return
		}
		trayHolder = tray
		tray.OnClick(a.ShowWindow)
		ok = true
	})
	return ok || trayHolder != nil
}

// ShowWindow brings the main window back from the tray.
func (a *App) ShowWindow() {
	if a.Window == nil {
		return
	}
	a.Window.Show()
	a.Window.Focus()
}

// openOutputFolder opens the configured output directory, or the directory of the
// first input when none is configured.
func (a *App) openOutputFolder() {
	if dir := a.outputFolder(); dir != "" {
		openPath(dir)
	}
}

// outputFolder resolves where results land: the configured directory, else the
// first input's own directory.
func (a *App) outputFolder() string {
	// The same locked read as ShouldHideOnClose: the settings can be rewritten
	// while a batch callback resolves where to open.
	if dir := a.Session.SettingsSnapshot().DefaultOutputDir; dir != "" {
		return dir
	}
	a.Session.Mu.Lock()
	defer a.Session.Mu.Unlock()
	if len(a.Session.Items) > 0 {
		return parentDir(a.Session.Items[0].Path)
	}
	return ""
}

// updateState is what the settings page shows about the last check.
type updateState struct {
	Checked bool
	Version string
	Notes   string
	Err     string
	// Busy is a check in flight; Installing is a download/install in flight.
	// They are separate so a check finishing cannot clear an install's busy
	// state and expose the "立即更新" button for a second concurrent install.
	Busy       bool
	Installing bool
	// Failed marks a check or install that could not complete.
	Failed bool
}

// AutoCheckUpdates runs one update check in the background when the setting is
// on. A build without update configuration reports ErrUpdatesDisabled, which is
// not an error worth showing.
func (a *App) AutoCheckUpdates() {
	if !a.Session.SettingsSnapshot().AutoCheckUpdates {
		return
	}
	go func() {
		// Let the window paint before the request competes for the disk.
		time.Sleep(1500 * time.Millisecond)
		a.CheckUpdates(false)
	}()
}

// CheckUpdates asks the release feed for a newer version. force re-checks even
// when a result is already known.
func (a *App) CheckUpdates(force bool) updateState {
	a.mu.Lock()
	if a.update.Busy || a.update.Installing {
		state := a.update
		a.mu.Unlock()
		return state
	}
	if a.update.Checked && !force {
		state := a.update
		a.mu.Unlock()
		return state
	}
	a.update.Busy = true
	a.mu.Unlock()
	a.Invalidate()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	up, err := mygo.Updater.Check(ctx)

	a.mu.Lock()
	a.update.Busy = false
	// Every outcome resets the stale parts of the previous result. Leaving
	// Failed/Notes behind made a later "no updates configured" reply render in
	// the red failure style with another version's release notes.
	a.update.Failed = false
	a.update.Notes = ""
	a.update.Err = ""
	a.update.Version = ""
	// Only a real conclusion counts as checked: a network error must leave the
	// automatic retry eligible on the next launch instead of short-circuiting.
	disabled := errors.Is(err, mygo.ErrUpdatesDisabled)
	if err == nil || disabled {
		a.update.Checked = true
	}
	switch {
	case err != nil:
		if !disabled {
			a.update.Err = err.Error()
			a.update.Failed = true
		}
	case up != nil:
		a.update.Version = up.Version
		a.update.Notes = up.Notes
	}
	a.lastUpdate = up
	state := a.update
	a.mu.Unlock()
	// Repaint after the lock is released: Invalidate crosses into the window, and
	// holding the state lock across a native call invites a deadlock.
	a.Invalidate()
	return state
}

// UpdateState returns the last check's result.
func (a *App) UpdateState() updateState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.update
}

// InstallUpdate downloads and installs the found update, then relaunches.
func (a *App) InstallUpdate() {
	a.mu.Lock()
	if a.update.Busy || a.update.Installing {
		a.mu.Unlock()
		return
	}
	up := a.lastUpdate
	if up == nil {
		a.mu.Unlock()
		return
	}
	a.update.Installing = true
	a.update.Err = ""
	a.update.Failed = false
	a.mu.Unlock()
	a.Invalidate()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		err := up.Install(ctx, func(downloaded, total int64) {
			if total <= 0 {
				return
			}
			percent := float32(downloaded) / float32(total) * 100
			a.SetProgress(percent, fmt.Sprintf("下载更新 %d%%", int(percent)))
			a.Invalidate()
		})
		a.mu.Lock()
		a.update.Installing = false
		if err != nil {
			// Clear the version so the settings page shows the error branch
			// instead of offering "立即更新" again with no explanation.
			a.update.Version = ""
			a.update.Err = err.Error()
			a.update.Failed = true
		}
		a.update.Busy = false
		a.mu.Unlock()
		if err != nil {
			a.SetStatus("更新失败：" + err.Error())
			a.SetProgress(0, "")
			a.Invalidate()
			return
		}
		a.SetStatus("更新已就绪，正在重启")
		a.Invalidate()
		// Relaunch closes the window. With 后台运行 enabled, OnClose would
		// otherwise prevent that close and hide the window instead, silently
		// cancelling the restart. Mark the quit intent first.
		a.mu.Lock()
		a.quitting = true
		a.mu.Unlock()
		time.Sleep(400 * time.Millisecond)
		mygo.App.Relaunch()
	}()
}

// updateBadge is the small nav decoration when a newer version is known.
func (a *App) updateBadge() string {
	state := a.UpdateState()
	if state.Version == "" {
		return ""
	}
	return "v" + state.Version
}

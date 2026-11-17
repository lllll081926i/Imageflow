package ui

// pickers_studio.go holds the background dialogs and small helpers the newer pages
// share.
//
// Two rules from the shell apply to every one of them. A dialog runs off the UI
// thread, so it never writes a page field directly: it hands the chosen path to the
// UI thread through the pending-picker queue, and the next frame applies it. And a
// decode or encode for a preview happens off the UI thread, publishing through
// Window.Update.

import (
	"image"

	"github.com/egoist/mygo"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// openSRGB decodes a picture to the app's working space.
func openSRGB(path string) (image.Image, error) {
	opened, err := codec.OpenSRGB(path)
	if err != nil {
		return nil, err
	}
	return opened.Image, nil
}

// saveImage encodes one picture to path with the format the extension names.
func saveImage(img image.Image, path string, quality int) error {
	return codec.Save(img, path, codec.SaveOptions{Quality: quality, SRGB: true})
}

// loadPreviewImage decodes a picture for a preview tile. The decode is bounded by
// the shared source budget, which Open already enforces.
func loadPreviewImage(path string) (image.Image, error) {
	return openSRGB(path)
}

// pickImageInto opens a file dialog and stores the chosen path in dst through the
// pending-picker queue. done runs on the UI thread after the path is applied, which
// is where a page starts the work that depends on it.
func (a *App) pickImageInto(dst *string, done func()) {
	a.pickInto(dst, done, mygo.OpenDialogOptions{
		Parent: a.Window,
		Title:  "选择图片",
		Filters: []mygo.FileFilter{{
			Name:       "图片",
			Extensions: []string{"png", "jpg", "jpeg", "webp", "gif", "bmp", "tif", "tiff", "avif", "ico"},
		}},
	})
}

// pickDirectoryInto opens a folder dialog and stores the path in dst.
func (a *App) pickDirectoryInto(dst *string, done func()) {
	a.pickInto(dst, done, mygo.OpenDialogOptions{
		Parent:    a.Window,
		Title:     "选择文件夹",
		Directory: true,
	})
}

// pickExecutable opens a file dialog for a local engine binary.
func (a *App) pickExecutable(dst *string, done func()) {
	a.pickInto(dst, done, mygo.OpenDialogOptions{
		Parent: a.Window,
		Title:  "选择推理引擎",
		Filters: []mygo.FileFilter{{
			Name:       "可执行文件",
			Extensions: []string{"exe", ""},
		}},
	})
}

// pickLUT opens a dialog for a .cube table. Selecting a table is the same dialog
// shape as a picture, so it reuses pickInto rather than a second path.
func (a *App) pickLUT(dst *string, done func()) {
	a.pickInto(dst, done, mygo.OpenDialogOptions{
		Parent: a.Window,
		Title:  "选择 .cube 颜色表",
		Filters: []mygo.FileFilter{{
			Name:       "3D LUT",
			Extensions: []string{"cube"},
		}},
	})
}

// pickInto runs one open dialog on its own goroutine.
//
// The dialog result goes into the same queue the shell already drains in
// applyPendingSettings: a dialog goroutine must never write a field the UI thread
// reads every frame. The completion callback is queued with it, so a page refreshes
// only after its field is really set.
func (a *App) pickInto(dst *string, done func(), opts mygo.OpenDialogOptions) {
	if a.Window == nil || dst == nil {
		return
	}
	opts.Parent = a.Window
	go func() {
		chosen, err := mygo.Dialog.Open(opts)
		if err != nil || len(chosen) == 0 {
			return
		}
		a.pickerMu.Lock()
		if a.pendingPicker == nil {
			a.pendingPicker = map[*string]string{}
		}
		a.pendingPicker[dst] = chosen[0]
		if done != nil {
			a.pendingDone = append(a.pendingDone, done)
		}
		a.pickerMu.Unlock()
		if a.Window != nil {
			a.Window.Update(func() {})
		}
	}()
}

// ensure the batch package stays referenced for the runner helpers this file's
// callers use.

var _ = batch.Item{}

package ui

import (
	"github.com/egoist/mygo"
	"github.com/lllll081926i/imageflow-go/internal/paths"
)

// picker.go holds the small dialog helpers the panels' "选择" buttons use.

// pickRichTextImage asks for one image and writes it into target.
func (a *App) pickRichTextImage(target *string) {
	if a.Window == nil {
		return
	}
	go func() {
		chosen, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.Window,
			Title:    "选择配图",
			Multiple: false,
			Filters: []mygo.FileFilter{{
				Name:       "图片",
				Extensions: paths.ImageExtensions,
				MIMETypes:  []string{"image/*"},
			}},
		})
		if err != nil || len(chosen) == 0 {
			return
		}
		// Never write the panel field from this goroutine: the TextInput bound
		// to it is read by the UI thread on every frame. Queue the result and
		// let applyPendingSettings apply it before the next render.
		a.pickerMu.Lock()
		if a.pendingPicker == nil {
			a.pendingPicker = make(map[*string]string)
		}
		a.pendingPicker[target] = chosen[0]
		a.pickerMu.Unlock()
		a.Invalidate()
	}()
}

// pickCollageCell asks for one picture to replace a collage slot.
//
// The result is queued like every other dialog result: the collage fields are
// read by the UI thread on every frame, and `applyPendingSettings` drains the
// queue before the controls read them.
func (a *App) pickCollageCell(t *CollageTool, cell int) {
	if a.Window == nil || t == nil || cell < 0 {
		return
	}
	go func() {
		chosen, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.Window,
			Title:    "替换这一格的图片",
			Multiple: false,
			Filters: []mygo.FileFilter{{
				Name:       "图片",
				Extensions: paths.ImageExtensions,
				MIMETypes:  []string{"image/*"},
			}},
		})
		if err != nil || len(chosen) == 0 {
			return
		}
		a.pickerMu.Lock()
		if t.pendingCells == nil {
			t.pendingCells = make(map[int]string)
		}
		t.pendingCells[cell] = chosen[0]
		a.pickerMu.Unlock()
		a.Invalidate()
	}()
}

// pickCollageBackground asks for one picture to use as the collage canvas
// background. The result is queued like every other dialog result: the collage
// tool's fields are read by the UI thread on every frame.
func (a *App) pickCollageBackground(t *CollageTool) {
	if a.Window == nil || t == nil {
		return
	}
	go func() {
		chosen, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.Window,
			Title:    "选择背景图片",
			Multiple: false,
			Filters: []mygo.FileFilter{{
				Name:       "图片",
				Extensions: paths.ImageExtensions,
				MIMETypes:  []string{"image/*"},
			}},
		})
		if err != nil || len(chosen) == 0 {
			return
		}
		a.pickerMu.Lock()
		if a.pendingPicker == nil {
			a.pendingPicker = make(map[*string]string)
		}
		a.pendingPicker[&t.bgPath] = chosen[0]
		a.pickerMu.Unlock()
		a.Invalidate()
	}()
}

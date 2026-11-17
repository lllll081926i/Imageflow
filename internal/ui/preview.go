package ui

import (
	"image"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/codec"
)

func (a *App) selectedPreviewPath() string {
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	return a.previewPath
}

func (a *App) beginPreview(path string) uint64 {
	rows := a.Session.Snapshot()
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	for _, row := range rows {
		if row.Path == path {
			a.previewPath = path
			a.previewRevealPath = path
			break
		}
	}
	a.previewVersion++
	a.previewLoading = true
	a.previewFocusPending = true
	return a.previewVersion
}

func (a *App) finishPreview(version uint64, img image.Image, err error) {
	a.previewMu.Lock()
	if version != a.previewVersion {
		a.previewMu.Unlock()
		return
	}
	a.lastPreview = img
	a.previewLoading = false
	a.previewMu.Unlock()
	if err != nil {
		a.SetStatus("预览失败：" + err.Error())
	}
	a.Invalidate()
}

func (a *App) setInitialPreview(path string, img image.Image) {
	a.previewMu.Lock()
	if a.previewPath == "" && a.lastPreview == nil && !a.previewLoading {
		a.previewPath = path
		a.previewRevealPath = path
		a.lastPreview = img
		a.previewVersion++
		a.previewFocusPending = true
	}
	a.previewMu.Unlock()
}

func (a *App) selectPreview(path string) {
	found := false
	for _, row := range a.Session.Snapshot() {
		if row.Path == path {
			found = true
			break
		}
	}
	if !found {
		return
	}
	a.previewMu.Lock()
	unchanged := a.previewPath == path && (a.previewLoading || a.lastPreview != nil)
	a.previewMu.Unlock()
	if unchanged {
		return
	}
	version := a.beginPreview(path)
	win := a.Window
	a.Invalidate()
	go func() {
		// 串行解码，只处理最新请求，限制快速按键时的解码内存。
		a.previewLoadMu.Lock()
		defer a.previewLoadMu.Unlock()
		a.previewMu.Lock()
		current := version == a.previewVersion
		a.previewMu.Unlock()
		if !current {
			return
		}
		var img image.Image
		err := batch.Execute(func(_ *batch.Cancel) error {
			opened, err := codec.OpenSRGB(path)
			if err == nil {
				img = opened.Image
			}
			return err
		}, nil)
		publish := func() { a.finishPreview(version, img, err) }
		if win != nil {
			win.Update(publish)
		} else {
			publish()
		}
	}()
}

func (a *App) movePreview(delta int) {
	rows := a.Session.Snapshot()
	if len(rows) == 0 {
		return
	}
	path := a.selectedPreviewPath()
	index := 0
	for i, row := range rows {
		if row.Path == path {
			index = i
			break
		}
	}
	next := max(0, min(len(rows)-1, index+delta))
	if rows[next].Path != path {
		a.selectPreview(rows[next].Path)
	}
}

package ui

import (
	"context"
	"fmt"
	"image"
	"log"
	"sync"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/engine"
)

type studioJob[Value any] struct {
	mu     sync.Mutex
	work   sync.Mutex
	key    string
	value  Value
	err    error
	busy   bool
	closed bool
	cancel context.CancelFunc
}

func (job *studioJob[Value]) request(a *App, key string, run func(context.Context) (Value, error)) (Value, bool, error) {
	job.mu.Lock()
	defer job.mu.Unlock()
	if key == job.key || job.closed {
		return job.value, job.busy, job.err
	}
	if job.cancel != nil {
		job.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	job.key, job.busy, job.err = key, true, nil
	var empty Value
	job.value = empty
	window := a.Window
	go func() {
		job.work.Lock()
		defer job.work.Unlock()
		if ctx.Err() != nil {
			return
		}
		var value Value
		err := batch.Execute(func(_ *batch.Cancel) error {
			var err error
			value, err = run(ctx)
			return err
		}, nil)
		publish := func() {
			job.mu.Lock()
			if ctx.Err() == nil && !job.closed {
				job.value, job.err, job.busy = value, err, false
			}
			job.mu.Unlock()
			a.Invalidate()
		}
		if window != nil {
			window.Update(publish)
		} else {
			publish()
		}
	}()
	return job.value, true, nil
}

func (job *studioJob[Value]) stop() {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.cancel != nil {
		job.cancel()
	}
	job.closed, job.busy = true, false
}

func (job *studioJob[Value]) cancelRequest() {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.cancel != nil {
		job.cancel()
	}
	var empty Value
	job.value, job.busy, job.err = empty, false, context.Canceled
}

func (job *studioJob[Value]) snapshot() (Value, bool, error) {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.value, job.busy, job.err
}

func (a *App) publishStudio(publish func()) {
	if window := a.Window; window != nil {
		window.Update(publish)
	} else {
		a.pickerMu.Lock()
		a.pendingDone = append(a.pendingDone, publish)
		a.pickerMu.Unlock()
	}
}

func (a *App) requestStudioPreview(tool Tool) {
	if tool == nil {
		return
	}
	a.previewMu.Lock()
	source, version := a.lastPreview, a.previewVersion
	a.previewMu.Unlock()
	var params any
	var render func(context.Context) (image.Image, error)
	switch current := tool.(type) {
	case *CropTool:
		current.ensure()
		req := current.request()
		if longest := max(req.ScaledWidth, req.ScaledHeight); longest > 2048 {
			req.ScaledWidth = max(1, req.ScaledWidth*2048/longest)
			req.ScaledHeight = max(1, req.ScaledHeight*2048/longest)
		}
		params = req
		render = func(context.Context) (image.Image, error) {
			preview := source
			if preview != nil && max(preview.Bounds().Dx(), preview.Bounds().Dy()) > 2048 {
				preview = imaging.Fit(preview, 2048, 2048, imaging.Lanczos)
			}
			return engine.RenderCropImage(preview, req)
		}
	case *WatermarkTool:
		current.ensure()
		req := current.request()
		params = req
		render = func(context.Context) (image.Image, error) { return engine.RenderWatermarkImage(source, req) }
	case *RichTextTool:
		current.ensure()
		req := current.request()
		params = req
		render = func(context.Context) (image.Image, error) { return engine.RenderRichTextImage(req) }
	case *AnnotateTool:
		marks := append([]engine.AnnotateMark(nil), current.marks...)
		for index := range marks {
			marks[index].Points = append([]engine.AnnotatePoint(nil), marks[index].Points...)
		}
		params = marks
		render = func(context.Context) (image.Image, error) { return engine.AnnotateImage(source, marks) }
	case *EditTool:
		req := current.request()
		params = req
		render = func(context.Context) (image.Image, error) { return engine.RenderEdit(source, req) }
	case *ScreenshotTool:
		req := current.request()
		params = req
		render = func(context.Context) (image.Image, error) { return engine.RenderScreenshot(source, req) }
	case *GradeTool:
		req := current.params()
		params = req
		render = func(context.Context) (image.Image, error) { return engine.Grade(source, req), nil }
	case *FilterTool:
		req := current.request()
		params = req
		render = func(context.Context) (image.Image, error) { return engine.ApplyFilter(source, req), nil }
	case *PixelTool:
		req := current.request()
		params = req
		render = func(context.Context) (image.Image, error) {
			result, _, _, err := engine.RenderPixelArt(source, req)
			return result, err
		}
	case *EmoteTool:
		req := current.request()
		params = req
		render = func(context.Context) (image.Image, error) { return engine.RenderEmote(source, req) }
	case *CollageStudioTool:
		current.ensure()
		req := engine.CanvasRequest{Width: int(current.width), Height: int(current.height), Background: current.bgHex,
			Layers: append([]engine.CanvasLayer(nil), current.layers...)}
		params = req
		render = func(context.Context) (image.Image, error) { return engine.RenderCanvas(req) }
	}
	if render == nil || source == nil && tool.ID() != ToolStudio && tool.ID() != ToolRichText {
		return
	}
	key := fmt.Sprintf("%s/%d/%#v", tool.ID(), version, params)
	img, busy, err := a.studioPreview.request(a, key, render)
	a.previewMu.Lock()
	if busy && img == nil && a.studioSourceVersion == version && a.studioToolID == tool.ID() {
		img = a.studioOutput
	}
	a.previewMu.Unlock()
	if current, ok := tool.(*CollageStudioTool); ok {
		current.preview, current.busy = img, busy
		current.previewErr = ""
		if err != nil {
			current.previewErr = err.Error()
		}
	}
	a.previewMu.Lock()
	a.studioOutput, a.studioSourceVersion, a.studioToolID = img, version, tool.ID()
	a.studioPreviewError, a.studioPreviewBusy = "", busy
	if err != nil {
		a.studioPreviewError = err.Error()
	}
	a.previewMu.Unlock()
}

func (a *App) CancelStudioWork() {
	a.cacheJob.stop()
	a.studioPreview.stop()
	for _, tool := range a.Tools {
		switch current := tool.(type) {
		case *GradeTool:
			current.lutJob.stop()
		case *CompareTool:
			current.referencePreview.stop()
			current.resultPreview.stop()
		case *UpscaleTool:
			current.clipboardImport.stop()
			current.resultPreview.stop()
			current.flushPreferences(a)
			current.modelImport.stop()
			current.probe.stop()
			current.modelDownload.stop()
		}
	}
	if err := a.clipboardCache.Close(); err != nil {
		log.Printf("临时图片清理失败，下次启动重试：%v", err)
	}
}

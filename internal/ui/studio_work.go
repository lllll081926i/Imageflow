package ui

import (
	"context"
	"fmt"
	"image"
	"sync"

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
	if render == nil || source == nil && tool.ID() != ToolStudio {
		return
	}
	key := fmt.Sprintf("%s/%d/%#v", tool.ID(), version, params)
	img, busy, err := a.studioPreview.request(a, key, render)
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
	a.studioPreview.stop()
	for _, tool := range a.Tools {
		switch current := tool.(type) {
		case *GradeTool:
			current.lutJob.stop()
		case *CompareTool:
			current.referencePreview.stop()
			current.resultPreview.stop()
		case *AnalysisTool:
			current.samplePreview.stop()
		case *UpscaleTool:
			current.probe.stop()
			current.manifestImport.stop()
			current.modelDownload.stop()
		}
	}
}

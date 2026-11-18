package ui

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	myui "github.com/egoist/mygo/ui"
	"github.com/lllll081926i/imageflow-go/internal/upscale"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

type upscaleResult struct {
	report     upscale.Report
	sourceSize int64
	sourceMod  int64
}

type upscalePair struct {
	before        *myui.Bitmap
	after         *myui.Bitmap
	width, height int
}

type upscaleRunState struct {
	mu         sync.Mutex
	name       string
	percent    int
	lines      []string
	lastNotice time.Time
}

func (state *upscaleRunState) start(name string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.name, state.percent, state.lines = name, 0, nil
}

func (state *upscaleRunState) record(percent int, line string) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if percent >= 0 {
		state.percent = percent
	} else if line != "" {
		state.lines = append(state.lines, line)
		if len(state.lines) > 12 {
			state.lines = state.lines[len(state.lines)-12:]
		}
	}
	if time.Since(state.lastNotice) < 100*time.Millisecond {
		return false
	}
	state.lastNotice = time.Now()
	return true
}

func (state *upscaleRunState) snapshot() (string, int, []string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.name, state.percent, append([]string(nil), state.lines...)
}

func (t *UpscaleTool) PreviewPanel(c *myui.Context, a *App) {
	st := styles(c)
	path := a.selectedPreviewPath()
	if t.comparisonPath != path {
		t.comparisonPath = path
		t.draggingComparison, t.panningComparison = false, false
		t.resetComparison()
	}
	var result upscaleResult
	if value, ok := t.results.Load(path); ok {
		result = value.(upscaleResult)
	}
	myui.Column(c).Fill().MinHeight(0).Clip().Gap(8).Padding(8).Radius(st.RadiusPanel).Background(st.Recessed).Children(func() {
		if t.skipPreview {
			myui.Spacer(c).Grow(1)
			myui.Text(c, "已跳过结果预览").FontSize(12).TextColor(st.TextDim)
			a.ghostButton(c, st, "恢复预览", lucide.Image(), func() { t.skipPreview = false })
			myui.Spacer(c).Grow(1)
			return
		}
		a.previewMu.Lock()
		version := a.previewVersion
		a.previewMu.Unlock()
		key := fmt.Sprintf("%s/%d/%s/%d", path, version, result.report.OutputPath, result.report.Elapsed)
		pair, busy, err := t.resultPreview.request(a, key, func(ctx context.Context) (upscalePair, error) {
			return loadUpscalePair(ctx, path, result)
		})
		if busy {
			myui.Text(c, "正在读取预览").FontSize(11).TextColor(st.TextDim)
		}
		if err != nil {
			warnAlert(c, err.Error())
		}
		if pair.before == nil {
			myui.Spacer(c).Grow(1)
			return
		}
		if t.zoom < 100 {
			t.resetComparison()
		}
		zoom, split, centerX, centerY := t.zoom/100, t.split, t.centerX, t.centerY
		heading := "原图 · 放大后可滑动对比"
		if pair.after != nil {
			heading = fmt.Sprintf("左：原图 %d × %d  ·  右：结果 %d × %d", pair.width, pair.height, result.report.Width, result.report.Height)
		}
		myui.Text(c, heading).FontSize(11).TextColor(st.TextSub).SingleLine().Tooltip(heading)
		canvas := myui.Box(c).FillWidth().Grow(1).MinHeight(0).Clip().Background(st.Card).
			Focusable().Label("AI 对比画布").Bind(&t.comparisonHandle).
			OnShortcut(0, myui.KeyLeft, func() { a.movePreview(-1) }).
			OnShortcut(0, myui.KeyRight, func() { a.movePreview(1) })
		if pair.after != nil {
			canvas.Cursor(myui.CursorResizeEW).Tooltip("拖动对比分隔线；滚轮缩放；右键拖动平移；双击复位")
		}
		canvas.HandleInput(func(event myui.InputEvent) bool {
			bounds := t.comparisonHandle.Bounds(c)
			rect := previewFitRect(myui.Rect{W: bounds.W, H: bounds.H}, pair.width, pair.height)
			if pair.after == nil && event.Button == 0 && event.Kind == myui.InputPointerDown && event.Clicks < 2 {
				return false
			}
			return t.comparisonPointer(event, rect)
		})
		canvas.Draw(func(painter *myui.Painter, bounds myui.Rect) {
			rect := previewFitRect(bounds, pair.width, pair.height)
			region := normalizedRegion(centerX, centerY, zoom)
			draw := func(bitmap *myui.Bitmap, source myui.Rect, target myui.Rect) {
				if bitmap == nil {
					bitmap = pair.before
				}
				width, height := bitmap.Size()
				painter.ImageRegion(bitmap, myui.Rect{X: source.X * float32(width), Y: source.Y * float32(height), W: source.W * float32(width), H: source.H * float32(height)}, target, myui.ImageOptions{})
			}
			draw(pair.before, region, rect)
			if pair.after == nil {
				return
			}
			boundary := rect.X + rect.W*float32(split)
			painter.Clip(myui.Rect{X: boundary, Y: rect.Y, W: rect.W * (1 - float32(split)), H: rect.H}, 0, func() { draw(pair.after, region, rect) })
			painter.Fill(myui.Rect{X: boundary - 1, Y: rect.Y, W: 2, H: rect.H}, st.Brand, 0)
			handle := myui.Rect{X: boundary - 12, Y: rect.Y + rect.H/2 - 20, W: 24, H: 40}
			painter.Fill(handle, st.Card, 12)
			painter.Fill(myui.Rect{X: boundary - 4, Y: handle.Y + 12, W: 2, H: 16}, st.Brand, 1)
			painter.Fill(myui.Rect{X: boundary + 2, Y: handle.Y + 12, W: 2, H: 16}, st.Brand, 1)
		})
	})
}

func loadUpscalePair(ctx context.Context, path string, result upscaleResult) (upscalePair, error) {
	if ctx.Err() != nil || path == "" {
		return upscalePair{}, ctx.Err()
	}
	before, err := loadPreviewImage(path)
	if err != nil {
		return upscalePair{}, err
	}
	pair := upscalePair{width: before.Bounds().Dx(), height: before.Bounds().Dy()}
	bitmap := func(source image.Image) *myui.Bitmap {
		if source.Bounds().Dx() > 2048 || source.Bounds().Dy() > 2048 {
			source = imaging.Fit(source, 2048, 2048, imaging.Lanczos)
		}
		return myui.NewBitmap(source)
	}
	pair.before = bitmap(before)
	if result.report.OutputPath == "" || ctx.Err() != nil {
		return pair, ctx.Err()
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() != result.sourceSize || info.ModTime().UnixNano() != result.sourceMod {
		return pair, fmt.Errorf("原图已变化，请重新处理")
	}
	after, err := loadPreviewImage(result.report.OutputPath)
	if err != nil {
		return pair, err
	}
	if ctx.Err() != nil {
		return upscalePair{}, ctx.Err()
	}
	pair.after = bitmap(after)
	return pair, nil
}

func normalizedRegion(centerX, centerY, zoom float64) myui.Rect {
	size := 1 / max(1, zoom)
	return myui.Rect{X: float32(max(0, min(1-size, centerX-size/2))), Y: float32(max(0, min(1-size, centerY-size/2))), W: float32(size), H: float32(size)}
}

func (t *UpscaleTool) resetComparison() {
	t.zoom, t.split, t.centerX, t.centerY = 100, .5, .5, .5
}

func (t *UpscaleTool) comparisonPointer(event myui.InputEvent, rect myui.Rect) bool {
	if rect.W <= 0 || rect.H <= 0 {
		return false
	}
	x, y := math.Max(0, math.Min(1, float64((event.X-rect.X)/rect.W))), math.Max(0, math.Min(1, float64((event.Y-rect.Y)/rect.H)))
	switch event.Kind {
	case myui.InputScroll:
		t.zoom = max(100, min(1000, t.zoom-float64(event.DY)*2))
		return true
	case myui.InputPointerDown:
		if event.Button == 0 && event.Clicks == 2 {
			t.resetComparison()
			return true
		}
		t.draggingComparison, t.panningComparison = true, event.Button != 0
		t.pointerX, t.pointerY = x, y
		if !t.panningComparison {
			t.split = x
		}
		return true
	case myui.InputPointerMove:
		if t.draggingComparison {
			if t.panningComparison {
				t.centerX = max(0, min(1, t.centerX-(x-t.pointerX)/(t.zoom/100)))
				t.centerY = max(0, min(1, t.centerY-(y-t.pointerY)/(t.zoom/100)))
			} else {
				t.split = x
			}
			t.pointerX, t.pointerY = x, y
		}
		return t.draggingComparison
	case myui.InputPointerUp:
		t.draggingComparison, t.panningComparison = false, false
		return true
	}
	return false
}

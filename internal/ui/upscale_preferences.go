package ui

import (
	"time"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func (t *UpscaleTool) applyPreferences(value settings.Upscale) {
	value.Normalize()
	t.downloadModel, t.modelName = value.Model, value.Model
	t.scale, t.double = value.Scale, value.Double
	t.useCustomWidth, t.customWidth = value.UseCustomWidth, float64(value.CustomWidth)
	t.tile, t.tta, t.gpuIDs = float64(value.Tile), value.TTA, value.GPU
	t.format, t.compression, t.skipPreview = value.Format, float64(value.Compression), value.SkipPreview
	t.notifyComplete = value.NotifyComplete
}

func (t *UpscaleTool) preferences() settings.Upscale {
	return settings.Upscale{Model: t.downloadModel, Scale: t.scale, Double: t.double,
		UseCustomWidth: t.useCustomWidth, CustomWidth: int(t.customWidth), Tile: int(t.tile), TTA: t.tta,
		GPU: t.gpuIDs, Format: t.format, Compression: int(t.compression), SkipPreview: t.skipPreview, NotifyComplete: t.notifyComplete}
}

func (t *UpscaleTool) loadPreferences(a *App) {
	if t.preferencesLoaded {
		return
	}
	t.preferencesLoaded = true
	t.applyPreferences(a.Session.SettingsSnapshot().Upscale)
	t.lastPreferences = t.preferences()
}

func (t *UpscaleTool) persistPreferences(a *App) {
	value := t.preferences()
	t.preferencesMu.Lock()
	defer t.preferencesMu.Unlock()
	if value == t.lastPreferences {
		return
	}
	t.lastPreferences, t.pendingPreferences = value, &value
	if t.preferencesTimer != nil {
		t.preferencesTimer.Stop()
	}
	t.preferencesTimer = time.AfterFunc(600*time.Millisecond, func() { t.flushPreferences(a) })
}

func (t *UpscaleTool) flushPreferences(a *App) {
	t.preferencesMu.Lock()
	defer t.preferencesMu.Unlock()
	if t.preferencesTimer != nil {
		t.preferencesTimer.Stop()
		t.preferencesTimer = nil
	}
	if value := t.pendingPreferences; value != nil {
		t.pendingPreferences = nil
		if err := a.Session.UpdateSettings(func(next *settings.Settings) { next.Upscale = *value }); err != nil {
			a.SetStatus("保存 AI 参数失败：" + err.Error())
		}
	}
}

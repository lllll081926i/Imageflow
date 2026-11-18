package ui

import (
	"fmt"
	"image"
	"os"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/paths"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

const fileThumbnailLimit = 64

type fileThumbnail struct {
	path   string
	data   string
	bitmap *myui.Bitmap
}

type fileThumbnailCache struct {
	slots [fileThumbnailLimit]fileThumbnail
	index map[string]int
	next  int
}

func (cache *fileThumbnailCache) bitmap(item *batch.Item) *myui.Bitmap {
	if item.Thumb == "" {
		return nil
	}
	slot, found := cache.index[item.Path]
	if found && cache.slots[slot].data == item.Thumb {
		return cache.slots[slot].bitmap
	}
	bitmap, _ := myui.DecodeBitmap([]byte(item.Thumb))
	if !found {
		if cache.index == nil {
			cache.index = make(map[string]int, fileThumbnailLimit)
		}
		slot = cache.next
		cache.next = (cache.next + 1) % len(cache.slots)
		delete(cache.index, cache.slots[slot].path)
		cache.index[item.Path] = slot
	}
	cache.slots[slot] = fileThumbnail{path: item.Path, data: item.Thumb, bitmap: bitmap}
	return bitmap
}

func (a *App) visibleInputRows() []batch.Item {
	items := a.Session.Snapshot()
	tool := a.ActiveTool()
	a.mu.Lock()
	owned := tool != nil && a.rowResultTool == tool.ID()
	a.mu.Unlock()
	if !owned {
		for index := range items {
			items[index].Status, items[index].Message, items[index].Output = "", "", ""
		}
	}
	return items
}

// fileRow draws one input row: thumbnail, name, format badge and size, status.
// Hover swaps the hairline for a tinted selection so the row reads as a list
// item rather than as a floating card, and the status column stays aligned by
// giving it a fixed width.
func (a *App) fileRow(c *myui.Context, st style, item *batch.Item) {
	c.Key("input-file-" + item.Path)
	row := myui.Row(c).FillWidth().Height(62).Gap(11).Padding(7, 9, 7, 9).
		AlignItems(myui.Center).Radius(st.RadiusControl).Transition(motion(springSoft)).
		Focusable().Cursor(myui.CursorPointer).Label("预览 " + item.Name).
		OnClick(func() { a.selectBrowserPreview(item.Path) })
	if a.selectedPreviewPath() == item.Path {
		row.Background(st.BrandSoft).Border(1, st.Brand)
		a.previewMu.Lock()
		reveal := a.previewRevealPath == item.Path
		if reveal {
			a.previewRevealPath = ""
		}
		a.previewMu.Unlock()
		if reveal {
			row.ScrollIntoView()
		}
	} else if row.Hovered() {
		row.Background(st.Recessed)
	}
	row.Children(func() {
		a.thumbBox(c, st, item)
		myui.Column(c).Grow(1).Gap(3).Children(func() {
			myui.Row(c).FillWidth().Gap(6).AlignItems(myui.Center).Children(func() {
				myui.Text(c, item.Name).Grow(1).FontSize(13).
					FontWeight(500).TextColor(st.Text).SingleLine().Tooltip(item.Path)
				if item.Format != "" {
					myui.Row(c).PaddingX(5).PaddingY(1).Radius(4).
						Background(st.Recessed).Children(func() {
						myui.Text(c, item.Format).FontSize(9).FontWeight(600).
							TextColor(st.TextDim)
					})
				}
			})
			meta := fmt.Sprintf("%d × %d", item.Width, item.Height)
			if item.Size > 0 {
				meta += "  ·  " + humanSize(item.Size)
			}
			myui.Text(c, meta).FontSize(11).TextColor(st.TextDim).SingleLine()
		})
		if item.Status != "" {
			myui.Column(c).Width(84).Gap(3).AlignItems(myui.End).Children(func() {
				statusPill(c, statusLabel(item.Status), statusColor(item.Status, st))
				if item.Message != "" {
					myui.Text(c, truncate(item.Message, 14)).FontSize(10).
						TextColor(statusColor(item.Status, st)).SingleLine().Tooltip(item.Message)
				}
			})
		}
		a.fileRemoveAction(c, st, item)
	})
}

func (a *App) selectBrowserPreview(path string) {
	a.selectPreview(path)
	a.previewMu.Lock()
	a.previewFocusPending = false
	a.previewMu.Unlock()
}

func (a *App) fileBrowser(c *myui.Context, st style, items []batch.Item) {
	c.Key("file-browser")
	holder := myui.Box(c).FillWidth().Grow(1).MinHeight(0).Clip()
	grid := myui.Local(holder, "grid", func() myui.GridState { return myui.GridState{} })
	holder.Children(func() { a.fileBrowserGrid(c, st, items, grid) })
}

func (a *App) fileBrowserGrid(c *myui.Context, st style, items []batch.Item, grid *myui.GridState) {
	selected := a.selectedPreviewPath()
	a.fileSelection = -1
	for index := range items {
		if items[index].Path == selected {
			a.fileSelection = index
			break
		}
	}
	grid.Key = func(index int) any { return items[index].Path }
	grid.Label = func(index int) string { return items[index].Name }
	grid.Selected = &a.fileSelection
	grid.Reorder = nil
	if !a.IsRunning() && !a.IsImporting() {
		grid.Reorder = func(moving []int, to int) { a.reorderInputs(items, moving, to) }
	}
	a.previewMu.Lock()
	reveal := a.previewRevealPath == selected && selected != ""
	if reveal {
		a.previewRevealPath = ""
	}
	a.previewMu.Unlock()
	if reveal && a.fileSelection >= 0 {
		grid.ScrollIntoView(a.fileSelection)
	}
	width, height := float32(100000), float32(62)
	if a.fileLayout == 1 {
		width, height = 132, 136
	} else if a.fileLayout == 2 {
		width, height = 210, 64
	}
	c.Key(fmt.Sprintf("file-browser-%d", a.fileLayout))
	myui.GridView(c, grid, len(items), width, height, func(index int) {
		item := &items[index]
		if a.fileLayout == 0 {
			a.fileRow(c, st, item)
			return
		}
		c.Key("input-tile-" + item.Path)
		tile := myui.Column(c).Fill().MinWidth(0).Clip().Padding(6).Gap(4).
			Radius(st.RadiusControl).Background(st.Card).Border(1, st.Divider).
			Label("预览 " + item.Name).Tooltip(item.Path).Cursor(myui.CursorPointer).
			OnClick(func() { a.selectBrowserPreview(item.Path) })
		if selected == item.Path {
			tile.Background(st.BrandSoft).Border(1, st.Brand)
		}
		tile.Children(func() {
			if a.fileLayout == 1 {
				myui.Box(c).FillWidth().Grow(1).MinHeight(0).Clip().Children(func() {
					if bitmap := a.fileThumbs.bitmap(item); bitmap != nil {
						myui.Image(c, bitmap).Fill().Fit(myui.Contain)
					} else {
						myui.Row(c).Fill().Center().Children(func() {
							myui.Icon(c, lucide.FileImage()).Size(17, 17).TextColor(st.TextDim)
						})
					}
				})
			}
			myui.Row(c).FillWidth().MinWidth(0).Gap(4).AlignItems(myui.Center).Children(func() {
				if a.fileLayout == 2 {
					a.thumbBox(c, st, item)
				}
				myui.Column(c).Grow(1).MinWidth(0).Clip().Children(func() {
					myui.Text(c, item.Name).FillWidth().FontSize(11).SingleLine().TextColor(st.Text)
					if item.Status != "" {
						myui.Text(c, statusLabel(item.Status)).FontSize(10).TextColor(statusColor(item.Status, st)).SingleLine().Tooltip(item.Message)
					}
				})
				a.fileRemoveAction(c, st, item)
			})
		})
	}).FillWidth().Grow(1).MinHeight(0).Clip().Label("图片列表").
		OnShortcut(0, myui.KeyDelete, func() {
			if !a.IsRunning() && !a.IsImporting() {
				a.removeByPath(a.selectedPreviewPath())
			}
		}).OnChange(func() {
		if a.fileSelection >= 0 && a.fileSelection < len(items) {
			a.selectBrowserPreview(items[a.fileSelection].Path)
		}
	})
}

func (a *App) thumbBox(c *myui.Context, st style, item *batch.Item) {
	box := myui.Row(c).Size(48, 48).Radius(st.RadiusPill).AlignItems(myui.Center).
		Justify(myui.Center).Background(st.Recessed).Clip()
	box.Children(func() {
		if bitmap := a.fileThumbs.bitmap(item); bitmap != nil {
			myui.Image(c, bitmap).Size(48, 48).Fit(myui.Contain)
			return
		}
		myui.Icon(c, lucide.FileImage()).Width(17).Height(17).TextColor(st.TextDim)
	})
}

func (a *App) fileRemoveAction(c *myui.Context, st style, item *batch.Item) {
	myui.ButtonBase(c).Size(28, 28).Label("移除 " + item.Name).
		Tooltip("从列表移除，不删除文件").Disabled(a.IsRunning() || a.IsImporting()).
		AlignItems(myui.Center).Justify(myui.Center).
		OnClick(func() { a.removeByPath(item.Path) }).Children(func() {
		myui.Icon(c, lucide.X()).Size(14, 14).TextColor(st.TextDim)
	})
}

func (a *App) reorderInputs(items []batch.Item, moving []int, to int) {
	if a.IsRunning() || a.IsImporting() || to < 0 || to > len(items) || len(moving) == 0 {
		return
	}
	chosen := make(map[int]bool, len(moving))
	for _, index := range moving {
		if index < 0 || index >= len(items) {
			return
		}
		chosen[index] = true
	}
	order := make([]string, 0, len(items))
	for index := 0; index <= len(items); index++ {
		if index == to {
			for source, item := range items {
				if chosen[source] {
					order = append(order, item.Path)
				}
			}
		}
		if index < len(items) && !chosen[index] {
			order = append(order, items[index].Path)
		}
	}
	if a.Session.ReorderFiles(order) {
		a.SetStatus("图片顺序已更新")
		a.Invalidate()
	}
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}

func statusLabel(status string) string {
	switch status {
	case "ok":
		return "已完成"
	case "failed":
		return "失败"
	case "skipped":
		return "已跳过"
	case "cancelled":
		return "已取消"
	}
	return ""
}

func statusColor(status string, st style) myui.Color {
	switch status {
	case "ok":
		return st.Success
	case "failed":
		return st.Danger
	case "cancelled":
		return st.Warning
	}
	return st.TextDim
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func (a *App) removeByPath(path string) {
	removed := a.Session.RemovePath(path)
	if removed >= 0 && a.selectedPreviewPath() == path {
		rows := a.Session.Snapshot()
		if len(rows) == 0 {
			a.SetPreview(nil)
		} else {
			a.selectPreview(rows[min(removed, len(rows)-1)].Path)
		}
	}
	a.Invalidate()
}

func (a *App) clearInputs() {
	if a.IsRunning() {
		return
	}
	a.CancelImport()
	a.Session.Clear()
	a.fileThumbs = fileThumbnailCache{}
	a.SetPreview(nil)
	a.SetStatus("已清空")
	a.Invalidate()
}

// AddPaths expands dropped or picked paths into the session.
func (a *App) AddPaths(raw []string) int {
	if a.cacheCleaning.Load() {
		a.SetStatus("正在清理缓存，请稍后添加图片")
		return 0
	}
	if a.Window != nil {
		a.ImportPaths(raw)
		return 0
	}
	expanded := paths.Expand(raw)
	for _, dir := range expanded.InputDirs {
		a.rememberInputDir(dir)
	}
	if len(expanded.Files) == 0 {
		a.SetStatus("没有找到可处理的图片")
		a.Invalidate()
		return 0
	}
	added := a.Session.Add(expanded.Files)
	// The rows are read through a snapshot: a drop or a dialog callback calls this
	// from a worker goroutine while the view thread is snapshotting the same
	// session, and ranging over Session.Items directly raced with both.
	rows := a.Session.Snapshot()
	a.loadMetaRows(rows)
	if added > 0 {
		a.SetStatus(fmt.Sprintf("已添加 %d 张", added))
	} else {
		a.SetStatus("这些文件已经在列表里")
	}
	if len(rows) > 0 && a.Preview() == nil {
		a.loadPreview(rows[0].Path)
	}
	a.Invalidate()
	return added
}

// loadMetaRows probes the rows that still have no thumbnail. It takes a snapshot
// index map, because Add may have appended rows since the caller started.
func (a *App) loadMetaRows(rows []batch.Item) {
	for i := range rows {
		if rows[i].Thumb != "" && rows[i].Width > 0 {
			continue
		}
		a.loadMeta(i, rows[i].Path)
	}
}

// rememberInputDir pushes a dropped folder into the recent list the settings
// page summarises, capped and de-duplicated by the settings package. Only the
// shared Session copy is written here: a drop can arrive on a window callback,
// and the editable settings belong to the UI thread.
func (a *App) rememberInputDir(dir string) {
	if dir == "" {
		return
	}
	if err := a.Session.UpdateSettings(func(s *settings.Settings) { s.RecentInputDirs = settings.PushRecent(s.RecentInputDirs, dir) }); err != nil {
		a.SetStatus("保存输入历史失败：" + err.Error())
	}
}

// loadPreview decodes one file so the preview box has something to show before
// the first run.
func (a *App) loadPreview(path string) {
	version := a.beginPreview(path)
	opened, err := codec.OpenSRGB(path)
	var img image.Image
	if err == nil {
		img = opened.Image
	}
	a.finishPreview(version, img, err)
}

// loadMeta probes one file for its dimensions, format and thumbnail.
func (a *App) loadMeta(index int, path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	width, height, format, err := codec.Probe(path)
	if err != nil {
		return
	}
	thumb := ""
	if info, terr := a.Thumbs.Get(path, info.ModTime().UnixNano(), info.Size()); terr == nil {
		thumb = string(info.Data)
	}
	a.Session.SetMeta(index, width, height, format, thumb)
}

func (a *App) pickFiles() {
	if a.Window == nil {
		return
	}
	go func() {
		chosen, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.Window,
			Title:    "选择图片",
			Multiple: true,
			Filters: []mygo.FileFilter{{
				Name:       "图片",
				Extensions: paths.ImageExtensions,
				MIMETypes:  []string{"image/*"},
			}},
		})
		if err != nil || len(chosen) == 0 {
			return
		}
		a.AddPaths(chosen)
	}()
}

func (a *App) pickDirectory() {
	if a.Window == nil {
		return
	}
	go func() {
		chosen, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.Window,
			Title:     "选择文件夹",
			Directory: true,
		})
		if err != nil || len(chosen) == 0 {
			return
		}
		a.AddPaths(chosen)
	}()
}

// pickOutputDirectory lets the user override the default output folder.
func (a *App) pickOutputDirectory() {
	if a.Window == nil {
		return
	}
	go func() {
		chosen, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.Window,
			Title:     "选择输出文件夹",
			Directory: true,
		})
		if err != nil || len(chosen) == 0 {
			return
		}
		// Session.Settings is written under its lock by SetSettings; a plain
		// struct copy here raced with it. SettingsSnapshot is the locked copy.
		if serr := a.Session.UpdateSettings(func(next *settings.Settings) {
			next.DefaultOutputDir = chosen[0]
			next.RecentOutputDirs = settings.PushRecent(next.RecentOutputDirs, chosen[0])
		}); serr != nil {
			a.SetStatus("保存输出目录失败：" + serr.Error())
			a.Invalidate()
			return
		}
		// Session.Settings is the shared copy; the editable one follows on the next
		// frame through applyPendingSettings, so the view is not written from here.
		a.SetStatus("输出目录已更新")
		a.Invalidate()
	}()
}

// Start runs the active tool over every row.
func (a *App) Start() {
	if a.cacheCleaning.Load() {
		a.SetStatus("正在清理缓存，请稍后开始处理")
		return
	}
	if a.IsImporting() {
		a.SetStatus("正在导入图片，请等待导入完成")
		return
	}
	tool := a.ActiveTool()
	if tool == nil {
		return
	}
	if a.Session.Len() == 0 && !toolRunsWithoutList(tool) {
		a.SetStatus("先添加图片")
		return
	}
	// Claim the run atomically. Start is public and can also be reached from a
	// second-instance callback or a test; a second start would replace the
	// session's cancel token and let an older batch's completion overwrite the
	// new one's status and Running flag.
	a.mu.Lock()
	if a.Running {
		a.mu.Unlock()
		return
	}
	a.Running = true
	delete(a.canvasOutputs, tool.ID())
	a.mu.Unlock()
	a.SetProgress(0, "准备中")
	tool.Run(a)
}

func toolRunsWithoutList(tool Tool) bool {
	return tool != nil && (tool.ID() == ToolCompare || tool.ID() == ToolAnalysis || tool.ID() == ToolStudio || tool.ID() == ToolRichText)
}

// Cancel asks the running batch to stop taking new items.
func (a *App) Cancel() {
	a.CancelImport()
	a.Session.Mu.Lock()
	cancel := a.Session.Cancel
	a.Session.Mu.Unlock()
	if cancel != nil {
		cancel.Cancel()
		a.SetStatus("正在取消…")
	}
}

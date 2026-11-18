// Package ui is the Lobe MyGo native interface of ImageFlow.
//
// settings.go keeps the original Lobe grouping in a scrolling, responsive form:
// two independent columns on wide windows, one column on narrow windows, with
// the save action pinned at the bottom.
package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/paths"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// settingsSection is one grouped block. The icon plate and title stay on their
// own row, so controls scan as a form rather than as an undifferentiated list.
func settingsSection(c *myui.Context, st style, icon *myui.SVG, title string, body func()) {
	card := myui.Column(c).FillWidth().Gap(14).Padding(16).
		Radius(st.RadiusPanel).Border(1, st.Divider).Background(st.Card).
		Label(title + "分组")
	card.Children(func() {
		myui.Row(c).FillWidth().AlignItems(myui.Center).Gap(10).Children(func() {
			myui.Row(c).Size(34, 34).Radius(st.RadiusControl).
				Justify(myui.Center).AlignItems(myui.Center).
				Background(st.BrandSoft).Children(func() {
				myui.Icon(c, icon).Width(17).Height(17).TextColor(st.Brand)
			})
			myui.Text(c, title).FontSize(14).FontWeight(600).TextColor(st.Text)
		})
		body()
	})
}

// settingsPage is the whole settings view: a header, the sections, and the
// action bar. It fills the same region the tool pages do.
func (a *App) settingsPage(c *myui.Context, st style) {
	s := &a.Settings
	width, _ := c.Size()
	wide := width >= 1200
	compact := width > 0 && width < 1080
	pagePadding := float32(18)
	if compact {
		pagePadding = 12
	}

	card := myui.Column(c).Grow(1).FillHeight().Radius(st.RadiusCard).
		Background(st.Card).Border(1, st.Divider).
		Shadow(0, 2, 12, 0, myui.RGBA(0, 0, 0, 0.04)).Clip().Label("设置卡")
	card.Children(func() {
		myui.Row(c).FillWidth().Height(68).PaddingX(pagePadding).Gap(12).
			AlignItems(myui.Center).Children(func() {
			myui.Row(c).Size(38, 38).Radius(st.RadiusControl).
				Justify(myui.Center).AlignItems(myui.Center).
				Background(st.BrandSoft).Children(func() {
				myui.Icon(c, lucide.Settings()).Width(19).Height(19).
					TextColor(st.Brand)
			})
			myui.Column(c).Gap(2).Children(func() {
				myui.Text(c, "全局设置").FontSize(16).FontWeight(600).TextColor(st.Text)
			})
			myui.Spacer(c).Grow(1)
		})
		myui.Row(c).FillWidth().Height(1).Background(st.Divider)

		myui.Scroll(c).FillWidth().Grow(1).Children(func() {
			content := myui.Column(c).FillWidth().Padding(pagePadding).Gap(14)
			content.Children(func() {
				if wide {
					// Independent columns keep unrelated groups from forcing one another
					// downwards; the reading order remains output first, app second.
					myui.Grid(c).FillWidth().Columns(2).Gap(14).Children(func() {
						myui.Column(c).FillWidth().Gap(14).Label("输出设置").Children(func() {
							a.settingsNaming(c, st, s)
							a.settingsOutput(c, st, s)
							a.cacheSection(c, st)
							a.aboutSection(c, st)
						})
						myui.Column(c).FillWidth().Gap(14).Label("应用设置").Children(func() {
							a.settingsBehaviour(c, st, s)
							a.modelDownloadSettings(c, st, s)
							a.updatesSection(c, st)
						})
					})
					return
				}
				myui.Column(c).FillWidth().Gap(12).Label("设置单列").Children(func() {
					a.settingsNaming(c, st, s)
					a.settingsOutput(c, st, s)
					a.cacheSection(c, st)
					a.aboutSection(c, st)
					a.settingsBehaviour(c, st, s)
					a.modelDownloadSettings(c, st, s)
					a.updatesSection(c, st)
				})
			})
		})
		a.settingsSaveBar(c, st, compact)
	})
}

func (a *App) settingsNaming(c *myui.Context, st style, s *settings.Settings) {
	settingsSection(c, st, lucide.FileText(), "输出命名", func() {
		grid2(c, func() {
			myui.Column(c).Gap(5).Children(func() {
				label(c, "文件名前缀")
				textField(c, &s.OutputPrefix, "IF")
			})
			myui.Column(c).Gap(5).Children(func() {
				label(c, "命名模板")
				textField(c, &s.OutputTemplate, "{prefix}{basename}")
			})
		})
	})
}
func (a *App) settingsOutput(c *myui.Context, st style, s *settings.Settings) {
	settingsSection(c, st, lucide.FolderOpen(), "输出位置", func() {
		myui.Row(c).FillWidth().Gap(8).AlignItems(myui.Center).Children(func() {
			value := s.DefaultOutputDir
			if value == "" {
				value = "与源文件同目录"
			}
			myui.Row(c).Grow(1).Height(38).PaddingX(12).Radius(st.RadiusControl).
				AlignItems(myui.Center).Border(1, st.Divider).Background(st.Recessed).
				Children(func() {
					myui.Text(c, value).FontSize(12.5).TextColor(st.TextSub).SingleLine()
				})
			a.ghostButton(c, st, "浏览", lucide.FolderOpen(), a.pickOutputDirectory)
			if s.DefaultOutputDir != "" {
				a.ghostButton(c, st, "还原", lucide.RotateCcw(), func() {
					s.DefaultOutputDir = ""
					a.persistSettings()
				})
			}
		})

		switchRow(c, st, "保留源文件夹结构", &s.PreserveFolderStructure)

		myui.Column(c).Gap(6).Children(func() {
			label(c, "同名文件")
			strategies := []string{"rename", "overwrite", "skip"}
			selected := indexOf(strategies, string(s.ConflictStrategy))
			segmented(c, &selected, []string{"自动重命名", "直接覆盖", "跳过"})
			if selected >= 0 && selected < len(strategies) {
				s.ConflictStrategy = settings.ConflictStrategy(strategies[selected])
			}
		})
	})
}

func (a *App) settingsBehaviour(c *myui.Context, st style, s *settings.Settings) {
	settingsSection(c, st, lucide.SlidersHorizontal(), "并发与行为", func() {
		concurrency := float64(s.MaxConcurrency)
		sliderRow(c, "并发任务数", &concurrency, 1, 32, 1, "%.0f 个")
		s.MaxConcurrency = int(concurrency)

		myui.Row(c).FillWidth().Height(1).Background(st.Divider).MarginY(2)
		switchRow(c, st, "完成后打开输出目录", &s.OpenOutputOnComplete)
		switchRow(c, st, "启动时检查更新", &s.AutoCheckUpdates)
		switchRow(c, st, "记住窗口位置与大小", &s.RememberWindowState)
		switchRow(c, st, "后台运行", &s.BackgroundRunning)
	})
}

func (a *App) updatesSection(c *myui.Context, st style) {
	state := a.UpdateState()
	settingsSection(c, st, lucide.Download(), "更新", func() {
		switch {
		case state.Busy || state.Installing:
			text := "正在检查更新…"
			if state.Installing || state.Version != "" {
				text = "正在下载并安装更新…"
			}
			myui.Row(c).FillWidth().MinHeight(34).Gap(8).AlignItems(myui.Center).
				Role(myui.RoleStatus).Children(func() {
				loadingIcon(c, 14, st.Brand)
				myui.Text(c, text).FontSize(12).TextColor(st.TextSub)
			})
		case state.Err != "":
			tint := st.TextSub
			icon := lucide.Info()
			if state.Failed {
				tint = st.Danger
				icon = lucide.CircleX()
			}
			myui.Column(c).FillWidth().Gap(10).Children(func() {
				stateAlert(c, state.Err, tint, icon)
				a.ghostButton(c, st, "重新检查", lucide.RefreshCw(), func() {
					go a.CheckUpdates(true)
				}).AlignSelf(myui.End)
			})
		case state.Version != "":
			myui.Row(c).FillWidth().Gap(10).AlignItems(myui.Center).Children(func() {
				myui.Column(c).Grow(1).Gap(5).Children(func() {
					myui.Row(c).Gap(6).AlignItems(myui.Center).Children(func() {
						statusPill(c, "可更新", st.Brand)
						myui.Text(c, "v"+state.Version).FontSize(12).
							FontWeight(600).TextColor(st.Text)
					})
					if state.Notes != "" {
						myui.Text(c, truncate(strings.SplitN(state.Notes, "\n", 2)[0], 40)).
							FontSize(11).TextColor(st.TextDim).SingleLine()
					}
				})
				a.ghostButton(c, st, "立即更新", lucide.Download(), a.InstallUpdate)
			})
		default:
			text := "尚未检查更新"
			tint := st.TextDim
			icon := lucide.Info()
			if state.Checked {
				text = "已是最新版本"
				tint = st.Success
				icon = lucide.CircleCheck()
			}
			myui.Row(c).FillWidth().Gap(10).AlignItems(myui.Center).Children(func() {
				myui.Row(c).Grow(1).Gap(7).AlignItems(myui.Center).Children(func() {
					myui.Icon(c, icon).Width(14).Height(14).TextColor(tint)
					myui.Text(c, text).FontSize(12).TextColor(st.TextSub)
				})
				a.ghostButton(c, st, "检查更新", lucide.RefreshCw(), func() {
					go a.CheckUpdates(true)
				})
			})
		}
	})
}

func (a *App) aboutSection(c *myui.Context, st style) {
	settingsSection(c, st, lucide.CircleHelp(), "关于", func() {
		// The whole block is the action: a version line that happens to be
		// clickable is not discoverable, so the row shows the repo in words and
		// carries the pointer cursor and focus ring of a button.
		row := myui.ButtonBase(c).FillWidth().Padding(10).Radius(st.RadiusControl).
			Gap(12).AlignItems(myui.Center).Label("打开项目主页").
			FocusRingColor(st.Brand).FocusRingOffset(2).
			Transition(motion(springTap))
		if row.Hovered() {
			row.Background(st.Recessed)
		}
		row.Children(func() {
			if mark := iconImage(logoAboutSize); mark != nil {
				myui.Image(c, myui.NewBitmap(mark)).Size(logoAboutSize, logoAboutSize).Fit(myui.Contain)
			}
			myui.Column(c).Grow(1).Gap(3).Children(func() {
				myui.Text(c, appVersionLabel()).FontSize(13).
					FontWeight(600).TextColor(st.Text)
				myui.Row(c).Gap(5).AlignItems(myui.Center).Children(func() {
					myui.Icon(c, lucide.GitFork()).Width(13).Height(13).
						TextColor(st.TextDim)
					myui.Text(c, projectRepoLabel).FontSize(11.5).
						TextColor(st.TextDim).SingleLine()
				})
			})
			myui.Icon(c, lucide.ExternalLink()).Width(14).Height(14).
				TextColor(st.TextDim)
		})
		if row.Clicked() {
			a.openProjectHome()
		}
	})
}

// openProjectHome opens the project's GitHub repository in the default browser.
func (a *App) openProjectHome() {
	if err := mygo.Shell.OpenExternal(projectRepoURL); err != nil {
		a.SetStatus("无法打开浏览器：" + err.Error())
		a.Invalidate()
	}
}

// switchRow is a labelled switch. The label says what the toggle does; a second
// line explaining it again was noise on a page that already has section headings.
func switchRow(c *myui.Context, st style, title string, on *bool) {
	myui.Row(c).FillWidth().MinHeight(36).AlignItems(myui.Center).Gap(12).Children(func() {
		myui.Text(c, title).FontSize(12.5).TextColor(st.Text)
		myui.Spacer(c).Grow(1)
		myui.Switch(c, on).Label(title).FocusRingColor(st.Brand).FocusRingOffset(2)
	})
}

// settingsSaveBar keeps feedback and actions in a stable pinned area. Narrow
// windows use two rows instead of squeezing the message between two buttons.
func (a *App) settingsSaveBar(c *myui.Context, st style, compact bool) {
	saved := a.SettingsSavedFlash()
	myui.Row(c).FillWidth().Height(1).Background(st.Divider)
	bar := myui.Column(c).FillWidth().Gap(10).Padding(12, 18, 14, 18)
	bar.Children(func() {
		feedback := func() {
			if saved == "" {
				return
			}
			tint := st.Success
			icon := lucide.CircleCheck()
			if strings.Contains(saved, "失败") {
				tint = st.Danger
				icon = lucide.CircleX()
			}
			myui.Row(c).FillWidth().MinHeight(24).Gap(7).AlignItems(myui.Center).
				Role(myui.RoleStatus).Label(saved).Children(func() {
				myui.Icon(c, icon).Width(14).Height(14).TextColor(tint)
				myui.Text(c, saved).FontSize(11.5).TextColor(tint).SingleLine()
			})
		}
		actions := func() {
			row := myui.Row(c).Gap(10).AlignItems(myui.Center)
			if compact {
				row.FillWidth()
			}
			row.Children(func() {
				restore := a.ghostButton(c, st, "还原默认值", lucide.RotateCcw(), func() {
					a.Settings = settings.Default()
					a.persistSettings()
				})
				save := primaryButton(c, st, "保存设置", lucide.Check(), false, false, a.persistSettings)
				if compact {
					restore.Grow(1)
					save.Grow(1)
				}
			})
		}
		if compact {
			feedback()
			actions()
			return
		}
		myui.Row(c).FillWidth().Gap(14).AlignItems(myui.Center).Children(func() {
			myui.Column(c).Grow(1).Children(feedback)
			actions()
		})
	})
}

// appVersionLabel is the About line's title. The packaged build carries its own
// version (mygo.json), and a plain `go build` reports the Go package's, so the
// label never claims a number the binary was not built with.
func appVersionLabel() string {
	v := mygo.App.Version()
	if v == "" {
		return "ImageFlow"
	}
	return "ImageFlow " + v
}

func (a *App) persistSettings() {
	next := a.Settings
	if err := a.Session.MergeSettings(a.settingsBase, next); err != nil {
		a.SetStatus("保存失败：" + err.Error())
		a.flashSettingsSaved("保存失败，请重试")
	} else {
		a.Settings = a.Session.SettingsSnapshot()
		a.settingsBase = a.Settings
		a.SetStatus("设置已保存")
		a.flashSettingsSaved("设置已保存")
	}
	a.Invalidate()
}

func outputDirFor(s settings.Settings, item batch.Item, suffix string) string {
	base := strings.TrimSuffix(filepath.Base(item.Path), filepath.Ext(item.Path))
	target := filepath.Join(batch.OutputDirFor(s, item.Path, item.RelPath), paths.SafeName(base+suffix))
	root := s.DefaultOutputDir
	if root == "" {
		root = batch.OutputDirFor(s, item.Path, "")
	}
	if err := paths.CheckContained(root, target); err != nil {
		return ""
	}
	return target
}

func baseNameOf(inputPath string) string {
	return strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
}

func itemPathExt(path string) string {
	return filepath.Ext(path)
}

func openPath(path string) {
	if path == "" {
		return
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		mygo.Shell.ShowItemInFolder(path)
		return
	}
	mygo.Shell.OpenPath(path)
}

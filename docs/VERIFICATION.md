# 桌面能力验证记录（2026-11-09）

## 3.2.1 发布验收（2026-11-17）

本地全量测试、Go/MyGo 静态检查、编译、漏洞扫描和真实 GPU 发布验收已通过。云端六平台构建、Windows 安装生命周期验收及正式发布全部通过，运行编号为 `44069964063`。39 个发布文件大小和摘要、38 项 SHA256SUMS、24 个完整更新包及增量包签名均通过核验。详细证据见 `RELEASE-3.2.1.md`。此前的 3.2.0 结果保留如下，不作为 3.2.1 云端通过证据。

## 3.2.0 发布验收（2026-11-17）

以下历史快照保留原验证范围；最新结果以 `RELEASE-ACCEPTANCE-2026-11-17.md` 为准。全量测试、全量竞态检查、Go/MyGo 静态检查、六平台 CGO=0 编译通过；Go 1.27.4 下漏洞扫描未发现漏洞。真实双 GPU 小图、大图、透明输入、自然照片及活动推理取消均通过。双源模型下载经代理实测通过。UI 自动交互与 100 张空输入布局截图通过，Windows 可见原生窗口另验真实导入。

云端强制执行 Windows 全新安装、3.1.0 升级、模型保留、卸载、便携移动验收；六平台打包和签名成功后才允许标签发布。Linux/macOS/Windows ARM64 尚未人工原生运行；未完成显存极限、长稳与完整画质/分发法务评测。

云端预验收运行 `44028205074`（源码 `a187037`）已全部通过，包含上述 Windows 安装生命周期检查与六平台打包。分支流程未发布版本。

本文件记录 Go 重写版的**真实进程级验证**，不是单元测试结论。
每条都给出执行命令、实际输出与判定，便于复核。

> **历史快照**：本文记录的是当时的 `0.1.0` 打包产物与 169 个测试的验收输出。
> 当前版本为 `2.3.0`（见 `mygo.json` 与 `build/` 产物），测试数量与几何常量
> 已变化。以下数字是原始记录，不是当前值；当前口径以 `README.md`、
> `docs/DESIGN.md` 与源码为准。

环境：Windows 11、Go 1.27.3、MyGo v0.8.4。

---

## 1. 单元 / 集成 / 竞态

```
$ gofmt -l .                       → 无输出
$ go vet ./...                     → 无输出
$ staticcheck ./...                → 无输出
$ go test ./... -count=1 -race     → 全部 ok
$ go test ./... -v | grep -c PASS  → 169
$ go test ./... -v | grep -c FAIL  → 0
```

依赖漏洞扫描：

```
$ govulncheck ./...
No vulnerabilities found.

Your code is affected by 0 vulnerabilities.
This scan also found 0 vulnerabilities in packages you import and 3
vulnerabilities in modules you require, but your code doesn't appear to call
these vulnerabilities.
```

三个未调用到的模块漏洞：`golang.org/x/net` 的 GO-2026-6866 / GO-2026-6860（来自 MyGo 的 publicsuffix 依赖），
`golang.org/x/crypto` 的 GO-2026-5932（来自 pdfcpu 的 OCSP 依赖）。本应用不调用对应 API，
升级到修复版本后会在下一轮依赖更新时回填。

覆盖率：

```
engine 80.9%  ui 75.4%  codec 67.5%  batch 46.1%  paths 87.4%  output 78.0%  settings 72.8%  svg 58.9%  thumbs 71.7%
```

其他门禁：

```
$ go tool mygo vet .                → 无输出
$ CGO_ENABLED=0 go test ./...       → 全部 ok（无 libc 依赖）
```

---

## 2. 真实进程：窗口、单实例、托盘驻留

### 2.1 窗口起来

```
$ start ImageFlow-dev.exe
$ Get-Process | MainWindowTitle
alive=True  title='ImageFlow'  hwnd=10488700
```

判定：原生窗口创建成功，标题正确。

### 2.2 单实例

```
$ start ImageFlow-dev.exe   # 第二个实例
second exited=True          # 3 秒内自行退出
```

判定：`App.RequestSingleInstanceLock` 生效，第二个实例把控制权交回第一个后退出。

### 2.3 后台运行（`background_running=true`）

```
$ PostMessage(hwnd, WM_CLOSE)          # 点关闭
after WM_CLOSE: alive=True             # 进程仍在
window visible=False                   # 窗口已隐藏 → 驻留托盘
```

### 2.4 从托盘恢复

```
$ start ImageFlow-dev.exe   # 再次启动（用户点托盘「显示主窗口」等价路径）
second exited=True
main window visible now=True
```

判定：`OnSecondInstance` 把隐藏的窗口重新显示并聚焦。

### 2.5 关闭即退出（`background_running=false`，默认）

```
$ PostMessage(hwnd, WM_CLOSE)
after WM_CLOSE: alive=False
```

判定：默认设置在关闭窗口时退出进程，行为符合开关语义；
`TestCloseHidesOnlyWhenBackgroundRunning` 覆盖同一判断。

---

## 3. 打包产物

```
$ mygo build -skip-installer
✓ Built ImageFlow 0.1.0
build/windows-amd64/ImageFlow.exe   27.5 MB

$ mygo build -platform windows/amd64,windows/arm64,linux/amd64,linux/arm64,darwin/arm64,darwin/amd64 -skip-installer -skip-dmg
darwin-amd64   5 files    darwin-arm64  5 files
linux-amd64   11 files    linux-arm64  11 files
windows-amd64  3 files    windows-arm64 3 files
```

Linux 产物含 `imageflow`、`imageflow.png`、`imageflow.desktop`、
`imageflow_0.1.0_amd64.deb`、`imageflow-0.1.0-linux-amd64.tar.gz`、
256/512 图标、`install.sh`、`PKGBUILD`。

### 3.0 品牌图标链路

```
$ go run ./cmd/imageflow-icon
wrote resources\icon.png (1024), resources\icon.ico, resources\favicon.ico, and 16 PNG sizes into resources\icons
```

判定：`internal/brand` 嵌入的唯一源图 `assets/logo.png` 裁切后渲染出 16–1024 全部尺寸；
`resources/icon.png`、`icon.ico`（7 个尺寸）、`favicon.ico`（3 个尺寸）与 `resources/icons/`
这 16 个 PNG 全部由 `brand.MarkAt` 得到，`mygo build` 会把它们拷到产物旁。
回归测试：`internal/brand/brand_test.go`、`internal/ui/logo_test.go`。

### 3.1 打包产物截图（无人值守）

```
$ build/windows-amd64/ImageFlow.exe -open <fixtures> -tool 5 -capture out.png
captured 158151 bytes
```

判定：打包后的 exe 能加载真实文件、切到指定工具页、渲染完整界面并截出 PNG。
截图与开发版同一帧布局（字体选择器、预览区文案位置一致）。

---

## 4. 引擎端到端（命令行，与 UI 同一批代码）

真实素材：`photo.jpg`（640×480）、`shot.png`、`anim.webp`、`anim.gif`（200×200，5 帧）。

| 工具 | 结果 |
| --- | --- |
| convert → png | 4 ok，输出 200×200 / 640×480 ×3 |
| convert → webp q85 | 4 ok，photo 7860 B |
| resize 长边 640 | 4 ok |
| grid 3×3 | 4 ok，每张 9 块 |
| stitch 竖拼 | 1 ok，640×2080 |
| watermark | 4 ok，产物目视确认文字与位置正确 |
| palette | 4 ok，11 色带 HEX，主题色 `#205AC3` |
| pdf 2×2 A4 | 1 ok，151 KB，含 4 图 |
| richtext | 4 ok，800×390，中文换行与列表符号正确 |
| gif probe | 1 ok，5 帧；3 张静图 + 单帧 GIF 均报「不是动图」 |
| gif reverse | 1 ok，2 帧动画重写成功 |
| gif export_frames 0–2 | 1 ok，只写前两帧，文件名保持原序号 |
| 富文本边距 | 首行 gap 清零 + 去掉重复 padding，上边距 89→78 px |

---

## 5. 界面视觉回归

`internal/ui/testdata/` 由 `go test ./internal/ui/` 自动生成：

- `tool-00.png … tool-11.png`：12 个工具页
- `settings.png`：设置页
- `dark-tool-00.png`、`dark-settings.png`：深色主题

守护测试：

| 测试 | 断言 |
| --- | --- |
| `TestShellGeometryAtDefaultSize` | 1440×900 下工具卡宽 ≈216（早期为 232）、参数卡宽 420、两卡不重叠、操作栏在参数卡下方、不压标题栏 |
| `TestShellGeometryAtMinimumSize` | 960×620 下仍能布局，参数卡不溢出 |
| `TestEveryToolPageRenders` | 每页都渲染出标题且截图像素足够 |
| `TestDarkToolPageHasNoWhiteBlocks` | 逐个工具：深色帧纯白像素占比 ≤3%（防「未着色的白色表面」回归） |
| `TestGhostButtonUsesSurfaceInDarkMode` | 次要按钮在深色下不是白块 |
| `TestDarkModeRenders` | 深色帧平均亮度 <120 |
| `TestSegmentedPillContrast` | 分段药丸明暗两套主题下都是可读的表面色 |
| `TestConvertToolKeepsAspectByDefault` | 锁定比例开关真生效：锁定时 120×90 → 60×45，解锁时 → 60×60 |
| `TestToolDefaultsMatchOriginalPanels` | GIF 保持比例、色板标 HEX、PDF 默认纵向 |
| `TestPreserveFolderStructureOutput` | 保留目录结构开关把 `trip/day2/photo.png` 输出到 `out/day2/` |
| `TestPartialGIFFramesKeepPreviousContent` | 只存变化子矩形的第二帧仍能看到第一帧的红底 |
| `TestGIFFrameRangeExportsSubset` | 只导出 1–3 帧时恰好 2 个文件且保留原序号 |
| `TestPaletteSwatchCountHonoursRequest` | 请求 4/6/16 色时色块数量真的变化 |
| `TestCancelledRunNeverReportsMoreThanTotal` | 取消时 Completed 不得超过 Total |
| `TestRichTextRejectsOversizedCanvas` | 超长文档返回尺寸超限错误而不是分配 |
| `TestProbeLabelsStillPNGAsPNG` | 静图 PNG 的格式徒标是 PNG 而不是 apng |
| `TestRoundCornersKeepTheLongEdges` | 宽画布圆角裁剪只抹四个角，顶/底/左边中点保持不透明 |
| `TestRoundCornersFillWithBackground` | 指定圆角外颜色时角上是该颜色而不是透明 |
| `TestGIFForeverLoopCanBeRequested` | 循环次数 0（无限）能真正写入，未指定时保留源值 |
| `TestPDFKeepsBothMetadataFields` | 同时设标题与作者时两个都写进 PDF Info 字典 |
| `TestPaletteBucketsDoNotCollide` | 两色图的直方图不再因键位重叠塌缩成一色 |
| `TestZeroValuesUseDocumentedDefaults` | 富文本宽 800 / 色板高 96 的零值默认值真的生效 |
| `TestSVGSourceGetsOneExtension` | SVG 输入输出不会得到 `.svg.png` 双后缀 |
| `TestICOEntriesAreSquare` | 非方形源生成的 ICO 条目仍为正方形 |
| `TestZeroMarginIsHonoured` | 水印边距 0 真是贴边而不是被替换成半个字号 |
| `TestZeroBorderIsHonoured` | 边框宽度 0 真的不画边框 |
| `TestSkipStrategyLeavesTheExistingFileAlone` | 同名文件 = 跳过时不写盘，行状态为已跳过 |
| `TestSkipStrategyOnlyAppliesToOccupiedTargets` | 跳过策略不会跳过空路径 |
| `TestBatchReportsSkipsSeparately` | 跳过计入 Skipped，不计入 Failed |
| `TestColorFieldRemembersItsText` | 色块十六进制输入框每帧保住当前值且支持半途输入 |

---

## 6. 仍然存在的限制（未做）

| 项 | 现状 |
| --- | --- |
| 图片压缩 | 按要求排除，未实现引擎与页面 |
| `ui_transparency` | 字段保留但不生效：新界面不用半透明材质，否则卡片边界会糊掉 |
| 缩略图 JPEG 降采样 | 首次加载解码整图再缩小 |
| 更新签名 | 用 MyGo 的 Ed25519 机制；需配置 `MYGO_UPDATER_PRIVATE_KEY` 与 `updates` 才能发布签名清单，否则设置页显示「此版本未配置自动更新」 |

---

## 追加：2026-11-17 全量审查与导航重构的验证范围

本轮验证与上表口径一致，但只覆盖本轮改动可达的部分：

```
$ gofmt -l ./internal ./cmd main.go   → 无输出
$ go vet ./...                        → 无输出
$ go tool mygo vet .                  → 无输出
$ go test ./...                       → 全部 ok
$ go test -race ./internal/ui/ ./internal/engine/  → 全部 ok
$ CGO_ENABLED=0 go build ./...        → 无输出
$ go build -o build/zz_check.exe .    → 产物 44.6 MB（单文件，含内嵌 oxipng 与 lcms.wasm）
```

**本轮未执行**：`govulncheck`、六平台云端构建、安装包签名验证、真实 GPU 推理、人工 E2E 点击。
这些都需要外部环境或获批模型，记录在 [REVIEW-2026-11-17.md](REVIEW-2026-11-17.md) §5 与
[benchmarks/baseline.md](benchmarks/baseline.md) §4。

## 追加：独立审查修复与 AI 模型下载（2026-11-17）

- 原 17 项独立复现已纳入常规测试并通过，修复明细见 `REPAIR-2026-11-17-INDEPENDENT.md`。
- 增加设置页双源及代理交互、配置保存重载、代理连接失败、下载取消、错误哈希、已有模型保护和清单合并上限回归，均通过。
- `CGO_ENABLED=0 go test ./... -count=1 -timeout 15m`、`go vet ./...`、`go tool mygo vet .` 通过。
- Windows/Linux/macOS 的 amd64、arm64 均通过 `CGO_ENABLED=0 go build ./...`；这不等于原生安装包构建或运行验收。
- GitHub 与国内加速实际下载测试通过。测试使用系统代理，下载后执行固定归档哈希及模型文件哈希校验。联网测试需显式设置 `IMAGEFLOW_MODEL_DOWNLOAD_LIVE=1`，默认测试不访问下载源。
- 含双源下载实现的全量 `go test -race ./... -count=1 -timeout 20m` 通过；随后补充的设置交互和清单上限测试也通过定向 race 检查。未执行云端发布或六平台原生安装验收。
- 后续执行 `TestRealGPUInference`：真实 RTX 4060 / Radeon 780M 推理和七条应用适配器路径全部通过。三个模型、2×/3× 后处理、PNG/JPG/WebP、TTA 均有实测输出。仅覆盖 128×128 插画输入；结果及证据见 `benchmarks/GPU-2026-11-17.md`，不等于大图性能、画质或显存验收。

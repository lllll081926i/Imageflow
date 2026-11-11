# 全量审查问题清单

> 本文件保留历史记录，部分条目已经修复。2026-11-11 的代码复核、压缩接入与验证结果见 [本次审查](REVIEW-2026-11-11.md)。

## 待处理 · 严重

- **[S1] `internal/codec/codec.go:63` `Open` / `internal/output/output.go:22` `CheckSize`** — 尺寸预算发生在解码之后。`Open` 直接调用 `jpeg.Decode` / `png.Decode` / `webp.Decode` / `heic.Decode` 等，`CheckSize` 只在部分引擎路径拿到图像后才调用；SVG 分支先按 viewBox 光栅化再返回。头部声明极大尺寸的高压缩比文件会先按头分配像素缓冲，单文件即可占用数 GB 内存、长时间冻结或直接 OOM。
- **[S2] `internal/engine/gif.go:72` `LoadAnimation`** — APNG / WebP 分支用 `io.ReadAll(f)` 整文件读入内存，GIF 分支用 `gif.DecodeAll` 一次性解码全部帧；全程没有文件字节上限、帧数上限或逐帧尺寸校验，`CheckSize` 只在个别下游调用点检查首帧。几百 MB 的动图或上万帧 GIF 会吃满内存。
- **[S3] `internal/paths/paths.go:200` `Naming.Render` / `internal/batch/session.go:203` `BuildOutputPath`** — 输出文件名由用户可编辑的模板与 `{prefix}`、`{basename}` 拼成，拼接前不做净化。模板或前缀含 `..`、路径分隔符、盘符、Windows 保留名（CON / NUL / AUX）、超长路径时，`filepath.Join(dir, name)` 可逃出输出目录或直接写失败；大小写不敏感的文件系统上，同批不同输入还可能静默互相覆盖。
- **[S4] `internal/ui/files.go:135` `AddPaths` / `:165` `loadMetaRows` / `:189` `loadPreview`** — 在窗口回调线程同步执行目录递归展开、逐文件 `Probe`、缩略图生成和首图全解码；`main.go:76/80/100` 的 `-open`、拖放、启动参数都走这条路径。拖入含数千张图的文件夹或大尺寸 HEIC/AVIF 时窗口长时间无响应，且过程不可取消。

## 待处理 · 中等

- **[M1] `internal/batch/session.go:167` `SetSettings`** — 接收完整 `settings.Settings` 并整体覆盖，且 `settings.Save(next)` 在释放 `Mu` 之后执行。两个调用方各自基于旧快照改一个字段时，后写覆盖先写；`files.go:182` `rememberInputDir` 直接追加 `RecentInputDirs`，与设置页保存并发即丢更新。落盘顺序也可能与内存顺序不一致。
- **[M2] `internal/batch/batch.go:53` `Run`** — worker goroutine 直接执行 `job.Run`，没有 `recover`。任一编解码器或第三方库 panic 会穿透 worker 终止整个进程，而不是把该行标记为失败。
- **[M3] `internal/svg/svg.go:41-50` `shared`** — 全局 `rendererOnce` 创建的 `*resvg.Renderer` 从不调用 `Close()`，Rust / CGO 侧资源常驻到进程退出；Renderer 的生命周期与复用状态没有管理点。
- **[M4] `internal/codec/codec.go:122` `Probe` 与 `:63` `Open`** — `Probe` 读头部原始宽高，`Open` 只对 JPEG 应用 EXIF 方向（`applyOrientation`），其它带方向元数据的格式不处理。竖拍图在列表中的尺寸、缩略图比例与实际处理尺寸不一致，方向元数据可能与已旋转的像素被重复应用。
- **[M5] `internal/codec/meta.go:63`** — 只有 JPEG 且 `KeepMetadata` 开启时才把 ICC profile 拼回；PNG / HEIF / WebP / AVIF 等格式的色彩描述在转换链路中丢失，广色域图片转出后色彩偏移。
- **[M6] `internal/engine/gif.go:99-103`、`:133-137`、`:164-168`、`:216-242`** — 三条动画分支各自换算帧延迟（GIF `×10`、APNG `GetDelay()*1000`、WebP 原值），换算后直接与合成帧按索引配对；但 `composeAPNG` 在 `frame.Image == nil` 时 `continue`，产出帧数可能少于 `a.Frames`，延迟按原索引取值就会错帧。另外 `composeAPNG` 只实现了 `DISPOSE_OP_BACKGROUND`，`DISPOSE_OP_PREVIOUS` 未恢复上一帧画布（GIF 分支在 `:188` 有对应处理），使用前一帧处置的 APNG 合成结果错误。
- **[M7] `../.github/workflows/ci.yml`** — 工作流不含任何 Go 相关步骤（无 `go test` / `go vet` / `go build`、无 `-race`），Go 侧改动不触发门禁。
- **[M8] `mygo.json:6-11`** — `updates` 只配置了 `publicKey`，仓库内没有签名私钥的注入路径，构建产出的更新清单可能未签名，客户端验签失败。
- **[M9] `internal/ui/controls.go:84` `motionReduced`** — 包级可变全局，由 `app.go:266` 每帧写入、`motion()` 及 segmented / loading 多处读取，依赖「单 UI 线程」这一未显式约束的假设；`segmented_test.go:52` 直接改写全局且未串行化，引入后台渲染或 `t.Parallel()` 即为数据竞争。
- **[M10] `internal/ui/desktop.go:18` `trayOnce`** — 托盘初始化用包级 `sync.Once` 包住，首次因平台不支持或资源缺失失败后无法重试，失败也没有回传给调用方；进程生命周期内后续调用直接跳过，托盘永久不可用且界面无提示。

## 待处理 · 轻微与观察

- **[L1] `internal/ui/desktop.go:202/259/265`** — 更新失败把底层 `err.Error()` 原样显示在设置页与状态栏，网络 / 文件系统错误可能带出本地绝对路径、临时文件名或凭据片段。
- **[L2] `internal/ui/tools.go` 的 12 个 `Run`** — 参数快照在每个方法里重复一整套样板，新增工具字段时容易漏拷，漏拷即重新引入 UI/worker 竞态。
- **[L3] `docs/VERIFICATION.md`** — 内容是 0.1.0 时期的进程级验证快照，与当前 2.3.0 的布局尺寸、动效和命令行行为不再对应，虽已标注历史快照，仍易被当作现行结论引用。

## 本轮审查确认

- **[F1] `internal/codec/codec.go:324` `flatten`** — `img.At().RGBA()` 返回的通道已经预乘 alpha，代码又乘了一次 alpha，半透明像素在白底合成时明显偏暗，转 JPEG 等无 alpha 格式颜色错误。
- **[F2] `internal/batch/session.go:203` `BuildOutputPath`** — `filepath.Ext(name)` 把 `photo.2024`（源自 `photo.2024.jpg`）的 `.2024` 当成扩展名，输出文件名没有后缀，编码器拒绝，整行处理失败。
- **[F3] `internal/output/output.go:100` `Commit`** — 目标不存在时走 `os.Stat → os.Create(target)` 直写，两个 worker 并发写同一路径时内容交错，且失败分支的 `os.Remove(target)` 会删掉另一方已成功写入的结果。
- **[F4] `internal/output/output.go:46` `Atomic`** — Windows 上 `os.Rename` 覆盖已存在目标会瞬时失败，原实现无重试，并发写同一路径偶发失败。
- **[F5] `internal/ui/tools.go`、`internal/ui/tools_extra.go` 的 12 个 `Run`** — 闭包在 worker 内直接读 `t.*` 字段，与 UI 控件的每帧读写构成数据竞争。
- **[F6] `internal/ui/tools.go:596` `StitchTool.Run` / `:1115` `PDFTool.Run`** — 自起裸 goroutine 绕开 `runBatch`，不登记 `planned`、不接取消：「停止处理」无效，上一批残留的输出路径预留污染后续批次。
- **[F7] `internal/ui/files.go:285` `Start`** — 无重入保护，处理中再次点击开始会叠加第二批并覆盖进度状态。
- **[F8] `internal/ui/runner.go:17` `runBatch`** — 结果统计缺 `Cancelled` 分支，取消后行状态与汇总文案缺失。
- **[F9] `internal/ui/app.go:138` `flashSettingsSaved`** — 每次保存都新起 `time.AfterFunc`，旧定时器到点会清掉新提示，连续保存时提示提前消失。
- **[F10] `internal/ui/picker.go`** — 文件对话框回调在后台 goroutine 直接写输入框绑定的 `*string`，与渲染线程读写竞态。
- **[F11] `internal/ui/desktop.go:226` `InstallUpdate` / `:24` `ShouldHideOnClose`** — 安装成功后 `Relaunch` 触发的关窗被「后台运行」的 `OnClose` 拦截，应用不重启；安装失败仍保留旧 `Version`，界面继续显示「可更新」；检查与安装共用一个 `Busy`，无重入保护。
- **[F12] `internal/svg/svg.go:125` `checkHrefAttributes`** — XML 解析报错时无条件 `return nil`，畸形标签之后的 `xlink:href` 外链属性逃过校验。
- **[F13] `cmd/imageflow-batch/main.go`** — `-tool pdf2` 命中死分支静默成功（无输出、退出码 0）；未知工具名在空输入时不校验；未指定 `-out` 时临时输出目录不打印，结果不可见。
- **[F14] `cmd/imageflow-icon/main.go`** — `os.Exit` 跳过 defer，生成的临时 PNG 残留。
- **[F15] `internal/codec/meta_test.go`** — EXIF fixture 写入 orientation 后又用 0 覆盖同一缓冲区，用例恒为 orientation 0，从未验证方向重置。
- **[F16] `internal/ui/integration_test.go`** — 取消断言写成 `if cancelled == 0 && status == ""`，两条件需同时成立才失败，等价恒真。
- **[F17] `internal/batch/batch_test.go` `TestRunClampsConcurrency`** — 只断言并发上界，未断言实际达到并发，worker 数为 1 也能通过。
- **[F18] `internal/ui/layout_test.go`、`page_swap_test.go`** — 断言写死 232 / 700 等字面量，与被测常量脱钩，改布局常量测试仍可能通过。
- **[F19] `docs/DESIGN.md`** — 圆角表写 24/16/12/8，实际代码为 16/12/10/7；修正记录编号 54–61 重复；动效表停留在旧时长与旧曲线。
- **[F20] `README.md`、`.gitignore`** — 减弱动效与切页动效描述、`@latest` 构建命令、`-tool 5` 页码均与当前实现不符；`.gitignore` 漏掉测试生成的截图。

## 此前几轮审查确认

- **[H1] `internal/ui/app.go` `pageSwap`** — 切页带 4~18 DIP 位移与尺寸过渡，大块内容切换时整个工作台像窗口在跳。
- **[H2] `internal/ui/app.go` `railItem`** — `ButtonBase` 默认居中，每行「图标 + 文字」按各自标签宽度整组居中，标签长短不一导致整列图标横向漂移；选中项图标底座放大 +2 又把文字顶右。
- **[H3] `internal/ui/controls.go` `ElementTransition`** — 预设未显式设置属性标志，MyGo 按「动画全部属性」处理，把父行每帧算好的侧栏宽度又缓动一遍；收起过程中间卡片最多滞后 157 DIP，与侧栏动画完全不匹配。
- **[H4] `internal/ui/app.go` `railList`** — 选中指示只在工具页构建，打开设置页移除列表首个子元素后，13 个无 Key 的行按位置重编号，整列被当作新元素重建，hover / focus / 动画通道全部丢失。
- **[H5] `internal/ui/controls.go` `motion()`** — 减弱动效返回零值过渡，等价于「动画全部属性 + 默认 200 ms」，比正常模式更慢。
- **[H6] `internal/engine/pdf.go`** — 把 `margin == 0` 当「未设置」强制改成 36，覆盖用户显式选择的贴边布局（水印边距的同类问题已修，PDF 漏掉）。
- **[H7] `internal/ui/desktop.go`、`files.go` 四处** — 无锁直读 `Session.Settings`，与 `SetSettings` 的锁内整结构写入构成数据竞争。
- **[H8] `internal/ui/files.go` `AddPaths`** — 内联探测循环与紧随的 `loadMetaRows(rows)` 完全相同，每个新文件被探测和生成缩略图两次。

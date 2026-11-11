# ImageFlow Go

Go + [MyGo](https://github.com/egoist/mygo) native UI + [Lobe MyGo](https://mygo.lobehub.com)
组件的图像工作台重写版。窗口由 MyGo 在 GPU 上直接绘制，没有 webview、没有 HTML、没有 JavaScript。

## 状态

| 工具 | 状态 | 引擎 |
| --- | --- | --- |
| 格式转换 | 完成 | JPG / PNG / WebP / AVIF / TIFF / BMP / ICO，比率 / 固定尺寸 / 长边缩放，EXIF 方向，SVG 输入，可选保留 EXIF / ICC / XMP |
| 尺寸调整 | 完成 | 百分比 / 长边 / 短边 / 多尺寸预设，LANCZOS |
| 图片裁剪 | 完成 | 9 种比例 + 自定义，九宫格取景，固定输出尺寸，圆角与外部填充色 |
| 宫格切图 | 完成 | 2×2 ~ 5×5 |
| 图片布局 | 完成 | 1–50 行列，正方形裁切或覆盖 |
| 长图拼接 | 完成 | 横 / 竖，间距、背景、边框模式 |
| 批量水印 | 完成 | 字体、字号、颜色、透明度、旋转、九宫格、平铺、描边、投影 |
| 色彩提取 | 完成 | 加权中位切分，主题色 + 2–16 色可调色板 |
| 图标生成 | 完成 | 16 档 PNG（16–1024）、`icon.ico`、`favicon.ico`、内嵌 PNG 的 `icon.svg`；内边距、圆角、透明或指定底色 |
| 转 PDF | 完成 | 15 种纸张、方向、边距、网格拼版、AES-256 加密 |
| GIF 工具 | 完成 | 帧导出、倒放、变速、压缩（质量→调色板档位）、缩放；GIF / APNG / WebP 互转；保留逐帧 dispose/blend 语义 |
| 富文本制图 | 完成 | 标题、正文、列表、引用、配图；4 套配色主题；CJK 自动换行；画布与排版参数 |
| 图片压缩 | 完成 | 独立侧边栏页面；OxiPNG / jpegli / libwebp / libavif；同格式防膨胀、原图保护、动图检查 |

「图片压缩」使用专用编码器：JPG 使用 jpegli，PNG 使用 OxiPNG 9.1.5，
WebP 使用 libwebp（Method 6），AVIF 使用 libavif（Speed 4、透明通道质量 100）。
默认保留格式和像素尺寸，质量 80；PNG 始终无损，WebP 可选无损。
同格式压缩未变小时输出原始文件副本；跨格式输出的体积可能增加。
原格式 PNG 优化保留元数据；其他压缩重新编码保留 ICC，移除 EXIF / XMP；原始副本保留全部元数据。
格式转换可选保留 EXIF / XMP（JPG、PNG、WebP）；ICC 支持 JPG、PNG、WebP、AVIF、TIFF。
使用内置 Little CMS 2 WASM 将 RGB、CMYK、灰度 ICC 输入转换到 sRGB，无需安装系统色彩引擎；预览、缩略图、拼接、图标、富文本、色卡和 PDF 复用同一转换。
PDF 图像嵌入 ICCBased 色彩配置，透明区域按白色纸面合成，输出采用原子写入。
支持 PNG gAMA / cHRM / cICP 和 AVIF / HEIF NCLX 色彩描述；PQ / HLG HDR 输入映射为 SDR，AVIF 高位深输入保留 16 位处理精度，HEIF 精度受现有 8 位解码器限制。无效配置仍明确报错。
压缩页拒绝动图，避免只保存第一帧；动图继续使用 GIF 工具。
Windows x64 构建内置 OxiPNG，校验 SHA-256 后释放到用户缓存，无需联网安装。
其他平台需安装 `oxipng` 并加入 PATH。依赖来源和许可证见 `internal/pngopt/README.md`。

## 桌面能力

| 能力 | 实现 |
| --- | --- |
| 无边框窗口 + **原生**标题栏 | `WindowOptions.Title`；应用不自绘标题栏，避免内部圆角与窗口圆角对不上 |
| 窗口位置记忆 | `WindowOptions.StateKey`，设置页可关闭 |
| 透明材质 | 不启用：界面为不透明阶梯表面，与 `docs/DESIGN.md` 第 4 条一致 |
| 文件拖放 | `Window.OnFileDrop` + 拖放区 `DroppedFiles()` |
| 图片预览切换 | 点击缩略图或文件行切换；预览区内使用 ← / ↑ 查看上一张，→ / ↓ 查看下一张。选中项高亮并自动滚入列表可见区域；后台解码拒绝过期结果 |
| 系统文件 / 目录对话框 | `mygo.Dialog.Open` |
| 打开输出目录 | `mygo.Shell.ShowItemInFolder` / `OpenPath` |
| 托盘图标 + 隐藏到托盘 | `mygo.NewTray`，关窗按「后台运行」设置隐藏或退出 |
| 单实例 | `App.RequestSingleInstanceLock` + `OnSecondInstance` 聚焦窗口 |
| 自动更新 | `mygo.Updater.Check/Install`，设置页有入口与进度 |
| 明暗主题 | `theme.Apply` + 自建 style tokens。Lobe 默认 accent 是近黑色，本应用每帧把 frame 主题改指品牌蓝 |
| 减弱动效 | 设置页开关让 `motion()` 返回 `ElementTransition{Duration: -1}`，MyGo 将所有变化与 Enter/Exit 立即到位（零值过渡反而会动画全部属性） |
| 保留目录结构 | 拖入文件夹后，输出目录按 `preserve_folder_structure` 重建子目录树 |

## 运行与验证

```bash
go test ./...            # 单元与集成测试
go test ./... -race      # 竞态检测
go build -o build/ImageFlow-dev.exe .
./build/ImageFlow-dev.exe                          # 打开窗口
./build/ImageFlow-dev.exe -hidden                  # 只驻留托盘
./build/ImageFlow-dev.exe -open ./photos -tool 6   # 预载文件并停在批量水印页
./build/ImageFlow-dev.exe -capture out.png         # 截图并退出（CI 用，不占单实例锁）
```

`-capture` / `-open` / `-tool` / `-hidden` 是验证参数：打包产物可以无人值守地截图，
用于确认界面与开发版一致。

发布构建：

```bash
# go tool 使用 go.mod 中锁定的 MyGo 版本，保证与开发/测试一致、产物可复现。
go tool mygo build -skip-installer
go tool mygo build -platform darwin/amd64,darwin/arm64,linux/amd64,linux/arm64,windows/arm64 -skip-installer -skip-dmg
```

发布前需在 `mygo.json` 的 `updates` 中配置 `privateKey`（私钥文件路径），
或把 **base64 编码的私钥内容**放进 `MYGO_UPDATER_PRIVATE_KEY` 环境变量；
否则非 Linux 平台不会产出签名清单，客户端无法校验更新。

命令行批处理（与 UI 走同一批引擎代码，用于脚本与 CI 验证）：

```bash
go run ./cmd/imageflow-batch -tool convert -format webp -quality 85 ./photos
go run ./cmd/imageflow-batch -tool compress -quality 80 ./photos
go run ./cmd/imageflow-batch -tool compress -format webp -lossless ./photos
go run ./cmd/imageflow-batch -tool watermark -text "ImageFlow" ./photos
go run ./cmd/imageflow-batch -tool richtext -theme dark -image cover.jpg ./photos
go run ./cmd/imageflow-batch -tool gif -action probe ./photos
go run ./cmd/imageflow-batch -tool gif -action export_frames -frame-start 0 -frame-end 3 ./photos
```

`-tool gif` 的动作由 `-action` 指定：`probe` `reverse` `change_speed` `compress` `resize` `export_frames`。

支持的 `-tool`：`convert` `compress` `resize` `grid` `layout` `stitch` `watermark` `palette` `pdf` `gif` `richtext`。

裁剪、图标生成与多尺寸预设由 UI 直接调用引擎，命令行批处理暂不暴露这三个工具。

## 目录

```
main.go                     应用入口：单实例、托盘、更新、-capture/-open/-tool 验证参数
cmd/imageflow-batch/        命令行批处理
internal/ui/                界面：外壳、工具卡、输入卡、参数卡、13 个工具页、设置页、托盘与更新
docs/DESIGN.md              界面样式契约（颜色、布局、圆角、动效）
internal/engine/            引擎：转换、裁剪、拼接、水印、取色、PDF、GIF、富文本
internal/codec/             编解码：jpegli / webp / avif / heic / ico / tiff / bmp
internal/pngopt/            OxiPNG 调用、Windows x64 内置程序与许可证
internal/svg/               SVG 安全校验与光栅化（resvg via wazero）
internal/output/            尺寸预算与原子写入
internal/paths/             路径规范化、目录展开、命名模板、冲突策略
internal/settings/          设置持久化（原子写）
internal/thumbs/            缩略图内存 + 磁盘双层缓存
internal/batch/             并发批处理与协作式取消
```

## 界面

布局与配色按 [docs/DESIGN.md](docs/DESIGN.md) 实现：

- 工具卡 216 DIP（可收起 72），输入卡占剩余宽度，参数卡 420 DIP；窗口小于 1080 DIP 时工具卡自动折成图标条。
- 单一品牌蓝 `#007AFF`，主操作与选中导航用 `#007AFF → #0055FF` 渐变加光晕。
- 明暗双主题对等；表面色为不透明阶梯，避免半透明叠层把卡片边界糊掉。
- 应用图标直接取自原项目 [`frontend/logo.svg`](../frontend/logo.svg)，
  窗标题、设置页、托盘与安装包用同一份 SVG：`internal/brand` 是唯一来源，`cmd/imageflow-icon` 用它生成 `resources/icon.png`。小尺寸下使用裁掉边距的 `IconSVG`。
- 动效：按压缩放是即时反馈，阴影淡变用 `Snappy` 弹簧；侧栏宽度、切页淡入、
  选中指示滑动与淡出共用一条 `shellCurve`（`EaseInOut` 220 ms），
  布局盒本身不参与过渡，保证中间卡片与侧栏同帧移动。
  切页只做不透明度 `0.72 → 1` 的 `Enter` 淡入，不做位移、不做 `Exit` 副本；
  设置页的「减弱动效」让所有变化立即到位。详见 `docs/DESIGN.md` 第 6 节。

`internal/ui/testdata/` 中留有工具页、设置页与深色模式的渲染截图，
由 `go test ./internal/ui/` 自动生成，可直接对比视觉回归。

字体：水印与富文本的字体选择器列出本机实际安装的字体（`engine.AvailableFonts()`），
新建面板默认取平台首选字体（Windows 是微软雅黑），不会因字母序落到 Arial。

## 已知限制

- OxiPNG 暂仅内置 Windows x64 版本；其他平台需要自行安装 OxiPNG。
- 设置里的 `ui_transparency` 保留字段但不起作用：新界面不使用半透明材质，
  否则卡片边界会糊掉（`docs/DESIGN.md` 第 4 条修正记录）。
- 缩略图首次加载时解码整图再缩小，没有 JPEG 解码器降采样。
- WebP 动图编码由 libwebp WASM 完成；帧内容完全相同的动图会被 libwebp 折叠为静图，这是 libwebp 的行为。
- 自动更新的公钥写在 `mygo.json` 的 `updates` 里，私钥在
  `%APPDATA%\imageflow\update-keys\mygo-update.key`（不进仓库）。发布时设
  `MYGO_UPDATER_PRIVATE_KEY`，`mygo build` 会生成签名清单；把
  `build/<platform>/` 下的 `update-*.json` 与 `*.tar.gz` 传到对应 tag 的 release。

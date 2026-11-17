# ImageFlow

一站式桌面图片工具。批量压缩、转换、裁剪、加水印、拼接和导出 PDF，在一个窗口中完成。

[![最新版本](https://img.shields.io/github/v/release/lllll081926i/Imageflow?label=版本)](https://github.com/lllll081926i/Imageflow/releases/latest)
[![许可证](https://img.shields.io/github/license/lllll081926i/Imageflow)](LICENSE)

**[下载最新版](https://github.com/lllll081926i/Imageflow/releases/latest)** · [反馈问题](https://github.com/lllll081926i/Imageflow/issues)

## 下载与安装

在 [Releases](https://github.com/lllll081926i/Imageflow/releases/latest) 中选择与系统、处理器匹配的文件。`amd64` 表示 Intel / AMD 64 位，`arm64` 表示 ARM 64 位。

| 系统 | 下载选择 |
| --- | --- |
| Windows | `imageflow-setup-版本号-架构.exe` 安装包；也可下载便携 ZIP，解压后运行 `ImageFlow.exe` |
| macOS | 下载对应架构的 `.tar.gz`，解压后将 `ImageFlow.app` 放入“应用程序” |
| Linux | 下载对应架构的 `.deb` 或 `.tar.gz`；压缩包中提供 `install.sh` |

Windows x64 已内置 PNG 压缩引擎。其他平台使用 PNG 压缩前，需安装 [OxiPNG](https://github.com/oxipng/oxipng) 并将其加入 `PATH`。

macOS 构建未经过 Apple 开发者签名和公证，系统可能要求在“隐私与安全性”中允许打开。各平台的实际构建和验证范围以发布说明为准。

## 功能

| 工具 | 能力 |
| --- | --- |
| 图片压缩 | JPG、PNG、WebP、AVIF；质量调整、PNG 无损优化、WebP 可选无损 |
| 格式转换 | 输出 JPG、PNG、WebP、AVIF、TIFF、BMP、ICO；可选保留部分元数据 |
| 尺寸调整 | 按比例、长边、短边或指定尺寸缩放，支持多尺寸预设 |
| 图片裁剪 | 常用比例、自定义比例、九宫格取景、圆角和填充 |
| 宫格切图 | 将图片切为 2×2 至 5×5 宫格 |
| 图片布局 | 自定义行列，选择裁切或覆盖方式 |
| 图片拼图 | 1–16 张预设模板与自定义网格，画布比例、边距、背景色/图、逐格取景（含左右镜像与上下翻转）、文字/箭头/方框/圆圈标注（可选中拖动编辑）、平铺水印；默认无接缝，可切到有接缝；可交互预览；全部合成一张 / 每 N 张一张；一键复制到剪贴板；参数持久保存 |
| 长图拼接 | 横向或纵向拼接，设置间距、背景和边框 |
| 批量水印 | 文本水印、系统字体、透明度、旋转、平铺、描边和投影 |
| 色彩提取 | 提取主题色和 2–16 色调色板 |
| 图标生成 | 多尺寸 PNG、ICO、favicon 和内嵌 PNG 的 SVG |
| 转 PDF | 纸张尺寸、方向、边距、网格拼版和密码加密 |
| GIF 工具 | 帧导出、倒放、变速、缩放、压缩，以及 GIF / APNG / WebP 互转 |
| 富文本制图 | 标题、正文、列表、引用和配图，支持中文换行与多套配色 |
| 基础编辑 | 旋转、翻转、任意角度、画布与九宫锚点、边框、圆角、投影、模糊/锐化、灰度/反相；参数快照撤销与重做 |
| 自由布局 | 图片 / 文字 / 形状图层画布，位置尺寸、旋转、不透明度、锁定与隐藏、层级调整、内置模板 |
| 图片标注 | 矩形、圆形、箭头、直线、画笔、文字、编号、高亮、马赛克、模糊与**安全遮盖**（不可逆像素覆盖） |
| 截图美化 | 留白、圆角、投影、描边、纯色或渐变背景、常用画布比例 |
| 专业调色 | 曝光（线性光）、亮度、对比、高光/阴影、白/黑色阶、饱和度、自然饱和度、色温/色调、色阶、曲线中点、3D `.cube` LUT |
| 滤镜效果 | 黑白、反相、复古、暖/冷色、胶片颗粒、暗角、高斯/运动模糊、锐化、浮雕、边缘检测、素描、像素化、减色 |
| 图片对比 | 共享对比组件，PSNR、SSIM、最大/平均通道差与差异图；尺寸不一致时可选对齐方式 |
| 图像分析 | R/G/B/亮度/Alpha 直方图、过曝与纯黑像素、半透明像素、不同颜色数、采样范围标注 |
| 像素画 / 点阵 | 2–16 色或灰阶、严格黑白、Bayer / Floyd–Steinberg / Atkinson 抖动、阈值与反相，可选**真正的 1-bit PNG 编码** |
| 表情包制作 | 上下字幕、居中大字、对话气泡三种模板；动图可按帧区间加字 |
| AI 高清放大 | 可选的本地推理组件（Real-ESRGAN-ncnn-vulkan）；基础包不包含引擎与模型 |

支持拖入文件或文件夹、批量处理、任务取消、输出命名和目录结构保留。启动后首先看到**首页**：
可以搜索工具、从六个常用入口直接开始，或按分组浏览全部 25 个工具。左侧侧栏同样按分组排列，
每组可折叠，顶部可搜索工具名称与同义词；工具行可长按拖动重新排序，顺序与折叠状态都会保存。
界面提供明暗主题、可收起侧边栏和减弱动效选项。

## 开始使用

1. 在左侧侧边栏选择工具，或在首页搜索、点选工具卡片。
2. 拖入图片或文件夹，也可使用文件选择按钮。
3. 点击缩略图或文件行切换预览。在预览区域使用 `←` / `↑` 查看上一张，使用 `→` / `↓` 查看下一张。
4. 调整参数和输出目录，开始处理。

压缩使用 jpegli、OxiPNG、libwebp 和 libavif。同格式压缩未减小文件时，保存原始文件副本。压缩页只处理静态图片，动画请使用 GIF 工具。

色彩处理支持 RGB、CMYK、灰度 ICC 配置，以及 PNG、AVIF、HEIF 的部分色彩描述。PQ / HLG HDR 输入会映射为 SDR。HEIF 当前受解码器限制，使用 8 位精度。

专业调色按 **8 位工作空间**处理：曝光在线性光中计算，其余参数在 sRGB 编码值上处理，页面会明确标注这一点。像素画的「严格黑白像素」与「真正 1-bit PNG 编码」是两个独立选项；勾选 1-bit 时输出格式只能是 PNG。

AI 高清放大是可选组件：需要单独安装推理引擎。模型可在「AI 高清放大 → 下载模型」选择普通 GitHub 或国内加速下载；下载后自动校验固定 SHA256 并登记可信清单。自行准备的模型可在 AI 页导入可信清单。未安装时其余全部工具正常使用。

在「设置 → AI 模型下载」选择默认下载源、加速前缀、代理模式、模型目录及引擎路径，然后保存设置。默认使用系统代理：优先环境代理，其次 Windows 静态代理，未找到配置时使用 `http://127.0.0.1:7890`。可改为自定义代理或不使用代理。代理失败不会自动直连；PAC 和非 Windows 原生代理配置不解析。

模型默认保存在 `ImageFlow.exe` 所在目录的 `models/`：安装版随安装目录，便携版随便携目录。下载 ZIP 和解压暂存也放在该目录，不使用系统临时目录或用户数据目录。默认目录以相对路径保存，移动便携目录后跟随程序；旧用户数据目录默认值自动切换，不移动或删除原有模型。用户自选绝对目录保持不变。目录不可写时明确报错，不自动回退。安装在 C 盘时，模型仍会占用 C 盘；需要避免时应将程序安装或解压到其他盘。

- 普通 GitHub：`https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-windows.zip`
- 国内加速：`https://ghfast.top/https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-windows.zip`

国内加速使用第三方服务，「直连」指文件下载地址，不表示绕过所选代理。两种来源执行相同校验；仅提取所选模型的 `.param` / `.bin`，不安装压缩包内的引擎程序。请核对权重许可证。

本次增量尚未完成的能力：长图自动接缝识别、压缩目标体积搜索、水印变量、GIF 时间轴图形界面、拼图画布内拖动换序。详见 `docs/roadmap_issue_plan.md`。

## 从源码运行

ImageFlow 3 使用 Go、[MyGo](https://github.com/egoist/mygo) 和 [Lobe MyGo](https://github.com/lobehub/lobe-mygo) 构建原生界面。依赖版本由 `go.mod` 和 `go.sum` 锁定。

安装 Go 1.27.4 或兼容的更新版本，然后执行：

```bash
git clone https://github.com/lllll081926i/Imageflow.git
cd Imageflow
go run .
```

在 Windows PowerShell 中也可执行 `./dev.ps1`，编译并启动开发版。

构建当前平台的发布包：

```bash
go tool mygo build
```

Windows 安装包使用 NSIS，MyGo 在缺少 NSIS 时会自动下载。仅构建可执行程序时，使用 `go tool mygo build -skip-installer`。macOS 的签名、公证和 DMG 制作需要 macOS 环境。

签名更新包需要在构建环境中设置 `MYGO_UPDATER_PRIVATE_KEY`，其内容必须与 `mygo.json` 中的更新公钥匹配。私钥不得提交到仓库。

## 命令行批处理

```bash
# 转换为 WebP
go run ./cmd/imageflow-batch -tool convert -format webp -quality 85 ./photos

# 保持原格式压缩
go run ./cmd/imageflow-batch -tool compress -quality 80 ./photos

# 无损 WebP 压缩
go run ./cmd/imageflow-batch -tool compress -format webp -lossless ./photos

# 添加水印
go run ./cmd/imageflow-batch -tool watermark -text "ImageFlow" ./photos

# 拼图：三张合成 16:9，默认无接缝
go run ./cmd/imageflow-batch -tool collage -preset 3-t1b2 -aspect 16:9 -margin 24 ./photos

# 拼图：带接缝（间距 12 + 圆角 8）与平铺水印
go run ./cmd/imageflow-batch -tool collage -preset 3-t1b2 -seam -gap 12 -radius 8 -watermark "ImageFlow" ./photos
```

使用 `go run ./cmd/imageflow-batch -h` 查看参数。裁剪、图标生成、拼图的可视化取景与多尺寸预设目前从桌面界面使用。

## 开发与验证

2026-11-17 独立审查的修复与回归说明见 [修复记录](docs/REPAIR-2026-11-17-INDEPENDENT.md)。AI 页支持导入可信模型清单，格式与目录约定见该记录；基础包仍不包含推理引擎和权重。

```bash
go test ./...
go vet ./...
go tool mygo vet .
```

代码主要位于 `internal/ui`（界面）、`internal/engine`（处理引擎）、`internal/codec`（编解码）和 `internal/colorprofile`（色彩转换）。[设计约定](docs/DESIGN.md)与[验证文档](docs/VERIFICATION.md)供开发参考。

## 许可证

项目使用 [MIT License](LICENSE)。第三方组件遵循各自许可证；内置 OxiPNG、Little CMS 和 ICC 配置的来源及许可证随对应目录提供。

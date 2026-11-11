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
| 长图拼接 | 横向或纵向拼接，设置间距、背景和边框 |
| 批量水印 | 文本水印、系统字体、透明度、旋转、平铺、描边和投影 |
| 色彩提取 | 提取主题色和 2–16 色调色板 |
| 图标生成 | 多尺寸 PNG、ICO、favicon 和内嵌 PNG 的 SVG |
| 转 PDF | 纸张尺寸、方向、边距、网格拼版和密码加密 |
| GIF 工具 | 帧导出、倒放、变速、缩放、压缩，以及 GIF / APNG / WebP 互转 |
| 富文本制图 | 标题、正文、列表、引用和配图，支持中文换行与多套配色 |

支持拖入文件或文件夹、批量处理、任务取消、输出命名和目录结构保留。界面提供明暗主题、可收起侧边栏和减弱动效选项。

## 开始使用

1. 在左侧侧边栏选择工具。
2. 拖入图片或文件夹，也可使用文件选择按钮。
3. 点击缩略图或文件行切换预览。在预览区域使用 `←` / `↑` 查看上一张，使用 `→` / `↓` 查看下一张。
4. 调整参数和输出目录，开始处理。

压缩使用 jpegli、OxiPNG、libwebp 和 libavif。同格式压缩未减小文件时，保存原始文件副本。压缩页只处理静态图片，动画请使用 GIF 工具。

色彩处理支持 RGB、CMYK、灰度 ICC 配置，以及 PNG、AVIF、HEIF 的部分色彩描述。PQ / HLG HDR 输入会映射为 SDR。HEIF 当前受解码器限制，使用 8 位精度。

## 从源码运行

ImageFlow 3 使用 Go、[MyGo](https://github.com/egoist/mygo) 和 [Lobe MyGo](https://github.com/lobehub/lobe-mygo) 构建原生界面。依赖版本由 `go.mod` 和 `go.sum` 锁定。

安装 Go 1.27.3 或兼容的更新版本，然后执行：

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
```

使用 `go run ./cmd/imageflow-batch -h` 查看参数。裁剪、图标生成和多尺寸预设目前从桌面界面使用。

## 开发与验证

```bash
go test ./...
go vet ./...
go tool mygo vet .
```

代码主要位于 `internal/ui`（界面）、`internal/engine`（处理引擎）、`internal/codec`（编解码）和 `internal/colorprofile`（色彩转换）。[设计约定](docs/DESIGN.md)与[验证文档](docs/VERIFICATION.md)供开发参考。

## 许可证

项目使用 [MIT License](LICENSE)。第三方组件遵循各自许可证；内置 OxiPNG、Little CMS 和 ICC 配置的来源及许可证随对应目录提供。

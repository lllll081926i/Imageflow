# ADR-002：编解码与压缩链保留既有实现

- 状态：已采纳
- 日期：2026-11-17
- 关联：CORE-01、CORE-05、CORE-08、M0-04

## 背景

PRD 候选库清单里提到 `kovidgoyal/imaging`（README 声称 ICC、APNG、动画 WebP，`go.mod` 要求 Go 1.26）与 `anthonynsimon/bild`（`go.mod` 要求 Go 1.25）。当前工程是 Go 1.27.3，并已锁定下列依赖：

- `gen2brain/jpegli`（JPEG 编码，含量化表优化）
- `gen2brain/webp`（WebP 读写，含动画）
- `gen2brain/avif`、`gen2brain/heic`
- `kettek/apng`（APNG 读写）
- `sergeymakinen/go-ico`（ICO）
- `pdfcpu`、`signintech/gopdf`（PDF，冻结维护）
- 内置 `oxipng.exe`（带 SHA256 校验）

## 决策

保留现有编解码与压缩链，不因候选库的功能描述替换任何已锁定的依赖。新增能力通过适配层实现，不迁移目录结构。

## 理由

1. `internal/codec/matrix_test.go` 已经对每个可写格式做"编码→重新打开"验证，是真实读写证据，不是 README 声明。
2. `internal/codec/color_support_test.go` 覆盖 CMYK ICC 输入、无 ICC 容器的 RGB 导出、HDR10 AVIF 色调映射、PNG cICP/色度坐标，这些是替换依赖必须重新建立的回归。
3. oxipng 与 jpegli 的可执行文件带 SHA256 校验（`internal/pngopt/bundled_windows_amd64.go`），替换会改变校验链与分发许可登记。

## 后果

- 格式能力对外只能依据 `internal/capabilities` 的矩阵声明，禁止按 README 添加"支持"文案。HEIF 只读、SVG 只栅格化，都是该矩阵的显式行。
- 新增写出能力（例如真正的 1-bit PNG）必须自带编码器与 IHDR 回归：本次通过 `internal/png1bit` 实现，并由 `TestPixelArtWritesARealOneBitPNG` 断言 `bitDepth=1`、`colorType∈{0,3}`。
- 若将来确需换库，必须另起 ADR 并附：同图基准（P50/P95、峰值 RSS）、编码质量数值对照、许可证与分发审查。

## 未采纳的方案

| 候选 | 未采纳原因 |
|---|---|
| `kovidgoyal/imaging` | 上游要求 Go 1.26 且对未知格式回退 ImageMagick；与"纯 Go 核心"边界冲突，且会引入外部进程依赖 |
| `anthonynsimon/bild` | 当前主分支要求 Go 1.25，功能与现有 imaging/x-image 重叠，无增量证据 |
| 更换 GIF 量化器 | 现有 `adaptivePalette` 已按帧自适应量化，替换需质量与体积对照，本次无此证据 |

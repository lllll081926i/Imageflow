# ADR-005：真正的 1-bit PNG 与"严格黑白"分开实现

- 状态：已采纳
- 日期：2026-11-17
- 关联：MEDIA-07、QA-01、E2E-11

## 背景

PRD 与 QA 明确区分两件事：

1. **严格黑白像素**：输出图的每个像素只能是黑或白，不能有中间灰。
2. **真正 1-bit PNG 编码**：IHDR 的 `bitDepth=1`，`colorType` 为 0（灰度）或 3（索引），索引模式的有效调色板只含黑白，且无半透明。

`assets/fixtures/bw_400x300_reference.png` 的 manifest 标注为 RGB，只是黑白像素参考，**不能**作为位深编码合格样本。用一个 8 位 RGB 的黑白图冒充 1-bit 输出属于虚假声明。

## 决策

新增独立包 `internal/png1bit`，自带 PNG 写入器：

- IHDR：`bitDepth=1`，`colorType` 0 或 3。
- 索引模式的 PLTE 固定写 `[黑, 白]`，因此置位比特恒等于"白"，灰度与索引共用同一比特布局。
- 行过滤固定 0，zlib 用 `BestCompression`。
- 写入 `sRGB` 块（渲染意图 0），避免查看器对两值图像套用未知传递曲线。

像素化引擎（`internal/engine/pixel.go`）分别返回**像素色数**与**预期位深**：

- 严格黑白模式返回 8 位，除非用户显式勾选"1-bit 编码"。
- 勾选 1-bit 时，若输出扩展名不是 `.png`，直接拒绝并给出原因，不静默降级成 8 位。

## 理由

- 位深是容器属性，像素集合是图像属性；两者用一条代码路径表达会让"通过"变得不可判定。
- 自带写入器只有 200 行，比引入一个"能写 1-bit"的第三方库更容易审计，也不会改变既有编解码链。
- 明确拒绝非 PNG 目标是诚实性要求：JPEG 无法表达 1-bit，静默写 8 位会让用户以为拿到了 1-bit 文件。

## 后果

- `TestPixelArtWritesARealOneBitPNG` 同时断言：IHDR 位深为 1、颜色类型为 0 或 3、解码后尺寸正确、全部像素只取 0 或 65535。
- `TestPixelArtRejectsOneBitNonPNG` 固定"1-bit + JPEG = 拒绝"。
- `TestStrictBlackWhiteHasExactlyTwoColours` 固定严格模式的像素集合恰为两色，且无半透明。
- `png1bit.Mask` 提供 `Raw()` 返回打包比特，便于后续把"像素断言"与"编码断言"分开报告。

## 未采纳的方案

| 方案 | 未采纳原因 |
|---|---|
| 用 `image/png` 写 8 位再声明为 1-bit | 直接违反 PRD，属虚假声明 |
| 用 GIF 表示两色图 | GIF 是索引色但容器位深不是 PNG 1-bit，无法通过 IHDR 验收 |
| 引入第三方 1-bit 编码库 | 功能单一，自行实现更易审计且不改变依赖链 |

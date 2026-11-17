# ADR-008：能力快照作为格式与组件声明的唯一来源

- 状态：已采纳
- 日期：2026-11-17
- 关联：SYS-01、CORE-01、CapabilitySnapshot（PRD §13.5）

## 背景

PRD 要求"界面应基于 `CodecCapability` 运行时结果显示读/写/动画/Alpha/ICC 能力；禁止对尚未测试的 AVIF、HEIC、JXL 或动态 WebP 写入显示'完全支持'"。此前每个页面各自维护格式列表（转换页用 `codec.OutputFormats`、压缩页用 `compressionFormats`、GIF 页写死 `gif/webp/apng`），三者一旦不一致，就会出现"导出下拉里有、编码器没有"的情况。

## 决策

新增 `internal/capabilities`：

- 一张 `formatTable` 按 `decode / encode / animatedDecode / animatedEncode / alpha / ICC / quality / lossless` 八个维度逐格式登记。
- 不支持的维度必须带 `Reason`，可写格式不得带 `Reason`（由 `TestSnapshotCoversEveryWritableFormat` 固定）。
- `EncoderFormats()` 从表中推导，供转换页使用；`AnimationTargets()` 供动画页使用。
- 设置页新增"编解码能力"区，直接渲染同一份快照，并显示色彩管理说明与告警。

## 理由

- 单一表格让"声明"与"实现"的偏差变成编译期与单元测试可捕获的问题，而不是用户投诉。
- 逐维度登记避免把"能解码"说成"能编码"——这是 PRD 反复强调的错误。
- 设置页展示同一快照，使帮助文案与导出菜单不可能给出两种说法。

## 后果

- HEIF 行只读并给出原因（缺 HEIC 编码组件）；SVG 行只读并说明"矢量描摹未开放"，与 P2 的 SVG 描摹能力边界一致。
- 新增格式必须先补 `internal/codec/matrix_test.go` 的编码→重开用例，再改这张表。
- 动画容器列表（GIF / APNG / WebP）由表推导，页面不再硬编码。

## 未采纳的方案

| 方案 | 未采纳原因 |
|---|---|
| 每个页面各自维护列表 | 本次已经出现三处不一致，属于根因未除 |
| 运行时真实编码探测每个格式 | 每次启动都要做一次编码回环，代价高且会写临时文件；回归测试更适合承担这一职责 |
| 只用一个布尔"支持"字段 | 无法表达"只读""只动画读"等真实状态 |

# ADR-004：侧栏顺序改用稳定工具 ID，并迁移旧索引

- 状态：已采纳
- 日期：2026-11-17
- 关联：COMP-02、DATA-01、CORE-03

## 背景

v3.1.0 的 `settings.Settings.RailOrder []int` 存的是"位置 → Tools 索引"。本次新增 11 个页面，并把它们追加到工具列表尾部。若继续用索引：

- 插入页面会让已保存的排列被重新解释（用户的第 6 行会变成另一个工具）。
- 删除或重排页面会让越界索引出现，只能整体回退到自然顺序，用户排列丢失。

## 决策

1. 新增 `RailOrderIDs []string`，存稳定工具 ID（`ToolConvert` 等常量）。
2. 旧字段 `RailOrder []int` 保留为**只读迁移输入**，迁移表 `legacyToolIDs` 固定记录 v3.1.0 的 14 个工具顺序，永不修改。
3. 首次加载时：有 `RailOrderIDs` 就以其为准；否则用 `migrateRailOrderIDs(RailOrder)` 转换。
4. 任何一次持久化都会写 `RailOrderIDs` 并清空 `RailOrder`，避免旧索引在后续版本被重放覆盖新顺序。
5. 修复规则：未知 ID 丢弃、重复 ID 跳过、缺失工具按自然顺序追加。结果必须是当前工具集的排列。

## 理由

- 索引是位置，ID 是身份。用户的排列表达的是身份顺序，只有 ID 能跨版本表达这一点。
- 迁移表固定记录旧顺序，才能让"第 13 行原来是图片压缩"这类事实在升级后仍然成立。
- 清空旧字段是幂等性的关键：否则一个只在旧字段上有值的场景（例如手工编辑过设置文件）会在下次启动被再次迁移，覆盖用户后来的调整。

## 后果

- 新增页面必须提供 `Tool.ID()`。`TestEveryNewPageHasAStableID` 与 `TestEveryToolHasItsOwnTint` 固定"ID 唯一"与"每页一个色板条目"两条不变量。
- `TestLegacyRailOrderMigratesThroughTheIdentityTable` 覆盖旧索引→ID 的映射，`TestRailOrderRepairsAStaleFile` 覆盖未知/重复/过短三种坏文件。
- 拼图参数（`settings.Collage`）不参与本次迁移，保持原字段与往返语义（`TestRailOrderSurvivesRestart` 及现有拼图回归）。

## 未采纳的方案

| 方案 | 未采纳原因 |
|---|---|
| 直接删除旧字段 | 老用户的侧栏排列会静默丢失，违反 NFR-06 |
| 用工具名称字符串作 ID | 名称会变（本地化、重命名），且与界面文案耦合 |
| 迁移时按新列表重新对齐 | 会把用户排列重解释成另一个顺序，等于丢失设置 |

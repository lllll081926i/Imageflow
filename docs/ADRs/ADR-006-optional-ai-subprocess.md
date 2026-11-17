# ADR-006：AI 高清放大作为可选外部进程，不预装模型

- 状态：已采纳
- 日期：2026-11-17
- 关联：AI-01…06、Gate D、TECH-07

## 背景

PRD 把 AI 高清放大定为 P1 但**独立发布线**，并要求：

- 基础安装包的常规工具在无引擎、无模型时 100% 可用（E2E-13）。
- 引擎与模型必须分别审查许可证；未经批准的权重不得打包。
- 子进程必须用可信绝对路径与显式参数列表执行，不得拼接 shell 命令。
- 取消后不得残留推理进程与临时文件（E2E-14）。

Upscayl 的 Electron 主程序用 `child_process.spawn()` 调用独立 `upscayl-bin`；`upscayl/upscayl-ncnn` 是 AGPL-3.0，`xinntao/Real-ESRGAN-ncnn-vulkan` 是 MIT，但**权重**另有许可。

## 决策

新增 `internal/upscale` 作为唯一与推理引擎通信的包：

1. **默认路由**：Real-ESRGAN-ncnn-vulkan（MIT 引擎）。Upscayl fork 仅在单独合规审查后接入，且不作为同一命令行处理。
2. **执行方式**：`exec.CommandContext` + 显式参数切片 + 绝对路径；Windows 用 `HideWindow` + `CREATE_NEW_PROCESS_GROUP`，取消时 `taskkill /T /F` 清理整棵进程树；非 Windows 用进程组 `SIGKILL`。
3. **模型注册表**：每个模型记录名称、`.param`/`.bin`、原生倍率、用途、许可文本与 **SHA256**。发布构建中所有 SHA256 为空，因此模型一律显示"未登记哈希，不能启用"，即**不预装、不放行**。
4. **启动校验**：每次运行前重算目录哈希（按 `.param`、`.bin` 顺序），不匹配即拒绝加载。
5. **输出校验**：引擎退出码为 0 且输出文件存在且非空才算成功；否则按 `ENGINE_FAILED` / `GPU_UNAVAILABLE` / `VERIFY_FAILED` 区分。
6. **倍率语义**：`-s` 永远传模型原生倍率；用户请求的非原生目标由 Go 后处理缩放，并在结果 `Warnings` 中明示。
7. **隔离**：`upscale.Registry` 与引擎路径只由页面提供，核心工具不引用该包的任何运行时路径。

## 理由

- AGPL 组件以独立进程运行**不自动**消除许可证义务，因此把 AGPL 路径设为非默认，并让发布方在看到哈希与许可登记后才放行。
- 空哈希即不可运行，是把"未经审查的权重不得启用"从文档要求变成代码约束的唯一可靠做法。
- 倍率不映射到 `-s` 是常见错误来源：引擎只认模型原生倍率，传其它值会失败或静默使用自身值。

## 后果

- `TestDetectWithoutEngineReportsTheMissingComponent`、`TestRunRefusesAModelWithoutARecordedHash`、`TestRunRefusesATamperedModel`、`TestCancelIsReportedAsCancellation` 固定上述约束。
- AI 页面的状态卡分别显示"引擎可用/组件缺失"、模型安装与哈希状态、许可文本；设置页的"模型与组件"区同步显示同一目录。
- GPU/显存与画质对照仍为**待实测**：本 ADR 不提供任何"A/B 更快"的结论。

## 未采纳的方案

| 方案 | 未采纳原因 |
|---|---|
| 把 `.bin` 权重打进安装包 | 许可未审查，且违反"未批准权重不得预装" |
| 用 shell 命令字符串拼路径 | 用户文件名会成为命令参数，构成注入面 |
| 只杀父进程 | 启动器可能在独立进程组里起真正的推理进程，GPU 会一直被占用 |
| 把 Upscayl fork 当同一 CLI | `-w/-c` 等参数并不通用，混用会误传参数 |

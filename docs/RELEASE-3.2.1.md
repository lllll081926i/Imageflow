# ImageFlow 3.2.1 发布记录

## 发布范围

- 汇总 3.2.0 后的四轮体验优化、Upscayl 兼容组件、临时原图与缓存管理。
- 逐项优化过程见 `应用体验逐项审计与优化记录.md`；AI 功能边界见 `UPSCAYL-功能对照.md`。
- 版本配置与标签必须分别为 `3.2.1` 和 `v3.2.1`，保持现有更新签名公钥。

## 发布门禁

- 最终源码执行全量测试、Go/MyGo 静态检查和本地编译。
- 使用固定提交的本地组件执行真实 GPU 发布验收。
- GitHub Actions 执行六平台打包、签名更新包和 Windows 安装生命周期验收；全部通过后自动发布。
- 发布后检查下载文件、更新清单与 SHA256SUMS。

## 验证边界

- Linux、macOS 和 Windows ARM64 以云端构建结果为准，不代表已完成人工原生运行验证。
- macOS 产物未经过 Apple 开发者签名和公证。
- 云端构建通过不等同于各平台人工原生运行通过。

## 本地门禁结果（2026-11-17）

- `CGO_ENABLED=0 go test ./... -count=1 -timeout 10m` 全部通过，UI 包耗时 87.081 秒；日志 `build/release-3.2.1-tests.log`。
- `go vet ./...`、`go tool mygo vet .`、本地编译、gofmt 检查和 `git diff --check` 通过。
- `govulncheck ./...` 未发现漏洞；日志 `build/release-3.2.1-vuln.log`。
- 真实 GPU 发布验收通过：6400×3600 大图、4004×2668 奇数尺寸、透明与半透明输出；活动推理取消耗时 0.608 秒，既有目标保持不变。日志 `build/release-3.2.1-gpu.log`。
- 本轮未改动业务源码，沿用第四轮已通过的定向竞态检查；证据见体验优化台账。

## 云端发布结果

- 发布源码：`1d520b2`；标签：`v3.2.1`。标签保持不变。
- Actions：<https://github.com/lllll081926i/Imageflow/actions/runs/44069964063>，六平台构建及发布任务全部成功。
- Windows x64 全新安装、3.1.0 升级、模型保留、卸载及便携移动启动验收通过。
- 正式版本：<https://github.com/lllll081926i/Imageflow/releases/tag/v3.2.1>，发布时间为 2026-11-17 11:07:36 UTC，已标记为最新正式版本。
- 39 个发布文件已全部下载，文件大小及 SHA256 与 GitHub 摘要一致；38 项 `SHA256SUMS` 全部匹配。
- 六个平台更新清单的版本和大小正确；24 个完整更新包与增量包的 Ed25519 签名均使用应用现有公钥验证通过。
- 本地下载与核验文件保存在 `build/release-3.2.1-assets/`，未纳入版本管理。

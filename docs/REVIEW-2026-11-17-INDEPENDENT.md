# 新版应用独立审查（2026-11-17）

> 修复前历史记录：本报告的 17 项问题已进入修复回归；当前结果见 `docs/REPAIR-2026-11-17-INDEPENDENT.md`。下文保留发现时的行为与结论，不应作为修复后版本的实时状态。

## 结论

**暂不建议发布。** 当前工作区存在 17 项确认问题：10 项 P1、7 项 P2。问题覆盖标注输入、批处理状态、预览、调色、输出安全、AI 调度和图像计算。现有测试通过，不代表新增功能已完成端到端验收。

P1 表示发布前应修复的崩溃、竞态、数据安全或核心流程问题；P2 表示功能正确性或交互问题。编号表示本报告引用编号，不表示精确的修复顺序。

审查基于工作区实际源码，包含未跟踪的新文件。基准 HEAD 为 `848da456019f2cc4be4ef0ebedbce82643dd0b8f`。`mygo.json` 版本仍为 `3.1.0`，更新记录明确写着“未发布（v3.2.0 规划中）”。本报告不将这些增量称为已发布版本。

本轮只新增审查报告和独立复现测试资产，没有修改业务代码，没有提交、推送或发布。原有 `docs/REVIEW-2026-11-17.md` 保留不变。

## 验证范围与结果

已执行并通过：

| 检查 | 结果与边界 |
| --- | --- |
| `CGO_ENABLED=0 go test ./... -count=1 -timeout 15m` | 现有全量测试通过 |
| `go vet ./...` | 通过 |
| `go tool mygo vet .` | 通过；没有发现本报告复现的跨帧 Element 问题 |
| `CGO_ENABLED=1 GOMAXPROCS=4 go test -race ./... -count=1 -timeout 20m` | 现有全量测试通过；现有用例未触发新增调色竞态 |
| `CGO_ENABLED=0 go build ./...` | Windows amd64 编译通过 |
| 五个交叉目标 `go build ./...` | Windows arm64、Linux amd64/arm64、macOS amd64/arm64 编译通过 |
| `git diff --check` | 已跟踪文件差异检查通过；未跟踪文件不在该命令覆盖范围 |

另外新增 17 个顶层独立测试函数，逐组执行均发现失败，其中包含 MyGo Tester 的生命周期 panic 和 race detector 的 DATA RACE。一个测试函数可以包含多个子用例；**17 个失败测试不与 17 项问题一一对应**。取消、UI 同步 I/O 等问题另有静态调用链证据。

这不是六平台原生运行验收，也不是云端打包验收。没有验证真实 GPU、真实 AI 模型画质、AI 性能、全部平台的原生窗口交互或发布产物安装。

## P1：发布前修复

### 01 标注输入访问跨帧 Element

- 定位：`internal/ui/tools_studio.go:1043`、`internal/ui/tools_studio.go:1071`。
- 输入回调捕获画布 `myui.Element`，随后调用 `canvas.Bounds()`。Element 不能跨构建 pass 保存。
- 复现：`TestAuditAnnotationGesture` 使用真实 `ui.NewTester`，经过多帧后点击标注画布，直接触发 `ui: element from pass 7 used in pass 7; keep a ui.Handle instead`。
- 影响：Tester/开发构建已确认 panic。正式构建也违反生命周期契约，可能读取空几何或不响应，不应推断所有正式构建都会崩溃。
- 修复：绑定稳定 `ui.Handle`；在输入回调使用 `handle.Bounds(c)`，不捕获历史 Element。

### 02 同步工具和早退分支不释放全局运行状态

- 定位：`internal/ui/files.go:317`、`internal/ui/files.go:339`；`internal/ui/tools_studio.go:697`、`:1982`、`:2205`。
- `Start()` 先设 `Running=true`，再调用工具。图片对比、图像分析及自由布局的空图层分支没有完成运行状态清理。
- 复现：`TestAuditRunStateReleased` 的三个子用例中，工具已返回提示，`Running` 仍为 true。对比成功写报告的分支同样没有释放状态。
- 影响：后续开始操作被运行中检查挡住，正常使用无法恢复。
- 修复：统一运行状态的取得与释放；同步工具和所有早退路径必须完成状态转换。另应按工具需求检查输入，避免独立选图和纯文字画布被全局导入列表的非空检查挡住。

### 03 标准 `.cube` LUT 红蓝索引轴反转

- 定位：`internal/engine/lut.go:41`。
- 当前索引让蓝通道变化最快，而标准 `.cube` 数据顺序为红通道变化最快。
- 复现：`TestAuditStandardCubeIdentity` 读入外部标准顺序的 2³ 恒等 LUT，纯红输入输出为纯蓝。
- 影响：正常外部 LUT 可产生严重错误颜色。现有恒等 LUT 的生成和读取共用错误索引，原测试无法检出。
- 修复：修正数据布局，并使用独立外部格式 fixture 验证三轴，不能只依赖自生成恒等表。

### 04 调色 worker 读取可变 UI 参数，存在真实数据竞态

- 定位：`internal/ui/tools_studio.go:1348`、`internal/ui/tools_studio.go:1556`。
- 批处理 worker 调用 `t.params()`，直接读 UI 正在编辑的曝光等字段；运行期间控件仍可写这些字段。
- 复现：`TestAuditGradeRunSnapshot` 在批处理期间修改曝光；race detector 报告 `params()` 与参数写入之间的 DATA RACE。
- 影响：同一批图片可能采用不同参数，且存在未同步内存访问。
- 修复：在 UI 线程构造不可变参数快照，worker 只读快照。AI 的 worker 同样直接读取多个工具字段，应在同一轮修复中检查；本轮实际 race 复现针对调色。

### 05 多个新增工具没有接通处理后预览

- 定位：`internal/ui/app.go:1272`；`internal/ui/tools_studio.go:588`。
- 预览容器仅对实现 `PreviewPanel` 的工具使用专用预览，其余显示共享导入原图。基础编辑、调色、滤镜、截图美化、点阵和表情页没有接通相应处理后预览。自由布局虽有 `request()` 合成方法，但没有调用入口，也没有接通专用预览。
- 复现：`TestAuditEditPreviewChanges` 设置 90° 旋转后，多帧预览仍为 120×90，而非 90×120。`TestAuditCanvasPreviewConnected` 中已有可见形状层，但多帧后预览为空且未启动任务。
- 影响：用户无法按预览判断导出结果；“预览与导出使用同一渲染函数”不足以证明预览链路已经可用。
- 修复：接通共享异步预览，使用相同参数快照和渲染内核；用输入/参数版本拒绝过期结果，并取消旧任务。

### 06 截图美化分配画布前没有执行输出预算检查

- 定位：`internal/engine/annotate.go:430`、`internal/engine/annotate.go:577`。
- `RenderScreenshot` 计算留白和画布比例后直接分配画布，没有调用统一 `output.CheckSize`；外层导出也没有补充检查。
- 复现：`TestAuditScreenshotBudget` 传入合法的 16000×1 图片和 1 像素留白，成功生成 16002×3 的超限画布。
- 影响：绕过单边 16000/总像素 6400 万的统一限制，较大留白或比例设置可导致过量内存分配。本轮没有通过制造 OOM 验证。
- 修复：在最终尺寸和所有中间画布尺寸确定后、内存分配前检查预算，并处理尺寸运算溢出。

### 07 AI 模型名称参数不符合外部引擎协议

- 定位：`internal/upscale/upscale.go:333`。
- 将带 `.param` 扩展名的 `model.ParamFile` 传给 Real-ESRGAN 的 `-n`。官方实现会自行拼接 `.param` 和 `.bin`，导致查找 `.param.param` 等错误路径。
- 复现：`TestAuditUpscaleModelName` 使用协议校验辅助进程，拒绝当前参数。另已核对官方 `src/main.cpp` 的文件名拼接逻辑。
- 影响：即使引擎、模型和哈希齐全，正常推理仍会失败。
- 修复：区分引擎模型标识与文件名；专项适配 anime-video 的倍率文件命名，不应仅去掉扩展名后就宣称所有模型兼容。
- 协议参考：https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan/blob/master/src/main.cpp

### 08 AI 直接写正式目标，失败仍破坏旧文件

- 定位：`internal/upscale/upscale.go:331`。
- `-o` 直接指向 `req.OutputPath`，没有使用统一的临时输出与原子提交机制。
- 复现：`TestAuditUpscaleFailurePreservesTarget` 先创建旧目标，辅助进程写入部分内容后以状态 9 失败；`Run` 返回错误，但旧目标已变成 `partial output`。
- 影响：引擎失败或取消可能破坏原有文件。
- 修复：在目标目录生成临时输出；验证成功后按冲突策略原子提交；失败和取消只清理本任务临时文件。

### 09 GUI 没有把批次取消传给 AI/多帧表情任务

- 定位：`internal/ui/tools_studio.go:2584`、`internal/ui/tools_studio.go:2753`；`main.go:89`。
- 两条长任务调用链使用 `context.Background()`。GUI 停止操作仅取消 Session token，无法取消当前传入的 context；关闭路径也未将新增 AI 工作纳入清理。
- 证据：静态调用链确认。AI 底层具备进程取消与进程树清理代码，但 GUI 没有触发该 context 的取消。没有进行真实 GPU 取消实测。
- 影响：停止批次只能阻止后续取任务，当前推理仍可持续到超时；窗口退出也缺少统一生命周期管理。
- 修复：将批次 token 桥接为任务 context；停止和关闭窗口均取消该 context，并等待必要清理。多帧处理在帧间检查取消。

### 10 AI 把非空损坏文件当作成功输出

- 定位：`internal/upscale/upscale.go:375`。
- 成功检查仅验证文件存在且非空，没有解码、实际格式、尺寸、倍率或预算校验；报告宽高也未填充。
- 复现：`TestAuditUpscaleRejectsCorruptOutput` 的辅助进程写入 `not an image` 并成功退出，`Run` 仍返回成功报告。
- 影响：界面可对损坏或错误尺寸的输出报告成功。`docs/roadmap_issue_plan.md` 已将 AI-05b 列为未完成；这是已知缺口在可执行路径上的具体风险，不是全部交付完成的状态。
- 修复：解码并核验输出；填充真实报告；不通过校验的临时文件不得提交到正式目标。

## P2：功能正确性与交互

### 11 AI 非原生倍率只警告，不执行承诺的后处理

- 定位：`internal/upscale/upscale.go:382` 及其后倍率处理分支。
- 复现：`TestAuditUpscaleRequestedScale` 用 2×2 输入和原生 4× 模型，请求 2×；最终仍为 8×8，`AppliedScale=4`，不是预期 4×4。
- 修复：实现推理后缩放和最终尺寸校验；若暂不支持，则禁用非原生倍率并修改界面说明。

### 12 渲染路径同步完整解码，AI 探测也阻塞 UI

- 定位：`internal/ui/tools_studio.go:1962`、`:2095`、`:2626`、`:2665`；`internal/ui/pickers_studio.go:36`。
- 对比图块和分析面板在视图构建时调用 `loadPreviewImage`，该函数实际完整解码文件；AI 面板/点击回调同步探测引擎并校验模型。探测子进程有最长 8 秒的等待路径。
- 证据：静态调用链确认；未测量实际卡顿时长，不将 8 秒上限当作每次实测耗时。
- 修复：缓存缩略图；将完整解码、哈希及探测移到后台；通过 `Window.Update` 发布结果并校验任务版本。

### 13 点阵色数按每通道计算，灰阶错误使用红通道

- 定位：`internal/engine/pixel.go:267`、`internal/engine/pixel.go:340`。
- 复现：`TestAuditFourColorLimit` 请求 4 色，输出实际有 64 色。`TestAuditGrayUsesLuminance` 将纯绿色转为 4 级灰阶，结果是黑色。
- 原因：彩色模式独立量化 RGB 三个通道，色数上限为所选等级的三次方；多级灰阶直接复制量化后的红通道，未计算亮度。
- 修复：采用总大小受限的调色板量化；灰阶先计算亮度，再量化和抖动。

### 14 基础编辑的 0% 不透明度没有生效

- 定位：`internal/engine/edit.go:172`。
- 复现：`TestAuditZeroOpacity` 输入不透明红像素，设置 `Opacity=0`，输出 alpha 仍为 65535。
- 原因：透明度处理仅在 `opacity > 0 && opacity < 100` 时执行。
- 修复：定义明确的 0–100 参数契约；0 必须清除 alpha，默认 100 应由初始化而非数值猜测实现。

### 15 多个工具每帧把合法零值重置为默认值

- 定位：`internal/ui/tools_studio.go:1186`、`:1328`、`:2247`、`:2442`、`:2612`。
- 复现：`TestAuditZeroDefaultsPersist` 确认截图留白/圆角/角度、点阵强度/阈值、GPU 0 号在再次 `ensure()` 后被重置。
- 其他同类静态路径包括调色曲线中点/LUT 强度、表情描边；清空表情字幕也会重新填默认文字。
- 影响：零留白、0% 强度、0 号 GPU 等合法设置无法稳定保留。
- 修复：采用一次性初始化标记，区分“尚未初始化”和“用户选择零值”；同时验证撤销/重做和配置恢复。

### 16 Fit 对齐将补边纳入指标，违反仅比较重叠区的口径

- 定位：`internal/engine/compare.go:95`、`internal/engine/compare.go:315`。
- 复现：`TestAuditFitComparesOverlap` 比较全白 4×4 与全白 4×2，真实重叠区完全一致，却得到 PSNR 约 3.0103 dB，而不是正无穷。
- 原因：Fit 补透明黑边后仍遍历全画布 RGB，补边影响结果，alpha 没有参与有效区域判断。
- 修复：按有效区域掩码比较，或明确改用统一背景策略并同步指标口径；不要把补边差异解释为图像质量差异。

### 17 对比报告忽略全局冲突策略并覆盖固定文件

- 定位：`internal/ui/tools_studio.go:1994` 及后续 `os.WriteFile`。
- 复现：`TestAuditCompareReportHonorsSkip` 设置 `ConflictSkip`，已有 `compare-report.txt` 仍被覆盖。
- 修复：接入统一目标预留、Skip/Rename/Overwrite 策略和原子提交机制，报告实际保存位置及跳过结果。

## 交付完成度与发布门槛

除上述 17 项外，AI 默认注册表的所有模型 SHA256 仍为空，`Run` 明确拒绝空哈希；`SetRegistry` 仅被测试调用，产品没有提供录入、导入和持久化可信哈希的完整路线。定位见 `internal/upscale/upscale.go:308`、`internal/upscale/upscale.go:410`、`internal/ui/settings.go:383`。因此当前 AI 页不是“装好引擎与权重即可使用”的完整功能。**不能通过删掉校验来绕过此问题**；需要完成可核验来源的模型清单或可信注册流程。

计划文件还明确保留目标体积、接缝优化、水印变量、GIF 时间轴、拼图换序、AI 输出校验等未完成项，以及 GPU/性能/六平台原生验收。应把这些内容分为本版本必须完成与明确延期，不应统一标记为已经全部实施。

建议先修复文件安全、生命周期、全局运行状态与参数快照；再修复计算结果和预览链路；最后核对需求矩阵与发布说明。修复后将独立用例整理为常规回归测试，并补充真实外部引擎和原生界面验证。

## 独立复现资产与命令

资产目录为 `docs/reviews/2026-11-17-independent/`。三个 `.go.txt` 文件通过 Go overlay 注入对应包，不参与默认 `go test ./...`。`overlay.json` 使用当前机器的绝对路径；在其他目录复现前需要调整映射。

AI 用例将测试可执行文件作为辅助子进程，并登记临时模型文件的真实哈希，用于验证外部进程协议和输出提交契约；**不使用真实 GPU 或权重，不验证 AI 画质与性能**。

分组运行以避免生命周期 panic 中断其他复现：

```powershell
$env:CGO_ENABLED = '0'
go test -overlay 'docs/reviews/2026-11-17-independent/overlay.json' ./internal/engine ./internal/ui -run '^TestAudit(Standard|Zero|Screenshot|Four|Gray|Fit|RunState|EditPreview|CanvasPreview)' -count=1 -v -timeout 2m
go test -overlay 'docs/reviews/2026-11-17-independent/overlay.json' ./internal/ui -run '^TestAuditAnnotationGesture$' -count=1 -v -timeout 1m
go test -overlay 'docs/reviews/2026-11-17-independent/overlay.json' ./internal/ui -run '^TestAudit(ZeroDefaults|CompareReport)' -count=1 -v -timeout 1m
go test -overlay 'docs/reviews/2026-11-17-independent/overlay.json' ./internal/upscale -run '^TestAuditUpscale' -count=1 -v -timeout 2m
$env:CGO_ENABLED = '1'
$env:GOMAXPROCS = '4'
go test -race -overlay 'docs/reviews/2026-11-17-independent/overlay.json' ./internal/ui -run '^TestAuditGradeRunSnapshot$' -count=1 -v -timeout 2m
```

上述命令用于复现缺陷，当前预期返回非零退出码。全量原测试通过与这些独立复现失败并不矛盾：现有回归覆盖尚未覆盖这些输入和生命周期。

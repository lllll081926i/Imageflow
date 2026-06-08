# ImageFlow Backend Architecture

## 概览

ImageFlow 当前采用 **React 前端 + Python 后端 + pywebview 桌面宿主**。后端不再包含 Go/Wails 层；前端通过 pywebview 的 `js_api` 调用 `backend.api.DesktopAPI`，后端再由应用层调度 `backend/engines/` 中的图像处理引擎。

核心目标：

- 保持桌面 API 调用面稳定，兼容前端现有绑定。
- 将窗口宿主、API 编排、路径规则、设置存储、图像引擎和发布构建分层。
- 对批量任务使用受控并发，避免大图处理时过度占用 CPU 和内存。
- 所有 Python 依赖由 `uv` 管理，确保开发、测试、发布使用一致环境。

## 架构图

```text
┌─────────────────────────────────────────────────────────────┐
│                   React + TypeScript Frontend                │
│  components / runtime / types / Vite build output            │
└───────────────────────────────┬─────────────────────────────┘
                                │ pywebview js_api
┌───────────────────────────────▼─────────────────────────────┐
│                         pywebview Host                       │
│  backend/main.py                                              │
│  backend/host/window.py                                       │
│  - create_window                                              │
│  - frontend entry resolution                                  │
│  - file drop dispatch                                         │
└───────────────────────────────┬─────────────────────────────┘
                                │ method calls
┌───────────────────────────────▼─────────────────────────────┐
│                         Desktop API                          │
│  backend/api/desktop_api.py                                   │
│  - settings                                                   │
│  - dialogs                                                    │
│  - path expansion and output path resolving                   │
│  - operation methods exposed to frontend                      │
└───────────────────────────────┬─────────────────────────────┘
                                │ orchestration
┌───────────────────────────────▼─────────────────────────────┐
│                       Application Layer                      │
│  backend/application/task_manager.py                          │
│  backend/application/image_ops.py                             │
│  backend/application/preview.py                               │
│  - cancellation state                                         │
│  - multiprocessing worker lifecycle                           │
│  - preview generation                                         │
└───────────────────────────────┬─────────────────────────────┘
                                │ allow-listed engine loading
┌───────────────────────────────▼─────────────────────────────┐
│                      Image Processing Engines                │
│  backend/engines/*.py                                         │
│  - converter / compressor / gif_splitter                      │
│  - pdf_generator / watermark / adjuster / filter              │
│  - metadata_tool / info_viewer / subtitle_stitcher            │
└───────────────────────────────┬─────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────┐
│                Pillow / ReportLab / piexif / exifread        │
│                         Local File System                    │
└─────────────────────────────────────────────────────────────┘
```

## 分层职责

### 宿主层

`backend/main.py` 是桌面入口：

- 调用 `multiprocessing.freeze_support()` 支持 Windows 打包运行。
- 创建 frameless pywebview 窗口。
- 通过 `build_window_api()` 将 `DesktopAPI` 暴露给前端。
- 调用 `configure_window()` 注册拖拽和窗口状态事件。

`backend/host/window.py` 负责：

- 解析前端入口：优先使用显式 URL 或 `IMAGEFLOW_FRONTEND_URL`，否则回退到 `frontend/dist/index.html`。
- 注册 pywebview DOM 拖拽事件，将本地文件路径转发为 `__imageflow_file_drop__` 事件。
- 同步窗口最大化、恢复、最小化状态。

### API 层

`backend/api/desktop_api.py` 是前端调用面的稳定边界。它负责：

- 将前端 payload 中的路径字段标准化。
- 调用设置存储、系统对话框、字体列表和预览生成。
- 为转换、压缩、PDF、GIF、水印、调整、滤镜、元数据等操作选择对应引擎。
- 对批处理调用 `execute_engine_batch()`，对单文件操作调用 `execute_engine()`。
- 提供 PascalCase 方法，兼容历史 Wails 风格绑定名称。

### 应用层

`TaskManager` 保存任务 ID、取消标记、当前任务和已挂载子进程。取消逻辑会先 `terminate()`，等待超时后再 `kill()`。

`image_ops.py` 管理 multiprocessing 生命周期：

- 单任务：每次启动一个子进程，结果通过 `Queue` 返回。
- 批处理：按 `max_concurrency` 启动有限数量 worker，从任务队列取 payload，按原始索引回填结果。
- 所有队列都在 `finally` 中关闭并 `join_thread()`，避免文件描述符和 feeder 线程泄漏。
- worker 启动失败时清理已启动进程，避免半启动状态残留。

`preview.py` 生成前端预览图：

- 使用 `IMAGEFLOW_PREVIEW_MAX_BYTES` 限制预览输入大小。
- 将大图缩略到 `1280` 边以内并输出 JPEG data URL。
- 通过引擎加载器复用 SVG 打开逻辑。

### 领域与基础设施

`backend/domain/paths.py` 集中处理：

- 支持的输入扩展名。
- 用户路径基础校验和标准化。
- 目录拖拽展开。
- 输出路径冲突重命名。
- Windows 系统字体列表。

`backend/infrastructure/settings_store.py` 负责：

- 从系统用户配置目录读取 `imageflow/settings.json`。
- 支持测试或特殊环境通过 `IMAGEFLOW_SETTINGS_FILE` 覆盖设置路径。
- 保存前规范化设置字段。
- 使用临时文件加 `os.replace()` 原子写入，降低设置文件损坏风险。

`backend/infrastructure/engine_loader.py` 负责：

- 维护允许加载的引擎白名单。
- 从 `backend/engines/<module>.py` 精确文件路径加载模块，避免被 `sys.path` 中同名模块遮蔽。
- 校验引擎存在 `process()` 且返回 `dict`。

## 数据流

单文件转换的典型链路：

```text
用户在前端选择文件
  ↓
DesktopAPI.Convert(payload)
  ↓
_normalize_payload_paths(payload)
  ↓
_run_operation(lambda: execute_engine("converter", payload, task_manager))
  ↓
multiprocessing.Process(target=_process_worker)
  ↓
invoke_engine_process("converter", payload)
  ↓
backend/engines/converter.py::process()
  ↓
Pillow / SVG 渲染后端 / 本地文件写入
  ↓
Queue 返回 {"success": true, "output_path": "..."}
```

批处理转换的典型链路：

```text
DesktopAPI.ConvertBatch(payloads)
  ↓
读取 AppSettings.max_concurrency
  ↓
execute_engine_batch("converter", payloads, settings, task_manager)
  ↓
创建 task_queue / result_queue
  ↓
启动 N 个 worker 进程
  ↓
worker 重复处理队列任务并回填原始 index
  ↓
按输入顺序返回结果列表
```

## 错误处理约定

后端对前端返回的数据以字典为主：

```json
{
  "success": true,
  "output_path": "D:/Images/out.png"
}
```

失败时：

```json
{
  "success": false,
  "error": "[BAD_INPUT] 路径不能为空"
}
```

约定：

- 入口 API 尽量捕获异常并返回 `success: false`。
- 引擎内部使用带前缀的错误码表达可识别错误，例如 `[BAD_INPUT]`、`[NOT_FOUND]`、`[UNSUPPORTED_FORMAT]`、`[PY_CANCELLED]`。
- worker 默认不返回 traceback；需要调试时可设置 `IMAGEFLOW_DEBUG_TRACEBACK=1`。

## 并发与性能

- 默认并发数为 `8`，保存设置时限制在 `1-32`。
- 单文件操作独立子进程执行，避免图像库异常影响宿主进程。
- 批处理复用固定数量 worker，减少一次一个进程的启动成本。
- 预览生成有文件大小阈值和缩略尺寸上限，避免前端渲染大图时内存过高。
- SVG 转位图优先尝试 CairoSVG，其次 svglib/reportlab，最后尝试系统 Inkscape。

## 安全边界

当前安全策略：

- 引擎加载使用白名单，并按精确文件路径加载。
- 用户路径拒绝空路径、空字符和以 `..` 开头的相对父级跳转。
- pywebview 文件拖拽 payload 使用 JSON 序列化后分发给前端。
- 设置保存使用原子替换，降低中断写入导致的配置损坏。
- SVG 内在尺寸解析遇到 `DOCTYPE` 或 `ENTITY` 时回退到根标签属性解析，避免直接解析不安全声明。

仍需持续关注：

- 绝对路径目前按桌面工具语义允许访问本机文件；如未来需要沙箱，应在 `domain/paths.py` 增加统一允许目录策略。
- 部分文档和历史类型仍保留 Wails 兼容命名，这是前端兼容面，不代表存在 Go 后端。

## 发布架构

发布构建由根目录 `package.json` 脚本驱动：

```bash
npm run build:frontend
npm run build:portable
npm run build:installer
npm run build:release
```

`backend/packaging/release_builder.py` 执行：

1. 校验 `frontend/dist/index.html` 存在且不是旧构建。
2. 使用 PyInstaller 从 `backend/main.py` 构建 onedir 应用。
3. 将 `frontend/dist` 和 `ico.png` 加入发布包。
4. 收集后端运行时包和 pywebview 子模块。
5. 便携版通过 zip 输出。
6. 安装版通过 Inno Setup 6 输出。

## 测试策略

当前测试命令：

```bash
uv run python -m unittest discover -s backend/tests -v
uv run python -m unittest discover -s backend/tests/engines -v
npm --prefix frontend run test
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

覆盖重点：

- `backend/tests/test_desktop_api.py`：前端 API 行为、路径解析、设置、预览和取消竞态。
- `backend/tests/test_task_manager.py`：任务状态、取消、子进程清理和队列关闭。
- `backend/tests/test_engine_bridge.py`：API 到引擎的真实桥接、批处理顺序、引擎加载安全。
- `backend/tests/engines/`：各图像处理引擎的格式、边界和回归行为。
- `frontend/**/*.test.ts`：前端运行时兼容层、GIF 辅助逻辑和错误映射。

## 扩展现有能力的约束

当前维护目标是不新增功能。修复或优化现有能力时应遵守：

- 优先补充失败测试，再修改实现。
- 不改变前端已使用的 DesktopAPI 方法名和 payload 字段，除非同步修改兼容层和测试。
- 不绕过 `engine_loader.py` 加载引擎。
- 图像处理引擎新增共享逻辑时优先抽到现有模块内的窄函数，避免引入大范围架构重排。
- 修改发布流程时同步更新 `package.json`、`README.md` 和本文件中的命令说明。

# ImageFlow Backend

ImageFlow 当前后端是 **Python + pywebview** 桌面宿主，不再使用 Go/Wails 后端。Python 代码同时负责窗口宿主、前端 API、任务编排、进程管理和图像处理引擎调用；依赖由根目录 `pyproject.toml` 和 `uv.lock` 管理。

## 当前能力

- 格式转换：JPG、PNG、WEBP、AVIF、TIFF、BMP、ICO 等输出。
- 图片压缩：多档压缩、目标体积、元数据剥离。
- PDF 生成：多图合并、页面尺寸、方向、边距和布局。
- GIF 工具：拆帧、倒放、变速、压缩、格式互转。
- 信息查看：基础信息、EXIF、直方图和多格式元数据解析。
- 元数据处理：EXIF 编辑和隐私清理。
- 图片水印：文字/图片水印、九宫格定位、平铺、混合与阴影。
- 图片调整与滤镜：旋转、翻转、裁剪、色彩调整和预设滤镜。

## 运行要求

- Python `>=3.10`
- uv
- Node.js LTS
- Windows 桌面运行时依赖 pywebview；发布构建依赖 PyInstaller，安装包构建依赖 Inno Setup 6。

Python 依赖以 `uv sync` 安装，不使用 `backend/go.mod` 或 `python/requirements.txt`。

## 目录职责

```text
backend/
├── main.py                  # pywebview 桌面入口
├── app.py                   # 创建 DesktopAPI 实例
├── api/desktop_api.py       # 暴露给前端的桌面 API
├── application/             # 任务管理、进程调度、预览编排
├── contracts/               # 设置等共享数据结构
├── domain/                  # 路径、扩展名和输出命名规则
├── engines/                 # 图像处理引擎
├── host/                    # 窗口创建、拖拽桥接、前端入口解析
├── infrastructure/          # 设置存储、对话框、引擎加载、窗口操作
├── packaging/               # PyInstaller / Inno 发布构建
└── tests/                   # 后端与引擎回归测试
```

## 调用链

```text
React UI
  ↓
frontend/runtime/desktopRuntime.ts
  ↓
pywebview js_api
  ↓
backend/api/DesktopAPI
  ↓
backend/application/image_ops.py
  ↓
backend/infrastructure/engine_loader.py
  ↓
backend/engines/*.py
  ↓
Pillow / ReportLab / piexif / exifread / 文件系统
```

## 开发命令

```bash
# 安装 Python 依赖
uv sync

# 安装前端依赖
npm --prefix frontend install

# 同时启动 Vite 和 pywebview 宿主
npm run dev

# 仅运行发布态后端入口，需先构建前端 dist
npm run build:frontend
uv run python -m backend.main
```

`npm run dev` 会启动前端 Vite 服务，并通过 `IMAGEFLOW_FRONTEND_URL` 把开发服务器地址传给 Python 宿主。`scripts/dev.mjs` 会监听 `backend/**/*.py` 的宿主层变更并重启后端。

## 测试与检查

```bash
# 后端主测试
uv run python -m unittest discover -s backend/tests -v

# 引擎专项回归测试
uv run python -m unittest discover -s backend/tests/engines -v

# 前端单元测试
npm --prefix frontend run test

# 前端类型检查
npm --prefix frontend run typecheck

# 前端生产构建
npm --prefix frontend run build
```

## 构建发布

```bash
# 同步运行与打包依赖
uv sync --group build

# 构建前端
npm run build:frontend

# 构建便携版
npm run build:portable

# 构建安装版
npm run build:installer

# 同时构建便携版和安装版
npm run build:release
```

发布脚本在 `backend/packaging/release_builder.py` 中：

- 使用 PyInstaller 从 `backend/main.py` 构建 Windows 桌面程序。
- 将 `frontend/dist` 作为 `frontend/dist` 数据目录打包。
- 收集 `backend.api`、`backend.application`、`backend.contracts`、`backend.domain`、`backend.engines`、`backend.host`、`backend.infrastructure` 等运行时包。
- 安装包由 Inno Setup 6 生成；`ISCC.exe` 不在 `PATH` 时可设置 `INNO_SETUP_ISCC`。

## 并发与取消

- `TaskManager` 跟踪当前任务、取消状态和已挂载子进程。
- 单任务通过 `execute_engine()` 启动独立子进程执行引擎。
- 批处理通过 `execute_engine_batch()` 根据 `settings.max_concurrency` 启动有限数量 worker。
- 取消任务时先 `terminate()`，超时后 `kill()`，并在 `finally` 中关闭 multiprocessing 队列。

## 配置

设置结构定义在 `backend/contracts/settings.py`，默认值包括：

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `max_concurrency` | `8` | 批处理并发数，保存时限制在 `1-32` |
| `output_prefix` | `IF` | 输出文件前缀 |
| `output_template` | `{prefix}{basename}` | 输出命名模板 |
| `preserve_folder_structure` | `true` | 目录拖拽时保留相对层级 |
| `conflict_strategy` | `rename` | 冲突时自动重命名 |
| `default_output_dir` | 空 | 默认输出目录 |
| `recent_input_dirs` | `[]` | 最近输入目录，最多 4 个 |
| `recent_output_dirs` | `[]` | 最近输出目录，最多 4 个 |

设置文件默认写入系统用户配置目录下的 `imageflow/settings.json`。测试或特殊环境可通过 `IMAGEFLOW_SETTINGS_FILE` 指定路径。

## 故障排查

### 启动后界面空白

先确认前端构建存在：

```bash
npm run build:frontend
uv run python -m backend.main
```

开发模式下确认 `npm run dev` 输出的 Vite 地址能访问，并且 `IMAGEFLOW_FRONTEND_URL` 是 `http://` 或 `https://` URL。

### Python 依赖缺失

```bash
uv sync
uv run python -m unittest discover -s backend/tests -v
```

### 打包失败

- 先运行 `uv sync --group build`。
- 确认 `frontend/dist/index.html` 是最新构建产物。
- 构建安装版前确认 Inno Setup 6 可用，或设置 `INNO_SETUP_ISCC`。

### 图片处理失败

- 查看 API 返回的 `success` 和 `error` 字段。
- 对应引擎位于 `backend/engines/`，优先补充 `backend/tests/engines/` 回归测试后再修复。
- 对 SVG 支持依赖当前 Python 环境中的 CairoSVG、svglib/reportlab 或 Inkscape 可用性。

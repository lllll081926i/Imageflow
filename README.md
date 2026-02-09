# ImageFlow

ImageFlow 是一款桌面端图像处理应用，聚合常用的格式转换、压缩、滤镜、批量处理、GIF 处理、PDF 生成与元数据管理能力。项目采用 **Wails + Go + React + Python** 的混合架构：由 Go 负责调度与并发，Python 负责高质量图像处理，前端提供现代化可视化操作界面。

---

## 亮点功能

- **格式转换**：JPG / PNG / WEBP / AVIF / TIFF / BMP / ICO 等格式互转
- **图像压缩**：无损/有损/智能压缩；支持 MozJPEG、PNGQuant、OxiPNG、Pillow 等引擎
- **批量处理**：多文件并发执行，进度可视化
- **GIF 工具**：拆帧、反转、变速、重组
- **PDF 生成**：多图合并 PDF，支持页面大小、方向与边距
- **水印**：文字/图片水印、平铺、透明度、混合模式、阴影
- **调色与修图**：亮度、对比度、饱和度、色相、锐化、翻转、旋转、裁剪
- **元数据**：查看/编辑/剥离 EXIF 与图片信息

---

## 技术栈

- **前端**：React 19 + Vite 6 + TypeScript + Tailwind CSS 4
- **后端**：Go 1.24 + Wails v2
- **图像处理**：Python 3.10+（Pillow / ReportLab / svglib / piexif 等）

---

## 架构概览

```
React UI
   │ (Wails JS Bridge)
Go Backend (services + task pool)
   │ (JSON over stdin/stdout)
Python Worker (scripts)
   │ (Pillow / ReportLab / svglib ...)
Image I/O
```

Go 负责并发、任务编排与错误处理，Python 负责真实图像处理逻辑。两者以 JSON 进行通信，便于调试与扩展。

---

## 目录结构

```
.
├── frontend/                 # React + Vite 前端
│   ├── components/           # UI 组件
│   ├── wailsjs/               # Wails JS 绑定
│   └── ...
├── backend/                  # Go + Wails 后端
│   ├── services/             # 核心业务服务
│   ├── models/               # 请求/响应模型
│   ├── utils/                # 工具与运行时支持
│   └── main.go               # Wails 入口
├── backend/python/           # Python 图像处理脚本
│   ├── converter.py
│   ├── compressor.py
│   ├── pdf_generator.py
│   ├── gif_splitter.py
│   ├── watermark.py
│   ├── info_viewer.py
│   └── worker.py             # Python Worker
└── docs/                     # 需求与架构文档
```

---

## 快速开始

### 1) 安装依赖

```bash
# 前端依赖
cd frontend
npm install

# Python 依赖（推荐 uv）
cd ..
uv sync

# Go 依赖
cd backend
go mod download
```

### 2) 开发模式

```bash
# 在项目根目录启动
wails dev
```

### 3) 构建发布

```bash
# 构建前端产物（输出到 backend/frontend/dist）
cd frontend
npm run build

# 构建桌面应用
cd ..
wails build
```

> 若直接运行 `wails build`，请确保前端产物已生成（输出目录由 `frontend/vite.config.ts` 指定）。

---

## 配置与环境变量

### 环境变量

- `IMAGEFLOW_PYTHON_EXE`：指定 Python 解释器路径（必须为 Python 3）
- `IMAGEFLOW_SCRIPTS_DIR`：指定 Python 脚本目录（默认自动探测）
- `IMAGEFLOW_FILE_LOG=1`：启用文件日志（输出到 `logs/`）
- `IMAGEFLOW_PREVIEW_MAX_BYTES`：大图预览阈值（字节）

### 全局设置（UI）

- `max_concurrency`：并发上限（1-32）
- `output_template`：输出命名模板
- `output_prefix`：输出默认前缀
- `preserve_folder_structure`：保持原目录结构
- `conflict_strategy`：冲突处理策略（当前为 `rename`）

配置文件保存在用户目录的 `imageflow/settings.json`。

---

## 输出命名模板

`output_template` 支持以下占位符：

- `{prefix}`：前缀
- `{basename}`：原文件名（不含扩展名）
- `{op}`：操作类型（convert / compress / watermark 等）
- `{date:YYYYMMDD}`：日期（格式可自定义）
- `{time:HHmmss}`：时间（格式可自定义）
- `{seq:3}`：序号（可指定补零位数）

示例：

```
{prefix}{basename}_{op}_{date:YYYYMMDD}_{seq:3}
```

---

## 测试

```bash
# Python 测试
python -m unittest discover -s backend/python/tests

# Go 测试
cd backend
go test ./...
```

---

## 常见问题

### 1) 提示找不到 Python
- 设置 `IMAGEFLOW_PYTHON_EXE` 指向 Python 3
- 或先执行 `uv sync` 创建 `.venv`

### 2) 提示找不到脚本目录
- 设置 `IMAGEFLOW_SCRIPTS_DIR` 指向 `backend/python/`

### 3) 打包后资源缺失
- 先执行 `npm run build` 生成 `backend/frontend/dist`

---

## License

本项目使用 **MIT License（完全开放、宽松许可）**。详见 `LICENSE`。

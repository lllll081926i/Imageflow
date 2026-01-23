# ImageFlow 图像处理应用

## 项目简介

ImageFlow 是一个功能强大的图像处理应用，采用现代化技术栈构建。该项目集成了多种图像处理功能，包括格式转换、压缩、调整大小、滤镜应用、PDF生成、GIF分割、水印添加等。

## 技术架构

本项目采用前后端分离架构：
- **前端**: 使用 React + TypeScript 构建，配合 Vite 构建工具
- **后端**: 使用 Go 语言开发，基于 Wails 框架实现桌面应用
- **图像处理引擎**: Python 脚本处理复杂的图像操作
- **UI框架**: TailwindCSS 样式框架

## 功能特性

- 🖼️ **图像格式转换**: 支持多种常见图像格式之间的相互转换
- 📉 **图像压缩**: 高效的图像压缩算法，保持图像质量的同时减小文件体积
- 🛠️ **图像调整**: 尺寸调整、旋转、亮度对比度调节等功能
- 🎨 **滤镜效果**: 多种滤镜效果供用户选择
- 📄 **PDF生成**: 将图像转换为PDF文档
- ✂️ **GIF分割**: GIF动画分割成单帧图像
- 💧 **水印添加**: 支持文字和图片水印功能
- 👁️ **信息查看**: 显示图像详细信息（尺寸、格式、大小等）

## 项目结构

```
.
├── backend/            # Go后端服务
│   ├── cmd/            # 主程序入口
│   ├── models/         # 数据模型
│   ├── services/       # 业务逻辑服务
│   ├── utils/          # 工具函数
│   └── app.go          # 应用主文件
├── frontend/           # 前端界面
│   ├── components/     # React组件
│   ├── public/         # 静态资源
│   ├── wailsjs/        # Wails JS绑定
│   └── ...             # 其他前端文件
├── python/             # Python图像处理脚本
│   ├── converter.py    # 格式转换
│   ├── compressor.py   # 图像压缩
│   ├── adjuster.py     # 图像调整
│   ├── filter.py       # 滤镜效果
│   ├── pdf_generator.py # PDF生成
│   ├── gif_splitter.py # GIF分割
│   ├── watermark.py    # 水印功能
│   └── ...             # 其他处理脚本
└── docs/               # 项目文档
```

## 安装和运行

### 系统要求

- Node.js >= 16.x
- Go >= 1.19
- Python >= 3.8
- Wails CLI

### Python 依赖（开发环境推荐 uv）

本项目的 Python 依赖通过 `uv` 管理，虚拟环境默认在项目根目录的 `.venv/` 中（依赖随项目一起管理，避免 conda run 并发临时文件冲突）。

1. 安装 uv（任选其一）：
   - Windows: `winget install Astral.uv`
   - macOS/Linux: 参考 uv 官方安装方式

2. 在项目根目录创建并同步依赖：
   ```bash
   uv sync
   ```

3. 运行开发模式：
   ```bash
   wails dev
   ```

后端会自动优先探测并使用 `.venv` 中的 Python（也可以用 `IMAGEFLOW_PYTHON_EXE` 手动指定）。

### 快速开始

1. 克隆项目：
   ```bash
   git clone https://github.com/lllll081926i/image-flow.git
   cd image-flow
   ```

2. 安装Wails CLI：
   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```

3. 安装前端依赖：
   ```bash
   cd frontend
   npm install
   ```

4. 返回项目根目录并运行：
   ```bash
   cd ..
   wails dev
   ```

## 开发说明

- 前端使用 React + TypeScript + TailwindCSS
- 后端使用 Go 语言和 Wails 框架
- 图像处理核心功能由 Python 脚本实现
- 前后端通过 Wails 绑定进行通信

## 部署

要构建生产版本，请运行：
```bash
wails build
```

这将在 `./build` 目录下生成可执行文件。

## 许可证

本项目采用 MIT 许可证 - 详情请参阅 [LICENSE](./LICENSE) 文件。


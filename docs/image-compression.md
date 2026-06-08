# 图片压缩模块说明

## 概览

图片压缩功能由当前 Python 后端直接提供，前端通过 pywebview 暴露的 `DesktopAPI.Compress` / `DesktopAPI.CompressBatch` 调用 `backend/engines/compressor.py`。依赖由根目录 `pyproject.toml` 和 `uv.lock` 管理，不再经过 Go/Wails 后端，也不依赖独立 `requirements.txt`。

压缩模块支持单图和批量处理，批量任务由 `backend/application/image_ops.py` 使用多进程调度，最大并发由全局设置 `max_concurrency` 控制。

## 压缩库

当前实现会按图片格式和可用依赖选择压缩路径：

1. `mozjpeg-lossless-optimization`：JPEG 无损优化。
2. `imagequant`：PNG 有损量化压缩。
3. `pyoxipng`：PNG 无损优化和 chunk 清理。
4. Pillow：JPEG、PNG、WEBP 以及兜底保存路径。

如果高级压缩库不可用，代码会退回到 Pillow 路径并返回可处理结果或结构化错误。

## 支持格式

- `JPEG/JPG`：支持无损优化、质量压缩、渐进式保存和可选元数据剥离。
- `PNG`：支持 OxiPNG 无损优化、imagequant 有损压缩和可选元数据剥离。
- `WEBP`：使用 Pillow WEBP 编码器，支持无损和有损压缩。
- `BMP`、`TIFF/TIF`、`AVIF`、`ICO`：会走 Pillow 兜底保存逻辑。
- `SVG`、`GIF/APNG` 不走普通图片压缩：SVG 请使用格式转换输出为位图，GIF/APNG 请使用 GIF 工具。

## 压缩等级

| 等级 | 名称 | 质量基准 | 说明 |
|---|---|---|---|
| `1` | Lossless | 100% | 优先使用无损优化 |
| `2` | Light | 90% | 轻量压缩 |
| `3` | Medium | 75% | 默认平衡压缩 |
| `4` | Heavy | 60% | 更高压缩率 |
| `5` | Extreme | 40% | 最大压缩倾向 |

非法等级会回退为 `Medium`。

## API 契约

前端调用面：

- `Compress(payload)`
- `CompressBatch(payloads)`

请求字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `input_path` | `string` | 输入图片路径 |
| `output_path` | `string` | 输出图片路径 |
| `level` | `number` | 压缩等级，范围 `1-5` |
| `engine` | `string` | 可选压缩引擎偏好 |
| `target_size_kb` | `number` | 可选目标大小，未设置或 `0` 表示不限制 |
| `strip_metadata` | `boolean` | 是否剥离元数据 |

响应字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `success` | `boolean` | 是否成功 |
| `input_path` | `string` | 输入路径 |
| `output_path` | `string` | 输出路径 |
| `original_size` | `number` | 原始字节数 |
| `compressed_size` | `number` | 输出字节数 |
| `compression_rate` | `number` | 压缩比例 |
| `compression_level` | `number` | 实际使用等级 |
| `warning` | `string` | 可选警告，例如目标大小未达成 |
| `error` | `string` | 失败原因 |

## 错误处理

常见错误包括：

1. 输入或输出路径缺失。
2. 输入文件不存在。
3. 图片格式不受当前编码路径支持。
4. 输出目录无写入权限。
5. 目标大小过低，无法在当前格式和质量范围内达成。

错误通过 `{ success: false, error: string }` 返回给前端；批量处理中单项失败不会阻断其他项，前端会按单项结果展示失败通知。

## 性能与内存

- 压缩引擎会完整打开图片，超大图片会占用较多内存。
- 批量处理会按 `max_concurrency` 开多个进程，过高并发可能增加内存峰值。
- `target_size_kb` 会触发质量搜索，处理时间会比普通等级压缩更长。
- 元数据剥离会额外读写文件，但可减少隐私信息和部分文件体积。

## 依赖安装

使用 uv 同步依赖：

```bash
uv sync
```

发布构建前同步构建依赖：

```bash
uv sync --group build
```

相关依赖在 `pyproject.toml` 中声明，锁定版本记录在 `uv.lock`。

## 测试

压缩模块测试位于 `backend/tests/engines/test_compressor.py`。常用验证命令：

```bash
npm run test:backend:engines
```

完整本地验证还应运行：

```bash
npm run test:backend
npm --prefix frontend run test
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

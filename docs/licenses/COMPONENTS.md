# 外部组件与许可证登记（TECH-07 / M0-06）

> **状态：基线登记，不是完整分发审查。** 下表逐项记录构建产物实际使用的每个外部依赖、二进制与权重。任何新增依赖、二进制或模型在进入发布产物之前必须补齐本表；未登记项视为发布阻断。
>
> 本文件的结论边界：**Go 模块的许可证可从上游仓库核对；随包分发的可执行文件与模型权重必须由发布方单独确认再分发权利。** 本文件不构成法律意见。

## 1. 直接依赖（`require` 块）

| 模块 | 版本 | 用途 | 许可证（上游仓库） | 是否随包分发 | 备注 |
|---|---|---|---|---|---|
| `github.com/egoist/mygo` | v0.8.4-0.20261108082913-9233663736ca | 原生 GUI、窗口、对话框、shell、更新 | 见上游仓库 LICENSE | 是（编译进主程序） | 版本固定为伪版本，不得自动升级 |
| `github.com/lobehub/lobe-mygo` | v0.0.0-20261108200421-40d9ea87a3c0 | 控件外观与 Lucide 图标集 | 见上游仓库 LICENSE（图标为 ISC） | 是（编译进主程序） | 图标字形数据由 `scripts/gen-lucide.ts` 生成 |
| `github.com/disintegration/imaging` | v1.6.2 | 缩放、裁剪、旋转、翻转、模糊 | MIT | 是 | 几何内核主力 |
| `golang.org/x/image` | v0.47.0 | `draw` 插值、`bmp`/`tiff` 编解码、`font`/`opentype`/`sfnt` 字体渲染 | BSD-3-Clause | 是 | 字体渲染路径 |
| `github.com/gen2brain/jpegli` | v0.4.2 | JPEG 编码（量化表优化） | MIT（绑定层）+ BSD（libjpegli） | 是 | 通过 `ebitengine/purego` 调用 |
| `github.com/gen2brain/webp` | v0.6.4 | WebP 静态与动画读写 | MIT（绑定层）+ BSD（libwebp） | 是 | 需登记 libwebp 版本 |
| `github.com/gen2brain/avif` | v0.6.0 | AVIF 读写 | MIT（绑定层）+ BSD（libavif/AOM） | 是 | 需登记 libavif/AOM 版本 |
| `github.com/gen2brain/heic` | v0.7.2 | HEIF 解码 | MIT（绑定层）；后端分别核验 | 是 | 默认尝试系统 libheif 动态库，失败使用内嵌 Rust heic 0.1.6 WASM；不内嵌 libheif |
| `github.com/kettek/apng` | v0.0.0-20250827064933-2bb5f5fcf253 | APNG 读写 | 见上游仓库 LICENSE | 是 | 动画容器 |
| `github.com/sergeymakinen/go-ico` | v1.0.0 | ICO 多尺寸读写 | MIT | 是 | 图标生成 |
| `github.com/rwcarlsen/goexif` | v0.0.0-20190401172101-9e8deecbddbd | EXIF 方向与元数据 | BSD-2-Clause | 是 | 方向归一路径 |
| `github.com/tetratelabs/wazero` | v1.12.0 | WASM 运行时（LCMS） | Apache-2.0 | 是 | 色彩管理执行环境 |
| `github.com/pdfcpu/pdfcpu` | v0.16.4 | PDF 生成与加密（冻结功能） | Apache-2.0 | 是 | LEG-01，不扩展 |
| `github.com/signintech/gopdf` | v0.38.4 | PDF 排版（冻结功能） | MIT | 是 | LEG-01，不扩展 |
| `github.com/boolean-maybe/go-resvg` | v0.0.2 | SVG 栅格化 | 见上游仓库 LICENSE | 是 | 只读 SVG |

## 2. 间接依赖（`require` 第二块）

| 模块 | 版本 | 引入者 | 许可证（上游） |
|---|---|---|---|
| `github.com/clipperhouse/uax29/v2` | v2.7.0 | pdfcpu | MIT |
| `github.com/ebitengine/purego` | v0.11.1 | avif / heic / jpegli / webp | Apache-2.0 |
| `github.com/go-text/typesetting` | v0.3.5 | pdfcpu | MIT / BSD 混合 |
| `github.com/hhrutter/tiff` | v1.0.7 | pdfcpu | BSD-3-Clause |
| `github.com/mattn/go-runewidth` | v0.0.31 | pdfcpu | MIT |
| `github.com/phpdave11/gofpdi` | v0.0.0-20211212211723-1f10f9844311 | gopdf | MIT |
| `github.com/pkg/errors` | v0.8.1 | pdfcpu | BSD-2-Clause |
| `github.com/sergeymakinen/go-bmp` | v1.0.0 | go-ico | MIT |
| `github.com/xdg-go/stringprep` | v1.0.4 | pdfcpu | Apache-2.0 |
| `go.yaml.in/yaml/v3` | v3.0.5 | pdfcpu | MIT / Apache-2.0 |
| `golang.org/x/crypto` | v0.59.0 | pdfcpu | BSD-3-Clause |
| `golang.org/x/net` | v0.62.0 | 下载代理 / pdfcpu，直接依赖 | BSD-3-Clause |
| `golang.org/x/sys` | v0.50.0 | 系统代理 / 多处，直接依赖 | BSD-3-Clause |
| `golang.org/x/text` | v0.44.0 | 多处 | BSD-3-Clause |

## 3. 随包分发的二进制

| 文件 | 位置 | 版本 | SHA256 | 许可证 | 备注 |
|---|---|---|---|---|---|
| `oxipng.exe` | `internal/pngopt/assets/` | 见上游发布 | `421ce9013342db4a2d9ee6c595adc673d4fe0c69bb0b5c6b9a8794c83226093d` | MIT | Windows 打包内嵌；运行时由 `bundled_windows_amd64.go` 校验哈希后再执行；非 Windows 走外部检测（`external.go`），不随包分发 |
| `lcms.wasm` | `internal/colorprofile/wasm/` | Little CMS（WASM 构建） | 构建时嵌入，哈希由 `go:embed` 保证内容一致 | MIT | 通过 wazero 执行；用于 ICC 转换 |

## 4. 测试与文档素材

| 文件 | 许可证 / 来源 | 用途 |
|---|---|---|
| `internal/codec/testdata/color/cmyk.jpeg` | 仓库自带，见同目录 `README.md` 与 `LICENSE-GO` | CMYK ICC 输入回归 |
| `internal/codec/testdata/color/hlg10.avif`、`pq10.avif` | 仓库自带 | HDR10 色调映射回归 |
| `internal/colorprofile/profiles/*.icc` | 同目录 `LICENSE`（Little CMS 附带配置文件） | sRGB、Display P3、CGATS001 回归 |
| `docs/Go_Image_Toolbox_Full_Delivery_v2.0/assets/fixtures/*` | 交付包原创合成，见 `fixture_manifest.json` | 回归与验收样本 |

## 5. 模型权重（不随包分发）

`internal/upscale` 的模型注册表（`upscale.Registry`）在发布构建中**所有 SHA256 为空**，因此任何模型都显示"未登记哈希，不能启用"。这意味着：

- 基础安装包**不包含**任何 AI 权重。
- 用户自行安装的权重必须显式导入可信清单；内置下载器只从固定官方归档提取所选 param/bin，并核验归档及模型哈希，再登记到本地可信清单。
- `DownloadableModels()` 的固定哈希不等于程序内已安装权重，不自动信任任何本地文件。

| 名称 | 用途 | 原生倍率 | 权重许可证 | 引擎许可证 | 哈希登记 | 再分发审查 |
|---|---|---|---|---|---|---|
| `realesrgan-x4plus` | 照片 | 4× | 上游项目 BSD-3-Clause；具体权重授权需核对 | MIT（Real-ESRGAN-ncnn-vulkan） | 下载器已登记固定哈希 | 不随安装包分发 |
| `realesrgan-x4plus-anime` | 插画 / 动漫 | 4× | 同上 | MIT | 下载器已登记固定哈希 | 不随安装包分发 |
| `realesr-animevideov3` | 动漫视频帧 | 2× | 同上 | MIT | 下载器已登记固定哈希 | 不随安装包分发 |

2026-11-17 实际核对：`https://github.com/xinntao/Real-ESRGAN/blob/master/LICENSE` 为 BSD-3-Clause；`https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan/blob/master/LICENSE` 为 MIT（包含 nihui 许可声明）。官方 ZIP 的 README 未单列权重许可。旧文档将三款权重一律写成 CC-BY-NC-SA-4.0，缺少依据，现已撤回。项目许可证不替代对具体权重的授权核对，不在此声称已取得权重再分发许可。

**高风险项（标红）**：

1. **HEIC 后端区分**：上游 README 与 `lib/Cargo.toml` 确认默认回退为 Rust heic 0.1.6 WASM，而非内嵌 libheif。系统存在 libheif 时会动态加载；不得把绑定层 MIT 当作所有后端许可证，也不得把系统库误记为内嵌库。
2. **权重分发边界**：当前安装包、便携包和更新包不含模型权重或 AI 引擎；未来若内置分发，必须另行完成具体权重授权审核，不能用哈希校验代替。
3. **`Upscayl/upscayl-ncnn` 为 AGPL-3.0**：若接入，其独立进程运行**不自动**消除 AGPL 义务；必须另行审查，且不得作为默认引擎。

## 6. 已知缺口

- 编解码后端（libjpegli / libwebp / libavif / AOM / Rust heic WASM）的完整版本与哈希尚未逐项登记到本表；系统动态加载的 libheif 不随包分发。
- 字体使用系统已安装字体，不随包分发；字体许可由用户系统承担，界面不内嵌字体文件。
- 六平台构建的产物哈希与签名清单由 CI 记录，不在本文件重复登记。

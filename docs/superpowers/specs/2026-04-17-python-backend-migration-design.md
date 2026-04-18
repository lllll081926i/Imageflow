# ImageFlow 纯 Python 后端迁移设计

> 目标：移除 Wails 与 Go 后端，保留现有 React UI 的视觉和交互，重建为 `React + pywebview + Python 应用层 + Python 图像引擎` 的桌面架构。

## 背景

当前项目的核心图像处理能力已经主要沉淀在 Python：

- 图像转换、压缩、PDF、GIF、元数据、水印、调色、滤镜都由 `backend/python/*.py` 执行
- Go 主要承担 Wails 桥接、任务调度、设置存储、预览缓存、路径处理、取消控制
- 前端实际直接依赖 Wails 生成的桥接代码，例如 `window.go.main.App.*`

这带来三类问题：

1. 语言边界复杂
   Go 与 Python 跨进程通信、启动握手、取消与重启恢复，本身就是稳定性负担。

2. 入口过重
   `backend/app.go` 既承载桌面桥接、生命周期、服务装配，又混入设置、缓存、进度与任务控制，扩展成本高。

3. 发布链条过长
   嵌入式 Python 运行时、脚本提取与执行器池化增加了打包与诊断复杂度。

## 迁移目标

本次迁移的目标不是重做产品，而是收敛后端复杂度：

- 删除 Go/Wails 依赖，桌面宿主统一改为 Python
- 保留现有 React UI 的页面结构、样式与主要交互
- 将跨进程 Python 脚本执行改为 Python 进程内模块调用
- 拆分过重的后端文件，建立清晰分层
- 在后端稳定后，清理前端对 Wails 及重桥接依赖的耦合

## 非目标

- 不在第一阶段重做前端页面
- 不在第一阶段替换现有图像算法实现
- 不追求一步完成所有技术债清理
- 不与本次迁移无关地重构现有业务规则

## 目标架构

迁移完成后的目标结构如下：

```text
frontend/                     React UI
  components/                 现有页面与组件
  types/desktop-api.ts        统一桌面桥接接口
  runtime/events.ts           统一事件订阅包装

backend/
  host/                       pywebview 宿主与窗口生命周期
  api/                        暴露给前端的桥接 API
  application/                应用编排、任务管理、预览缓存、取消控制
  domain/                     业务规则、参数校验、命名规则、错误码
  engines/                    图像处理模块（由旧脚本模块化迁入）
  infrastructure/             日志、设置存储、对话框、文件系统、执行器
  contracts/                  前后端共享请求/响应模型
  tests/                      Python 侧单元测试与集成测试
```

## 分层职责

### 1. Host 层

负责：

- 启动 pywebview 窗口
- 加载前端静态资源
- 注册 JS <-> Python 桥接对象
- 处理拖拽、文件对话框、窗口生命周期

禁止：

- 写业务逻辑
- 直接操作图像处理模块
- 写请求校验与路径规则

### 2. API 层

负责：

- 对外暴露与现有前端兼容的方法名
- 请求反序列化
- 调用 application 服务
- 返回统一响应结构

按能力拆文件：

- `settings_api.py`
- `dialog_api.py`
- `convert_api.py`
- `compress_api.py`
- `pdf_api.py`
- `gif_api.py`
- `info_api.py`
- `watermark_api.py`
- `adjust_api.py`
- `filter_api.py`
- `metadata_api.py`
- `preview_api.py`

### 3. Application 层

负责：

- 单任务与批任务编排
- 取消控制
- 预览缓存
- 进度事件派发
- 输出路径解析与批量冲突控制
- 应用设置读写

该层替代现在 Go `App` 中最重的非 UI 职责。

### 4. Domain 层

负责：

- 参数归一化
- 路径校验规则
- 格式兼容规则
- 输出命名规则
- 错误码和错误消息约定

这层必须保持可测试、可独立运行，不依赖 pywebview。

### 5. Engines 层

负责：

- 真正的图像处理实现
- 将旧的 `backend/python/*.py` 从“脚本入口”改为“可导入模块”

重构原则：

- 先保持算法不变，先做结构迁移
- 旧 `process(input_data)` 可以短期保留作为兼容入口
- 逐步拆出纯函数或类方法，减少全局状态与脚本式分支

### 6. Infrastructure 层

负责：

- 日志
- 设置文件读写
- 字体枚举
- 本地对话框
- 线程池/进程池封装
- 临时目录和缓存目录管理

## 前端兼容策略

前端目标是“界面不变、组件尽量不动”，因此采取桥接层替换而不是页面重写。

现状：

- `frontend/types/wails-api.ts` 已经是一个半抽象入口
- 业务组件通过它获取 `window.go.main.App.*` 能力

迁移策略：

1. 保留组件调用语义
2. 将 `wails-api.ts` 演进为宿主无关桥接层
3. 新增 Python 宿主适配器，例如：
   - 优先调用 `window.pywebview.api.*`
   - 回退兼容现有 `window.go.main.App.*`
4. 将事件监听从 Wails runtime 包装成统一事件总线接口

这样可实现：

- 迁移初期 React 组件基本不改
- 中期完全移除 `frontend/wailsjs` 依赖
- 后期按需再瘦前端运行时和重依赖

## 迁移阶段

### 阶段 1：建立 Python 宿主骨架

- 新增 `backend/`
- 建立 `pywebview` 启动入口
- 建立桥接 API 空壳
- 将前端桥接改造成“优先 Python，兼容 Wails”的抽象层

交付标准：

- 前端页面可在 Python 宿主内加载
- `Ping`、设置读取等低风险能力先跑通

### 阶段 2：迁移应用层通用能力

- 设置存储
- 文件/目录选择
- 最近路径维护
- 输出路径解析
- 预览缓存
- 任务取消与进度事件

交付标准：

- 不依赖 Go 即可完成基础系统交互

### 阶段 3：迁移图像能力

按低风险到高风险迁移：

1. `GetInfo`
2. `StripMetadata` / `EditMetadata`
3. `Convert`
4. `Adjust`
5. `ApplyFilter`
6. `AddWatermark`
7. `Compress`
8. `GeneratePDF`
9. `SplitGIF`
10. `GenerateSubtitleLongImage`

交付标准：

- 每迁一个域，就在 Python 宿主下通过对应测试与冒烟验证

### 阶段 4：移除 Go/Wails

- 删除 Wails 入口与 Go 绑定
- 停止生成 `frontend/wailsjs/go/*`
- 用 Python 打包流程替代现有桌面构建

### 阶段 5：前端桥接与依赖瘦身

- 移除对 `wailsjs` 的直接依赖
- 合并重复桥接包装
- 清理不再需要的 runtime 包装和类型依赖
- 检查是否存在体积大但已无收益的前端依赖

## 文件拆分策略

### 当前问题

- `backend/app.go` 是超重入口
- Python 侧多个脚本同时承担入口解析、业务规则、底层处理，模块边界不清晰

### 迁移后的拆分原则

1. 一个文件只负责一个能力域
2. API、应用编排、业务规则、底层实现分层独立
3. 所有批处理逻辑单独封装，不嵌在单文件处理里
4. 路径校验、输出命名、错误码统一集中，不分散复制

## 风险与对策

### 风险 1：前端桥接切换导致大面积调用失效

对策：

- 先保持方法名一致
- 桥接层先做双栈兼容
- 用最小范围组件验证桥接切换

### 风险 2：旧 Python 脚本是“脚本式结构”，直接导入后会暴露隐藏耦合

对策：

- 先加回归测试
- 先抽出模块入口，再做内部拆分
- 每次只迁一个能力域

### 风险 3：取消与进度机制回归

对策：

- 先定义统一任务管理器
- 以单任务可取消、批任务可观测为第一目标
- 再做更高阶并发优化

### 风险 4：发布流程中断

对策：

- 阶段 1-3 保留旧 Wails 链路作回退
- 新旧宿主并存一段时间
- 新宿主稳定后再删除 Go/Wails

## 验收标准

满足以下条件才可视为迁移完成：

1. 所有面向前端的核心能力由 Python 宿主提供
2. React UI 保持主要页面、交互与视觉效果不变
3. 旧图像功能在关键样例上通过回归测试
4. Go/Wails 从运行链路中移除
5. 前端不再依赖 `wailsjs/go/*`
6. 后端不再依赖跨进程 `worker.py` 执行模型

## 首批迁移落点

第一批落地内容优先选择低风险、高杠杆改动：

1. 新建 `backend` 宿主骨架与包结构
2. 迁移设置读写模型
3. 新建 Python 侧 `Ping` 与 `GetSettings/SaveSettings`
4. 将前端桥接抽象成可同时支持 Wails/Python 的接口
5. 为后续图像能力迁移建立统一任务与事件基座

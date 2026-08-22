# ImageFlow Design System

> 本文档是 ImageFlow 的**样式风格契约**。所有新界面、重构与第三方贡献必须遵循此处定义的
> Token 与规则；修改本文档中任何 Token 前，必须同步更新 `frontend/index.css`、
> `frontend/styles/motion.css` 及相关组件。

设计气质：**Apple 式的克制与灵动** —— 大留白、大圆角、毛玻璃层级、单一强调色，
交互反馈使用弹簧物理（spring），拒绝生硬的线性动画。

---

## 1. 设计原则

1. **内容优先**：控件退后，图片与结果前置。
2. **单一强调色**：全应用只有一个品牌蓝，语义色仅用于状态反馈。
3. **物理感动效**：所有可交互元素都有弹簧响应（按下回弹、悬浮上浮、入场过冲）。
4. **明暗双主题对等**：每个颜色 Token 必须同时给出 light / dark 值。
5. **尊重系统设置**：`prefers-reduced-motion` 下禁用全部动效。

---

## 2. 颜色 Tokens

### 2.1 品牌色（唯一强调色）

| Token | Light | Dark | 用途 |
|---|---|---|---|
| `primary` | `#007AFF` | `#0A84FF` | 主按钮、选中态、焦点环、进度条 |
| `primary-deep` | `#0055FF` | — | 主按钮渐变终点 `to-[#0055FF]` |
| `primary-hover` | `#0044DD` | — | 主按钮 hover 渐变终点 |

主按钮固定使用渐变：`bg-gradient-to-r from-[#007AFF] to-[#0055FF]`。

### 2.2 中性色（表面层级）

| 层级 | Light | Dark | 用途 |
|---|---|---|---|
| 页面背景 | `#F5F5F7` | `#1E1E1E` | 应用外壳背景（App.tsx `imageflow-app-shell`） |
| 侧栏表面 | `rgba(255,255,255,0.8)` + glass | `rgba(28,28,30,0.8)` + glass | Sidebar 毛玻璃容器 |
| 卡片表面 | `white` | `#2C2C2E` | 设置面板、仪表盘卡片 |
| 控件凹陷面 | `gray-100` / `rgba(249,250,251,0.9)` | `black/20` / `white/10` | SegmentedControl 轨道、输入框底 |
| 分隔线 | `gray-100/200` | `white/5` ~ `white/10` | 边框、分割线 |
| 主文本 | `gray-900` | `white` | 标题 |
| 次级文本 | `gray-600/700` | `gray-300` | 正文、标签 |
| 弱文本 | `gray-400/500` | `gray-400/500` | 占位符、辅助说明 |

> Dark 模式下的焦点/强调边框使用 `rgba(10, 132, 255, 0.9)`（即 iOS dark 蓝
> `#0A84FF`），见 index.css `.dark .imageflow-select-trigger` 规则。

### 2.3 语义色（仅状态反馈）

| 语义 | Light | Dark | 场景 |
|---|---|---|---|
| 成功 | `green-500/600` | 同色系 | 完成提示 |
| 警告/进行中 | `amber-500` | 同色系 | 取消按钮、失败记录面板 |
| 错误 | `red-500/600` | 同色系 | 失败状态、删除确认 |

### 2.4 功能卡配色（Dashboard 图标底色）

每张功能卡有独立的浅色底 + 深色底组合（定义于 `frontend/constants.ts`
的 `FEATURES[].bg / darkBg`），图标颜色跟随 Tailwind 色板（如 `text-blue-500`）。
新增功能时从 Tailwind 默认色板选取，不引入新 hex。

---

## 3. 字体与排版

- 字体栈：系统字体栈（`system-ui, -apple-system, "Segoe UI", sans-serif`），
  不引入外部字体文件。
- 数字输入：`font-mono text-xs`（滑块数值框）。
- 层级：

| 层级 | 类 | 用途 |
|---|---|---|
| 页面标题 | `text-3xl font-bold tracking-tight` | Dashboard「欢迎回来」 |
| 卡片标题 | `text-lg font-semibold` | 功能卡标题 |
| 正文/标签 | `text-sm font-medium` | 表单标签、侧栏项 |
| 辅助文本 | `text-xs font-medium` | 计数、说明文字 |

---

## 4. 圆角与阴影

### 4.1 圆角阶梯

| 阶梯 | Tailwind | 用途 |
|---|---|---|
| XL | `rounded-2xl` (16px) | 侧栏容器、功能卡、图标底座 |
| 3XL | `rounded-3xl` (24px) | 设置面板等大型表面 |
| L | `rounded-xl` (12px) | 按钮、输入框、下拉触发器、侧栏项 |
| M | `rounded-lg` (8px) | 下拉选项、SegmentedControl 药丸、小按钮 |
| Full | `rounded-full` | 开关、进度条、徽章 |

### 4.2 阴影

| 层级 | 值 | 用途 |
|---|---|---|
| 静态 | `shadow-sm` | 卡片默认 |
| 悬浮 | `hover:shadow-xl` | 功能卡 hover |
| 强调 | `shadow-lg shadow-blue-500/25` | 主按钮的品牌光晕 |
| 浮层 | `0 14px 34px rgba(0,0,0,.16), 0 3px 10px rgba(0,0,0,.08)` | 下拉菜单（见 index.css） |
| 滑块拇指 | `0 2px 4px rgba(0,0,0,.2)` | Switch/Slider 白色圆点 |

---

## 5. 毛玻璃（Glassmorphism）

- 工具类：`.glass` = `backdrop-filter: blur(20px)`（index.css 定义）。
- 仅用于**顶层容器**：Sidebar。浮层菜单使用更轻的
  `blur(18px) saturate(160%)`。
- 禁止嵌套毛玻璃（glass 内不再放 glass）。

---

## 6. 动效系统（Spring Motion）

完整实现见 `frontend/styles/motion.css`，本节为使用契约。

### 6.1 Spring 缓动 Tokens

| Token | 曲线 | 用途 |
|---|---|---|
| `--if-spring-snappy` | `cubic-bezier(0.34, 1.56, 0.64, 1)` | 按压回弹、开关滑块、药丸滑动 |
| `--if-spring-soft` | `cubic-bezier(0.22, 1.28, 0.36, 1)` | 卡片上浮、侧栏指示器 |
| `--if-spring-gentle` | `cubic-bezier(0.175, 0.885, 0.32, 1.08)` | 页面切换、进度条 |
| `--if-ease-out-quiet` | `cubic-bezier(0.16, 1, 0.3, 1)` | 退出/折叠（无过冲） |

### 6.2 时长 Tokens

| Token | 值 | 用途 |
|---|---|---|
| `--if-motion-fast` | 180ms | 按压反馈 |
| `--if-motion-base` | 320ms | 入场、开关、药丸 |
| `--if-motion-slow` | 520ms | 页面切换、指示器滑动 |

### 6.3 工具类速查

| 类名 | 效果 | 应用位置 |
|---|---|---|
| `.spring-tap` | hover 放大 1.02 + 按下缩至 0.965 | 大型可点击块 |
| `.spring-tap-sm` | 仅按下缩至 0.97 | 侧栏项、主按钮 |
| `.spring-lift` | hover 上浮 -4px + 微放大 | Dashboard 卡片 |
| `.spring-enter` | 入场：上移 12px→-2px 过冲→归位 | 页面内区块 |
| `.spring-pop` | 原地弹出 0.92→1.02→1 | 菜单、对话框、徽章 |
| `.spring-page-forward/backward` | 页面级弹簧滑入 | App.tsx 页面切换 |
| `.spring-stagger`（父容器） | 子元素 45ms 级联入场（≤8 个） | Dashboard 卡片网格 |
| `.spring-icon` | 父级 hover 时放大+微旋转 | 功能卡图标 |
| `.spring-progress-fill` | 进度宽度弹簧过渡 | ProgressBar、Slider |
| `.spring-switch-thumb` | 开关滑块弹性滑动 | Switch |
| `.spring-indicator` | 侧栏选中指示器弹性滑动 | Sidebar |
| `.spring-segment-pill` | 分段控件药丸弹性滑动 | SegmentedControl |
| `.spring-collapse` | 折叠时淡出+上移 | 可折叠区块 |

### 6.4 动效规则

1. **只动 transform 和 opacity**，禁止动画 width/height/top/left 布局属性
   （进度条宽度除外——它是唯一例外，且已用 will-change 优化）。
2. **入场有过冲，出场无过冲**：进入用 spring 曲线，退出用 quiet 曲线。
3. **按压必须有反馈**：所有可点击元素至少挂 `.spring-tap-sm` 或
   `active:scale-*`。
4. **级联不超过 8 级**，延迟步长 45ms，总入场时间 ≤ 700ms。
5. **`prefers-reduced-motion: reduce` 下全部动效自动禁用**
   （motion.css 底部统一守卫，勿在组件内重复处理）。

---

## 7. 组件模式

### 7.1 主按钮（Primary Action）

```
bg-gradient-to-r from-[#007AFF] to-[#0055FF] hover:to-[#0044DD]
text-white rounded-xl font-semibold shadow-lg shadow-blue-500/25
py-3~3.5 spring-tap-sm
```

处理中的取消态：`bg-amber-500`；不可用态：`bg-gray-400 cursor-not-allowed`。

### 7.2 设置面板

```
bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm
border border-gray-100 dark:border-white/5
flex flex-col overflow-hidden h-full min-h-0
```

内部滚动区：`.imageflow-settings-scroll flex-1 min-h-0 overflow-y-auto no-scrollbar`。

### 7.3 表单控件行

标签左对齐 `text-sm font-medium text-gray-700 dark:text-gray-300`，
控件右对齐或独占一行；行间距 `space-y-3`；分组间距 `gap-4/5`。

### 7.4 下拉选择（CustomSelect）

触发器最小高 44px、`rounded-xl`；菜单 `rounded-xl` + 浮层阴影 +
`saturate` 毛玻璃；开合动画 260ms snappy spring（index.css
`imageflow-select-menu-in`）。

### 7.5 空状态 / 拖放区

居中虚线边框 + 弱文本引导；拖入悬停时边框转 `primary` 并轻微放大
（FileDropZone 的 `--if-drop-target` 机制）。

---

## 8. 布局体系

- **双栏工作台**（≥900px）：左侧输入区 `1.45fr` + 右侧设置面板
  `minmax(26rem, 0.9fr)`（`.imageflow-detail-layout--standard`）。
- **窄窗单列**（<900px）：自然文档流，页面整体滚动。
- 断点只有两个：`899px`（单列）与 `900px`（双栏）、`1119px`
  （美化类宽控制面提前降级为纵向）。
- 全局禁止横向滚动：所有容器 `min-w-0`。

---

## 9. 无障碍与国际化

- 所有交互控件必须有可访问名（`aria-label` / 可见文本）。
- Switch 使用 `aria-pressed`；CustomSelect 实现 listbox 语义与键盘导航。
- 中文为唯一 UI 语言；文案简洁、动词开头（如「开始转换」「清空失败列表」）。
- 焦点可见性：`focus-visible` 显示 `primary` 色 3px 半透明焦点环
  （参考 `.imageflow-select-trigger:focus-visible`）。

---

## 10. Token 变更流程

1. 修改 `frontend/styles/motion.css`（动效）或 `frontend/index.css`（视觉）。
2. 全文检索旧值并同步组件内的硬编码副本（Tailwind 任意值语法
   `[#007AFF]` 等）。
3. 运行 `npm run typecheck && npm test && npm run build` 验证。
4. 更新本文档对应表格，并在变更描述中注明「Design System 同步」。

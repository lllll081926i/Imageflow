# Image Info Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让文件信息页对主流图片格式返回尽可能多的真实可验证信息，并以稳定、可搜索、可编辑的方式展示。

**Architecture:** Python `info_viewer.py` 负责格式解析、元数据归类和字段来源标注；Go 只做模型桥接和错误归一化；前端 `DetailView.tsx` 只消费结构化信息，不再自己猜分组和优先级。兼容旧字段，逐步迁移到新的 `basic / format_details / fields / warnings` 结构。

**Tech Stack:** Python 3, Pillow, ExifRead, piexif, Go, React, TypeScript, Wails

---

### Task 1: 固化后端结构化输出测试

**Files:**
- Modify: `backend/python/tests/test_info_viewer_metadata.py`
- Modify: `backend/app_test.go`

**Step 1: Write the failing test**

补充 Python 测试，验证：
- JPEG 返回 `basic`、`fields`、`warnings`
- PNG / GIF / TIFF / SVG / WEBP 至少返回真实基础字段和分组 metadata
- 同一字段多来源时保留来源信息

补充 Go 测试，验证：
- `GetInfo` 透传新的结构化字段
- 旧字段仍兼容

**Step 2: Run test to verify it fails**

Run: `.\.venv\Scripts\python.exe -m unittest discover -s backend/python/tests`
Expected: FAIL for missing structured keys

Run: `go test ./...`
Expected: FAIL for missing Go model fields

**Step 3: Write minimal implementation**

先只加最小结构，让测试红转绿，不同时做前端。

**Step 4: Run test to verify it passes**

Run: `.\.venv\Scripts\python.exe -m unittest discover -s backend/python/tests`
Expected: PASS

Run: `go test ./...`
Expected: PASS

### Task 2: 重构 Python 信息读取结果

**Files:**
- Modify: `backend/python/info_viewer.py`
- Modify: `backend/python/tests/test_info_viewer_metadata.py`

**Step 1: Write the failing test**

增加格式覆盖测试：
- JPEG: EXIF / XMP / JFIF
- PNG: text / DPI / ICC / color type
- GIF: 帧数 / loop / comment / duration
- WEBP: alpha / animation / XMP
- SVG: 宽高 / viewBox / title/desc
- HEIC/HEIF: 容器级基础字段和 warning

**Step 2: Run test to verify it fails**

Run: `.\.venv\Scripts\python.exe -m unittest discover -s backend/python/tests`
Expected: FAIL with missing format detail fields

**Step 3: Write minimal implementation**

在 `info_viewer.py` 中增加：
- 结构化 `basic`
- `format_details`
- `metadata`
- `fields`
- `warnings`
- 字段 `source`

**Step 4: Run test to verify it passes**

Run: `.\.venv\Scripts\python.exe -m unittest discover -s backend/python/tests`
Expected: PASS

### Task 3: 更新 Go 模型和桥接

**Files:**
- Modify: `backend/models/types.go`
- Modify: `backend/services/info_viewer.go`
- Modify: `backend/app_test.go`
- Modify: `frontend/wailsjs/go/models.ts`

**Step 1: Write the failing test**

Go 测试验证：
- 新结构反序列化成功
- 错误归一化不丢失
- 编辑元数据后刷新仍兼容

**Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL with unknown fields / missing struct members

**Step 3: Write minimal implementation**

新增 `InfoBasic`、`InfoField`、`InfoWarning` 等模型，保留旧字段。

**Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS

### Task 4: 重构信息页展示

**Files:**
- Modify: `frontend/components/DetailView.tsx`
- Modify: `frontend/components/gifHelpers.test.ts`

**Step 1: Write the failing test**

前端测试验证：
- 基础信息、格式细节、元数据、读取状态四区块
- 搜索过滤
- editable 字段只来自后端标记
- 导出 JSON 走完整结构

**Step 2: Run test to verify it fails**

Run: `npm test`
Expected: FAIL because new sections and field mapping are missing

**Step 3: Write minimal implementation**

让前端直接消费 `fields` / `warnings` / `basic`，不再本地拼接扁平 metadata。

**Step 4: Run test to verify it passes**

Run: `npm test`
Expected: PASS

### Task 5: 全量验证与兼容检查

**Files:**
- Modify: `backend/python/tests/test_info_viewer_metadata.py`
- Modify: `backend/app_test.go`
- Modify: `frontend/components/DetailView.tsx`

**Step 1: Run complete validation**

Run: `go test ./...`
Expected: PASS

Run: `npm test`
Expected: PASS

Run: `npm run typecheck`
Expected: PASS

Run: `.\.venv\Scripts\python.exe -m unittest discover -s backend/python/tests`
Expected: PASS

**Step 2: Fix any regressions**

只修复本次信息链路相关问题，不扩散到无关模块。

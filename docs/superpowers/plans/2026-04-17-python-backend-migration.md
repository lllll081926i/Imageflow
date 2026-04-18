# Python Backend Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用纯 Python 宿主和应用层替换现有 Wails/Go 后端，同时保持现有 React UI 的页面和交互尽量不变，并为后续清理前端桥接依赖打基础。

**Architecture:** 先建立 `backend` 宿主、桥接 API、应用层和共享契约，再把前端桥接从 Wails 绑定抽离成宿主无关接口。图像能力按能力域逐步从旧 `backend/python/*.py` 模块化迁移，期间保持新旧链路可并存。

**Tech Stack:** Python 3.10+, pywebview, unittest, React, TypeScript, Vite

---

### Task 1: 建立 Python 后端骨架

**Files:**
- Create: `backend/__init__.py`
- Create: `backend/app.py`
- Create: `backend/host/__init__.py`
- Create: `backend/host/window.py`
- Create: `backend/api/__init__.py`
- Create: `backend/contracts/__init__.py`
- Create: `backend/tests/test_bootstrap.py`

- [ ] **Step 1: Write the failing test**

```python
import unittest

from backend.app import create_app


class BootstrapTests(unittest.TestCase):
    def test_create_app_returns_api_with_ping(self):
        app = create_app()
        self.assertTrue(hasattr(app, "ping"))


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run python -m unittest backend.tests.test_bootstrap -v`
Expected: FAIL with `ModuleNotFoundError: No module named 'backend'`

- [ ] **Step 3: Write minimal implementation**

```python
class DesktopAPI:
    def ping(self):
        return "pong"


def create_app():
    return DesktopAPI()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run python -m unittest backend.tests.test_bootstrap -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat: 初始化 python 宿主骨架"
```

### Task 2: 迁移设置契约与存储

**Files:**
- Create: `backend/contracts/settings.py`
- Create: `backend/infrastructure/__init__.py`
- Create: `backend/infrastructure/settings_store.py`
- Create: `backend/tests/test_settings_store.py`

- [ ] **Step 1: Write the failing test**

```python
import unittest

from backend.contracts.settings import AppSettings
from backend.infrastructure.settings_store import normalize_settings


class SettingsStoreTests(unittest.TestCase):
    def test_normalize_settings_clamps_concurrency_and_paths(self):
        raw = AppSettings(max_concurrency=99, default_output_dir="C:/tmp///")
        normalized = normalize_settings(raw)
        self.assertEqual(normalized.max_concurrency, 32)
        self.assertEqual(normalized.default_output_dir, "C:/tmp")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run python -m unittest backend.tests.test_settings_store -v`
Expected: FAIL with missing module or missing symbol

- [ ] **Step 3: Write minimal implementation**

```python
from dataclasses import dataclass, field


@dataclass
class AppSettings:
    max_concurrency: int = 8
    output_prefix: str = "IF"
    output_template: str = "{prefix}{basename}"
    preserve_folder_structure: bool = True
    conflict_strategy: str = "rename"
    default_output_dir: str = ""
    recent_input_dirs: list[str] = field(default_factory=list)
    recent_output_dirs: list[str] = field(default_factory=list)
```

```python
def normalize_settings(settings: AppSettings) -> AppSettings:
    settings.max_concurrency = max(1, min(settings.max_concurrency, 32))
    settings.default_output_dir = settings.default_output_dir.rstrip("/\\")
    return settings
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run python -m unittest backend.tests.test_settings_store -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat: 迁移 python 设置存储"
```

### Task 3: 建立 Python 侧基础 API

**Files:**
- Create: `backend/api/settings_api.py`
- Create: `backend/api/system_api.py`
- Modify: `backend/app.py`
- Create: `backend/tests/test_system_api.py`

- [ ] **Step 1: Write the failing test**

```python
import unittest

from backend.app import create_app


class SystemAPITests(unittest.TestCase):
    def test_ping_and_settings_roundtrip(self):
        app = create_app()
        self.assertEqual(app.ping(), "pong")
        settings = app.get_settings()
        self.assertEqual(settings["max_concurrency"], 8)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run python -m unittest backend.tests.test_system_api -v`
Expected: FAIL because `get_settings` is missing

- [ ] **Step 3: Write minimal implementation**

```python
class DesktopAPI:
    def __init__(self, settings_api, system_api):
        self._settings_api = settings_api
        self._system_api = system_api

    def ping(self):
        return self._system_api.ping()

    def get_settings(self):
        return self._settings_api.get_settings()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run python -m unittest backend.tests.test_system_api -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat: 添加 python 基础桥接 api"
```

### Task 4: 抽离前端桥接层

**Files:**
- Create: `frontend/types/desktop-api.ts`
- Modify: `frontend/types/wails-api.ts`
- Test: `frontend/components/gifHelpers.test.ts`

- [ ] **Step 1: Write the failing test**

```typescript
import { getAppBindings } from '../types/wails-api';

test('prefers pywebview api when available', () => {
  (window as any).pywebview = { api: { Ping: jest.fn() } };
  expect(getAppBindings()).toBe((window as any).pywebview.api);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npm test -- --runInBand`
Expected: FAIL because `getAppBindings` only checks `window.go`

- [ ] **Step 3: Write minimal implementation**

```typescript
export function getDesktopBindings(): Record<string, unknown> | null {
    const pywebviewApi = (window as any).pywebview?.api;
    if (pywebviewApi) return pywebviewApi;
    const goApi = window.go?.main?.App;
    if (goApi) return goApi as Record<string, unknown>;
    return null;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npm test -- --runInBand`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/types
git commit -m "refactor: 抽离前端桌面桥接层"
```

### Task 5: 建立任务与事件基座

**Files:**
- Create: `backend/application/__init__.py`
- Create: `backend/application/task_manager.py`
- Create: `backend/application/events.py`
- Create: `backend/tests/test_task_manager.py`

- [ ] **Step 1: Write the failing test**

```python
import unittest

from backend.application.task_manager import TaskManager


class TaskManagerTests(unittest.TestCase):
    def test_begin_and_cancel_task(self):
        manager = TaskManager()
        task_id = manager.begin_task("convert")
        self.assertTrue(manager.cancel_task(task_id))
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run python -m unittest backend.tests.test_task_manager -v`
Expected: FAIL due to missing module

- [ ] **Step 3: Write minimal implementation**

```python
class TaskManager:
    def __init__(self):
        self._tasks = {}
        self._next_task_id = 0

    def begin_task(self, kind: str) -> int:
        self._next_task_id += 1
        self._tasks[self._next_task_id] = {"kind": kind, "cancelled": False}
        return self._next_task_id

    def cancel_task(self, task_id: int) -> bool:
        task = self._tasks.get(task_id)
        if not task:
            return False
        task["cancelled"] = True
        return True
```

- [ ] **Step 4: Run test to verify it passes**

Run: `uv run python -m unittest backend.tests.test_task_manager -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat: 初始化 python 任务管理器"
```

### Task 6: 迁移第一个业务域 GetInfo

**Files:**
- Create: `backend/engines/info_engine.py`
- Create: `backend/api/info_api.py`
- Create: `backend/tests/test_info_api.py`
- Reference: `backend/python/info_viewer.py`

- [ ] **Step 1: Write the failing test**

```python
import unittest

from backend.api.info_api import InfoAPI


class InfoAPITests(unittest.TestCase):
    def test_get_info_returns_success_shape_for_existing_file(self):
        api = InfoAPI()
        result = api.get_info({"input_path": "backend/python/testdata/simple.svg"})
        self.assertIn("success", result)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `uv run python -m unittest backend.tests.test_info_api -v`
Expected: FAIL because API does not exist

- [ ] **Step 3: Write minimal implementation**

```python
class InfoAPI:
    def get_info(self, payload):
        return {"success": False, "error": "not implemented"}
```

- [ ] **Step 4: Run test to verify it passes/then refine**

Run: `uv run python -m unittest backend.tests.test_info_api -v`
Expected: PASS for shape test, then add next failing behavior test and implement real info engine

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "feat: 初始化 python 信息查看 api"
```

### Task 7: 清理前端 Wails 直连依赖

**Files:**
- Modify: `frontend/components/Controls.tsx`
- Modify: `frontend/components/DetailView.tsx`
- Modify: `frontend/components/SettingsView.tsx`
- Modify: `frontend/components/SubtitleStitchPage.tsx`
- Modify: `frontend/types/wails-api.ts`

- [ ] **Step 1: Write the failing test**

```typescript
test('components use desktop bridge instead of direct Wails binding', () => {
  // Add focused assertions around helper imports or mocked bridge behavior.
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npm test -- --runInBand`
Expected: FAIL because components still rely on old binding assumptions

- [ ] **Step 3: Write minimal implementation**

```typescript
// Replace direct binding lookups with desktop-api wrapper imports.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && npm test -- --runInBand`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/components frontend/types
git commit -m "refactor: 前端切换到宿主无关桥接"
```

### Task 8: 迁移剩余业务域并移除 Go/Wails

**Files:**
- Modify/Create across `backend/api`, `backend/application`, `backend/engines`
- Remove later: `backend/`, `frontend/wailsjs/go/*`, Wails config files

- [ ] **Step 1: 为每个业务域重复 TDD 迁移循环**

顺序：

```text
metadata -> convert -> adjust -> filter -> watermark -> compress -> pdf -> gif -> subtitle_stitch
```

- [ ] **Step 2: 每个域先补 Python 侧回归测试**

Run: `uv run python -m unittest discover -s backend/tests -v`
Expected: 新增测试先红后绿

- [ ] **Step 3: 每迁一个域执行一次前端冒烟**

Run: `cd frontend && npm run build`
Expected: exit 0

- [ ] **Step 4: 完成全部域后再删除 Go/Wails**

Run:

```bash
git rm -r backend
git rm -r frontend/wailsjs/go
```

Expected: 仅在 Python 宿主链路完全验证后执行

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "refactor: 完成纯 python 后端迁移"
```

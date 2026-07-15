from __future__ import annotations

import atexit
import os
import threading
import time
from concurrent.futures import FIRST_COMPLETED, ProcessPoolExecutor, wait
from typing import Any

from backend.application.task_manager import TaskManager
from backend.contracts.settings import AppSettings
from backend.infrastructure.engine_loader import invoke_engine_process

_pool_lock = threading.Lock()
_pool: ProcessPoolExecutor | None = None
_pool_size = 0
_pool_disabled = str(os.getenv("IMAGEFLOW_DISABLE_PROCESS_POOL", "") or "").strip().lower() in {
    "1",
    "true",
    "yes",
    "on",
}


def _invoke_engine_job(module_name: str, payload: dict[str, Any]) -> dict[str, Any]:
    """Top-level worker entry so ProcessPoolExecutor can pickle it on Windows."""
    try:
        return invoke_engine_process(module_name, payload)
    except Exception as exc:
        return {"success": False, "error": str(exc)}


def _desired_pool_size(requested: int | None = None) -> int:
    if requested is not None:
        return max(1, min(32, int(requested)))
    env = str(os.getenv("IMAGEFLOW_PROCESS_POOL_SIZE", "") or "").strip()
    if env:
        try:
            return max(1, min(32, int(env)))
        except ValueError:
            pass
    cpu = os.cpu_count() or 4
    if os.name == "nt":
        return max(1, min(4, max(2, cpu // 2)))
    return max(1, min(8, cpu))


def _shutdown_pool() -> None:
    global _pool, _pool_size
    with _pool_lock:
        if _pool is not None:
            try:
                _pool.shutdown(wait=False, cancel_futures=True)
            except TypeError:
                _pool.shutdown(wait=False)
            except Exception:
                pass
        _pool = None
        _pool_size = 0


atexit.register(_shutdown_pool)


def reset_process_pool_for_tests() -> None:
    """Test helper to drop the global pool between cases."""
    _shutdown_pool()


def warm_process_pool(min_size: int | None = None) -> int:
    """Pre-create the process pool so first user action avoids cold spawn."""
    if _pool_disabled:
        return 0
    size = _desired_pool_size(min_size)
    pool = _get_pool(size)
    # Touch pool object to ensure workers can start lazily on first submit.
    return getattr(pool, "_max_workers", size)


def _get_pool(min_size: int = 1) -> ProcessPoolExecutor:
    global _pool, _pool_size
    target = max(_desired_pool_size(), max(1, int(min_size)))
    target = min(32, target)
    with _pool_lock:
        if _pool is not None and _pool_size >= target:
            return _pool
        if _pool is not None:
            try:
                _pool.shutdown(wait=False, cancel_futures=True)
            except TypeError:
                _pool.shutdown(wait=False)
            except Exception:
                pass
            _pool = None
        _pool = ProcessPoolExecutor(max_workers=target)
        _pool_size = target
        return _pool


def _run_jobs(
    module_name: str,
    payloads: list[dict[str, Any]],
    max_workers: int,
    task_manager: TaskManager | None = None,
    task_id: int | None = None,
) -> list[dict[str, Any]]:
    if not payloads:
        return []

    if _pool_disabled:
        results: list[dict[str, Any]] = []
        for payload in payloads:
            if task_manager is not None and task_id is not None and task_manager.is_cancelled(task_id):
                results.append({"success": False, "error": "[PY_CANCELLED] operation cancelled"})
                continue
            results.append(_invoke_engine_job(module_name, payload))
        return results

    worker_count = max(1, min(int(max_workers), len(payloads)))
    pool = _get_pool(worker_count)
    futures = [pool.submit(_invoke_engine_job, module_name, payload) for payload in payloads]
    results: list[dict[str, Any]] = [{"success": False, "error": "处理失败"} for _ in payloads]
    pending = set(futures)
    future_to_index = {future: index for index, future in enumerate(futures)}

    try:
        while pending:
            if task_manager is not None and task_id is not None and task_manager.is_cancelled(task_id):
                for future in list(pending):
                    future.cancel()
                    index = future_to_index[future]
                    results[index] = {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
                break

            done, pending = wait(pending, timeout=0.1, return_when=FIRST_COMPLETED)
            if not done:
                continue
            for future in done:
                index = future_to_index[future]
                try:
                    value = future.result()
                    if isinstance(value, dict):
                        results[index] = value
                    else:
                        results[index] = {"success": False, "error": "处理返回格式异常"}
                except Exception as exc:
                    results[index] = {"success": False, "error": str(exc)}
    finally:
        if task_manager is not None and task_id is not None and task_manager.is_cancelled(task_id):
            for future in futures:
                future.cancel()

    return results


def execute_engine(
    module_name: str,
    payload: dict[str, Any],
    task_manager: TaskManager | None = None,
    task_id: int | None = None,
) -> dict[str, Any]:
    effective_task_id = task_id if task_id is not None else (task_manager.current_task_id if task_manager else None)
    if task_manager and effective_task_id is not None and task_manager.is_cancelled(effective_task_id):
        return {"success": False, "error": "[PY_CANCELLED] operation cancelled"}

    results = _run_jobs(
        module_name,
        [payload],
        max_workers=1,
        task_manager=task_manager,
        task_id=effective_task_id,
    )
    result = results[0] if results else {"success": False, "error": "处理失败"}
    if task_manager and effective_task_id is not None and task_manager.is_cancelled(effective_task_id):
        return {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
    return result


def execute_engine_batch(
    module_name: str,
    payloads: list[dict[str, Any]],
    settings: AppSettings,
    task_manager: TaskManager,
) -> list[dict[str, Any]]:
    if not payloads:
        return []

    task_id = task_manager.current_task_id
    max_workers = max(1, min(settings.max_concurrency, len(payloads)))
    return _run_jobs(
        module_name,
        payloads,
        max_workers=max_workers,
        task_manager=task_manager,
        task_id=task_id,
    )

import time
from multiprocessing import Process, Queue
from queue import Empty
from typing import Any

from backend.application.task_manager import TaskManager
from backend.contracts.settings import AppSettings
from backend.infrastructure.engine_loader import invoke_engine_process


def _process_worker(module_name: str, payload: dict[str, Any], queue: Queue) -> None:
    try:
        queue.put(invoke_engine_process(module_name, payload))
    except Exception as exc:
        queue.put({"success": False, "error": str(exc)})


def execute_engine(module_name: str, payload: dict[str, Any], task_manager: TaskManager | None = None) -> dict[str, Any]:
    task_id = task_manager.current_task_id if task_manager else None
    queue: Queue = Queue()
    process = Process(target=_process_worker, args=(module_name, payload, queue))
    process.start()
    if task_manager and task_id is not None:
        task_manager.attach_process(task_id, process)

    try:
        while process.is_alive():
            if task_manager and task_id is not None and task_manager.is_cancelled(task_id):
                process.terminate()
                process.join()
                return {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
            time.sleep(0.05)

        process.join()
        try:
            result = queue.get_nowait()
        except Empty:
            result = {"success": False, "error": "处理失败"}
        if task_manager and task_id is not None and task_manager.is_cancelled(task_id):
            return {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
        return result
    finally:
        if task_manager and task_id is not None:
            task_manager.detach_process(task_id, process)


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
    results: list[dict[str, Any] | None] = [None] * len(payloads)
    pending = list(enumerate(payloads))
    active: list[tuple[int, Process, Queue]] = []

    def start_one(index: int, payload: dict[str, Any]) -> None:
        queue: Queue = Queue()
        process = Process(target=_process_worker, args=(module_name, payload, queue))
        process.start()
        if task_id is not None:
            task_manager.attach_process(task_id, process)
        active.append((index, process, queue))

    while pending or active:
        if task_id is not None and task_manager.is_cancelled(task_id):
            for index, process, _queue in active:
                if process.is_alive():
                    process.terminate()
                process.join()
                results[index] = {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
                task_manager.detach_process(task_id, process)
            active.clear()
            for index, _payload in pending:
                results[index] = {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
            break

        while pending and len(active) < max_workers:
            index, payload = pending.pop(0)
            start_one(index, payload)

        for index, process, queue in list(active):
            if process.is_alive():
                continue
            process.join()
            try:
                results[index] = queue.get_nowait()
            except Empty:
                results[index] = {"success": False, "error": "处理失败"}
            active.remove((index, process, queue))
            if task_id is not None:
                task_manager.detach_process(task_id, process)

        time.sleep(0.02)

    return [result or {"success": False, "error": "处理失败"} for result in results]

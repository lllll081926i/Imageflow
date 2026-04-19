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


def _batch_process_worker(module_name: str, task_queue: Queue, result_queue: Queue) -> None:
    while True:
        item = task_queue.get()
        if item is None:
            return
        index, payload = item
        try:
            result = invoke_engine_process(module_name, payload)
        except Exception as exc:
            result = {"success": False, "error": str(exc)}
        result_queue.put((index, result))


def _close_queue(queue: Queue) -> None:
    close = getattr(queue, "close", None)
    if callable(close):
        close()
    join_thread = getattr(queue, "join_thread", None)
    if callable(join_thread):
        join_thread()


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
    task_queue: Queue = Queue()
    result_queue: Queue = Queue()
    workers: list[Process] = []

    for item in enumerate(payloads):
        task_queue.put(item)
    for _ in range(max_workers):
        task_queue.put(None)

    for _ in range(max_workers):
        process = Process(target=_batch_process_worker, args=(module_name, task_queue, result_queue))
        process.start()
        if task_id is not None:
            task_manager.attach_process(task_id, process)
        workers.append(process)

    completed = 0
    try:
        while completed < len(payloads):
            if task_id is not None and task_manager.is_cancelled(task_id):
                for process in workers:
                    if process.is_alive():
                        process.terminate()
                    process.join()
                for index, result in enumerate(results):
                    if result is None:
                        results[index] = {"success": False, "error": "[PY_CANCELLED] operation cancelled"}
                break

            try:
                index, result = result_queue.get(timeout=0.05)
            except Empty:
                if all(not process.is_alive() for process in workers):
                    for process in workers:
                        process.join()
                    while completed < len(payloads):
                        try:
                            index, result = result_queue.get_nowait()
                        except Empty:
                            break
                        if 0 <= index < len(results) and results[index] is None:
                            results[index] = result
                            completed += 1
                    break
                continue

            if 0 <= index < len(results) and results[index] is None:
                results[index] = result
                completed += 1

        for process in workers:
            process.join()
    finally:
        for process in workers:
            if process.is_alive():
                process.terminate()
                process.join()
            if task_id is not None:
                task_manager.detach_process(task_id, process)
        _close_queue(task_queue)
        _close_queue(result_queue)

    return [result or {"success": False, "error": "处理失败"} for result in results]

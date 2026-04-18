from multiprocessing.process import BaseProcess


class TaskManager:
    def __init__(self):
        self._tasks: dict[int, dict[str, object]] = {}
        self._next_task_id = 0
        self._active_task_ids: list[int] = []
        self.current_task_id: int | None = None

    def begin_task(self, kind: str) -> int:
        self._next_task_id += 1
        self._tasks[self._next_task_id] = {
            "kind": kind,
            "cancelled": False,
            "processes": [],
        }
        self._active_task_ids.append(self._next_task_id)
        self.current_task_id = self._next_task_id
        return self._next_task_id

    def cancel_task(self, task_id: int) -> bool:
        task = self._tasks.get(task_id)
        if task is None:
            return False
        task["cancelled"] = True
        for process in list(task.get("processes", [])):
            try:
                if process.is_alive():
                    process.terminate()
            except Exception:
                continue
        return True

    def is_cancelled(self, task_id: int) -> bool:
        task = self._tasks.get(task_id)
        if task is None:
            return False
        return bool(task["cancelled"])

    def cancel_current_task(self) -> bool:
        self._refresh_current_task_id()
        if self.current_task_id is None:
            return False
        return self.cancel_task(self.current_task_id)

    def finish_task(self, task_id: int) -> None:
        self._tasks.pop(task_id, None)
        if task_id in self._active_task_ids:
            self._active_task_ids.remove(task_id)
        self._refresh_current_task_id()

    def attach_process(self, task_id: int, process: BaseProcess) -> None:
        task = self._tasks.get(task_id)
        if task is None:
            return
        processes = task.setdefault("processes", [])
        if isinstance(processes, list):
            processes.append(process)

    def detach_process(self, task_id: int, process: BaseProcess) -> None:
        task = self._tasks.get(task_id)
        if task is None:
            return
        processes = task.get("processes", [])
        if isinstance(processes, list) and process in processes:
            processes.remove(process)

    def _refresh_current_task_id(self) -> None:
        while self._active_task_ids and self._active_task_ids[-1] not in self._tasks:
            self._active_task_ids.pop()
        self.current_task_id = self._active_task_ids[-1] if self._active_task_ids else None

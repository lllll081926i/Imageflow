import unittest

from backend.application import image_ops
from backend.application.task_manager import TaskManager
from backend.contracts.settings import AppSettings


class TaskManagerTests(unittest.TestCase):
    def test_begin_and_cancel_task(self):
        manager = TaskManager()

        task_id = manager.begin_task("convert")

        self.assertGreater(task_id, 0)
        self.assertTrue(manager.cancel_task(task_id))
        self.assertTrue(manager.is_cancelled(task_id))

    def test_finish_task_clears_current_task(self):
        manager = TaskManager()

        task_id = manager.begin_task("convert")
        manager.finish_task(task_id)

        self.assertIsNone(manager.current_task_id)
        self.assertFalse(manager.is_cancelled(task_id))

    def test_cancel_current_task_without_active_task_returns_false(self):
        manager = TaskManager()

        self.assertFalse(manager.cancel_current_task())

    def test_finishing_older_task_keeps_newer_task_current(self):
        manager = TaskManager()

        first = manager.begin_task("info")
        second = manager.begin_task("convert")
        manager.finish_task(first)

        self.assertEqual(manager.current_task_id, second)
        self.assertTrue(manager.cancel_current_task())
        self.assertTrue(manager.is_cancelled(second))

    def test_execute_engine_batch_reuses_worker_processes(self):
        original_process = image_ops.Process
        original_queue = image_ops.Queue
        original_invoke = image_ops.invoke_engine_process
        started_processes: list[object] = []

        class InMemoryQueue:
            def __init__(self):
                self.items = []

            def put(self, item):
                self.items.append(item)

            def get(self, timeout=None):
                if not self.items:
                    raise image_ops.Empty()
                return self.items.pop(0)

            def get_nowait(self):
                return self.get()

        class FakeProcess:
            def __init__(self, target, args):
                self._target = target
                self._args = args
                self._alive = False

            def start(self):
                started_processes.append(self)
                self._alive = True
                self._target(*self._args)
                self._alive = False

            def is_alive(self):
                return self._alive

            def terminate(self):
                self._alive = False

            def join(self):
                return None

        try:
            image_ops.Process = FakeProcess
            image_ops.Queue = InMemoryQueue
            image_ops.invoke_engine_process = lambda _module, payload: {"success": True, "value": payload["value"]}
            manager = TaskManager()
            task_id = manager.begin_task("batch")

            results = image_ops.execute_engine_batch(
                "converter",
                [{"value": index} for index in range(5)],
                AppSettings(max_concurrency=2),
                manager,
            )

            self.assertEqual([item["value"] for item in results], [0, 1, 2, 3, 4])
            self.assertEqual(len(started_processes), 2)
            manager.finish_task(task_id)
        finally:
            image_ops.Process = original_process
            image_ops.Queue = original_queue
            image_ops.invoke_engine_process = original_invoke


if __name__ == "__main__":
    unittest.main()

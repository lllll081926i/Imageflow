import os
import unittest
from unittest import mock

from backend.application import image_ops
from backend.application.task_manager import TaskManager
from backend.contracts.settings import AppSettings


class TaskManagerTests(unittest.TestCase):
    def setUp(self):
        os.environ["IMAGEFLOW_DISABLE_PROCESS_POOL"] = "1"
        image_ops._pool_disabled = True
        image_ops.reset_process_pool_for_tests()

    def tearDown(self):
        os.environ.pop("IMAGEFLOW_DISABLE_PROCESS_POOL", None)
        image_ops._pool_disabled = False
        image_ops.reset_process_pool_for_tests()

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

    def test_execute_engine_batch_preserves_order_with_pool_disabled(self):
        original_job = image_ops._invoke_engine_job
        calls: list[int] = []

        def fake_job(_module, payload):
            calls.append(payload["value"])
            return {"success": True, "value": payload["value"]}

        try:
            image_ops._invoke_engine_job = fake_job
            manager = TaskManager()
            task_id = manager.begin_task("batch")
            results = image_ops.execute_engine_batch(
                "converter",
                [{"value": index} for index in range(5)],
                AppSettings(max_concurrency=2),
                manager,
            )
            self.assertEqual([item["value"] for item in results], [0, 1, 2, 3, 4])
            self.assertEqual(calls, [0, 1, 2, 3, 4])
            manager.finish_task(task_id)
        finally:
            image_ops._invoke_engine_job = original_job

    def test_execute_engine_skips_work_for_cancelled_task(self):
        called = {"value": False}

        def boom(_module, _payload):
            called["value"] = True
            raise AssertionError("cancelled task should not run")

        original_job = image_ops._invoke_engine_job
        try:
            image_ops._invoke_engine_job = boom
            manager = TaskManager()
            task_id = manager.begin_task("info", set_current=False)
            manager.cancel_task(task_id)
            result = image_ops.execute_engine("info_viewer", {"input_path": "D:/dummy.png"}, manager, task_id=task_id)
            self.assertEqual(result, {"success": False, "error": "[PY_CANCELLED] operation cancelled"})
            self.assertFalse(called["value"])
        finally:
            image_ops._invoke_engine_job = original_job

    def test_execute_engine_batch_marks_remaining_cancelled(self):
        original_job = image_ops._invoke_engine_job

        def fake_job(_module, payload):
            return {"success": True, "value": payload["value"]}

        try:
            image_ops._invoke_engine_job = fake_job
            manager = TaskManager()
            task_id = manager.begin_task("batch")
            manager.cancel_task(task_id)
            results = image_ops.execute_engine_batch(
                "converter",
                [{"value": 1}, {"value": 2}],
                AppSettings(max_concurrency=2),
                manager,
            )
            self.assertTrue(all(item.get("success") is False for item in results))
            self.assertTrue(all("[PY_CANCELLED]" in str(item.get("error") or "") for item in results))
            manager.finish_task(task_id)
        finally:
            image_ops._invoke_engine_job = original_job

    def test_process_pool_enabled_uses_executor(self):
        # Temporarily enable pool path and stub executor.
        image_ops._pool_disabled = False
        image_ops.reset_process_pool_for_tests()
        submit_calls: list[tuple] = []

        class FakeFuture:
            def __init__(self, value):
                self._value = value

            def done(self):
                return True

            def result(self, timeout=None):
                return self._value

            def cancel(self):
                return True

        class FakePool:
            def submit(self, fn, module_name, payload):
                submit_calls.append((module_name, payload))
                return FakeFuture({"success": True, "value": payload["value"]})

            def shutdown(self, wait=False, cancel_futures=False):
                return None

        with mock.patch.object(image_ops, "_get_pool", return_value=FakePool()):
            with mock.patch.object(image_ops, "wait", side_effect=lambda done_set, timeout=None, return_when=None: (set(done_set), set())):
                manager = TaskManager()
                task_id = manager.begin_task("batch")
                results = image_ops.execute_engine_batch(
                    "converter",
                    [{"value": 7}, {"value": 8}],
                    AppSettings(max_concurrency=2),
                    manager,
                )
                manager.finish_task(task_id)

        self.assertEqual([item["value"] for item in results], [7, 8])
        self.assertEqual(len(submit_calls), 2)
        image_ops._pool_disabled = True
        image_ops.reset_process_pool_for_tests()


if __name__ == "__main__":
    unittest.main()

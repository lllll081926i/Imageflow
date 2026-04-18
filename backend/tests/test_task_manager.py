import unittest

from backend.application.task_manager import TaskManager


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


if __name__ == "__main__":
    unittest.main()

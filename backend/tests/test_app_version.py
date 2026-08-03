import unittest
from backend.infrastructure.app_version import is_remote_newer, read_local_version, _parse_version_tuple

class AppVersionTests(unittest.TestCase):
    def test_parse_and_compare(self):
        self.assertEqual(_parse_version_tuple("v2.1.1"), (2, 1, 1))
        self.assertTrue(is_remote_newer("2.1.0", "2.1.1"))
        self.assertFalse(is_remote_newer("2.1.1", "2.1.1"))
        self.assertFalse(is_remote_newer("2.2.0", "2.1.9"))

    def test_local_version_readable(self):
        v = read_local_version()
        self.assertTrue(v)
        self.assertRegex(v, r"^\d+")

if __name__ == "__main__":
    unittest.main()

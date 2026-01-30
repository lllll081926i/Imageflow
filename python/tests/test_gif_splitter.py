import os
import sys
import tempfile
import unittest

from PIL import Image

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from gif_splitter import handle_request


class GifSplitterTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name):
        return os.path.join(self.temp_dir.name, name)

    def _make_gif(self):
        path = self._path("sample.gif")
        frames = [
            Image.new("RGB", (12, 12), (255, 0, 0)),
            Image.new("RGB", (12, 12), (0, 255, 0)),
        ]
        frames[0].save(
            path,
            format="GIF",
            save_all=True,
            append_images=frames[1:],
            duration=100,
            loop=0,
        )
        return path

    def test_export_frames_jpg(self):
        gif_path = self._make_gif()
        out_dir = self._path("frames")
        result = handle_request(
            {
                "action": "export_frames",
                "input_path": gif_path,
                "output_dir": out_dir,
                "output_format": "jpg",
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("export_count"), 2)
        for frame_path in result.get("frame_paths", []):
            self.assertTrue(os.path.exists(frame_path))
            with Image.open(frame_path) as img:
                self.assertEqual(img.format, "JPEG")

    def test_get_frame_count_success(self):
        gif_path = self._make_gif()
        result = handle_request({"action": "get_frame_count", "input_path": gif_path})
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("frame_count"), 2)


if __name__ == "__main__":
    unittest.main()

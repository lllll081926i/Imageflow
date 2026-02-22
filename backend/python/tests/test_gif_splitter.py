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

    def _make_png(self, name, color):
        path = self._path(name)
        Image.new("RGB", (12, 12), color).save(path, format="PNG")
        return path

    def test_export_frames_png(self):
        gif_path = self._make_gif()
        out_dir = self._path("frames")
        result = handle_request(
            {
                "action": "export_frames",
                "input_path": gif_path,
                "output_dir": out_dir,
                "output_format": "png",
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("export_count"), 2)
        for frame_path in result.get("frame_paths", []):
            self.assertTrue(os.path.exists(frame_path))
            with Image.open(frame_path) as img:
                self.assertEqual(img.format, "PNG")

    def test_export_frames_jpg_request_normalized_to_png(self):
        gif_path = self._make_gif()
        out_dir = self._path("frames_jpg_normalized")
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
            self.assertTrue(frame_path.lower().endswith(".png"))
            self.assertTrue(os.path.exists(frame_path))
            with Image.open(frame_path) as img:
                self.assertEqual(img.format, "PNG")

    def test_get_frame_count_success(self):
        gif_path = self._make_gif()
        result = handle_request({"action": "get_frame_count", "input_path": gif_path})
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("frame_count"), 2)

    def test_compress_gif_success(self):
        gif_path = self._make_gif()
        output_path = self._path("sample_compress.gif")
        result = handle_request(
            {
                "action": "compress",
                "input_path": gif_path,
                "output_path": output_path,
                "quality": 90,
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("quality"), 90)
        self.assertTrue(os.path.exists(output_path))
        with Image.open(output_path) as img:
            self.assertEqual(img.format, "GIF")

    def test_compress_gif_quality_clamped(self):
        gif_path = self._make_gif()
        output_path = self._path("sample_compress_clamped.gif")
        result = handle_request(
            {
                "action": "compress",
                "input_path": gif_path,
                "output_path": output_path,
                "quality": 999,
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("quality"), 100)

    def test_compress_gif_quality_invalid_defaults_to_90(self):
        gif_path = self._make_gif()
        output_path = self._path("sample_compress_default.gif")
        result = handle_request(
            {
                "action": "compress_gif",
                "input_path": gif_path,
                "output_path": output_path,
                "quality": "abc",
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("quality"), 90)

    def test_reverse_gif_success(self):
        gif_path = self._make_gif()
        output_path = self._path("sample_reverse.gif")
        result = handle_request(
            {
                "action": "reverse",
                "input_path": gif_path,
                "output_path": output_path,
            }
        )
        self.assertTrue(result.get("success"))
        self.assertTrue(os.path.exists(output_path))
        with Image.open(output_path) as img:
            self.assertEqual(img.format, "GIF")
            first_frame = img.convert("RGB")
            self.assertEqual(first_frame.getpixel((0, 0)), (0, 255, 0))

    def test_change_speed_success(self):
        gif_path = self._make_gif()
        output_path = self._path("sample_speed.gif")
        result = handle_request(
            {
                "action": "change_speed",
                "input_path": gif_path,
                "output_path": output_path,
                "speed_factor": 0.5,
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("speed_factor"), 0.5)
        with Image.open(output_path) as img:
            self.assertEqual(img.info.get("duration"), 200)

    def test_build_gif_success(self):
        p1 = self._make_png("frame1.png", (255, 0, 0))
        p2 = self._make_png("frame2.png", (0, 0, 255))
        output_path = self._path("built.gif")
        result = handle_request(
            {
                "action": "build_gif",
                "input_paths": [p1, p2],
                "output_path": output_path,
                "fps": 5,
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("frame_count"), 2)
        self.assertEqual(result.get("fps"), 5.0)
        with Image.open(output_path) as img:
            self.assertEqual(img.format, "GIF")
            self.assertEqual(getattr(img, "n_frames", 1), 2)


if __name__ == "__main__":
    unittest.main()

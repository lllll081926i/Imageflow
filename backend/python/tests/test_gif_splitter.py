import os
import sys
import tempfile
import unittest

from PIL import Image, ImageSequence

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

import gif_splitter

handle_request = gif_splitter.handle_request


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

    def _make_rect_gif(self, size=(20, 10)):
        path = self._path("sample_rect.gif")
        frames = [
            Image.new("RGB", size, (255, 0, 0)),
            Image.new("RGB", size, (0, 255, 0)),
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

    def _make_transparent_gif(self):
        path = self._path("sample_transparent.gif")
        frames = []
        for idx in range(4):
            frame = Image.new("RGBA", (24, 24), (0, 0, 0, 0))
            x0 = 2 + idx * 4
            x1 = x0 + 8
            for x in range(x0, min(x1, 24)):
                for y in range(8, 16):
                    frame.putpixel((x, y), (255, 0, 0, 255))
            frames.append(frame)
        frames[0].save(
            path,
            format="GIF",
            save_all=True,
            append_images=frames[1:],
            duration=[90] * len(frames),
            loop=0,
            disposal=2,
        )
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

    def test_export_frames_jpg_request_rejected(self):
        gif_path = self._make_gif()
        out_dir = self._path("frames_jpg_rejected")
        result = handle_request(
            {
                "action": "export_frames",
                "input_path": gif_path,
                "output_dir": out_dir,
                "output_format": "jpg",
            }
        )
        self.assertFalse(result.get("success"))
        self.assertIn("Unsupported output format: jpg", result.get("error", ""))
        self.assertEqual(result.get("error_code"), "GIF_EXPORT_UNSUPPORTED_FORMAT")

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

    def test_compress_gif_preserves_transparent_background(self):
        gif_path = self._make_transparent_gif()
        output_path = self._path("sample_transparent_compress.gif")
        result = handle_request(
            {
                "action": "compress",
                "input_path": gif_path,
                "output_path": output_path,
                "quality": 75,
            }
        )
        self.assertTrue(result.get("success"))
        with Image.open(output_path) as img:
            self.assertEqual(img.format, "GIF")
            for frame in ImageSequence.Iterator(img):
                rgba = frame.convert("RGBA")
                self.assertEqual(rgba.getpixel((0, 0))[3], 0)

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

    def test_resize_gif_keep_aspect_by_width(self):
        gif_path = self._make_rect_gif((20, 10))
        output_path = self._path("sample_resize_width.gif")
        result = handle_request(
            {
                "action": "resize",
                "input_path": gif_path,
                "output_path": output_path,
                "width": 10,
                "maintain_aspect": True,
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("width"), 10)
        self.assertEqual(result.get("height"), 5)
        with Image.open(output_path) as img:
            self.assertEqual(img.size, (10, 5))

    def test_resize_gif_keep_aspect_in_box(self):
        gif_path = self._make_rect_gif((20, 10))
        output_path = self._path("sample_resize_box.gif")
        result = handle_request(
            {
                "action": "resize_gif",
                "input_path": gif_path,
                "output_path": output_path,
                "width": 16,
                "height": 16,
                "maintain_aspect": True,
            }
        )
        self.assertTrue(result.get("success"))
        self.assertEqual(result.get("width"), 16)
        self.assertEqual(result.get("height"), 8)
        with Image.open(output_path) as img:
            self.assertEqual(img.size, (16, 8))

    def test_resize_gif_requires_width_or_height(self):
        gif_path = self._make_rect_gif((20, 10))
        output_path = self._path("sample_resize_missing.gif")
        result = handle_request(
            {
                "action": "resize",
                "input_path": gif_path,
                "output_path": output_path,
            }
        )
        self.assertFalse(result.get("success"))
        self.assertIn("Missing width or height", result.get("error", ""))
        self.assertEqual(result.get("error_code"), "GIF_RESIZE_INVALID_SIZE")

    def test_resize_gif_returns_memory_limit_error_code(self):
        gif_path = self._make_rect_gif((20, 10))
        output_path = self._path("sample_resize_oom.gif")
        original_budget = gif_splitter.MAX_FRAME_PIXEL_BUDGET
        gif_splitter.MAX_FRAME_PIXEL_BUDGET = 10
        try:
            result = handle_request(
                {
                    "action": "resize",
                    "input_path": gif_path,
                    "output_path": output_path,
                    "width": 20,
                }
            )
        finally:
            gif_splitter.MAX_FRAME_PIXEL_BUDGET = original_budget
        self.assertFalse(result.get("success"))
        self.assertEqual(result.get("error_code"), "GIF_MEMORY_LIMIT")

    def test_unsupported_action_returns_error_code(self):
        result = handle_request({"action": "unknown_action"})
        self.assertFalse(result.get("success"))
        self.assertEqual(result.get("error_code"), "GIF_UNSUPPORTED_ACTION")


if __name__ == "__main__":
    unittest.main()

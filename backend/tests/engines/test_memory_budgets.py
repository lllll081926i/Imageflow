"""Detailed memory-budget and safety tests for GIF / subtitle / filter engines."""

from __future__ import annotations

import os
import sys
import tempfile
import unittest
from pathlib import Path

from PIL import Image

ENGINE_DIR = Path(__file__).resolve().parents[2] / "engines"
if str(ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(ENGINE_DIR))

import filter as filter_engine  # noqa: E402
import gif_splitter  # noqa: E402
import subtitle_stitcher  # noqa: E402


class GifMemoryBudgetTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.tool = gif_splitter.GIFTool()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _path(self, name: str) -> str:
        return os.path.join(self.temp_dir.name, name)

    def _make_gif(self, name: str, size=(16, 16), frames: int = 4, duration: int = 80) -> str:
        path = self._path(name)
        images = [
            Image.new("RGB", size, ((index * 40) % 255, 20, 90))
            for index in range(frames)
        ]
        images[0].save(
            path,
            format="GIF",
            save_all=True,
            append_images=images[1:],
            duration=duration,
            loop=0,
        )
        return path

    def test_extract_animation_frames_enforces_total_pixel_budget(self):
        path = self._make_gif("budget.gif", size=(32, 32), frames=3)
        original_total = gif_splitter.MAX_TOTAL_FRAME_PIXEL_BUDGET
        original_frame = gif_splitter.MAX_FRAME_PIXEL_BUDGET
        try:
            # 32*32*3 = 3072; set total budget below that.
            gif_splitter.MAX_FRAME_PIXEL_BUDGET = 100_000
            gif_splitter.MAX_TOTAL_FRAME_PIXEL_BUDGET = 1000
            with Image.open(path) as animated:
                with self.assertRaises(MemoryError) as ctx:
                    self.tool._extract_animation_frames(animated, require_animated=True, source_path=path)
            self.assertIn("too many pixels", str(ctx.exception).lower())
        finally:
            gif_splitter.MAX_TOTAL_FRAME_PIXEL_BUDGET = original_total
            gif_splitter.MAX_FRAME_PIXEL_BUDGET = original_frame

    def test_extract_gif_frames_enforces_single_frame_budget(self):
        path = self._make_gif("frame-budget.gif", size=(40, 40), frames=2)
        original = gif_splitter.MAX_FRAME_PIXEL_BUDGET
        try:
            gif_splitter.MAX_FRAME_PIXEL_BUDGET = 100  # 40*40=1600 > 100
            with Image.open(path) as gif:
                with self.assertRaises(MemoryError):
                    self.tool._extract_gif_frames(gif)
        finally:
            gif_splitter.MAX_FRAME_PIXEL_BUDGET = original

    def test_export_frames_returns_structured_error_when_budget_exceeded(self):
        path = self._make_gif("export-budget.gif", size=(24, 24), frames=5)
        out_dir = self._path("frames")
        original = gif_splitter.MAX_TOTAL_FRAME_PIXEL_BUDGET
        try:
            gif_splitter.MAX_TOTAL_FRAME_PIXEL_BUDGET = 200
            result = self.tool.export_frames(path, out_dir, output_format="png", frame_range="all")
        finally:
            gif_splitter.MAX_TOTAL_FRAME_PIXEL_BUDGET = original
        self.assertFalse(result.get("success"))
        # export_frames catches Exception and returns GIF_EXPORT_FAILED for MemoryError path.
        self.assertIn(result.get("error_code"), {"GIF_EXPORT_FAILED", "GIF_MEMORY_LIMIT"})
        self.assertTrue(str(result.get("error") or ""))

    def test_convert_animation_closes_frames_on_success(self):
        path = self._make_gif("convert-close.gif", size=(12, 12), frames=3)
        output = self._path("out.webp")
        # Spy by wrapping extract to track returned frames.
        original_extract = self.tool._extract_animation_frames
        closed = {"count": 0}
        tracked = []

        def tracking_extract(*args, **kwargs):
            frames, durations, loop, disposals, fmt = original_extract(*args, **kwargs)
            for frame in frames:
                original_close = frame.close

                def close_with_count(close=original_close):
                    closed["count"] += 1
                    return close()

                frame.close = close_with_count  # type: ignore[method-assign]
                tracked.append(frame)
            return frames, durations, loop, disposals, fmt

        self.tool._extract_animation_frames = tracking_extract  # type: ignore[method-assign]
        try:
            result = self.tool.convert_animation(path, output, "webp", quality=80)
        finally:
            self.tool._extract_animation_frames = original_extract  # type: ignore[method-assign]

        self.assertTrue(result.get("success"), result)
        self.assertTrue(os.path.exists(output))
        self.assertGreaterEqual(closed["count"], len(tracked))
        self.assertGreater(len(tracked), 0)

    def test_convert_animation_closes_frames_on_failure(self):
        path = self._make_gif("convert-fail-close.gif", size=(10, 10), frames=2)
        output = self._path("fail.webp")
        original_extract = self.tool._extract_animation_frames
        closed = {"count": 0}

        def tracking_extract(*args, **kwargs):
            frames, durations, loop, disposals, fmt = original_extract(*args, **kwargs)
            for frame in frames:
                original_close = frame.close

                def close_with_count(close=original_close):
                    closed["count"] += 1
                    return close()

                frame.close = close_with_count  # type: ignore[method-assign]
            return frames, durations, loop, disposals, fmt

        original_save = self.tool._save_webp

        def boom(*_args, **_kwargs):
            raise RuntimeError("save failed")

        self.tool._extract_animation_frames = tracking_extract  # type: ignore[method-assign]
        self.tool._save_webp = boom  # type: ignore[method-assign]
        try:
            result = self.tool.convert_animation(path, output, "webp", quality=80)
        finally:
            self.tool._extract_animation_frames = original_extract  # type: ignore[method-assign]
            self.tool._save_webp = original_save  # type: ignore[method-assign]

        self.assertFalse(result.get("success"))
        self.assertGreaterEqual(closed["count"], 2)


class SubtitleCanvasBudgetTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _frame(self, name: str, size=(80, 80)) -> str:
        path = os.path.join(self.temp_dir.name, name)
        Image.new("RGB", size, (30, 40, 50)).save(path, format="PNG")
        return path

    def test_canvas_edge_limit_is_enforced(self):
        original_edge = subtitle_stitcher.MAX_CANVAS_EDGE
        original_pixels = subtitle_stitcher.MAX_CANVAS_PIXELS
        try:
            subtitle_stitcher.MAX_CANVAS_EDGE = 100
            subtitle_stitcher.MAX_CANVAS_PIXELS = 100_000_000
            frames = [self._frame(f"edge-{index}.png", size=(120, 40)) for index in range(2)]
            result = subtitle_stitcher.process(
                {
                    "action": "subtitle_stitch",
                    "input_paths": frames,
                    "output_path": os.path.join(self.temp_dir.name, "edge-out.png"),
                    "subtitle_crop_ratio": 0.25,
                    "header_keep_full": True,
                    "dedup_enabled": False,
                    "minimum_strip_height": 8,
                }
            )
            self.assertFalse(result.get("success"))
            self.assertEqual(result.get("error_code"), "SUBTITLE_CANVAS_TOO_LARGE")
        finally:
            subtitle_stitcher.MAX_CANVAS_EDGE = original_edge
            subtitle_stitcher.MAX_CANVAS_PIXELS = original_pixels

    def test_canvas_pixel_limit_message_includes_budget(self):
        original = subtitle_stitcher.MAX_CANVAS_PIXELS
        try:
            subtitle_stitcher.MAX_CANVAS_PIXELS = 500
            frames = [self._frame(f"px-{index}.png", size=(60, 60)) for index in range(3)]
            result = subtitle_stitcher.process(
                {
                    "action": "subtitle_stitch",
                    "input_paths": frames,
                    "output_path": os.path.join(self.temp_dir.name, "px-out.png"),
                    "subtitle_crop_ratio": 0.2,
                    "header_keep_full": True,
                    "dedup_enabled": False,
                    "minimum_strip_height": 8,
                }
            )
            self.assertFalse(result.get("success"))
            self.assertEqual(result.get("error_code"), "SUBTITLE_CANVAS_TOO_LARGE")
            self.assertIn("500", str(result.get("error") or ""))
        finally:
            subtitle_stitcher.MAX_CANVAS_PIXELS = original


class FilterSafetyDetailedTests(unittest.TestCase):
    def setUp(self):
        self.tool = filter_engine.ImageFilterApplier()

    def test_fast_noise_path_returns_image(self):
        with Image.new("RGB", (24, 24), (40, 40, 40)) as img:
            result = self.tool._add_noise(img, 0.35)
            try:
                self.assertEqual(result.size, img.size)
                self.assertEqual(result.mode, "RGB")
            finally:
                if result is not img:
                    result.close()

    def test_fast_vignette_path_returns_image(self):
        with Image.new("RGB", (24, 24), (80, 80, 80)) as img:
            result = self.tool._add_vignette(img, 0.5)
            try:
                self.assertEqual(result.size, img.size)
            finally:
                if result is not img:
                    result.close()

    def test_slow_fallbacks_raise_runtime_error_with_clear_message(self):
        with Image.new("RGB", (8, 8), (10, 10, 10)) as img:
            with self.assertRaises(RuntimeError) as noise_ctx:
                self.tool._add_noise_slow(img, 0.2)
            with self.assertRaises(RuntimeError) as vignette_ctx:
                self.tool._add_vignette_slow(img, 0.2)
        self.assertIn("disabled", str(noise_ctx.exception).lower())
        self.assertIn("disabled", str(vignette_ctx.exception).lower())

    def test_zero_strength_noise_and_vignette_are_noops(self):
        with Image.new("RGB", (12, 12), (7, 8, 9)) as img:
            noise = self.tool._add_noise(img, 0)
            vignette = self.tool._add_vignette(img, 0)
            self.assertIs(noise, img)
            self.assertIs(vignette, img)


if __name__ == "__main__":
    unittest.main()

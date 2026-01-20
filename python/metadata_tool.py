import json
import os
import shutil
import sys
import tempfile
import traceback
from pathlib import Path

from PIL import Image


def _as_bool(v):
    if isinstance(v, bool):
        return v
    if v is None:
        return False
    return str(v).strip().lower() in ("1", "true", "yes", "y", "on")


def _strip_jpeg_lossless(input_path: str, output_path: str):
    try:
        import piexif

        piexif.remove(input_path, output_path)
        return True, ""
    except Exception as e:
        return False, str(e)


def _strip_png_lossless(input_path: str, output_path: str):
    try:
        import oxipng

        shutil.copyfile(input_path, output_path)
        oxipng.optimize(output_path, output_path, level=2, strip=oxipng.StripChunks.all())
        return True, ""
    except Exception as e:
        return False, str(e)


def _rewrite_without_metadata(input_path: str, output_path: str):
    img = Image.open(input_path)
    fmt = (img.format or "").upper()
    if fmt in ("JPEG", "JPG"):
        if img.mode in ("RGBA", "P", "LA"):
            img = img.convert("RGB")
        img.save(output_path, format="JPEG", quality=95, optimize=True, progressive=True)
    elif fmt == "PNG":
        img.save(output_path, format="PNG", optimize=True, compress_level=6)
    elif fmt == "WEBP":
        if img.mode in ("P",):
            img = img.convert("RGBA")
        img.save(output_path, format="WEBP", lossless=True, method=6)
    else:
        if not fmt:
            fmt = Path(output_path).suffix.lstrip(".").upper() or "PNG"
        img.save(output_path, format=fmt)
    try:
        img.close()
    except Exception:
        pass


def strip_metadata(input_path: str, output_path: str, overwrite: bool):
    input_abs = os.path.abspath(input_path)
    output_abs = os.path.abspath(output_path)
    same_file = input_abs == output_abs or overwrite

    tmp_output_path = None
    final_output_path = output_path
    if same_file:
        final_dir = os.path.dirname(input_abs) or "."
        os.makedirs(final_dir, exist_ok=True)
        with tempfile.NamedTemporaryFile(
            delete=False,
            dir=final_dir,
            suffix=Path(input_path).suffix or ".tmp",
        ) as tmp:
            tmp_output_path = tmp.name
        final_output_path = tmp_output_path

    try:
        ext = Path(input_path).suffix.lower()
        if ext in (".jpg", ".jpeg"):
            ok, detail = _strip_jpeg_lossless(input_path, final_output_path)
            if not ok:
                _rewrite_without_metadata(input_path, final_output_path)
        elif ext == ".png":
            ok, detail = _strip_png_lossless(input_path, final_output_path)
            if not ok:
                _rewrite_without_metadata(input_path, final_output_path)
        else:
            _rewrite_without_metadata(input_path, final_output_path)

        if tmp_output_path:
            os.replace(tmp_output_path, input_path)
            tmp_output_path = None
            return {"success": True, "input_path": input_path, "output_path": input_path}
        return {"success": True, "input_path": input_path, "output_path": output_path}
    finally:
        try:
            if tmp_output_path:
                os.remove(tmp_output_path)
        except Exception:
            pass


def process(input_data):
    try:
        action = str(input_data.get("action") or "").strip().lower() or "strip_metadata"
        input_path = input_data.get("input_path")
        output_path = input_data.get("output_path")
        overwrite = _as_bool(input_data.get("overwrite", False))

        if action != "strip_metadata":
            return {"success": False, "error": "[INVALID_ACTION] unsupported action", "details": action}
        if not input_path:
            return {"success": False, "error": "[BAD_INPUT] missing input_path"}
        if overwrite:
            output_path = input_path
        if not output_path:
            return {"success": False, "error": "[BAD_INPUT] missing output_path"}
        if not os.path.exists(input_path):
            return {"success": False, "error": f"[NOT_FOUND] input file not found: {input_path}"}

        return strip_metadata(str(input_path), str(output_path), overwrite)
    except PermissionError as e:
        return {"success": False, "error": f"[PERMISSION_DENIED] {e}"}
    except Exception as e:
        tb = traceback.format_exc()
        return {"success": False, "error": f"[INTERNAL] {e}", "traceback": tb[-4000:]}


def main():
    try:
        input_data = json.load(sys.stdin)
        out = process(input_data)
        json.dump(out, sys.stdout, ensure_ascii=False)
    except Exception as e:
        json.dump({"success": False, "error": f"[INTERNAL] {e}"}, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()

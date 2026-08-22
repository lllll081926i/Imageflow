"""Shared image format support registration for engine processes."""

_opener_registered = False
_pixel_limit_applied = False


def _apply_pixel_budget() -> None:
    """Match the host's conservative decompression budget in worker processes."""
    global _pixel_limit_applied
    if _pixel_limit_applied:
        return
    try:
        from PIL import Image

        Image.MAX_IMAGE_PIXELS = 64_000_000
    except Exception:
        pass
    _pixel_limit_applied = True


def ensure_heif_supported() -> bool:
    """Register the HEIF/HEIC Pillow plugin once per process.

    Returns True when HEIC input can be decoded. Failures are swallowed so
    engines keep working for all other formats when the wheel is unavailable.
    """
    global _opener_registered
    if _opener_registered:
        return True
    try:
        import pillow_heif

        pillow_heif.register_heif_opener()
        _opener_registered = True
    except Exception:
        return False
    return True


_apply_pixel_budget()

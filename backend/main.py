from backend.host.window import build_window_api, configure_window, resolve_frontend_entry


def main() -> None:
    import threading

    import webview
    from PIL import Image

    # Pillow default MAX_IMAGE_PIXELS is ~89M; keep a tighter host-side cap so
    # accidental huge bitmaps fail fast with DecompressionBombError instead of OOM.
    # Engines may still process large images in worker processes where needed.
    try:
        Image.MAX_IMAGE_PIXELS = min(int(Image.MAX_IMAGE_PIXELS or 89_478_485), 64_000_000)
    except Exception:
        Image.MAX_IMAGE_PIXELS = 64_000_000

    window = webview.create_window(
        title="ImageFlow",
        url=resolve_frontend_entry(),
        js_api=build_window_api(),
        width=1366,
        height=900,
        min_size=(1024, 600),
        frameless=True,
        easy_drag=False,
    )
    configure_window(window)

    def _warm_runtime() -> None:
        try:
            from backend.application.image_ops import warm_process_pool
            warm_process_pool()
        except Exception:
            pass
        try:
            from backend.infrastructure.engine_loader import load_engine_module
            load_engine_module("converter")
        except Exception:
            pass

    threading.Thread(target=_warm_runtime, name="imageflow-warmup", daemon=True).start()
    webview.start()


def bootstrap() -> None:
    multiprocessing_module = globals().get("multiprocessing")
    if multiprocessing_module is None:
        import multiprocessing as multiprocessing_module

    multiprocessing_module.freeze_support()
    main()


if __name__ == "__main__":
    bootstrap()

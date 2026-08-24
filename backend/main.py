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

    width, height = 2000, 1300
    x = y = None
    try:
        from backend.infrastructure.settings_store import load_settings
        from backend.infrastructure.window_state import clamp_window_state, load_window_state

        settings = load_settings()
        if getattr(settings, "remember_window_state", True):
            state = load_window_state()
            if state:
                state = clamp_window_state(state)
                width = int(state.get("width") or width)
                height = int(state.get("height") or height)
                if "x" in state:
                    x = int(state["x"])
                if "y" in state:
                    y = int(state["y"])
    except Exception:
        pass

    window_kwargs = dict(
        title="ImageFlow",
        url=resolve_frontend_entry(),
        js_api=build_window_api(),
        width=width,
        height=height,
        min_size=(900, 560),
        resizable=True,
        frameless=True,
        easy_drag=False,
    )
    if x is not None and y is not None:
        window_kwargs["x"] = x
        window_kwargs["y"] = y
    window = webview.create_window(**window_kwargs)
    configure_window(window)

    def _warm_runtime() -> None:
        # Let the window paint first: heavy warmup (process-pool spin-up +
        # converter engine import) competes with GUI init for CPU/disk.
        threading.Event().wait(1.5)
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

    warmup_started = threading.Event()

    def _schedule_runtime_warmup() -> None:
        # Start the delay only after WebView reports its document as loaded.
        # Starting it before webview.start() made slow machines spawn workers
        # while the first frame was still competing for CPU and disk I/O.
        if warmup_started.is_set():
            return
        warmup_started.set()
        threading.Thread(target=_warm_runtime, name="imageflow-warmup", daemon=True).start()

    loaded_event = getattr(getattr(window, "events", None), "loaded", None)
    if loaded_event is not None:
        loaded_event += _schedule_runtime_warmup
    webview.start()


def bootstrap() -> None:
    multiprocessing_module = globals().get("multiprocessing")
    if multiprocessing_module is None:
        import multiprocessing as multiprocessing_module

    multiprocessing_module.freeze_support()
    main()


if __name__ == "__main__":
    bootstrap()

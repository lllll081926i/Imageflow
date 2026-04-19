import multiprocessing

from backend.host.window import build_window_api, configure_window, resolve_frontend_entry


def main() -> None:
    import webview

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
    if window is not None:
        configure_window(window)
    webview.start()


def bootstrap() -> None:
    multiprocessing.freeze_support()
    main()


if __name__ == "__main__":
    bootstrap()

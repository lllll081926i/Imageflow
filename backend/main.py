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
    configure_window(window)
    webview.start()


def bootstrap() -> None:
    multiprocessing_module = globals().get("multiprocessing")
    if multiprocessing_module is None:
        import multiprocessing as multiprocessing_module

    multiprocessing_module.freeze_support()
    main()


if __name__ == "__main__":
    bootstrap()

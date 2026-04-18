type RuntimeDialogFilter = {
    DisplayName: string;
    Pattern: string;
};

type RuntimeOpenFileDialogOptions = {
    title?: string;
    canChooseFiles?: boolean;
    canChooseDirectories?: boolean;
    allowsMultipleSelection?: boolean;
    filters?: RuntimeDialogFilter[];
};

type RuntimeOpenDirectoryDialogOptions = {
    title?: string;
};

type DesktopRuntime = {
    Quit?: () => void;
    WindowMinimise?: () => void;
    WindowToggleMaximise?: () => void;
    OnFileDrop?: (callback: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean) => void;
    OnFileDropOff?: () => void;
    OpenFileDialog?: (options?: RuntimeOpenFileDialogOptions) => Promise<string | string[] | null | undefined>;
    OpenDirectoryDialog?: (options?: RuntimeOpenDirectoryDialogOptions) => Promise<string | null | undefined>;
    ResolveFilePaths?: (files: File[]) => Promise<unknown>;
    CanResolveFilePaths?: () => boolean;
};

type PywebviewApi = {
    OpenFileDialog?: (options?: RuntimeOpenFileDialogOptions) => Promise<string | string[] | null | undefined>;
    OpenDirectoryDialog?: (options?: RuntimeOpenDirectoryDialogOptions) => Promise<string | null | undefined>;
    Quit?: () => void | Promise<void>;
    WindowMinimise?: () => void | Promise<void>;
    WindowToggleMaximise?: () => void | Promise<void>;
    ResolveFilePaths?: (files: File[]) => Promise<unknown>;
    CanResolveFilePaths?: () => boolean;
};

declare global {
    interface Window {
        pywebview?: {
            api?: PywebviewApi;
        };
        runtime?: DesktopRuntime;
    }
}

const getPywebviewApi = () => window.pywebview?.api;
const FILE_DROP_EVENT = '__imageflow_file_drop__';
let fileDropListener: ((event: Event) => void) | null = null;

const isDropTargetElement = (element: Element | null): boolean => {
    let current: Element | null = element;
    while (current) {
        if (getComputedStyle(current).getPropertyValue('--wails-drop-target').trim() === 'drop') {
            return true;
        }
        current = current.parentElement;
    }
    return false;
};

export function installDesktopRuntime() {
    if (window.runtime) {
        return;
    }

    window.runtime = {
        Quit: () => {
            void getPywebviewApi()?.Quit?.();
        },
        WindowMinimise: () => {
            void getPywebviewApi()?.WindowMinimise?.();
        },
        WindowToggleMaximise: () => {
            void getPywebviewApi()?.WindowToggleMaximise?.();
        },
        OnFileDrop: (callback, useDropTarget) => {
            if (fileDropListener) {
                window.removeEventListener(FILE_DROP_EVENT, fileDropListener);
            }

            fileDropListener = (event: Event) => {
                const detail = (event as CustomEvent<{ x?: number; y?: number; paths?: string[] }>).detail;
                const x = Number(detail?.x ?? 0);
                const y = Number(detail?.y ?? 0);
                const paths = Array.isArray(detail?.paths)
                    ? detail.paths.filter((item): item is string => typeof item === 'string' && item.trim() !== '')
                    : [];
                if (!paths.length) {
                    return;
                }
                if (useDropTarget) {
                    const dropTarget = document.elementFromPoint(x, y);
                    if (!isDropTargetElement(dropTarget)) {
                        return;
                    }
                }
                callback(x, y, paths);
            };

            window.addEventListener(FILE_DROP_EVENT, fileDropListener);
        },
        OnFileDropOff: () => {
            if (!fileDropListener) {
                return;
            }
            window.removeEventListener(FILE_DROP_EVENT, fileDropListener);
            fileDropListener = null;
        },
        OpenFileDialog: async (options) => {
            const api = getPywebviewApi();
            if (!api?.OpenFileDialog) {
                return undefined;
            }
            return api.OpenFileDialog(options);
        },
        OpenDirectoryDialog: async (options) => {
            const api = getPywebviewApi();
            if (!api?.OpenDirectoryDialog) {
                return undefined;
            }
            return api.OpenDirectoryDialog(options);
        },
        ResolveFilePaths: async (files) => {
            const api = getPywebviewApi();
            if (!api?.ResolveFilePaths) {
                return [];
            }
            return api.ResolveFilePaths(files);
        },
        CanResolveFilePaths: () => {
            const api = getPywebviewApi();
            if (!api?.CanResolveFilePaths) {
                return false;
            }
            return Boolean(api.CanResolveFilePaths());
        },
    };
}

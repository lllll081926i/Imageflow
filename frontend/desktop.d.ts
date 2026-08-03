import type { AppBindings } from './types/backend-bindings';

export {};

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

type DesktopRuntimeBridge = {
  Quit?: () => void;
  WindowMinimise?: () => void;
  WindowToggleMaximise?: () => void;
  OnFileDrop?: (callback: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean) => void;
  OnFileDropOff?: () => void;
  OpenFileDialog?: (options?: RuntimeOpenFileDialogOptions) => Promise<string | string[] | null | undefined>;
  OpenDirectoryDialog?: (options?: RuntimeOpenDirectoryDialogOptions) => Promise<string | null | undefined>;
  ResolveFilePaths?: (files: File[]) => Promise<unknown> | unknown;
  CanResolveFilePaths?: () => boolean;
};

declare global {
  interface Window {
    runtime?: DesktopRuntimeBridge;
    pywebview?: {
      api?: Partial<AppBindings> & {
        OpenFileDialog?: (options?: RuntimeOpenFileDialogOptions) => Promise<string | string[] | null | undefined>;
        OpenDirectoryDialog?: (options?: RuntimeOpenDirectoryDialogOptions) => Promise<string | null | undefined>;
        Quit?: () => void | Promise<void>;
        WindowMinimise?: () => void | Promise<void>;
        WindowToggleMaximise?: () => void | Promise<void>;
        ResolveFilePaths?: (files: File[]) => Promise<unknown> | unknown;
        CanResolveFilePaths?: () => boolean;
      };
    };
  }
}


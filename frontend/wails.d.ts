import type { AppBindings } from './types/wails-api';

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

type WailsRuntimeBridge = {
  Quit?: () => void;
  WindowMinimise?: () => void;
  WindowToggleMaximise?: () => void;
  OnFileDrop?: (callback: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean) => void;
  OnFileDropOff?: () => void;
  OpenFileDialog?: (options?: RuntimeOpenFileDialogOptions) => Promise<string | string[] | null | undefined>;
  OpenDirectoryDialog?: (options?: RuntimeOpenDirectoryDialogOptions) => Promise<string | null | undefined>;
};

declare global {
  interface Window {
    runtime?: WailsRuntimeBridge;
    go?: {
      main?: {
        App?: Partial<AppBindings>;
      };
    };
  }
}


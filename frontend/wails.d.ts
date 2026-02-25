import type { AppBindings } from './types/wails-api';

export {};

declare global {
  interface Window {
    runtime?: unknown;
    go?: {
      main?: {
        App?: Partial<AppBindings>;
      };
    };
  }
}


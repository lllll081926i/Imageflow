import type { AppBindings } from './backend-bindings';
export type DesktopBindings = Partial<AppBindings> & Record<string, unknown>;

type PywebviewWindow = Window & {
    pywebview?: {
        api?: DesktopBindings;
    };
};

export function getDesktopBindings(): DesktopBindings | null {
    const pywebviewApi = (window as PywebviewWindow).pywebview?.api;
    if (pywebviewApi) {
        return pywebviewApi;
    }

    const goApi = window.go?.main?.App;
    if (goApi) {
        return goApi as DesktopBindings;
    }

    return null;
}

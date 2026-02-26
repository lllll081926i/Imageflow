import type * as AppModule from '../wailsjs/go/main/App';

export type AppBindings = typeof AppModule;

export function getAppBindings(): Partial<AppBindings> | null {
    const app = window.go?.main?.App;
    if (!app) return null;
    return app as Partial<AppBindings>;
}

// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';

import { getAppBindings } from './wails-api';

type MutableWindow = Window & {
    go?: { main?: { App?: unknown } };
    pywebview?: { api?: unknown };
};

describe('getAppBindings', () => {
    afterEach(() => {
        const mutableWindow = window as MutableWindow;
        delete mutableWindow.pywebview;
        delete mutableWindow.go;
    });

    it('prefers pywebview api when available', () => {
        const pyApi = { Ping: () => Promise.resolve('pong') };
        const goApi = { Ping: () => Promise.resolve('go-pong') };
        const mutableWindow = window as MutableWindow;
        mutableWindow.pywebview = { api: pyApi as any };
        mutableWindow.go = { main: { App: goApi } };

        expect(getAppBindings()).toBe(pyApi);
    });
});

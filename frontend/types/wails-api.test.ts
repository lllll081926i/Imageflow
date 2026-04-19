// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';

import { DEFAULT_APP_SETTINGS, getAppBindings, normalizeAppSettings, pushRecentPath } from './wails-api';

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

describe('normalizeAppSettings', () => {
    it('对非法并发值回退默认值而不是产生 NaN', () => {
        const normalized = normalizeAppSettings({ max_concurrency: 'bad' as unknown as number });

        expect(normalized.max_concurrency).toBe(DEFAULT_APP_SETTINGS.max_concurrency);
    });

    it('路径去重时把反斜杠和正斜杠视为同一路径', () => {
        const normalized = normalizeAppSettings({
            recent_input_dirs: ['D:\\Input', 'd:/input///', 'E:/Other'],
        });

        expect(normalized.recent_input_dirs).toEqual(['D:\\Input', 'E:/Other']);
        expect(pushRecentPath(['D:\\Input'], 'd:/input///')).toEqual(['d:/input']);
    });
});

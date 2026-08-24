// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';

import { DEFAULT_APP_SETTINGS, getAppBindings, normalizeAppSettings, pushRecentPath } from './app-api';
import { __thumbnailSchedulerForTests } from '../components/thumbnails';

type MutableWindow = Window & {
    pywebview?: { api?: unknown };
};

describe('getAppBindings', () => {
    afterEach(() => {
        const mutableWindow = window as MutableWindow;
        delete mutableWindow.pywebview;
    });

    it('uses pywebview api when available', () => {
        const pyApi = { Ping: () => Promise.resolve('pong') };
        const mutableWindow = window as MutableWindow;
        mutableWindow.pywebview = { api: pyApi as any };

        expect(getAppBindings()).toBe(pyApi);
    });

    it('returns null when no bridge is present', () => {
        expect(getAppBindings()).toBeNull();
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

describe('thumbnail request scheduler', () => {
    afterEach(() => {
        __thumbnailSchedulerForTests.reset();
    });

    it('同一时刻最多执行四个缩略图解码请求', async () => {
        let active = 0;
        let maxActive = 0;
        const releases: Array<() => void> = [];
        const requests = Array.from({ length: 9 }, (_, index) => (
            __thumbnailSchedulerForTests.request(`thumb-${index}`, async () => {
                active += 1;
                maxActive = Math.max(maxActive, active);
                await new Promise<void>((resolve) => releases.push(resolve));
                active -= 1;
                return `data:image/webp;base64,${index}`;
            })
        ));

        await Promise.resolve();
        expect(active).toBe(4);
        expect(maxActive).toBe(4);

        let released = 0;
        while (released < requests.length) {
            while (releases.length === 0) {
                await Promise.resolve();
            }
            const batch = releases.splice(0);
            released += batch.length;
            batch.forEach((release) => release());
            await Promise.resolve();
        }
        await Promise.all(requests);
        expect(maxActive).toBe(4);
    });
});

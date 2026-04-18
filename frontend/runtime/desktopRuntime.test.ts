// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest';

import { installDesktopRuntime } from './desktopRuntime';

type MutableWindow = Window & {
    runtime?: Window['runtime'];
    pywebview?: {
        api?: Record<string, unknown>;
    };
};

const mutableWindow = window as MutableWindow;

describe('installDesktopRuntime', () => {
    afterEach(() => {
        delete mutableWindow.runtime;
        delete mutableWindow.pywebview;
        document.body.innerHTML = '';
        vi.restoreAllMocks();
    });

    const mockElementFromPoint = (value: Element | null) => {
        Object.defineProperty(document, 'elementFromPoint', {
            configurable: true,
            value: vi.fn().mockReturnValue(value),
        });
        return document.elementFromPoint as unknown as ReturnType<typeof vi.fn>;
    };

    it('通过自定义事件向前端回传 pywebview 拖拽路径', () => {
        mutableWindow.pywebview = { api: {} };
        installDesktopRuntime();

        const target = document.createElement('div');
        target.style.setProperty('--wails-drop-target', 'drop');
        document.body.appendChild(target);

        const callback = vi.fn();
        mutableWindow.runtime?.OnFileDrop?.(callback, true);

        const pointSpy = mockElementFromPoint(target);
        window.dispatchEvent(new CustomEvent('__imageflow_file_drop__', {
            detail: {
                x: 12,
                y: 34,
                paths: ['D:/drop/a.png', 'D:/drop/b.png'],
            },
        }));

        expect(pointSpy).toHaveBeenCalledWith(12, 34);
        expect(callback).toHaveBeenCalledWith(12, 34, ['D:/drop/a.png', 'D:/drop/b.png']);
    });

    it('在要求命中 drop target 时忽略非目标区域拖拽', () => {
        mutableWindow.pywebview = { api: {} };
        installDesktopRuntime();

        const callback = vi.fn();
        mutableWindow.runtime?.OnFileDrop?.(callback, true);
        mockElementFromPoint(document.body);

        window.dispatchEvent(new CustomEvent('__imageflow_file_drop__', {
            detail: {
                x: 1,
                y: 2,
                paths: ['D:/drop/outside.png'],
            },
        }));

        expect(callback).not.toHaveBeenCalled();
    });

    it('注销后不再响应文件拖拽事件', () => {
        mutableWindow.pywebview = { api: {} };
        installDesktopRuntime();

        const callback = vi.fn();
        mutableWindow.runtime?.OnFileDrop?.(callback, false);
        mutableWindow.runtime?.OnFileDropOff?.();

        window.dispatchEvent(new CustomEvent('__imageflow_file_drop__', {
            detail: {
                x: 5,
                y: 6,
                paths: ['D:/drop/after-off.png'],
            },
        }));

        expect(callback).not.toHaveBeenCalled();
    });
});

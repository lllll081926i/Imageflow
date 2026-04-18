// @vitest-environment jsdom
import React, { act, forwardRef, useImperativeHandle } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
    buildGifProcessSuffix,
    detectAnimatedImagePath,
    getGifModesForInputKind,
    getPreferredGifModeForInputKind,
    planIcoConversionSizes,
    resolveConverterOverwritePath,
    resolveGifAction,
    summarizeGifInputPaths,
} from './gifHelpers';
import { CustomSelect } from './Controls';
import { useGifResizeState } from './hooks/useGifResizeState';
import { getAppBindings, resolveSelectedFilePaths } from '../types/wails-api';
import { FEATURES } from '../constants';
import { shouldPreventWindowDragEvent } from '../App';

vi.mock('../types/wails-api', async () => {
    const actual = await vi.importActual<typeof import('../types/wails-api')>('../types/wails-api');
    return {
        ...actual,
        getAppBindings: vi.fn(),
    };
});

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

type HookSnapshot = ReturnType<typeof useGifResizeState>;

const GifResizeStateProbe = forwardRef<HookSnapshot | null, { featureId: string; files: Array<{ input_path: string }> }>(
    ({ featureId, files }, ref) => {
        const state = useGifResizeState({
            featureId,
            files,
            isGifPath: (path: string) => path.toLowerCase().endsWith('.gif'),
            normalizePath: (path: string) => path.replace(/\\/g, '/'),
        });
        useImperativeHandle(ref, () => state, [state]);
        return null;
    }
);

GifResizeStateProbe.displayName = 'GifResizeStateProbe';

const flushMicrotasks = async () => {
    await Promise.resolve();
    await Promise.resolve();
};

const renderElement = async (element: React.ReactElement) => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);
    await act(async () => {
        root.render(element);
        await flushMicrotasks();
    });
    return { container, root };
};

afterEach(() => {
    vi.clearAllMocks();
    document.body.innerHTML = '';
});

describe('resolveSelectedFilePaths', () => {
    it('优先返回 File 对象自带的本地路径', async () => {
        const files = [
            { path: 'C:/tmp/a.png', name: 'a.png' },
            { path: 'C:/tmp/b.png', name: 'b.png' },
        ] as any;

        await expect(resolveSelectedFilePaths(files)).resolves.toEqual(['C:/tmp/a.png', 'C:/tmp/b.png']);
    });

    it('支持 pywebview 拖拽事件提供的 pywebviewFullPath', async () => {
        const files = [
            { pywebviewFullPath: 'D:/drop/a.png', name: 'a.png' },
            { pywebviewFullPath: 'D:/drop/b.png', name: 'b.png' },
        ] as any;

        await expect(resolveSelectedFilePaths(files)).resolves.toEqual(['D:/drop/a.png', 'D:/drop/b.png']);
    });

    it('在没有直接路径时会使用运行时解析本地路径', async () => {
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(true),
            ResolveFilePaths: vi.fn().mockResolvedValue(['D:/resolved/a.png']),
        };
        const files = [{ name: 'a.png' }] as any;

        await expect(resolveSelectedFilePaths(files, runtime as any)).resolves.toEqual(['D:/resolved/a.png']);
        expect(runtime.ResolveFilePaths).toHaveBeenCalledWith(files);
    });

    it('在既没有直接路径也没有运行时解析能力时返回空数组，而不是文件名', async () => {
        const files = [{ name: 'a.png' }, { name: 'b.png' }] as any;

        await expect(resolveSelectedFilePaths(files)).resolves.toEqual([]);
    });

    it('在运行时明确声明无法解析文件路径时直接返回空数组', async () => {
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(false),
            ResolveFilePaths: vi.fn(),
        };
        const files = [{ name: 'a.png' }] as any;

        await expect(resolveSelectedFilePaths(files, runtime as any)).resolves.toEqual([]);
        expect(runtime.ResolveFilePaths).not.toHaveBeenCalled();
    });

    it('在运行时返回非数组结果时回退为空数组', async () => {
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(true),
            ResolveFilePaths: vi.fn().mockResolvedValue('D:/resolved/a.png'),
        };
        const files = [{ name: 'a.png' }] as any;

        await expect(resolveSelectedFilePaths(files, runtime as any)).resolves.toEqual([]);
    });

    it('在运行时抛错时回退为空数组', async () => {
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(true),
            ResolveFilePaths: vi.fn().mockRejectedValue(new Error('boom')),
        };
        const files = [{ name: 'a.png' }] as any;
        const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

        await expect(resolveSelectedFilePaths(files, runtime as any)).resolves.toEqual([]);

        errorSpy.mockRestore();
    });
});

describe('resolveGifAction', () => {
    it('maps 倒放 to reverse', () => {
        expect(resolveGifAction('倒放')).toBe('reverse');
    });

    it('maps 修改帧率 to change_speed', () => {
        expect(resolveGifAction('修改帧率')).toBe('change_speed');
    });

    it('maps 压缩 to compress', () => {
        expect(resolveGifAction('压缩')).toBe('compress');
    });

    it('maps 缩放 to resize', () => {
        expect(resolveGifAction('缩放')).toBe('resize');
    });

    it('maps 互转 to convert_animation', () => {
        expect(resolveGifAction('互转')).toBe('convert_animation');
    });

    it('falls back to change_speed for unknown mode', () => {
        expect(resolveGifAction('未知模式')).toBe('change_speed');
    });
});

describe('buildGifProcessSuffix', () => {
    it('returns reverse suffix', () => {
        expect(buildGifProcessSuffix('reverse', 100, 90)).toBe('_reverse');
    });

    it('returns speed suffix', () => {
        expect(buildGifProcessSuffix('change_speed', 125, 90)).toBe('_speed_125');
    });

    it('returns compress suffix', () => {
        expect(buildGifProcessSuffix('compress', 100, 77)).toBe('_compress_q77');
    });

    it('returns resize suffix', () => {
        expect(buildGifProcessSuffix('resize', 100, 77, 320, 180)).toBe('_resize_320x180');
    });

    it('returns convert suffix', () => {
        expect(buildGifProcessSuffix('convert_animation', 100, 77, 0, 0, 'WEBP')).toBe('_to_webp');
    });
});

describe('resolveConverterOverwritePath', () => {
    it('在目标格式与源扩展一致时允许原位覆盖', () => {
        expect(resolveConverterOverwritePath('C:/tmp/sample.png', 'png')).toBe('C:/tmp/sample.png');
        expect(resolveConverterOverwritePath('C:/tmp/sample.jpg', 'jpeg')).toBe('C:/tmp/sample.jpg');
    });

    it('在目标格式与源扩展不一致时切换到同目录新扩展名', () => {
        expect(resolveConverterOverwritePath('C:/tmp/sample.png', 'jpg')).toBe('C:/tmp/sample.jpg');
        expect(resolveConverterOverwritePath('C:/tmp/sample.webp', 'avif')).toBe('C:/tmp/sample.avif');
    });
});

describe('planIcoConversionSizes', () => {
    it('在未选择尺寸时保留空数组，让后端决定默认尺寸集', () => {
        expect(planIcoConversionSizes([], false)).toEqual([[]]);
    });

    it('在非覆盖模式下为一个 ICO 请求保留多尺寸', () => {
        expect(planIcoConversionSizes([64, 16, 32, 32], false)).toEqual([[16, 32, 64]]);
    });

    it('在覆盖源文件模式下拆成多个单尺寸任务，避免同一路径冲突', () => {
        expect(planIcoConversionSizes([64, 16, 32], true)).toEqual([[16], [32], [64]]);
    });
});

describe('detectAnimatedImagePath', () => {
    it('对 gif 和 apng 扩展名直接判定为动图', async () => {
        const probe = vi.fn();

        await expect(detectAnimatedImagePath('C:/tmp/a.gif', probe)).resolves.toBe(true);
        await expect(detectAnimatedImagePath('C:/tmp/a.apng', probe)).resolves.toBe(true);

        expect(probe).not.toHaveBeenCalled();
    });

    it('会探测 png 的帧数来识别 apng', async () => {
        const probe = vi.fn().mockResolvedValue(2);

        await expect(detectAnimatedImagePath('C:/tmp/a.png', probe)).resolves.toBe(true);
        expect(probe).toHaveBeenCalledWith('C:/tmp/a.png');
    });

    it('会把单帧 webp 识别为静态图片', async () => {
        const probe = vi.fn().mockResolvedValue(1);

        await expect(detectAnimatedImagePath('C:/tmp/a.webp', probe)).resolves.toBe(false);
        expect(probe).toHaveBeenCalledWith('C:/tmp/a.webp');
    });
});

describe('summarizeGifInputPaths', () => {
    it('会把带 png 扩展名的 APNG 归类为 animated，而不是 images', async () => {
        const probe = vi.fn().mockResolvedValue(4);

        await expect(summarizeGifInputPaths(['C:/tmp/sample.png'], probe)).resolves.toMatchObject({
            kind: 'animated',
            hasAnimated: true,
            hasStatic: false,
            hasGif: false,
            hasNonGifAnimated: true,
        });
    });

    it('会把单帧 webp 归类为 images', async () => {
        const probe = vi.fn().mockResolvedValue(1);

        await expect(summarizeGifInputPaths(['C:/tmp/still.webp'], probe)).resolves.toMatchObject({
            kind: 'images',
            hasAnimated: false,
            hasStatic: true,
            hasGif: false,
            hasNonGifAnimated: false,
        });
    });

    it('会把 gif 与 apng 混合输入归类为 animated，以允许导出和互转', async () => {
        const probe = vi.fn().mockResolvedValue(3);

        await expect(summarizeGifInputPaths(['C:/tmp/a.gif', 'C:/tmp/b.png'], probe)).resolves.toMatchObject({
            kind: 'animated',
            hasAnimated: true,
            hasStatic: false,
            hasGif: true,
            hasNonGifAnimated: true,
        });
    });

    it('会把动图和静态图混合输入归类为 mixed', async () => {
        const probe = vi.fn().mockResolvedValue(2);

        await expect(summarizeGifInputPaths(['C:/tmp/a.png', 'C:/tmp/b.jpg'], probe)).resolves.toMatchObject({
            kind: 'mixed',
            hasAnimated: true,
            hasStatic: true,
        });
    });
});

describe('GIF 输入模式选项', () => {
    it('animated 输入仅开放导出和互转', () => {
        expect(getGifModesForInputKind('animated')).toEqual(['导出', '互转']);
        expect(getPreferredGifModeForInputKind('animated')).toBe('互转');
    });

    it('gif 输入保留全部模式', () => {
        expect(getGifModesForInputKind('gif')).toEqual(['导出', '互转', '倒放', '修改帧率', '压缩', '缩放']);
        expect(getPreferredGifModeForInputKind('gif')).toBe('导出');
    });
});

describe('useGifResizeState', () => {
    it('在 GIF 模式下会拉取原始尺寸并初始化宽高', async () => {
        const mockedGetAppBindings = vi.mocked(getAppBindings);
        const getInfo = vi.fn().mockResolvedValue({ success: true, width: 320, height: 160 });
        mockedGetAppBindings.mockReturnValue({ GetInfo: getInfo } as any);

        const ref = React.createRef<HookSnapshot>();
        let root: Root | null = null;
        await act(async () => {
            const container = document.createElement('div');
            document.body.appendChild(container);
            root = createRoot(container);
            root.render(React.createElement(GifResizeStateProbe, { ref, featureId: 'gif', files: [{ input_path: 'C:/tmp/a.gif' }] }));
            await flushMicrotasks();
        });

        expect(getInfo).toHaveBeenCalledTimes(1);
        expect(ref.current?.gifResizeWidth).toBe(320);
        expect(ref.current?.gifResizeHeight).toBe(160);
        expect(ref.current?.gifOriginalSize).toEqual({ width: 320, height: 160 });
        await act(async () => {
            root?.unmount();
            await flushMicrotasks();
        });
    });

    it('保持宽高比时，修改宽度会自动联动高度', async () => {
        const mockedGetAppBindings = vi.mocked(getAppBindings);
        mockedGetAppBindings.mockReturnValue({
            GetInfo: vi.fn().mockResolvedValue({ success: true, width: 300, height: 150 }),
        } as any);

        const ref = React.createRef<HookSnapshot>();
        let root: Root | null = null;
        await act(async () => {
            const container = document.createElement('div');
            document.body.appendChild(container);
            root = createRoot(container);
            root.render(React.createElement(GifResizeStateProbe, { ref, featureId: 'gif', files: [{ input_path: 'C:/tmp/a.gif' }] }));
            await flushMicrotasks();
        });

        await act(async () => {
            ref.current?.onResizeWidthChange(600);
            await flushMicrotasks();
        });

        expect(ref.current?.gifResizeWidth).toBe(600);
        expect(ref.current?.gifResizeHeight).toBe(300);
        await act(async () => {
            root?.unmount();
            await flushMicrotasks();
        });
    });

    it('非 GIF 功能页不会触发尺寸读取', async () => {
        const mockedGetAppBindings = vi.mocked(getAppBindings);
        const getInfo = vi.fn().mockResolvedValue({ success: true, width: 200, height: 100 });
        mockedGetAppBindings.mockReturnValue({ GetInfo: getInfo } as any);

        const ref = React.createRef<HookSnapshot>();
        let root: Root | null = null;
        await act(async () => {
            const container = document.createElement('div');
            document.body.appendChild(container);
            root = createRoot(container);
            root.render(React.createElement(GifResizeStateProbe, { ref, featureId: 'converter', files: [{ input_path: 'C:/tmp/a.gif' }] }));
            await flushMicrotasks();
        });

        expect(getInfo).not.toHaveBeenCalled();
        expect(ref.current?.gifResizeWidth).toBe(0);
        expect(ref.current?.gifResizeHeight).toBe(0);
        await act(async () => {
            root?.unmount();
            await flushMicrotasks();
        });
    });
});

describe('FEATURES', () => {
    it('保留 GIF 工具并单独提供字幕拼接入口', () => {
        const gifFeature = FEATURES.find((feature) => feature.id === 'gif');
        const subtitleFeature = FEATURES.find((feature) => feature.id === 'subtitle_stitch');

        expect(gifFeature?.title).toBe('GIF 工具');
        expect(subtitleFeature?.title).toBe('字幕拼接');
    });
});

describe('shouldPreventWindowDragEvent', () => {
    it('仅拦截文件拖拽，不拦截普通文本或链接拖拽', () => {
        expect(shouldPreventWindowDragEvent({ types: ['Files'] } as any)).toBe(true);
        expect(shouldPreventWindowDragEvent({ types: ['text/plain'] } as any)).toBe(false);
        expect(shouldPreventWindowDragEvent(null)).toBe(false);
    });
});

describe('CustomSelect', () => {
    it('提供 listbox 语义并支持键盘选择', async () => {
        const onChange = vi.fn();
        const { container, root } = await renderElement(
            React.createElement(CustomSelect, {
                label: '目标格式',
                options: ['PNG', 'JPG', 'WEBP'],
                value: 'PNG',
                onChange,
            })
        );

        const trigger = container.querySelector('button');
        expect(trigger).toBeTruthy();
        expect(trigger?.getAttribute('aria-haspopup')).toBe('listbox');

        await act(async () => {
            trigger?.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
            await flushMicrotasks();
        });

        const listbox = document.body.querySelector('[role="listbox"]');
        expect(listbox).toBeTruthy();
        expect(trigger?.getAttribute('aria-expanded')).toBe('true');

        const options = Array.from(document.body.querySelectorAll('[role="option"]'));
        expect(options).toHaveLength(3);
        expect(options[0]?.getAttribute('aria-selected')).toBe('true');

        await act(async () => {
            trigger?.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
            trigger?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
            await flushMicrotasks();
        });

        expect(onChange).toHaveBeenCalledWith('JPG');
        expect(document.body.querySelector('[role="listbox"]')).toBeNull();

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

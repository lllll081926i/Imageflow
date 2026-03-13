import React, { forwardRef, useImperativeHandle } from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
    buildGifProcessSuffix,
    detectAnimatedImagePath,
    resolveConverterOverwritePath,
    resolveGifAction,
} from './gifHelpers';
import { useGifResizeState } from './hooks/useGifResizeState';
import { getAppBindings } from '../types/wails-api';
import { FEATURES } from '../constants';

vi.mock('../types/wails-api', () => ({
    getAppBindings: vi.fn(),
}));

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

afterEach(() => {
    vi.clearAllMocks();
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

describe('useGifResizeState', () => {
    it('在 GIF 模式下会拉取原始尺寸并初始化宽高', async () => {
        const mockedGetAppBindings = vi.mocked(getAppBindings);
        const getInfo = vi.fn().mockResolvedValue({ success: true, width: 320, height: 160 });
        mockedGetAppBindings.mockReturnValue({ GetInfo: getInfo } as any);

        const ref = React.createRef<HookSnapshot>();
        let renderer: ReactTestRenderer | null = null;
        await act(async () => {
            renderer = create(React.createElement(GifResizeStateProbe, { ref, featureId: 'gif', files: [{ input_path: 'C:/tmp/a.gif' }] }));
            await flushMicrotasks();
        });

        expect(getInfo).toHaveBeenCalledTimes(1);
        expect(ref.current?.gifResizeWidth).toBe(320);
        expect(ref.current?.gifResizeHeight).toBe(160);
        expect(ref.current?.gifOriginalSize).toEqual({ width: 320, height: 160 });
        await act(async () => {
            renderer?.unmount();
            await flushMicrotasks();
        });
    });

    it('保持宽高比时，修改宽度会自动联动高度', async () => {
        const mockedGetAppBindings = vi.mocked(getAppBindings);
        mockedGetAppBindings.mockReturnValue({
            GetInfo: vi.fn().mockResolvedValue({ success: true, width: 300, height: 150 }),
        } as any);

        const ref = React.createRef<HookSnapshot>();
        let renderer: ReactTestRenderer | null = null;
        await act(async () => {
            renderer = create(React.createElement(GifResizeStateProbe, { ref, featureId: 'gif', files: [{ input_path: 'C:/tmp/a.gif' }] }));
            await flushMicrotasks();
        });

        await act(async () => {
            ref.current?.onResizeWidthChange(600);
            await flushMicrotasks();
        });

        expect(ref.current?.gifResizeWidth).toBe(600);
        expect(ref.current?.gifResizeHeight).toBe(300);
        await act(async () => {
            renderer?.unmount();
            await flushMicrotasks();
        });
    });

    it('非 GIF 功能页不会触发尺寸读取', async () => {
        const mockedGetAppBindings = vi.mocked(getAppBindings);
        const getInfo = vi.fn().mockResolvedValue({ success: true, width: 200, height: 100 });
        mockedGetAppBindings.mockReturnValue({ GetInfo: getInfo } as any);

        const ref = React.createRef<HookSnapshot>();
        let renderer: ReactTestRenderer | null = null;
        await act(async () => {
            renderer = create(React.createElement(GifResizeStateProbe, { ref, featureId: 'converter', files: [{ input_path: 'C:/tmp/a.gif' }] }));
            await flushMicrotasks();
        });

        expect(getInfo).not.toHaveBeenCalled();
        expect(ref.current?.gifResizeWidth).toBe(0);
        expect(ref.current?.gifResizeHeight).toBe(0);
        await act(async () => {
            renderer?.unmount();
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

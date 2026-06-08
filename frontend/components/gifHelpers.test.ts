// @vitest-environment jsdom
import React, { act, forwardRef, useImperativeHandle } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
    buildGifProcessSuffix,
    detectAnimatedImagePath,
    getGifModesForInputKind,
    getPreferredGifModeForInputKind,
    normalizeGifSpeedPercent,
    planIcoConversionSizes,
    resolveConverterOverwritePath,
    resolveGifAction,
    resolveWatermarkBackendPosition,
    selectAnimatedProbeCandidatePaths,
    summarizeGifInputPaths,
} from './gifHelpers';
import { CustomSelect, FileDropZone } from './Controls';
import DetailView from './DetailView';
import GifSettingsPanel from './GifSettingsPanel';
import { useGifResizeState } from './hooks/useGifResizeState';
import { getAppBindings, resolveSelectedFilePaths } from '../types/wails-api';
import { FEATURES } from '../constants';

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
    vi.useRealTimers();
    vi.clearAllMocks();
    document.body.innerHTML = '';
    delete (window as { runtime?: unknown }).runtime;
    delete (window as { pywebview?: unknown }).pywebview;
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

    it('混合直接路径和待解析文件时会保留直接路径并补齐缺失路径', async () => {
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(true),
            ResolveFilePaths: vi.fn().mockResolvedValue(['D:/resolved/a.png', 'D:/resolved/d.png']),
        };
        const files = [
            { name: 'a.png' },
            { path: 'C:/direct/b.png', name: 'b.png' },
            { pywebviewFullPath: 'E:/drop/c.png', name: 'c.png' },
            { name: 'd.png' },
        ] as any;

        await expect(resolveSelectedFilePaths(files, runtime as any)).resolves.toEqual([
            'D:/resolved/a.png',
            'C:/direct/b.png',
            'E:/drop/c.png',
            'D:/resolved/d.png',
        ]);
        expect(runtime.ResolveFilePaths).toHaveBeenCalledWith([files[0], files[3]]);
    });

    it('混合文件中缺失路径无法补齐时回退为空数组，避免静默漏处理文件', async () => {
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(true),
            ResolveFilePaths: vi.fn().mockResolvedValue([]),
        };
        const files = [
            { path: 'C:/direct/a.png', name: 'a.png' },
            { name: 'b.png' },
        ] as any;

        await expect(resolveSelectedFilePaths(files, runtime as any)).resolves.toEqual([]);
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

    it('运行时解析本地路径超时时回退为空数组', async () => {
        vi.useFakeTimers();
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(true),
            ResolveFilePaths: vi.fn().mockReturnValue(new Promise(() => {})),
        };
        const files = [{ name: 'slow.png' }] as any;

        const promise = resolveSelectedFilePaths(files, runtime as any, { timeoutMs: 25 });
        await vi.advanceTimersByTimeAsync(25);

        await expect(promise).resolves.toEqual([]);
    });
});

describe('FileDropZone', () => {
    it('拖入文件后在等待运行时路径解析时展示进行中状态', async () => {
        vi.useFakeTimers();
        const runtime = {
            CanResolveFilePaths: vi.fn().mockReturnValue(true),
            ResolveFilePaths: vi.fn().mockReturnValue(new Promise(() => {})),
        };
        (window as { runtime?: unknown }).runtime = runtime;
        vi.mocked(getAppBindings).mockReturnValue({
            ExpandDroppedPaths: vi.fn(),
        } as any);

        const { container, root } = await renderElement(React.createElement(FileDropZone, {
            onFilesSelected: vi.fn(),
            onPathsExpanded: vi.fn(),
        }));
        const dropTarget = container.firstElementChild as HTMLElement;
        const file = {
            name: 'slow.png',
            size: 12,
            lastModified: 1710000000000,
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
        });

        expect(container.textContent).toContain('正在解析文件路径');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });

    it('在桌面端拖拽已携带 pywebviewFullPath 的文件时直接展开路径，不等待 runtime 回传', async () => {
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: false,
            files: [
                {
                    input_path: 'D:/drop/a.png',
                    source_root: 'D:/drop',
                    relative_path: 'a.png',
                    is_from_dir_drop: false,
                    size: 12,
                    mod_time: 1710000000,
                },
            ],
        });
        vi.mocked(getAppBindings).mockReturnValue({
            ExpandDroppedPaths: expandDroppedPaths,
        } as any);

        const onFilesSelected = vi.fn();
        const onPathsExpanded = vi.fn();
        const onFileDrop = vi.fn();
        const onFileDropOff = vi.fn();
        (window as { runtime?: unknown }).runtime = {
            OnFileDrop: onFileDrop,
            OnFileDropOff: onFileDropOff,
        };

        const { container, root } = await renderElement(React.createElement(FileDropZone, {
            onFilesSelected,
            onPathsExpanded,
        }));
        const dropTarget = container.firstElementChild as HTMLElement;
        const file = {
            name: 'a.png',
            size: 12,
            lastModified: 1710000000000,
            pywebviewFullPath: 'D:/drop/a.png',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(onFileDrop).toHaveBeenCalledTimes(1);
        expect(expandDroppedPaths).toHaveBeenCalledWith(['D:/drop/a.png']);
        expect(onFilesSelected).toHaveBeenCalledWith([file]);
        expect(onPathsExpanded).toHaveBeenCalledWith({
            has_directory: false,
            files: [
                {
                    input_path: 'D:/drop/a.png',
                    source_root: 'D:/drop',
                    relative_path: 'a.png',
                    is_from_dir_drop: false,
                    size: 12,
                    mod_time: 1710000000,
                },
            ],
        });

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
        expect(onFileDropOff).toHaveBeenCalledTimes(1);
    });

    it('大目录拖入后默认折叠节点，避免首屏渲染全部子项', async () => {
        const nestedFiles = Array.from({ length: 121 }, (_, index) => ({
            input_path: `D:/drop/batch/group/file-${index}.png`,
            source_root: 'D:/drop/batch',
            relative_path: `group/file-${index}.png`,
            is_from_dir_drop: true,
            size: 12,
            mod_time: 1710000000 + index,
        }));
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: true,
            files: nestedFiles,
        });
        vi.mocked(getAppBindings).mockReturnValue({
            ExpandDroppedPaths: expandDroppedPaths,
        } as any);

        (window as { runtime?: unknown }).runtime = {
            OnFileDrop: vi.fn(),
            OnFileDropOff: vi.fn(),
        };

        const { container, root } = await renderElement(React.createElement(FileDropZone, {
            onFilesSelected: vi.fn(),
            onPathsExpanded: vi.fn(),
        }));
        const dropTarget = container.firstElementChild as HTMLElement;
        const file = {
            name: 'batch',
            size: 0,
            lastModified: 1710000000000,
            pywebviewFullPath: 'D:/drop/batch',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(container.textContent).toContain('batch');
        expect(container.textContent).toContain('121 项');
        expect(container.textContent).not.toContain('file-0.png');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });

    it('桌面文件选择器使用传入的格式过滤规则', async () => {
        const selectInputFiles = vi.fn().mockResolvedValue([]);
        vi.mocked(getAppBindings).mockReturnValue({
            SelectInputFiles: selectInputFiles,
            ExpandDroppedPaths: vi.fn(),
        } as any);
        const filters = [{ DisplayName: 'Compressible bitmaps', Pattern: '*.jpg;*.jpeg;*.png;*.webp' }];

        const { container, root } = await renderElement(React.createElement(FileDropZone, {
            onFilesSelected: vi.fn(),
            onPathsExpanded: vi.fn(),
            fileDialogFilters: filters,
        }));
        const dropTarget = container.firstElementChild as HTMLElement;

        await act(async () => {
            dropTarget.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
            await flushMicrotasks();
        });

        expect(selectInputFiles).toHaveBeenCalledWith(expect.objectContaining({
            filters,
        }));

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });

    it('按 acceptedFormats 过滤后端展开结果，避免不可处理格式进入任务列表', async () => {
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: true,
            files: [
                {
                    input_path: 'D:/drop/photo.png',
                    source_root: 'D:/drop',
                    relative_path: 'photo.png',
                    is_from_dir_drop: true,
                    size: 12,
                    mod_time: 1710000000,
                },
                {
                    input_path: 'D:/drop/camera.heic',
                    source_root: 'D:/drop',
                    relative_path: 'camera.heic',
                    is_from_dir_drop: true,
                    size: 12,
                    mod_time: 1710000000,
                },
            ],
        });
        const onPathsExpanded = vi.fn();
        vi.mocked(getAppBindings).mockReturnValue({
            ExpandDroppedPaths: expandDroppedPaths,
        } as any);

        const { container, root } = await renderElement(React.createElement(FileDropZone, {
            onFilesSelected: vi.fn(),
            onPathsExpanded,
            acceptedFormats: '.jpg,.jpeg,.png,.webp',
        }));
        const dropTarget = container.firstElementChild as HTMLElement;
        const file = {
            name: 'drop',
            size: 0,
            lastModified: 1710000000000,
            pywebviewFullPath: 'D:/drop',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(onPathsExpanded).toHaveBeenCalledWith(expect.objectContaining({
            files: [expect.objectContaining({ relative_path: 'photo.png' })],
        }));
        expect(container.textContent).toContain('photo.png');
        expect(container.textContent).not.toContain('camera.heic');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });

    it('当所选文件全部被当前格式限制过滤时给出明确提示', async () => {
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: false,
            files: [{
                input_path: 'D:/drop/camera.heic',
                source_root: 'D:/drop',
                relative_path: 'camera.heic',
                is_from_dir_drop: false,
                size: 12,
                mod_time: 1710000000,
            }],
        });
        vi.mocked(getAppBindings).mockReturnValue({
            ExpandDroppedPaths: expandDroppedPaths,
        } as any);

        const { container, root } = await renderElement(React.createElement(FileDropZone, {
            onFilesSelected: vi.fn(),
            onPathsExpanded: vi.fn(),
            acceptedFormats: '.jpg,.jpeg,.png,.webp',
        }));
        const dropTarget = container.firstElementChild as HTMLElement;
        const file = {
            name: 'camera.heic',
            size: 12,
            lastModified: 1710000000000,
            pywebviewFullPath: 'D:/drop/camera.heic',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(container.textContent).toContain('没有符合当前功能格式限制的文件');
        expect(container.textContent).not.toContain('camera.heic');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

describe('DetailView 输出模板', () => {
    it('识别大小写不同的 prefix 占位符，避免输出文件名重复添加前缀', async () => {
        const settings = {
            max_concurrency: 4,
            output_prefix: 'IF',
            output_template: '{PREFIX}{basename}',
            preserve_folder_structure: true,
            conflict_strategy: 'rename',
            default_output_dir: 'D:/out',
            recent_input_dirs: [],
            recent_output_dirs: [],
        };
        const convert = vi.fn().mockResolvedValue({ success: true });
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: false,
            files: [{
                input_path: 'C:/input/sample.png',
                source_root: 'C:/input',
                relative_path: 'sample.png',
                is_from_dir_drop: false,
                size: 12,
                mod_time: 1710000000,
            }],
        });
        const resolveOutputPath = vi.fn(async (payload: { base_path: string }) => ({
            success: true,
            output_path: payload.base_path,
        }));
        const appBindings = {
            GetSettings: vi.fn().mockResolvedValue(settings),
            UpdateRecentPaths: vi.fn().mockResolvedValue(settings),
            ExpandDroppedPaths: expandDroppedPaths,
            ResolveOutputPath: resolveOutputPath,
            Convert: convert,
            ConvertBatch: vi.fn(),
        };
        vi.mocked(getAppBindings).mockReturnValue(appBindings as any);
        (window as { pywebview?: unknown }).pywebview = { api: appBindings };

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'converter',
            onBack: vi.fn(),
        }));
        const dropTarget = container.querySelector('[style*="--wails-drop-target"]') as HTMLElement;
        const file = {
            name: 'sample.png',
            size: 12,
            lastModified: 1710000000000,
            path: 'C:/input/sample.png',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        const startButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.includes('开始处理')) as HTMLButtonElement | undefined;
        expect(startButton).toBeTruthy();

        await act(async () => {
            startButton?.click();
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(convert).toHaveBeenCalledTimes(1);
        expect(convert.mock.calls[0]?.[0]?.output_path).toBe('D:/out/IF_sample.jpg');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

describe('DetailView 输入格式限制', () => {
    it('转换页不把当前环境无法解码的 HEIC/HEIF 暴露为普通转换输入', async () => {
        vi.mocked(getAppBindings).mockReturnValue({
            GetSettings: vi.fn().mockResolvedValue({
                max_concurrency: 4,
                output_prefix: 'IF',
                output_template: '{prefix}_{basename}',
                preserve_folder_structure: true,
                conflict_strategy: 'rename',
                default_output_dir: '',
                recent_input_dirs: [],
                recent_output_dirs: [],
            }),
        } as any);

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'converter',
            onBack: vi.fn(),
        }));
        const fileInput = container.querySelector('input[type="file"][accept]') as HTMLInputElement | null;

        expect(fileInput).toBeTruthy();
        const accept = fileInput?.getAttribute('accept') || '';
        expect(accept).toContain('.avif');
        expect(accept).toContain('.svg');
        expect(accept).not.toContain('.heic');
        expect(accept).not.toContain('.heif');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });

    it('信息查看页保留 HEIC/HEIF 输入声明用于读取容器信息', async () => {
        vi.mocked(getAppBindings).mockReturnValue({
            GetSettings: vi.fn().mockResolvedValue({
                max_concurrency: 4,
                output_prefix: 'IF',
                output_template: '{prefix}_{basename}',
                preserve_folder_structure: true,
                conflict_strategy: 'rename',
                default_output_dir: '',
                recent_input_dirs: [],
                recent_output_dirs: [],
            }),
        } as any);

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'info',
            onBack: vi.fn(),
        }));
        const fileInput = container.querySelector('input[type="file"][accept]') as HTMLInputElement | null;

        expect(fileInput).toBeTruthy();
        const accept = fileInput?.getAttribute('accept') || '';
        expect(accept).toContain('.heic');
        expect(accept).toContain('.heif');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });

    it('压缩页不把 SVG/GIF 暴露为普通压缩输入', async () => {
        vi.mocked(getAppBindings).mockReturnValue({
            GetSettings: vi.fn().mockResolvedValue({
                max_concurrency: 4,
                output_prefix: 'IF',
                output_template: '{prefix}_{basename}',
                preserve_folder_structure: true,
                conflict_strategy: 'rename',
                default_output_dir: '',
                recent_input_dirs: [],
                recent_output_dirs: [],
            }),
        } as any);

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'compressor',
            onBack: vi.fn(),
        }));
        const fileInput = container.querySelector('input[type="file"][accept]') as HTMLInputElement | null;

        expect(fileInput).toBeTruthy();
        const accept = fileInput?.getAttribute('accept') || '';
        expect(accept).toContain('.jpg');
        expect(accept).toContain('.png');
        expect(accept).toContain('.webp');
        expect(accept).not.toContain('image/*');
        expect(accept).not.toContain('.svg');
        expect(accept).not.toContain('.gif');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

describe('DetailView 图片压缩', () => {
    it('压缩 warning 显示为提示而不是未达目标', async () => {
        const settings = {
            max_concurrency: 4,
            output_prefix: '',
            output_template: '{basename}',
            preserve_folder_structure: false,
            conflict_strategy: 'rename',
            default_output_dir: 'D:/out',
            recent_input_dirs: [],
            recent_output_dirs: [],
        };
        const compress = vi.fn().mockResolvedValue({
            success: true,
            warning: '压缩结果大于原图，已保留原文件内容',
        });
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: false,
            files: [{
                input_path: 'C:/input/icon.ico',
                source_root: 'C:/input',
                relative_path: 'icon.ico',
                is_from_dir_drop: false,
                size: 12,
                mod_time: 1710000000,
            }],
        });
        const resolveOutputPath = vi.fn(async (payload: { base_path: string }) => ({
            success: true,
            output_path: payload.base_path,
        }));
        const appBindings = {
            GetSettings: vi.fn().mockResolvedValue(settings),
            UpdateRecentPaths: vi.fn().mockResolvedValue(settings),
            ExpandDroppedPaths: expandDroppedPaths,
            ResolveOutputPath: resolveOutputPath,
            Compress: compress,
            CompressBatch: vi.fn(),
        };
        vi.mocked(getAppBindings).mockReturnValue(appBindings as any);
        (window as { pywebview?: unknown }).pywebview = { api: appBindings };

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'compressor',
            onBack: vi.fn(),
        }));
        const dropTarget = container.querySelector('[style*="--wails-drop-target"]') as HTMLElement;
        const file = {
            name: 'icon.ico',
            size: 12,
            lastModified: 1710000000000,
            path: 'C:/input/icon.ico',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        const startButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.includes('开始处理')) as HTMLButtonElement | undefined;
        expect(startButton).toBeTruthy();

        await act(async () => {
            startButton?.click();
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(compress).toHaveBeenCalledTimes(1);
        expect(container.textContent).toContain('提示 1');
        expect(container.textContent).not.toContain('未达目标');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

describe('DetailView ICO 转换', () => {
    it('多尺寸 ICO 转换会向后端传递单尺寸参数并为输出文件添加尺寸后缀', async () => {
        const settings = {
            max_concurrency: 4,
            output_prefix: '',
            output_template: '{basename}',
            preserve_folder_structure: false,
            conflict_strategy: 'rename',
            default_output_dir: 'D:/out',
            recent_input_dirs: [],
            recent_output_dirs: [],
        };
        const convertBatch = vi.fn().mockResolvedValue([
            { success: true },
            { success: true },
        ]);
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: false,
            files: [{
                input_path: 'C:/input/sample.png',
                source_root: 'C:/input',
                relative_path: 'sample.png',
                is_from_dir_drop: false,
                size: 12,
                mod_time: 1710000000,
            }],
        });
        const resolveOutputPath = vi.fn(async (payload: { base_path: string }) => ({
            success: true,
            output_path: payload.base_path,
        }));
        const appBindings = {
            GetSettings: vi.fn().mockResolvedValue(settings),
            UpdateRecentPaths: vi.fn().mockResolvedValue(settings),
            ExpandDroppedPaths: expandDroppedPaths,
            ResolveOutputPath: resolveOutputPath,
            Convert: vi.fn(),
            ConvertBatch: convertBatch,
        };
        vi.mocked(getAppBindings).mockReturnValue(appBindings as any);
        (window as { pywebview?: unknown }).pywebview = { api: appBindings };

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'converter',
            onBack: vi.fn(),
        }));

        const formatTrigger = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.trim() === 'JPG') as HTMLButtonElement | undefined;
        expect(formatTrigger).toBeTruthy();

        await act(async () => {
            formatTrigger?.click();
            await flushMicrotasks();
        });

        const icoOption = Array.from(document.body.querySelectorAll('[role="option"]'))
            .find((option) => option.textContent?.trim() === 'ICO') as HTMLElement | undefined;
        expect(icoOption).toBeTruthy();

        await act(async () => {
            icoOption?.click();
            await flushMicrotasks();
        });

        for (const size of ['48px', '64px', '128px', '256px']) {
            const sizeButton = Array.from(container.querySelectorAll('button'))
                .find((button) => button.textContent?.trim() === size) as HTMLButtonElement | undefined;
            expect(sizeButton).toBeTruthy();
            await act(async () => {
                sizeButton?.click();
                await flushMicrotasks();
            });
        }

        const dropTarget = container.querySelector('[style*="--wails-drop-target"]') as HTMLElement;
        const file = {
            name: 'sample.png',
            size: 12,
            lastModified: 1710000000000,
            path: 'C:/input/sample.png',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        const startButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.includes('开始处理')) as HTMLButtonElement | undefined;
        expect(startButton).toBeTruthy();

        await act(async () => {
            startButton?.click();
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(convertBatch).toHaveBeenCalledTimes(1);
        const requests = convertBatch.mock.calls[0]?.[0] || [];
        expect(requests.map((item: any) => item.output_path.replace(/^.*\//, ''))).toEqual([
            'IF_sample_ico16.ico',
            'IF_sample_ico32.ico',
        ]);
        expect(new Set(requests.map((item: any) => item.output_path)).size).toBe(2);
        expect(requests.map((item: any) => item.ico_sizes)).toEqual([[16], [32]]);
        expect(requests.map((item: any) => item.icoSizes)).toEqual([[16], [32]]);

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

describe('DetailView 图片调整', () => {
    it('垂直翻转控件会把 flip_v 参数传给后端', async () => {
        const settings = {
            max_concurrency: 4,
            output_prefix: '',
            output_template: '{basename}',
            preserve_folder_structure: false,
            conflict_strategy: 'rename',
            default_output_dir: 'D:/out',
            recent_input_dirs: [],
            recent_output_dirs: [],
        };
        const adjust = vi.fn().mockResolvedValue({ success: true });
        const expandDroppedPaths = vi.fn().mockResolvedValue({
            has_directory: false,
            files: [{
                input_path: 'C:/input/sample.png',
                source_root: 'C:/input',
                relative_path: 'sample.png',
                is_from_dir_drop: false,
                size: 12,
                mod_time: 1710000000,
            }],
        });
        const resolveOutputPath = vi.fn(async (payload: { base_path: string }) => ({
            success: true,
            output_path: payload.base_path,
        }));
        const appBindings = {
            GetSettings: vi.fn().mockResolvedValue(settings),
            UpdateRecentPaths: vi.fn().mockResolvedValue(settings),
            ExpandDroppedPaths: expandDroppedPaths,
            ResolveOutputPath: resolveOutputPath,
            GetImagePreview: vi.fn().mockResolvedValue({ success: true, data_url: 'data:image/png;base64,' }),
            Adjust: adjust,
            AdjustBatch: undefined,
        };
        vi.mocked(getAppBindings).mockReturnValue(appBindings as any);
        (window as { pywebview?: unknown }).pywebview = { api: appBindings };

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'adjust',
            onBack: vi.fn(),
        }));
        const dropTarget = container.querySelector('[style*="--wails-drop-target"]') as HTMLElement;
        const file = {
            name: 'sample.png',
            size: 12,
            lastModified: 1710000000000,
            path: 'C:/input/sample.png',
        } as any;

        await act(async () => {
            const event = new Event('drop', { bubbles: true, cancelable: true });
            Object.defineProperty(event, 'dataTransfer', {
                value: { files: [file] },
                configurable: true,
            });
            dropTarget.dispatchEvent(event);
            await flushMicrotasks();
            await flushMicrotasks();
        });

        const verticalFlipButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.includes('垂直翻转')) as HTMLButtonElement | undefined;
        expect(verticalFlipButton).toBeTruthy();

        await act(async () => {
            verticalFlipButton?.click();
            await flushMicrotasks();
        });

        const startButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.includes('开始处理')) as HTMLButtonElement | undefined;
        expect(startButton).toBeTruthy();

        await act(async () => {
            startButton?.click();
            await flushMicrotasks();
            await flushMicrotasks();
        });

        expect(adjust).toHaveBeenCalledTimes(1);
        expect(adjust.mock.calls[0]?.[0]).toEqual(expect.objectContaining({
            flip_v: true,
        }));

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

describe('DetailView 图片水印', () => {
    it('水印图片选择器不暴露后端无法直接读取的 SVG', async () => {
        const settings = {
            max_concurrency: 4,
            output_prefix: '',
            output_template: '{basename}',
            preserve_folder_structure: false,
            conflict_strategy: 'rename',
            default_output_dir: '',
            recent_input_dirs: [],
            recent_output_dirs: [],
        };
        vi.mocked(getAppBindings).mockReturnValue({
            GetSettings: vi.fn().mockResolvedValue(settings),
        } as any);
        const openFileDialog = vi.fn().mockResolvedValue(null);
        (window as { runtime?: unknown }).runtime = {
            OpenFileDialog: openFileDialog,
        };

        const { container, root } = await renderElement(React.createElement(DetailView, {
            id: 'watermark',
            onBack: vi.fn(),
        }));

        const imageModeButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.trim() === '图片') as HTMLButtonElement | undefined;
        expect(imageModeButton).toBeTruthy();

        await act(async () => {
            imageModeButton?.click();
            await flushMicrotasks();
        });

        const uploadButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.includes('点击上传水印图')) as HTMLButtonElement | undefined;
        expect(uploadButton).toBeTruthy();

        await act(async () => {
            uploadButton?.click();
            await flushMicrotasks();
        });

        expect(openFileDialog).toHaveBeenCalledTimes(1);
        const filters = openFileDialog.mock.calls[0]?.[0]?.filters || [];
        expect(String(filters[0]?.Pattern || '')).toContain('*.png');
        expect(String(filters[0]?.Pattern || '')).not.toContain('*.svg');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
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

describe('normalizeGifSpeedPercent', () => {
    it('会把速度限制在 50 到 300 之间，并对齐到 10 的倍数', () => {
        expect(normalizeGifSpeedPercent(44)).toBe(50);
        expect(normalizeGifSpeedPercent(95)).toBe(100);
        expect(normalizeGifSpeedPercent(234)).toBe(230);
        expect(normalizeGifSpeedPercent(305)).toBe(300);
    });
});

describe('resolveWatermarkBackendPosition', () => {
    it('把前端九宫格位置映射为后端水印锚点名称', () => {
        expect(resolveWatermarkBackendPosition('tl')).toBe('top-left');
        expect(resolveWatermarkBackendPosition('tc')).toBe('top-center');
        expect(resolveWatermarkBackendPosition('tr')).toBe('top-right');
        expect(resolveWatermarkBackendPosition('cl')).toBe('center-left');
        expect(resolveWatermarkBackendPosition('c')).toBe('center');
        expect(resolveWatermarkBackendPosition('cr')).toBe('center-right');
        expect(resolveWatermarkBackendPosition('bl')).toBe('bottom-left');
        expect(resolveWatermarkBackendPosition('bc')).toBe('bottom-center');
        expect(resolveWatermarkBackendPosition('br')).toBe('bottom-right');
    });

    it('保留已经是后端格式的位置值', () => {
        expect(resolveWatermarkBackendPosition('bottom-right')).toBe('bottom-right');
    });
});

describe('selectAnimatedProbeCandidatePaths', () => {
    it('只选择需要探测帧数的 PNG/WEBP，去重并限制数量', () => {
        expect(selectAnimatedProbeCandidatePaths(
            [
                'C:/tmp/a.png',
                'C:/tmp/A.PNG',
                'C:/tmp/b.webp',
                'C:/tmp/c.gif',
                'C:/tmp/d.jpg',
                'C:/tmp/e.webp',
            ],
            2,
        )).toEqual(['C:/tmp/a.png', 'C:/tmp/b.webp']);
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

    it('在选择多个尺寸时拆成单尺寸任务，便于输出文件名追加尺寸后缀', () => {
        expect(planIcoConversionSizes([64, 16, 32, 32], false)).toEqual([[16], [32], [64]]);
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

describe('SubtitleStitchPage', () => {
    it('桌面文件选择器会传递静态截图格式过滤规则', async () => {
        const selectInputFiles = vi.fn().mockResolvedValue([]);
        vi.mocked(getAppBindings).mockReturnValue({
            SelectInputFiles: selectInputFiles,
        } as any);
        const { default: SubtitleStitchPage } = await import('./SubtitleStitchPage');

        const { container, root } = await renderElement(
            React.createElement(SubtitleStitchPage, { isActive: true })
        );
        const selectButton = Array.from(container.querySelectorAll('button'))
            .find((button) => button.textContent?.includes('选择截图')) as HTMLButtonElement | undefined;
        expect(selectButton).toBeTruthy();

        await act(async () => {
            selectButton?.click();
            await flushMicrotasks();
        });

        expect(selectInputFiles).toHaveBeenCalledWith(expect.objectContaining({
            allowsMultipleSelection: true,
            filters: [expect.objectContaining({
                DisplayName: 'Screenshots',
                Pattern: expect.stringContaining('*.png'),
            })],
        }));

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

describe('App 启动加载', () => {
    it('导入 App 时不会同步求值功能页模块', async () => {
        vi.resetModules();
        vi.doMock('./DetailView', () => {
            throw new Error('DetailView should be lazy-loaded');
        });
        vi.doMock('./SubtitleStitchPage', () => {
            throw new Error('SubtitleStitchPage should be lazy-loaded');
        });

        try {
            await expect(import('../App')).resolves.toHaveProperty('default');
        } finally {
            vi.doUnmock('./DetailView');
            vi.doUnmock('./SubtitleStitchPage');
            vi.resetModules();
        }
    });
});

describe('shouldPreventWindowDragEvent', () => {
    it('仅拦截文件拖拽，不拦截普通文本或链接拖拽', async () => {
        const { shouldPreventWindowDragEvent } = await import('../App');

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

describe('GifSettingsPanel', () => {
    it('修改帧率模式的滑杆使用 10 的步进', async () => {
        const noop = vi.fn();
        const { container, root } = await renderElement(
            React.createElement(GifSettingsPanel, {
                mode: '修改帧率',
                setMode: noop,
                exportFormat: 'PNG',
                setExportFormat: noop,
                convertFormat: 'GIF',
                setConvertFormat: noop,
                speedPercent: 100,
                setSpeedPercent: noop,
                compressQuality: 80,
                setCompressQuality: noop,
                sourceType: 'gif',
                buildFps: 12,
                setBuildFps: noop,
                resizeWidth: 0,
                resizeHeight: 0,
                onResizeWidthChange: noop,
                onResizeHeightChange: noop,
                resizeMaintainAR: true,
                setResizeMaintainAR: noop,
                originalWidth: 0,
                originalHeight: 0,
            })
        );

        const rangeInput = container.querySelector('input[type="range"]');
        expect(rangeInput?.getAttribute('step')).toBe('10');

        await act(async () => {
            root.unmount();
            await flushMicrotasks();
        });
    });
});

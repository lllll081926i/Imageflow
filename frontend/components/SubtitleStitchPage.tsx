import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Icon from './Icon';
import { StyledSlider, Switch } from './Controls';
import {
    getAppBindings,
    loadAppSettings,
    resolveSelectedFilePaths,
    updateRecentPaths,
} from '../types/wails-api';

const MIN_CROP_RATIO = 0.1;
const MAX_CROP_RATIO = 0.35;
const DEFAULT_CROP_RATIO = 0.18;

type SubtitleStitchPageProps = {
    isActive?: boolean;
    onTaskFailure?: (payload: { taskName: string; imageName: string; reason: string }) => void;
};

type SubtitleStitchResponse = {
    success: boolean;
    output_path?: string;
    input_count?: number;
    kept_count?: number;
    skipped_count?: number;
    strip_height?: number;
    error?: string;
};

type PreviewResponse = {
    success: boolean;
    data_url?: string;
    error?: string;
};

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value));

const toBasename = (path: string) => {
    const normalized = path.replace(/\\/g, '/');
    const parts = normalized.split('/');
    return parts[parts.length - 1] || path;
};

const buildDefaultOutputName = () => {
    const now = new Date();
    const pad = (value: number) => String(value).padStart(2, '0');
    const timestamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}_${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
    return `subtitle_stitch_${timestamp}.png`;
};

const joinPath = (dir: string, fileName: string) => {
    const cleanDir = dir.replace(/[\\/]+$/, '');
    const separator = cleanDir.includes('\\') ? '\\' : '/';
    return `${cleanDir}${separator}${fileName}`;
};

const dirname = (inputPath: string) => {
    const normalized = inputPath.replace(/\\/g, '/');
    const index = normalized.lastIndexOf('/');
    if (index <= 0) return '';
    return normalized.slice(0, index);
};

const SubtitleStitchPage: React.FC<SubtitleStitchPageProps> = ({ isActive = false, onTaskFailure }) => {
    const fileInputRef = useRef<HTMLInputElement | null>(null);
    const previewOverlayRef = useRef<HTMLDivElement | null>(null);
    const [inputPaths, setInputPaths] = useState<string[]>([]);
    const [previewSrc, setPreviewSrc] = useState<string>('');
    const [previewImageSize, setPreviewImageSize] = useState<{ width: number; height: number } | null>(null);
    const [outputDir, setOutputDir] = useState<string>('');
    const [outputName, setOutputName] = useState<string>(buildDefaultOutputName);
    const [subtitleCropRatio, setSubtitleCropRatio] = useState<number>(DEFAULT_CROP_RATIO);
    const [dedupEnabled, setDedupEnabled] = useState<boolean>(true);
    const [dedupThreshold, setDedupThreshold] = useState<number>(2);
    const [minimumStripHeight, setMinimumStripHeight] = useState<number>(24);
    const [isGenerating, setIsGenerating] = useState<boolean>(false);
    const [isDraggingLine, setIsDraggingLine] = useState<boolean>(false);
    const [message, setMessage] = useState<string>('');
    const [error, setError] = useState<string>('');

    const cropPercent = useMemo(() => Math.round(subtitleCropRatio * 100), [subtitleCropRatio]);
    const lineTopPercent = useMemo(() => (1 - subtitleCropRatio) * 100, [subtitleCropRatio]);
    const estimatedStripHeight = useMemo(() => {
        if (!previewImageSize) return 0;
        return Math.max(minimumStripHeight, Math.round(previewImageSize.height * subtitleCropRatio));
    }, [minimumStripHeight, previewImageSize, subtitleCropRatio]);

    useEffect(() => {
        if (!isActive) return;
        let cancelled = false;
        const loadDefaults = async () => {
            const settings = await loadAppSettings();
            if (cancelled) return;
            if (settings.default_output_dir) {
                setOutputDir((previous) => previous || settings.default_output_dir);
            }
        };
        void loadDefaults();
        return () => {
            cancelled = true;
        };
    }, [isActive]);

    useEffect(() => {
        if (!isActive) return;
        const firstPath = inputPaths[0];
        if (!firstPath) {
            setPreviewSrc('');
            setPreviewImageSize(null);
            return;
        }

        let cancelled = false;
        const loadPreview = async () => {
            setError('');
            try {
                const app = getAppBindings() as any;
                if (!app?.GetImagePreview) {
                    setPreviewSrc('');
                    return;
                }
                const result = (await app.GetImagePreview({ input_path: firstPath })) as PreviewResponse;
                if (cancelled) return;
                if (!result?.success || !result?.data_url) {
                    setPreviewSrc('');
                    if (result?.error && result.error !== 'PREVIEW_SKIPPED') {
                        setError(result.error);
                    }
                    return;
                }
                setPreviewSrc(result.data_url);
            } catch (loadError) {
                if (cancelled) return;
                console.error(loadError);
                setPreviewSrc('');
                setError('首图预览加载失败');
            }
        };
        void loadPreview();
        return () => {
            cancelled = true;
        };
    }, [inputPaths, isActive]);

    const updateRatioByClientY = useCallback((clientY: number) => {
        const overlay = previewOverlayRef.current;
        if (!overlay) return;
        const rect = overlay.getBoundingClientRect();
        if (rect.height <= 0) return;
        const localY = clamp(clientY - rect.top, 0, rect.height);
        const nextRatio = clamp((rect.height - localY) / rect.height, MIN_CROP_RATIO, MAX_CROP_RATIO);
        setSubtitleCropRatio(nextRatio);
    }, []);

    const handlePreviewPointerDown = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
        const target = event.currentTarget;
        target.setPointerCapture(event.pointerId);
        setIsDraggingLine(true);
        updateRatioByClientY(event.clientY);
    }, [updateRatioByClientY]);

    const handlePreviewPointerMove = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
        if (!isDraggingLine) return;
        updateRatioByClientY(event.clientY);
    }, [isDraggingLine, updateRatioByClientY]);

    const handlePreviewPointerUp = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
        if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId);
        }
        setIsDraggingLine(false);
    }, []);

    const handleSelectInput = useCallback(async () => {
        setError('');
        try {
            const app = getAppBindings() as any;
            if (app?.SelectInputFiles) {
                const selected = await app.SelectInputFiles();
                const paths = Array.isArray(selected)
                    ? selected.filter((item) => typeof item === 'string' && item.trim())
                    : (typeof selected === 'string' && selected.trim() ? [selected.trim()] : []);
                if (paths.length > 0) {
                    setInputPaths(paths);
                    setMessage('');
                    void updateRecentPaths({ inputDir: dirname(paths[0]) });
                }
                return;
            }
        } catch (selectError) {
            console.error(selectError);
        }
        fileInputRef.current?.click();
    }, []);

    const handleSelectOutputDirectory = useCallback(async () => {
        setError('');
        try {
            const app = getAppBindings() as any;
            if (app?.SelectOutputDirectory) {
                const selected = await app.SelectOutputDirectory();
                if (typeof selected === 'string' && selected.trim()) {
                    const trimmed = selected.trim();
                    setOutputDir(trimmed);
                    void updateRecentPaths({ outputDir: trimmed });
                }
            }
        } catch (selectError) {
            console.error(selectError);
            setError('输出目录选择失败');
        }
    }, []);

    const handleFileInputChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        const files = Array.from(event.target.files || []);
        if (files.length === 0) return;
        void (async () => {
            const resolvedPaths = await resolveSelectedFilePaths(files);
            if (resolvedPaths.length === 0) {
                setError('当前环境无法解析所选文件的本地路径，请优先使用桌面端原生文件选择器。');
                return;
            }
            setInputPaths(resolvedPaths);
            setMessage('');
            setError('');
            void updateRecentPaths({ inputDir: dirname(resolvedPaths[0]) });
        })();
    }, []);

    const handleRemoveInput = useCallback((targetPath: string) => {
        setInputPaths((previous) => previous.filter((path) => path !== targetPath));
    }, []);

    const handleGenerate = useCallback(async () => {
        setError('');
        setMessage('');
        if (inputPaths.length === 0) {
            setError('请先选择按时间顺序排列的截图');
            return;
        }
        if (!outputDir.trim()) {
            setError('请先选择输出目录');
            return;
        }

        const cleanOutputName = outputName.trim() || buildDefaultOutputName();
        const outputPath = joinPath(outputDir, cleanOutputName);

        try {
            setIsGenerating(true);
            void updateRecentPaths({ inputDir: dirname(inputPaths[0] || ''), outputDir });
            const appAny = getAppBindings() as any;
            if (!appAny?.GenerateSubtitleLongImage) {
                throw new Error('当前版本未导出 GenerateSubtitleLongImage 接口');
            }

            const payload = {
                input_paths: inputPaths,
                output_path: outputPath,
                subtitle_crop_ratio: subtitleCropRatio,
                header_keep_full: true,
                dedup_enabled: dedupEnabled,
                dedup_threshold: dedupThreshold,
                minimum_strip_height: minimumStripHeight,
            };

            const result = (await appAny.GenerateSubtitleLongImage(payload)) as SubtitleStitchResponse;
            if (!result?.success) {
                const failureReason = result?.error || '字幕拼接失败';
                setError(failureReason);
                onTaskFailure?.({
                    taskName: '字幕拼接',
                    imageName: toBasename(inputPaths[0]),
                    reason: failureReason,
                });
                return;
            }

            const summary = `完成：输入 ${result.input_count ?? inputPaths.length} 张，保留 ${result.kept_count ?? 0} 段，跳过 ${result.skipped_count ?? 0} 段`;
            setMessage(summary);
            setOutputName(buildDefaultOutputName());
        } catch (generateError: any) {
            const failureReason = typeof generateError?.message === 'string' ? generateError.message : '字幕拼接失败';
            setError(failureReason);
            onTaskFailure?.({
                taskName: '字幕拼接',
                imageName: toBasename(inputPaths[0] || 'unknown'),
                reason: failureReason,
            });
        } finally {
            setIsGenerating(false);
        }
    }, [dedupEnabled, dedupThreshold, inputPaths, minimumStripHeight, onTaskFailure, outputDir, outputName, subtitleCropRatio]);

    return (
        <div className="h-full p-1">
            <div className="h-full grid grid-cols-1 xl:grid-cols-[380px_minmax(0,1fr)] gap-6 min-h-0">
                <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col min-h-0">
                    <div className="flex items-start justify-between gap-3 mb-4">
                        <div>
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">字幕拼接</h2>
                            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">首帧保留，自动拼接字幕长图</p>
                        </div>
                        <button
                            type="button"
                            onClick={handleSelectInput}
                            className="px-3 py-1.5 rounded-lg bg-[#007AFF] text-white text-xs font-medium hover:bg-[#0066D6] transition-colors"
                        >
                            选择截图
                        </button>
                    </div>

                    <div className="rounded-2xl border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 flex-1 min-h-0 overflow-hidden flex flex-col">
                        <div className="px-3 py-2 border-b border-gray-200/70 dark:border-white/10 text-xs text-gray-600 dark:text-gray-300 flex items-center justify-between">
                            <span>按时间顺序输入 ({inputPaths.length})</span>
                            {inputPaths.length > 0 && (
                                <button
                                    type="button"
                                    onClick={() => setInputPaths([])}
                                    className="text-gray-500 hover:text-red-500 transition-colors"
                                >
                                    清空
                                </button>
                            )}
                        </div>
                        <div className="flex-1 overflow-y-auto no-scrollbar">
                            {inputPaths.length === 0 ? (
                                <div className="h-full flex items-center justify-center text-xs text-gray-400 px-4 text-center">选择连续截图后开始处理</div>
                            ) : (
                                <div className="divide-y divide-gray-200/70 dark:divide-white/10">
                                    {inputPaths.map((path, index) => (
                                        <div key={`${path}-${index}`} className="px-3 py-2.5 flex items-center gap-2 text-xs">
                                            <span className="w-6 shrink-0 text-gray-400 text-center">{index + 1}</span>
                                            <span className="flex-1 truncate text-gray-700 dark:text-gray-200">{toBasename(path)}</span>
                                            <button
                                                type="button"
                                                onClick={() => handleRemoveInput(path)}
                                                className="text-gray-400 hover:text-red-500 transition-colors"
                                                title="移除"
                                            >
                                                <Icon name="Close" size={12} />
                                            </button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                </div>

                <div className="grid grid-rows-[minmax(320px,1fr)_auto] gap-6 min-h-0">
                    <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col min-h-0">
                        <div className="flex items-center justify-between mb-4 gap-3">
                            <div>
                                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">首帧预览</span>
                                <p className="text-xs text-gray-400 mt-1">拖动虚线调整保留高度</p>
                            </div>
                            <span className="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
                                保留高度 {estimatedStripHeight}px ({cropPercent}%)
                            </span>
                        </div>

                        <div className="flex-1 min-h-[260px] rounded-2xl border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 overflow-hidden flex items-center justify-center px-4 py-4">
                            {previewSrc ? (
                                <div className="relative max-w-full max-h-full">
                                    <img
                                        src={previewSrc}
                                        alt="首帧预览"
                                        className="max-w-full max-h-[56vh] object-contain rounded-lg shadow-sm"
                                        onLoad={(event) => {
                                            const image = event.currentTarget;
                                            if (image.naturalWidth > 0 && image.naturalHeight > 0) {
                                                setPreviewImageSize({ width: image.naturalWidth, height: image.naturalHeight });
                                            }
                                        }}
                                    />
                                    <div
                                        ref={previewOverlayRef}
                                        className="absolute inset-0 cursor-row-resize touch-none"
                                        onPointerDown={handlePreviewPointerDown}
                                        onPointerMove={handlePreviewPointerMove}
                                        onPointerUp={handlePreviewPointerUp}
                                        onPointerCancel={handlePreviewPointerUp}
                                    >
                                        <div className="absolute inset-x-0 top-0 bottom-0 bg-black/20 pointer-events-none" />
                                        <div
                                            className="absolute inset-x-0 bg-black/40 pointer-events-none"
                                            style={{ top: `${lineTopPercent}%`, bottom: 0 }}
                                        />
                                        <div
                                            className="absolute inset-x-0 border-t-2 border-dashed border-[#0A84FF] pointer-events-none"
                                            style={{ top: `${lineTopPercent}%` }}
                                        />
                                    </div>
                                </div>
                            ) : (
                                <div className="text-xs text-gray-400 text-center px-6">选择截图后可在此调整裁剪线</div>
                            )}
                        </div>
                    </div>

                    <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5">
                        <div className="grid grid-cols-1 xl:grid-cols-[1fr_1fr] gap-6">
                            <div className="space-y-4">
                                <StyledSlider
                                    label="字幕区域高度"
                                    value={cropPercent}
                                    min={10}
                                    max={35}
                                    unit="%"
                                    onChange={(value) => setSubtitleCropRatio(clamp(value / 100, MIN_CROP_RATIO, MAX_CROP_RATIO))}
                                />
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">最小字幕条高度 (px)</label>
                                    <input
                                        type="number"
                                        value={minimumStripHeight}
                                        min={1}
                                        onChange={(event) => setMinimumStripHeight(Math.max(1, Number(event.target.value || 1)))}
                                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white"
                                    />
                                </div>
                                <Switch checked={dedupEnabled} onChange={setDedupEnabled} label="启用相邻字幕去重" />
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">去重阈值（哈希距离）</label>
                                    <input
                                        type="number"
                                        value={dedupThreshold}
                                        min={0}
                                        onChange={(event) => setDedupThreshold(Math.max(0, Number(event.target.value || 0)))}
                                        disabled={!dedupEnabled}
                                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white disabled:opacity-50"
                                    />
                                </div>
                            </div>

                            <div className="space-y-4">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">输出目录</label>
                                    <div className="flex items-center gap-2">
                                        <input
                                            value={outputDir}
                                            onChange={(event) => setOutputDir(event.target.value)}
                                            placeholder="请选择输出目录"
                                            className="flex-1 px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white"
                                        />
                                        <button
                                            type="button"
                                            onClick={handleSelectOutputDirectory}
                                            className="px-3 py-2.5 rounded-xl border border-gray-200 dark:border-white/10 text-sm text-gray-600 dark:text-gray-300 hover:border-[#007AFF] hover:text-[#007AFF] transition-colors"
                                        >
                                            浏览
                                        </button>
                                    </div>
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">输出文件名</label>
                                    <input
                                        value={outputName}
                                        onChange={(event) => setOutputName(event.target.value)}
                                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white"
                                    />
                                </div>

                                <button
                                    type="button"
                                    onClick={handleGenerate}
                                    disabled={isGenerating}
                                    className="w-full py-2.5 rounded-xl bg-[#007AFF] hover:bg-[#0066D6] disabled:opacity-60 disabled:cursor-not-allowed text-white text-sm font-medium transition-colors"
                                >
                                    {isGenerating ? '处理中...' : '开始拼接'}
                                </button>

                                {message && (
                                    <div className="text-xs text-green-600 dark:text-green-300 bg-green-50 dark:bg-green-500/10 border border-green-100 dark:border-green-500/30 rounded-xl px-3 py-2">
                                        {message}
                                    </div>
                                )}
                                {error && (
                                    <div className="text-xs text-red-600 dark:text-red-300 bg-red-50 dark:bg-red-500/10 border border-red-100 dark:border-red-500/30 rounded-xl px-3 py-2 break-all">
                                        {error}
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <input
                ref={fileInputRef}
                type="file"
                className="hidden"
                multiple
                accept=".jpg,.jpeg,.png,.webp,.bmp,.tif,.tiff"
                onChange={handleFileInputChange}
            />
        </div>
    );
};

export default SubtitleStitchPage;

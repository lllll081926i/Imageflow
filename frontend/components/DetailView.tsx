import React, { useCallback, useMemo, useState, useEffect, useRef, memo } from 'react';
import Icon from './Icon';
import { FEATURES } from '../constants';
import { ViewState } from '../types';
import { Switch, StyledSlider, CustomSelect, SegmentedControl, FileDropZone, ProgressBar } from './Controls';
import {
    buildGifProcessSuffix,
    detectAnimatedImagePath,
    normalizeGifSpeedPercent,
    planIcoConversionSizes,
    resolveConverterOverwritePath,
    resolveGifAction,
    selectAnimatedProbeCandidatePaths,
} from './gifHelpers';
import { resolveGifErrorMessage } from './gifErrors';
import GifSettingsPanel from './GifSettingsPanel';
import { GridSettings } from './detail/GridSettingsPanel';
import GridPreviewCard from './detail/GridPreviewCard';
import { SplitSettings } from './detail/SplitSettingsPanel';
import { WatermarkSettings } from './detail/WatermarkSettingsPanel';
import { ResizeSettings } from './detail/ResizeSettingsPanel';
import { StitchSettings } from './detail/StitchSettingsPanel';
import { PalettePanel, type PaletteData } from './detail/PalettePanel';
import { RichTextSettings } from './detail/RichTextPanel';
import { useGifResizeState } from './hooks/useGifResizeState';
import { useThrottledProgress } from './hooks/useThrottledProgress';
import { formatBytes } from './batchHelpers';
import { showToast } from './Toast';
import { useTweenNumber } from './hooks/useTweenNumber';
import {
    useConverterParams,
    useCompressorParams,
    usePdfParams,
    useGifParams,
    useGridParams,
    useSplitParams,
    useWatermarkParams,
    useResizeParams,
    useStitchParams,
    useRichTextParams,
    type WatermarkStyle,
    type RichTextBlock,
} from './hooks/useFeatureParams';
import {
    defaultOutputSettings,
    type DroppedFile,
    type ExpandDroppedPathsResult,
    type OutputSettings,
    getBatchChunkSize as sharedGetBatchChunkSize,
} from './detail/detailTypes';
import {
    dedupeDroppedFiles,
    joinPath,
    pathLookupKey,
    basename,
    extname,
    dirname,
    normalizePath,
    stripExtension,
    buildOutputName,
    getRelPath,
    getRelDir,
    buildOutputRelPath,
    sanitizePdfInputName,
    normalizePdfFileName,
    buildSuggestedPdfName,
} from './detail/outputNaming';
import {
    isCancellationError,
    normalizeBatchResults,
    summarizeBatchProgress,
} from './batchHelpers';
import {
    DEFAULT_APP_SETTINGS,
    getAppBindings,
    loadAppSettings,
    updateRecentPaths,
} from '../types/app-api';
import type { models } from '../types/backend-models';
import { ConverterSettings } from './detail/ConverterSettingsPanel';
import { CompressorSettings } from './detail/CompressorSettingsPanel';
import { PdfSettings } from './detail/PdfSettingsPanel';
import {
    runConvertBatch,
    runCompressBatch,
    runSequentialToolBatch,
    type CompressBatchStats,
    type RunDoneSummary,
} from './detail/batchRunners';

const DEFAULT_IMAGE_INPUT_EXTENSIONS = [
    '.jpg',
    '.jpeg',
    '.png',
    '.webp',
    '.gif',
    '.bmp',
    '.avif',
    '.heic',
    '.heif',
    '.ico',
    '.tiff',
    '.tif',
    '.svg',
];

const COMPRESSOR_INPUT_EXTENSIONS = [
    '.jpg',
    '.jpeg',
    '.png',
    '.webp',
    '.bmp',
    '.avif',
    '.heic',
    '.heif',
    '.ico',
    '.tiff',
    '.tif',
];

const extensionsToAccept = (extensions: string[]) => extensions.join(',');
const extensionsToDialogPattern = (extensions: string[]) => extensions.map((ext) => `*${ext}`).join(';');

const DEFAULT_IMAGE_ACCEPTED_FORMATS = extensionsToAccept(DEFAULT_IMAGE_INPUT_EXTENSIONS);
const COMPRESSOR_ACCEPTED_FORMATS = extensionsToAccept(COMPRESSOR_INPUT_EXTENSIONS);
const DEFAULT_IMAGE_FILE_DIALOG_FILTERS = [{
    DisplayName: 'Images',
    Pattern: extensionsToDialogPattern(DEFAULT_IMAGE_INPUT_EXTENSIONS),
}];
const COMPRESSOR_FILE_DIALOG_FILTERS = [{
    DisplayName: 'Compressible bitmaps',
    Pattern: extensionsToDialogPattern(COMPRESSOR_INPUT_EXTENSIONS),
}];

interface DetailViewProps {
    id: ViewState;
    isActive?: boolean;
    onTaskFailure?: (payload: { taskName: string; imageName: string; reason: string }) => void;
}



const DetailView: React.FC<DetailViewProps> = ({ id, isActive = true, onTaskFailure }) => {
    const feature = FEATURES.find(f => f.id === id);
    if (!feature) return null;

    const [dropResult, setDropResult] = useState<ExpandDroppedPathsResult | null>(null);
    const [outputDir, setOutputDir] = useState('');
    const [isProcessing, setIsProcessing] = useState(false);
    const [cancelRequested, setCancelRequested] = useState(false);
    const [isExpandingPaths, setIsExpandingPaths] = useState(false);
    const cancelRequestedRef = useRef(false);
    const processingLockRef = useRef(false);
    const processingRunIdRef = useRef(0);
    const expansionCountRef = useRef(0);
    const handleExpansionStateChange = useCallback((expanding: boolean) => {
        expansionCountRef.current = expanding
            ? expansionCountRef.current + 1
            : Math.max(0, expansionCountRef.current - 1);
        setIsExpandingPaths(expansionCountRef.current > 0);
    }, []);
    const {
        progress,
        setProgress,
        setProgressThrottled,
        flushProgress,
        resetProgress,
    } = useThrottledProgress(0);

    const [lastMessage, setLastMessage] = useState<string>('');
    const [outputSettings, setOutputSettings] = useState<OutputSettings>(defaultOutputSettings);
    const [failedRecords, setFailedRecords] = useState<DroppedFile[]>([]);
    const [retryFailedOnly, setRetryFailedOnly] = useState(false);
    const [compressStats, setCompressStats] = useState<CompressBatchStats | null>(null);
    const [runSummary, setRunSummary] = useState<(RunDoneSummary & { outputDir: string }) | null>(null);
    const currentRunFailedPathsRef = useRef<Set<string> | null>(null);
    const openOutputOnCompleteRef = useRef<{ enabled: boolean; dir: string }>({ enabled: false, dir: '' });

    const {
        format: convFormat, setFormat: setConvFormat,
        quality: convQuality, setQuality: setConvQuality,
        compressLevel: convCompressLevel, setCompressLevel: setConvCompressLevel,
        icoSizes: convIcoSizes, setIcoSizes: setConvIcoSizes,
        resizeMode: convResizeMode, setResizeMode: setConvResizeMode,
        scalePercent: convScalePercent, setScalePercent: setConvScalePercent,
        fixedWidth: convFixedWidth, setFixedWidth: setConvFixedWidth,
        fixedHeight: convFixedHeight, setFixedHeight: setConvFixedHeight,
        longEdge: convLongEdge, setLongEdge: setConvLongEdge,
        keepMetadata: convKeepMetadata, setKeepMetadata: setConvKeepMetadata,
        maintainAR: convMaintainAR, setMaintainAR: setConvMaintainAR,
        overwriteSource: convOverwriteSource, setOverwriteSource: setConvOverwriteSource,
    } = useConverterParams();

    const {
        mode: compMode, setMode: setCompMode,
        targetSize: compTargetSize, setTargetSize: setCompTargetSize,
        targetSizeKB: compTargetSizeKB, setTargetSizeKB: setCompTargetSizeKB,
        engine: compEngine, setEngine: setCompEngine,
        overwriteSource: compOverwriteSource, setOverwriteSource: setCompOverwriteSource,
        resizeMode: compResizeMode, setResizeMode: setCompResizeMode,
        resizePercent: compResizePercent, setResizePercent: setCompResizePercent,
        resizeWidth: compResizeWidth, setResizeWidth: setCompResizeWidth,
        resizeHeight: compResizeHeight, setResizeHeight: setCompResizeHeight,
        maintainAspect: compMaintainAspect, setMaintainAspect: setCompMaintainAspect,
    } = useCompressorParams();
    const {
        size: pdfSize, setSize: setPdfSize,
        layout: pdfLayout, setLayout: setPdfLayout,
        fit: pdfFit, setFit: setPdfFit,
        marginMm: pdfMarginMm, setMarginMm: setPdfMarginMm,
        compression: pdfCompression, setCompression: setPdfCompression,
        fileName: pdfFileName, setFileName: setPdfFileName,
        title: pdfTitle, setTitle: setPdfTitle,
        author: pdfAuthor, setAuthor: setPdfAuthor,
        password: pdfPassword, setPassword: setPdfPassword,
    } = usePdfParams();
    const {
        mode: gifMode, setMode: setGifMode,
        exportFormat: gifExportFormat, setExportFormat: setGifExportFormat,
        convertFormat: gifConvertFormat, setConvertFormat: setGifConvertFormat,
        speedPercent: gifSpeedPercent, setSpeedPercent: setGifSpeedPercent,
        compressQuality: gifCompressQuality, setCompressQuality: setGifCompressQuality,
        buildFps: gifBuildFps, setBuildFps: setGifBuildFps,
    } = useGifParams();
    const {
        gridSize, setGridSize,
        format: gridFormat, setFormat: setGridFormat,
    } = useGridParams();
    const splitParams = useSplitParams();
    const wmParams = useWatermarkParams();
    const wmPatch = wmParams.patch;
    const resizeParams = useResizeParams();
    const stitchParams = useStitchParams();
    const rtParams = useRichTextParams();

    const [paletteData, setPaletteData] = useState<PaletteData>(null);
    const [paletteLoading, setPaletteLoading] = useState(false);
    const [paletteBusy, setPaletteBusy] = useState(false);
    const [swatchHeight, setSwatchHeight] = useState(96);
    const [includeText, setIncludeText] = useState(true);
    const [wmPreviewUrl, setWmPreviewUrl] = useState('');
    const [rtPreviewUrl, setRtPreviewUrl] = useState('');
    const normalizeOutputSettings = (raw?: Partial<OutputSettings> | null): OutputSettings => {
        if (!raw) return defaultOutputSettings;
        return {
            output_prefix: typeof raw.output_prefix === 'string' && raw.output_prefix.trim()
                ? raw.output_prefix
                : defaultOutputSettings.output_prefix,
            output_template: typeof raw.output_template === 'string' && raw.output_template.trim()
                ? raw.output_template
                : defaultOutputSettings.output_template,
            preserve_folder_structure: typeof raw.preserve_folder_structure === 'boolean'
                ? raw.preserve_folder_structure
                : defaultOutputSettings.preserve_folder_structure,
            conflict_strategy: typeof raw.conflict_strategy === 'string'
                && ['rename', 'overwrite', 'skip'].includes(raw.conflict_strategy.trim())
                ? (raw.conflict_strategy.trim() as OutputSettings['conflict_strategy'])
                : defaultOutputSettings.conflict_strategy,
        };
    };

    const loadOutputSettings = async () => {
        try {
            const loaded = await loadAppSettings();
            const normalized = normalizeOutputSettings(loaded);
            setOutputSettings(normalized);
            setOutputDir((previous) => previous || loaded.default_output_dir || '');
            return loaded;
        } catch (e) {
            console.error(e);
            setOutputDir((previous) => previous || DEFAULT_APP_SETTINGS.default_output_dir);
            return { ...DEFAULT_APP_SETTINGS };
        }
    };

    const isGifPath = (p: string) => {
        const normalized = p.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('.');
        if (idx === -1) return false;
        const ext = normalized.slice(idx + 1).toLowerCase();
        return ext === 'gif' || ext === 'apng' || ext === 'webp';
    };
    const rememberRecentDirs = useCallback(async (payload: { inputDir?: string; outputDir?: string }) => {
        try {
            await updateRecentPaths(payload);
        } catch (error) {
            console.error('Failed to persist recent paths:', error);
        }
    }, []);

    const gifInputType = useMemo(() => {
        const list = dropResult?.files || [];
        if (list.length === 0) return 'empty' as const;
        let hasAnimated = false;
        let hasOther = false;
        list.forEach((f) => {
            if (isGifPath(f.input_path)) {
                hasAnimated = true;
            } else {
                hasOther = true;
            }
        });
        if (hasAnimated && !hasOther) return 'gif' as const;
        if (!hasAnimated && hasOther) return 'images' as const;
        return 'mixed' as const;
    }, [dropResult]);
    const {
        gifResizeWidth,
        gifResizeHeight,
        gifResizeMaintainAR,
        setGifResizeMaintainAR,
        gifOriginalSize,
        onResizeWidthChange: handleGifResizeWidthChange,
        onResizeHeightChange: handleGifResizeHeightChange,
    } = useGifResizeState({
        featureId: id,
        files: dropResult?.files || [],
        isGifPath,
        normalizePath,
    });

    useEffect(() => {
        loadOutputSettings();
    }, [id]);

    // Watermark live preview (debounced; keyed on serialized style so unrelated
    // re-renders such as progress ticks never reset the timer)
    const wmStyleKey = JSON.stringify(wmParams);
    useEffect(() => {
        if (id !== 'watermark') {
            setWmPreviewUrl('');
            return;
        }
        const file = dropResult?.files?.[0];
        if (!file) {
            setWmPreviewUrl('');
            return;
        }
        let cancelled = false;
        const timer = window.setTimeout(async () => {
            try {
                const style = JSON.parse(wmStyleKey) as WatermarkStyle;
                const app = getAppBindings();
                const res = await app?.AddWatermark?.({
                    action: 'preview',
                    input_path: normalizePath(file.input_path),
                    text: style.text,
                    font_name: style.fontName,
                    size_mode: style.sizeMode,
                    font_size: style.fontSize,
                    color: style.color,
                    opacity: style.opacity,
                    rotation: style.rotation,
                    position: style.position,
                    margin: style.margin,
                    tiled: style.tiled,
                    tile_gap_x: style.tileGapX,
                    tile_gap_y: style.tileGapY,
                    stroke_width: style.strokeWidth,
                    stroke_color: style.strokeColor,
                    shadow: style.shadow,
                });
                if (!cancelled) setWmPreviewUrl(res?.success && res.data_url ? res.data_url : '');
            } catch {
                if (!cancelled) setWmPreviewUrl('');
            }
        }, 350);
        return () => {
            cancelled = true;
            window.clearTimeout(timer);
        };
    }, [id, wmStyleKey, dropResult?.files]);

    // Palette auto-extract
    useEffect(() => {
        if (id !== 'palette') {
            setPaletteData(null);
            return;
        }
        const file = dropResult?.files?.[0];
        if (!file) {
            setPaletteData(null);
            return;
        }
        let cancelled = false;
        setPaletteLoading(true);
        void (async () => {
            try {
                const app = getAppBindings();
                const res = await app?.PaletteTool?.({ action: 'extract', input_path: normalizePath(file.input_path) });
                if (!cancelled) {
                    setPaletteData(res?.success && res.theme_color && Array.isArray(res.colors)
                        ? { theme_color: res.theme_color, colors: res.colors }
                        : null);
                }
            } catch {
                if (!cancelled) setPaletteData(null);
            } finally {
                if (!cancelled) setPaletteLoading(false);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [id, dropResult?.files]);

    // Rich text live preview (debounced, serialized key)
    const rtKey = JSON.stringify(rtParams);
    useEffect(() => {
        if (id !== 'richtext') {
            setRtPreviewUrl('');
            return;
        }
        let cancelled = false;
        const timer = window.setTimeout(async () => {
            try {
                const style = JSON.parse(rtKey) as typeof rtParams;
                const app = getAppBindings();
                const res = await app?.RenderRichText?.({
                    blocks: style.blocks as unknown as Array<Record<string, any>>,
                    width: style.width,
                    padding: style.padding,
                    background: style.background,
                    text_color: style.textColor,
                    heading_color: style.headingColor,
                    accent_color: style.accentColor,
                    font_size: style.fontSize,
                    line_height: style.lineHeight,
                    font_name: style.fontName,
                    preview: true,
                });
                if (!cancelled) setRtPreviewUrl(res?.success && res.data_url ? res.data_url : '');
            } catch {
                if (!cancelled) setRtPreviewUrl('');
            }
        }, 450);
        return () => {
            cancelled = true;
            window.clearTimeout(timer);
        };
    }, [id, rtKey]);

    useEffect(() => {
        if (gifInputType === 'images' && gifMode !== '导出') {
            setGifMode('导出');
        }
    }, [gifInputType, gifMode]);

    useEffect(() => {
        const normalized = normalizeGifSpeedPercent(gifSpeedPercent);
        if (normalized !== gifSpeedPercent) {
            setGifSpeedPercent(normalized);
        }
    }, [gifSpeedPercent]);

    const renderSettings = () => {
        switch(id) {
            case 'converter':
                return (
                    <ConverterSettings
                        format={convFormat}
                        setFormat={setConvFormat}
                        quality={convQuality}
                        setQuality={setConvQuality}
                        compressLevel={convCompressLevel}
                        setCompressLevel={setConvCompressLevel}
                        icoSizes={convIcoSizes}
                        setIcoSizes={setConvIcoSizes}
                        resizeMode={convResizeMode}
                        setResizeMode={setConvResizeMode}
                        scalePercent={convScalePercent}
                        setScalePercent={setConvScalePercent}
                        fixedWidth={convFixedWidth}
                        setFixedWidth={setConvFixedWidth}
                        fixedHeight={convFixedHeight}
                        setFixedHeight={setConvFixedHeight}
                        longEdge={convLongEdge}
                        setLongEdge={setConvLongEdge}
                        keepMetadata={convKeepMetadata}
                        setKeepMetadata={setConvKeepMetadata}
                        maintainAR={convMaintainAR}
                        setMaintainAR={setConvMaintainAR}
                        overwriteSource={convOverwriteSource}
                        setOverwriteSource={setConvOverwriteSource}
                    />
                );
            case 'compressor':
                return (
                    <CompressorSettings
                        mode={compMode}
                        setMode={setCompMode}
                        targetSize={compTargetSize}
                        setTargetSize={setCompTargetSize}
                        targetSizeKB={compTargetSizeKB}
                        setTargetSizeKB={setCompTargetSizeKB}
                        engine={compEngine}
                        setEngine={setCompEngine}
                        overwriteSource={compOverwriteSource}
                        setOverwriteSource={setCompOverwriteSource}
                        resizeMode={compResizeMode}
                        setResizeMode={setCompResizeMode}
                        resizePercent={compResizePercent}
                        setResizePercent={setCompResizePercent}
                        resizeWidth={compResizeWidth}
                        setResizeWidth={setCompResizeWidth}
                        resizeHeight={compResizeHeight}
                        setResizeHeight={setCompResizeHeight}
                        maintainAspect={compMaintainAspect}
                        setMaintainAspect={setCompMaintainAspect}
                    />
                );
            case 'pdf':
                return (
                    <PdfSettings
                        fileName={pdfFileName}
                        setFileName={(value) => setPdfFileName(sanitizePdfInputName(value))}
                        size={pdfSize}
                        setSize={setPdfSize}
                        layout={pdfLayout}
                        setLayout={setPdfLayout}
                        fit={pdfFit}
                        setFit={setPdfFit}
                        marginMm={pdfMarginMm}
                        setMarginMm={setPdfMarginMm}
                        compression={pdfCompression}
                        setCompression={setPdfCompression}
                        title={pdfTitle}
                        setTitle={setPdfTitle}
                        author={pdfAuthor}
                        setAuthor={setPdfAuthor}
                        password={pdfPassword}
                        setPassword={setPdfPassword}
                    />
                );
            case 'gif':
                return (
                    <GifSettingsPanel
                        mode={gifMode}
                        setMode={setGifMode}
                        exportFormat={gifExportFormat}
                        setExportFormat={setGifExportFormat}
                        convertFormat={gifConvertFormat}
                        setConvertFormat={setGifConvertFormat}
                        speedPercent={gifSpeedPercent}
                        setSpeedPercent={setGifSpeedPercent}
                        compressQuality={gifCompressQuality}
                        setCompressQuality={setGifCompressQuality}
                        sourceType={gifInputType}
                        buildFps={gifBuildFps}
                        setBuildFps={setGifBuildFps}
                        resizeWidth={gifResizeWidth}
                        resizeHeight={gifResizeHeight}
                        onResizeWidthChange={handleGifResizeWidthChange}
                        onResizeHeightChange={handleGifResizeHeightChange}
                        resizeMaintainAR={gifResizeMaintainAR}
                        setResizeMaintainAR={setGifResizeMaintainAR}
                        originalWidth={gifOriginalSize.width}
                        originalHeight={gifOriginalSize.height}
                    />
                );
            case 'grid':
                return (
                    <GridSettings
                        gridSize={gridSize}
                        setGridSize={setGridSize}
                        format={gridFormat}
                        setFormat={setGridFormat}
                    />
                );
            case 'split':
                return (
                    <SplitSettings
                        rows={splitParams.rows} setRows={splitParams.setRows}
                        cols={splitParams.cols} setCols={splitParams.setCols}
                        mode={splitParams.mode} setMode={splitParams.setMode}
                        format={splitParams.format} setFormat={splitParams.setFormat}
                    />
                );
            case 'watermark':
                return <WatermarkSettings style={wmParams} patch={wmPatch} />;
            case 'resize':
                return (
                    <ResizeSettings
                        mode={resizeParams.mode} setMode={resizeParams.setMode}
                        value={resizeParams.value} setValue={resizeParams.setValue}
                        format={resizeParams.format as 'auto' | 'PNG' | 'JPG'}
                        setFormat={resizeParams.setFormat as (v: 'auto' | 'PNG' | 'JPG') => void}
                    />
                );
            case 'stitch':
                return (
                    <StitchSettings
                        direction={stitchParams.direction} setDirection={stitchParams.setDirection}
                        gap={stitchParams.gap} setGap={stitchParams.setGap}
                        background={stitchParams.background} setBackground={stitchParams.setBackground}
                        targetLength={stitchParams.targetLength} setTargetLength={stitchParams.setTargetLength}
                        border={stitchParams.border} setBorder={stitchParams.setBorder}
                        outputFormat={stitchParams.outputFormat} setOutputFormat={stitchParams.setOutputFormat}
                    />
                );
            case 'palette':
                return (
                    <PalettePanel
                        data={paletteData}
                        loading={paletteLoading}
                        swatchHeight={swatchHeight}
                        setSwatchHeight={setSwatchHeight}
                        includeText={includeText}
                        setIncludeText={setIncludeText}
                        onExport={(kind) => void handlePaletteExport(kind)}
                        busy={paletteBusy}
                    />
                );
            case 'richtext':
                return (
                    <RichTextSettings
                        blocks={rtParams.blocks} setBlocks={rtParams.setBlocks}
                        width={rtParams.width} setWidth={rtParams.setWidth}
                        padding={rtParams.padding} setPadding={rtParams.setPadding}
                        background={rtParams.background} setBackground={rtParams.setBackground}
                        textColor={rtParams.textColor} setTextColor={rtParams.setTextColor}
                        headingColor={rtParams.headingColor} setHeadingColor={rtParams.setHeadingColor}
                        accentColor={rtParams.accentColor} setAccentColor={rtParams.setAccentColor}
                        fontSize={rtParams.fontSize} setFontSize={rtParams.setFontSize}
                        lineHeight={rtParams.lineHeight} setLineHeight={rtParams.setLineHeight}
                        fontName={rtParams.fontName} setFontName={rtParams.setFontName}
                        fileName={rtParams.fileName} setFileName={rtParams.setFileName}
                    />
                );
            default: return <div className="text-gray-400 text-sm text-center py-10">暂无特殊设置</div>;
        }
    };

    const inputCount = dropResult?.files?.length ?? 0;
    const effectiveOutputDir = useMemo(() => {
        if (outputDir) return outputDir;
        return '';
    }, [dropResult, outputDir]);

    const handleSelectOutputDir = async () => {
        setLastMessage('');
        try {
            const appAny = getAppBindings();
            if (appAny?.SelectOutputDirectory) {
                const dir = await appAny.SelectOutputDirectory();
                if (typeof dir === 'string' && dir.trim()) {
                    const trimmed = dir.trim();
                    setOutputDir(trimmed);
                    void rememberRecentDirs({ outputDir: trimmed });
                }
                return;
            }
            if (window.runtime?.OpenDirectoryDialog) {
                const dir = await window.runtime.OpenDirectoryDialog({ title: '选择输出位置' });
                if (typeof dir === 'string' && dir.trim()) {
                    const trimmed = dir.trim();
                    setOutputDir(trimmed);
                    void rememberRecentDirs({ outputDir: trimmed });
                }
            }
        } catch (e) {
            console.error(e);
            setLastMessage('选择输出目录失败');
        }
    };

    const updateFailedRecordsByPath = (filePath: string) => {
        const normalized = normalizePath(filePath);
        if (!normalized) return;
        if (currentRunFailedPathsRef.current) {
            currentRunFailedPathsRef.current.add(normalized);
        }
        const candidates = [...(dropResult?.files || []), ...failedRecords];
        const matched = candidates.find((f) => normalizePath(f.input_path) === normalized);
        if (!matched) return;
        setFailedRecords((prev) => {
            const exists = prev.some((f) => normalizePath(f.input_path) === normalized);
            if (exists) return prev;
            return [...prev, matched];
        });
    };
    const normalizeBackendError = (raw: string) => {
        const text = raw.trim();
        if (!text) return text;
        const match = text.match(/^\[([A-Z_]+)\]\s*(.*)$/);
        if (!match) return text;
        const code = match[1];
        const detail = match[2] || '';
        const codeMap: Record<string, string> = {
            NOT_FOUND: '未找到文件',
            BAD_INPUT: '输入参数无效',
            PERMISSION_DENIED: '没有权限访问目标路径',
            UNSUPPORTED_FORMAT: '当前格式不受支持',
            INVALID_ACTION: '操作类型无效',
            INTERNAL: '处理过程中发生内部错误',
            PY_CANCELLED: '已取消当前任务',
        };
        const prefix = codeMap[code] || '处理失败';
        if (!detail) return prefix;
        return `${prefix}：${detail}`;
    };
    const getErrorMessage = (error: unknown, fallback: string) => {
        if (typeof error === 'string' && error.trim()) return normalizeBackendError(error);
        if (typeof (error as any)?.message === 'string' && (error as any).message.trim()) {
            return normalizeBackendError((error as any).message);
        }
        return fallback;
    };
    const reportTaskFailure = (taskName: string, filePath: string, reason: unknown, fallback = '处理失败') => {
        if (isCancellationError(reason)) return;
        updateFailedRecordsByPath(filePath);
        if (!onTaskFailure) return;
        onTaskFailure({
            taskName,
            imageName: basename(filePath),
            reason: getErrorMessage(reason, fallback),
        });
    };
    const reportBatchTaskFailure = (taskName: string, files: DroppedFile[], reason: unknown, fallback = '处理失败') => {
        if (isCancellationError(reason)) return;
        files.forEach((f) => updateFailedRecordsByPath(f.input_path));
        if (!onTaskFailure) return;
        const firstPath = files[0]?.input_path || '';
        const firstName = firstPath ? basename(firstPath) : '批量输入';
        const imageName = files.length > 1 ? `${firstName} 等` : firstName;
        onTaskFailure({
            taskName,
            imageName,
            reason: getErrorMessage(reason, fallback),
        });
    };
    const clampTwoLinesStyle: React.CSSProperties = {
        maxHeight: '5rem',
        overflowY: 'auto',
    };
    function clampNumber(value: number, min: number, max: number) {
        return Math.min(max, Math.max(min, value));
    }
    useEffect(() => {
        if (!dropResult || dropResult.files.length === 0) {
            setPdfFileName('');
            return;
        }
        const suggested = buildSuggestedPdfName(dropResult.files, dropResult.has_directory);
        setPdfFileName(suggested);
    }, [dropResult]);


    const handlePathsExpanded = useCallback((result: ExpandDroppedPathsResult) => {
        setDropResult(result);
        const firstFile = result?.files?.[0];
        const inputDirForHistory = result?.has_directory
            ? (firstFile?.source_root || '')
            : dirname(firstFile?.input_path || '');
        if (inputDirForHistory) {
            void rememberRecentDirs({ inputDir: inputDirForHistory });
        }
        setLastMessage('');
        resetProgress(0);
    }, [dirname, normalizePath, rememberRecentDirs]);

    const requestCancelProcessing = useCallback(async () => {
        if (!isProcessing) return;
        cancelRequestedRef.current = true;
        setCancelRequested(true);
        setLastMessage('正在停止处理，请稍候...');
        try {
            const appAny = getAppBindings();
            if (appAny?.CancelProcessing) {
                await appAny.CancelProcessing();
            }
        } catch (err) {
            console.error('Failed to cancel processing on backend:', err);
        }
    }, [isProcessing]);

    // DetailView is keyed by the active tool.  Switching tools unmounts the
    // old view, so explicitly cancel its backend task instead of allowing a
    // stale run to continue writing files in the background.
    useEffect(() => {
        return () => {
            if (!processingLockRef.current) return;
            processingRunIdRef.current += 1;
            cancelRequestedRef.current = true;
            try {
                const cancel = (getAppBindings() as any)?.CancelProcessing;
                if (typeof cancel === 'function') void cancel();
            } catch (err) {
                console.error('Failed to cancel processing on view unmount:', err);
            }
        };
    }, []);

    const handlePaletteExport = async (kind: 'palette' | 'combined') => {
        const file = dropResult?.files?.[0];
        if (!file) {
            showToast('请先拖入图片', 'error');
            return;
        }
        if (!effectiveOutputDir) {
            showToast('请先选择输出目录', 'error');
            return;
        }
        setPaletteBusy(true);
        try {
            const suffix = `_palette_${kind === 'palette' ? 'strip' : 'combined'}`;
            const rel = buildOutputRelPath(file, {
                ext: 'png',
                suffix,
                seq: 1,
                op: 'palette',
                template: outputSettings.output_template,
                prefix: outputSettings.output_prefix,
                preserveStructure: outputSettings.preserve_folder_structure,
                date: new Date(),
            });
            const candidate = joinPath(effectiveOutputDir, rel);
            const app = getAppBindings();
            let outPath = candidate;
            if (app?.ResolveOutputPath) {
                const resolved = await app.ResolveOutputPath({
                    base_path: candidate,
                    strategy: outputSettings.conflict_strategy,
                    reserved: [],
                });
                if (outputSettings.conflict_strategy === 'skip' && resolved?.exists) {
                    showToast('输出文件已存在，已跳过', 'info');
                    return;
                }
                if (resolved?.success && resolved.output_path) outPath = normalizePath(resolved.output_path);
            }
            const res = await app?.PaletteTool?.({
                action: kind === 'palette' ? 'export_palette' : 'export_combined',
                input_path: normalizePath(file.input_path),
                output_path: outPath,
                swatch_height: swatchHeight,
                include_hex_text: includeText,
            });
            if (res?.success) {
                showToast('已导出', 'success');
                const openPath = (getAppBindings() as any)?.OpenPath;
                if (typeof openPath === 'function') void openPath({ path: effectiveOutputDir });
            } else {
                showToast(res?.error || '导出失败', 'error');
            }
        } catch (err) {
            console.error(err);
            showToast('导出失败', 'error');
        } finally {
            setPaletteBusy(false);
        }
    };

    const handleStartProcessing = async (overrideFiles?: DroppedFile[]) => {
        if (isProcessing) return;
        if (!Array.isArray(overrideFiles) && expansionCountRef.current > 0) {
            setLastMessage('正在展开文件夹，请稍候再开始处理');
            return;
        }
        cancelRequestedRef.current = false;
        setCancelRequested(false);
        setLastMessage('');
        resetProgress(0);
        const useOverrideFiles = Array.isArray(overrideFiles);
        const normalizedOverrideFiles = useOverrideFiles ? dedupeDroppedFiles(overrideFiles || []) : [];
        if (!useOverrideFiles && id !== 'richtext' && (!dropResult || dropResult.files.length === 0)) {
            setLastMessage('请先拖入文件或文件夹');
            return;
        }
        if (useOverrideFiles && normalizedOverrideFiles.length === 0) {
            setLastMessage('监听目录中暂无可处理的新文件');
            return;
        }
        const appAny = getAppBindings();
        if (!appAny) {
            setLastMessage('未检测到桌面宿主运行环境');
            return;
        }
        const manualRetryOnly = !useOverrideFiles && retryFailedOnly;
        if (manualRetryOnly && failedRecords.length === 0) {
            setLastMessage('失败列表为空，无法仅重试失败项');
            return;
        }

        const outDir = effectiveOutputDir;
        const requiresOutputDir =
            !((id === 'converter' && convOverwriteSource) || (id === 'compressor' && compOverwriteSource));
        if (requiresOutputDir && !outDir) {
            setLastMessage('请选择输出目录');
            return;
        }

        // Acquire synchronously immediately before the first await.  React
        // state updates are asynchronous, so relying on `isProcessing` alone
        // lets two rapid clicks enter this function concurrently.
        if (processingLockRef.current) return;
        processingLockRef.current = true;
        const runId = ++processingRunIdRef.current;
        setIsProcessing(true);

        const appSettingsSnapshot = await loadOutputSettings();
        const preserveStructure = Boolean(appSettingsSnapshot.preserve_folder_structure);
        const outputTemplate = appSettingsSnapshot.output_template || defaultOutputSettings.output_template;
        const outputPrefix = appSettingsSnapshot.output_prefix || defaultOutputSettings.output_prefix;
        const conflictStrategy = appSettingsSnapshot.conflict_strategy || defaultOutputSettings.conflict_strategy;
        const reservedPaths = new Set<string>();
        const batchTime = new Date();
        openOutputOnCompleteRef.current = {
            enabled: Boolean(appSettingsSnapshot.open_output_folder_on_complete),
            dir: outDir || '',
        };

        let currentRunFiles: DroppedFile[] = [];
        const activeHasDirectory = useOverrideFiles ? true : Boolean(dropResult?.has_directory);
        // Returns '' when conflictStrategy==='skip' and the target already exists (caller must skip the item).
        const resolveUniquePath = async (candidate: string) => {
            const normalized = normalizePath(candidate);
            if (conflictStrategy === 'overwrite') {
                reservedPaths.add(normalized);
                return normalized;
            }
            if (conflictStrategy === 'skip') {
                if (reservedPaths.has(normalized)) return '';
                try {
                    const res = await appAny?.ResolveOutputPath?.({
                        base_path: normalized,
                        strategy: 'skip',
                        reserved: Array.from(reservedPaths),
                    });
                    if (res?.success && res.exists) return '';
                } catch (err) {
                    console.error(err);
                }
                reservedPaths.add(normalized);
                return normalized;
            }
            if (!appAny?.ResolveOutputPath) {
                reservedPaths.add(normalized);
                return normalized;
            }
            try {
                const res = await appAny.ResolveOutputPath({
                    base_path: normalized,
                    strategy: 'rename',
                    reserved: Array.from(reservedPaths),
                });
                const resolved = res?.success && res.output_path ? normalizePath(res.output_path) : normalized;
                reservedPaths.add(resolved);
                return resolved;
            } catch (err) {
                console.error(err);
                reservedPaths.add(normalized);
                return normalized;
            }
        };

        const resolveUniquePathsBatch = async (candidates: string[]) => {
            const normalized = candidates.map((item) => normalizePath(item));
            if (!normalized.length) return [] as string[];
            if (conflictStrategy === 'overwrite' || !appAny?.ResolveOutputPaths) {
                const out: string[] = [];
                for (const item of normalized) {
                    out.push(await resolveUniquePath(item));
                }
                return out;
            }
            if (conflictStrategy === 'skip') {
                try {
                    const res = await appAny.ResolveOutputPaths({
                        items: normalized,
                        strategy: 'skip',
                        reserved: Array.from(reservedPaths),
                    });
                    if (
                        res?.success
                        && Array.isArray(res.paths)
                        && res.paths.length === normalized.length
                        && Array.isArray(res.exists)
                    ) {
                        const existsFlags = res.exists;
                        return normalized.map((item, idx) => {
                            if (existsFlags[idx] || reservedPaths.has(item)) return '';
                            reservedPaths.add(item);
                            return item;
                        });
                    }
                } catch (err) {
                    console.error(err);
                }
                const out: string[] = [];
                for (const item of normalized) {
                    out.push(await resolveUniquePath(item));
                }
                return out;
            }
            try {
                const res = await appAny.ResolveOutputPaths({
                    items: normalized,
                    reserved: Array.from(reservedPaths),
                });
                if (res?.success && Array.isArray(res.paths) && res.paths.length === normalized.length) {
                    const out = res.paths.map((item) => normalizePath(String(item || '')));
                    out.forEach((item) => reservedPaths.add(item));
                    return out;
                }
            } catch (err) {
                console.error(err);
            }
            const fallback: string[] = [];
            for (const item of normalized) {
                fallback.push(await resolveUniquePath(item));
            }
            return fallback;
        };

        const resolveToolOutputPath = async (
            file: DroppedFile,
            options: { ext?: string; suffix?: string; op: string; seq: number },
        ) => {
            const rel = buildOutputRelPath(file, {
                ...options,
                template: outputTemplate,
                prefix: outputPrefix,
                preserveStructure,
                date: batchTime,
            });
            return resolveUniquePath(joinPath(outDir, rel));
        };

        currentRunFailedPathsRef.current = new Set<string>();
        setCompressStats(null);
        setRunSummary(null);
        try {
            const files = useOverrideFiles
                ? normalizedOverrideFiles
                : (manualRetryOnly ? failedRecords : (dropResult?.files || []));
            currentRunFiles = files;
            const firstInput = files[0];
            const inputDirForHistory = activeHasDirectory
                ? (firstInput?.source_root || '')
                : dirname(firstInput?.input_path || '');
            void rememberRecentDirs({ inputDir: inputDirForHistory, outputDir: outDir });
            const total = files.length;
            let completed = 0;
            const getBatchChunkSize = (itemCount: number, requestsPerItem?: number) =>
                sharedGetBatchChunkSize(itemCount, requestsPerItem, appSettingsSnapshot.max_concurrency);

            if (id === 'converter') {
                if (!appAny.Convert || !appAny.ConvertBatch) {
                    setLastMessage('后端未接入格式转换接口');
                    return;
                }
                const format = convFormat.toLowerCase();
                const isIcoFormat = format === 'ico';
                const icoSizeGroups = isIcoFormat ? planIcoConversionSizes(convIcoSizes || [], convOverwriteSource) : [[]];
                const resizeModeMap: Record<string, string> = {
                    '原图尺寸': 'original',
                    '按比例': 'percent',
                    '固定宽高': 'fixed',
                    '最长边': 'long_edge',
                };
                const resize_mode = resizeModeMap[convResizeMode] ?? 'original';
                await runConvertBatch({
                    ctx: {
                        app: appAny,
                        files,
                        total,
                        cancelRequestedRef,
                        setCancelRequested,
                        setProgressThrottled,
                        flushProgress,
                        setLastMessage,
                        reportTaskFailure,
                        reportBatchTaskFailure,
                        getBatchChunkSize,
                        normalizePath,
                    },
                    format,
                    quality: convQuality,
                    compressLevel: convCompressLevel,
                    icoSizeGroups,
                    overwriteSource: convOverwriteSource,
                    resizeMode: resize_mode,
                    scalePercent: convScalePercent,
                    fixedWidth: convFixedWidth,
                    fixedHeight: convFixedHeight,
                    longEdge: convLongEdge,
                    maintainAR: convMaintainAR,
                    keepMetadata: convKeepMetadata,
                    outputDir: outDir,
                    outputTemplate,
                    outputPrefix,
                    preserveStructure,
                    batchTime,
                    buildOutputRelPath,
                    resolveUniquePath,
                    resolveConverterOverwritePath,
                    joinPath,
                    basename,
                    stripExtension,
                    onDone: (summary) => setRunSummary({ ...summary, outputDir: outDir }),
                });
                return;
            }

            if (id === 'compressor') {
                if (!appAny.Compress || !appAny.CompressBatch) {
                    setLastMessage('后端未接入压缩接口');
                    return;
                }
                const levelMap: Record<string, number> = {
                    '智能（推荐）': 0,
                    '更清晰': 2,
                    '更小': 4,
                    '无损': 1,
                    '轻度': 2,
                    '标准': 3,
                    '强力': 4,
                    '极限': 5,
                };
                const engineMap: Record<string, string> = {
                    '自动 (推荐)': 'auto',
                    'MozJPEG (JPEG)': 'mozjpeg',
                    'PNGQuant (PNG)': 'pngquant',
                    'OxiPNG (PNG 无损)': 'oxipng',
                    'Pillow (兼容)': 'pillow',
                };
                const level = levelMap[compMode] ?? 3;
                const engine = engineMap[compEngine] ?? 'auto';
                const targetSizeKB = compTargetSize ? Math.max(0, Number(compTargetSizeKB) || 0) : 0;
                const resizeModeMap: Record<string, string> = {
                    '不缩放': 'none',
                    '百分比': 'percent',
                    '像素': 'pixels',
                };
                await runCompressBatch({
                    ctx: {
                        app: appAny,
                        files,
                        total,
                        cancelRequestedRef,
                        setCancelRequested,
                        setProgressThrottled,
                        flushProgress,
                        setLastMessage,
                        reportTaskFailure,
                        reportBatchTaskFailure,
                        getBatchChunkSize,
                        normalizePath,
                    },
                    level,
                    engine,
                    targetSizeKB,
                    resizeMode: resizeModeMap[compResizeMode] ?? 'none',
                    resizePercent: Math.max(1, Math.min(500, Number(compResizePercent) || 100)),
                    resizeWidth: Math.max(0, Math.min(65535, Number(compResizeWidth) || 0)),
                    resizeHeight: Math.max(0, Math.min(65535, Number(compResizeHeight) || 0)),
                    maintainAspect: compMaintainAspect,
                    overwriteSource: compOverwriteSource,
                    outputDir: outDir,
                    outputTemplate,
                    outputPrefix,
                    preserveStructure,
                    batchTime,
                    buildOutputRelPath,
                    resolveUniquePath,
                    joinPath,
                    onStats: setCompressStats,
                    onDone: (summary) => setRunSummary({ ...summary, outputDir: outDir }),
                });
                return;
            }

            if (id === 'pdf') {
                if (!appAny.GeneratePDF) {
                    setLastMessage('后端未接入 PDF 接口');
                    return;
                }
                const sizeMap: Record<string, string> = {
                    'A0 (841 x 1189 mm)': 'A0',
                    'A1 (594 x 841 mm)': 'A1',
                    'A2 (420 x 594 mm)': 'A2',
                    'A3 (297 x 420 mm)': 'A3',
                    'A4 (210 x 297 mm)': 'A4',
                    'A5 (148 x 210 mm)': 'A5',
                    'A6 (105 x 148 mm)': 'A6',
                    'B4 (250 x 353 mm)': 'B4',
                    'B5 (176 x 250 mm)': 'B5',
                    'B6 (125 x 176 mm)': 'B6',
                    'Letter (8.5 x 11 in)': 'Letter',
                    'Legal (8.5 x 14 in)': 'Legal',
                    'Tabloid (11 x 17 in)': 'Tabloid',
                    'Ledger (17 x 11 in)': 'Ledger',
                };
                const compressionMap: Record<string, number> = {
                    '不压缩': 0,
                    '轻度': 1,
                    '标准': 2,
                    '强力': 3,
                };
                const fitMap: Record<string, string> = {
                    '适应页面 (保持比例)': 'contain',
                    '充满页面 (可能裁剪)': 'cover',
                    '原始大小 (居中)': 'original',
                };
                const marginPoints = Math.max(0, Math.round((Number(pdfMarginMm) || 0) * 72 / 25.4));
                const pdfBaseName = normalizePdfFileName(pdfFileName) || 'output';
                const pdfName = buildOutputName(pdfBaseName, {
                    template: outputTemplate,
                    prefix: outputPrefix,
                    seq: 1,
                    op: 'pdf',
                    date: batchTime,
                });
                const pdfOutputPath = await resolveUniquePath(joinPath(outDir, `${pdfName}.pdf`));
                if (!pdfOutputPath) {
                    setLastMessage('输出文件已存在，已跳过（冲突策略：跳过同名）');
                    setProgress(100);
                    return;
                }
                const req: models.PDFRequest = {
                    image_paths: files.map(f => normalizePath(f.input_path)),
                    output_path: pdfOutputPath,
                    page_size: sizeMap[pdfSize] ?? 'A4',
                    layout: pdfLayout === '横向' ? 'landscape' : 'portrait',
                    margin: marginPoints,
                    compression_level: compressionMap[pdfCompression] ?? 0,
                    fit_mode: fitMap[pdfFit] ?? 'contain',
                    title: pdfTitle.trim(),
                    author: pdfAuthor.trim(),
                    user_password: pdfPassword.trim() || undefined,
                };
                const res = await appAny.GeneratePDF(req);
                setProgress(100);
                if (!res?.success) {
                    if (isCancellationError(res?.error)) {
                        cancelRequestedRef.current = true;
                        setCancelRequested(true);
                        setLastMessage('PDF 处理已取消');
                        return;
                    }
                    setRunSummary({ total: 1, failed: 1, skipped: 0, outputDir: outDir });
                    reportTaskFailure('转 PDF', files[0]?.input_path || '', res?.error, 'PDF 生成失败');
                    setLastMessage(res?.error || 'PDF 生成失败');
                } else {
                    setRunSummary({ total: 1, failed: 0, skipped: 0, outputDir: outDir });
                }
                return;
            }

            if (id === 'gif') {
                const appAny = getAppBindings();
                if (!appAny?.SplitGIF) {
                    setLastMessage('后端未接入 GIF 接口');
                    return;
                }

                const probeCandidatePaths = selectAnimatedProbeCandidatePaths(files.map((file) => file.input_path));
                const frameCountByPath = new Map<string, number | null>();
                if (probeCandidatePaths.length > 0 && appAny?.ProbeAnimatedPaths) {
                    try {
                        const probeResults = await appAny.ProbeAnimatedPaths(probeCandidatePaths);
                        if (Array.isArray(probeResults)) {
                            probeResults.forEach((item) => {
                                const inputPath = normalizePath(String(item?.input_path || ''));
                                const key = pathLookupKey(inputPath);
                                if (!key) {
                                    return;
                                }
                                const frameCount = Number(item?.frame_count);
                                frameCountByPath.set(key, Number.isFinite(frameCount) ? frameCount : null);
                            });
                        }
                    } catch (error) {
                        console.error('Failed to batch probe animated inputs:', error);
                    }
                }
                const probeAnimatedFrameCount = async (inputPath: string) => {
                    const normalizedInputPath = normalizePath(inputPath);
                    const key = pathLookupKey(normalizedInputPath);
                    if (frameCountByPath.has(key)) {
                        return frameCountByPath.get(key) ?? null;
                    }
                    try {
                        const res = await appAny.SplitGIF?.({
                            action: 'get_frame_count',
                            input_path: normalizedInputPath,
                            maintain_aspect: true,
                        });
                        if (!res?.success) {
                            return null;
                        }
                        const frameCount = Number(res.frame_count);
                        return Number.isFinite(frameCount) ? frameCount : null;
                    } catch {
                        return null;
                    }
                };
                const animatedFlags = await Promise.all(
                    files.map((file) => detectAnimatedImagePath(file.input_path, probeAnimatedFrameCount))
                );
                const animatedFiles = files.filter((_, index) => animatedFlags[index]);
                const pureGifFiles = animatedFiles.filter((f) => extname(f.input_path) === 'gif');
                const otherFiles = files.filter((_, index) => !animatedFlags[index]);

                if (gifMode === '导出') {
                    if (animatedFiles.length > 0 && otherFiles.length > 0) {
                        setLastMessage('导出模式请只选择动图文件或图片序列');
                        return;
                    }

                    if (animatedFiles.length > 0) {
                        const outputFormat = gifExportFormat.toLowerCase();
                        let failed = 0;
                        let skipped = 0;
                        let seq = 1;
                        for (const f of animatedFiles) {
                            if (cancelRequestedRef.current) break;
                            const name = basename(f.input_path).replace(/\.[^.]+$/, '');
                            const rel = getRelPath(f, preserveStructure);
                            const relDir = getRelDir(rel);
                            const folderName = buildOutputName(`${name}_frames`, {
                                template: outputTemplate,
                                prefix: outputPrefix,
                                seq,
                                op: 'gif_frames',
                                date: batchTime,
                            });
                            const rawOutputDir = relDir ? joinPath(outDir, `${relDir}/${folderName}`) : joinPath(outDir, folderName);
                            const outputDir = await resolveUniquePath(rawOutputDir);
                            if (!outputDir) {
                                skipped++;
                                completed++;
                                seq += 1;
                                setProgress((completed / animatedFiles.length) * 100);
                                continue;
                            }
                            const req: models.GIFSplitRequest = {
                                action: 'export_frames',
                                input_path: normalizePath(f.input_path),
                                output_dir: outputDir,
                                output_format: outputFormat,
                                maintain_aspect: true,
                            };
                            try {
                                const res = await appAny.SplitGIF(req);
                                if (!res?.success) {
                                    const normalizedError = resolveGifErrorMessage(res?.error_code, res?.error);
                                    if (isCancellationError(res?.error)) {
                                        cancelRequestedRef.current = true;
                                        setCancelRequested(true);
                                        break;
                                    }
                                    failed++;
                                    reportTaskFailure('GIF 导出', f.input_path, normalizedError, '导出失败');
                                }
                            } catch (err) {
                                if (isCancellationError(err) || cancelRequestedRef.current) {
                                    cancelRequestedRef.current = true;
                                    setCancelRequested(true);
                                    break;
                                }
                                console.error(`Failed to export animated frames ${f.input_path}:`, err);
                                failed++;
                                reportTaskFailure('动图导出', f.input_path, err, '导出失败');
                            }
                            completed++;
                            seq += 1;
                            setProgress((completed / animatedFiles.length) * 100);
                        }
                        const cancelled = cancelRequestedRef.current;
                        setRunSummary({ total: animatedFiles.length, failed, skipped, outputDir: outDir });
                        const success = Math.max(0, completed - failed - skipped);
                        const extraParts = [
                            failed > 0 ? `失败 ${failed}` : '',
                            skipped > 0 ? `跳过 ${skipped}` : '',
                        ].filter(Boolean);
                        const extra = extraParts.length > 0 ? `（${extraParts.join('，')}）` : '';
                        if (cancelled || failed > 0 || skipped > 0) {
                            setLastMessage(cancelled
                                ? `导出已停止：成功 ${success}/${animatedFiles.length} 项${extra}`
                                : `导出失败：成功 ${success}/${animatedFiles.length} 项${extra}`);
                        }
                        return;
                    }

                    const first = files[0];
                    const baseName = activeHasDirectory && first?.source_root
                        ? basename(first.source_root)
                        : basename(first.input_path).replace(/\.[^.]+$/, '');
                    const safeName = baseName || 'output';
                    const combinedName = buildOutputName(`${safeName}_combined`, {
                        template: outputTemplate,
                        prefix: outputPrefix,
                        seq: 1,
                        op: 'gif_build',
                        date: batchTime,
                    });
                    const outputPath = await resolveUniquePath(joinPath(outDir, `${combinedName}.gif`));
                    if (!outputPath) {
                        setLastMessage('输出文件已存在，已跳过（冲突策略：跳过同名）');
                        setProgress(100);
                        return;
                    }
                    let buildFailed = false;
                        try {
                            const req: models.GIFSplitRequest = {
                                action: 'build_gif',
                                input_paths: files.map(f => normalizePath(f.input_path)),
                                output_path: outputPath,
                                fps: gifBuildFps,
                                maintain_aspect: true,
                            };
                            const res = await appAny.SplitGIF(req);
                            if (!res?.success) {
                                const normalizedError = resolveGifErrorMessage(res?.error_code, res?.error);
                                if (isCancellationError(res?.error)) {
                                    cancelRequestedRef.current = true;
                                    setCancelRequested(true);
                                    setLastMessage('GIF 合成已取消');
                                    return;
                                }
                                buildFailed = true;
                                reportTaskFailure('GIF 合成', files[0]?.input_path || '', normalizedError, '合成失败');
                                setLastMessage(normalizedError);
                            }
                        } catch (err) {
                            if (isCancellationError(err) || cancelRequestedRef.current) {
                                cancelRequestedRef.current = true;
                                setCancelRequested(true);
                                setLastMessage('GIF 合成已取消');
                                return;
                            }
                            buildFailed = true;
                            console.error('Failed to build GIF:', err);
                            reportTaskFailure('GIF 合成', files[0]?.input_path || '', err, '合成失败');
                            setLastMessage('合成失败');
                    } finally {
                        setProgress(100);
                    }
                    setRunSummary({ total: 1, failed: buildFailed ? 1 : 0, skipped: 0, outputDir: outDir });
                    return;
                }

                if (gifMode === '互转') {
                    if (animatedFiles.length === 0) {
                        setLastMessage('请先选择 GIF/APNG/WEBP 动图文件');
                        return;
                    }
                    if (otherFiles.length > 0) {
                        setLastMessage('互转模式仅支持 GIF/APNG/WEBP 动图输入');
                        return;
                    }

                    const targetExtMap: Record<string, string> = {
                        GIF: 'gif',
                        APNG: 'apng',
                        WEBP: 'webp',
                    };
                    const targetLabel = (gifConvertFormat || 'WEBP').toUpperCase();
                    const targetExt = targetExtMap[targetLabel] || 'webp';
                    let failed = 0;
                    let skipped = 0;
                    let seq = 1;
                    for (const f of animatedFiles) {
                        if (cancelRequestedRef.current) break;
                        const suffix = buildGifProcessSuffix(
                            'convert_animation',
                            gifSpeedPercent,
                            gifCompressQuality,
                            gifResizeWidth,
                            gifResizeHeight,
                            targetLabel,
                        );
                        const outRel = buildOutputRelPath(f, {
                            suffix,
                            seq,
                            op: 'gif_convert',
                            ext: targetExt,
                            template: outputTemplate,
                            prefix: outputPrefix,
                            preserveStructure,
                            date: batchTime,
                        });
                        const outputPath = await resolveUniquePath(joinPath(outDir, outRel));
                        if (!outputPath) {
                            skipped++;
                            completed++;
                            seq += 1;
                            setProgress((completed / animatedFiles.length) * 100);
                            continue;
                        }
                        try {
                            const req: models.GIFSplitRequest = {
                                action: 'convert_animation',
                                input_path: normalizePath(f.input_path),
                                output_path: outputPath,
                                output_format: targetExt,
                                maintain_aspect: true,
                            };
                            const res = await appAny.SplitGIF(req);
                            if (!res?.success) {
                                const normalizedError = resolveGifErrorMessage(res?.error_code, res?.error);
                                if (isCancellationError(res?.error)) {
                                    cancelRequestedRef.current = true;
                                    setCancelRequested(true);
                                    break;
                                }
                                failed++;
                                reportTaskFailure('动图互转', f.input_path, normalizedError, '互转失败');
                            }
                        } catch (err) {
                            if (isCancellationError(err) || cancelRequestedRef.current) {
                                cancelRequestedRef.current = true;
                                setCancelRequested(true);
                                break;
                            }
                            console.error(`Failed to convert animated image ${f.input_path}:`, err);
                            failed++;
                            reportTaskFailure('动图互转', f.input_path, err, '互转失败');
                        }
                        completed++;
                        seq += 1;
                        setProgress((completed / animatedFiles.length) * 100);
                    }
                    const cancelled = cancelRequestedRef.current;
                    setRunSummary({ total: animatedFiles.length, failed, skipped, outputDir: outDir });
                    const success = Math.max(0, completed - failed - skipped);
                    const extraParts = [
                        failed > 0 ? `失败 ${failed}` : '',
                        skipped > 0 ? `跳过 ${skipped}` : '',
                    ].filter(Boolean);
                    const extra = extraParts.length > 0 ? `（${extraParts.join('，')}）` : '';
                    if (cancelled || failed > 0 || skipped > 0) {
                        setLastMessage(cancelled
                            ? `互转已停止：成功 ${success}/${animatedFiles.length} 项${extra}`
                            : `互转失败：成功 ${success}/${animatedFiles.length} 项${extra}`);
                    }
                    return;
                }

                if (pureGifFiles.length === 0) {
                    setLastMessage('请先选择 GIF 文件');
                    return;
                }
                if (animatedFiles.length !== pureGifFiles.length || otherFiles.length > 0) {
                    setLastMessage('倒放、修改帧率、压缩和缩放只支持 GIF 输入');
                    return;
                }

                const action = resolveGifAction(gifMode);
                const speedFactor = clampNumber(gifSpeedPercent / 100, 0.5, 3);
                const resizeWidth = Math.max(0, Math.round(Number(gifResizeWidth) || 0));
                const resizeHeight = Math.max(0, Math.round(Number(gifResizeHeight) || 0));
                if (action === 'resize' && resizeWidth <= 0 && resizeHeight <= 0) {
                    setLastMessage('请至少填写宽度或高度');
                    return;
                }
                let failed = 0;
                let skipped = 0;
                let seq = 1;
                for (const f of pureGifFiles) {
                    if (cancelRequestedRef.current) break;
                    const suffix = buildGifProcessSuffix(
                        action,
                        gifSpeedPercent,
                        gifCompressQuality,
                        resizeWidth,
                        resizeHeight,
                    );
                    const processTargetMap: Record<string, string> = { GIF: 'gif', APNG: 'apng', WEBP: 'webp' };
                    const processTargetLabel = (gifConvertFormat || 'GIF').toUpperCase();
                    const processTargetExt = processTargetMap[processTargetLabel] || 'gif';
                    const outRel = buildOutputRelPath(f, {
                        suffix,
                        seq,
                        op: 'gif',
                        ext: processTargetExt,
                        template: outputTemplate,
                        prefix: outputPrefix,
                        preserveStructure,
                        date: batchTime,
                    });
                    const outputPath = await resolveUniquePath(joinPath(outDir, outRel));
                    if (!outputPath) {
                        skipped++;
                        completed++;
                        seq += 1;
                        setProgress((completed / pureGifFiles.length) * 100);
                        continue;
                    }
                    const req: models.GIFSplitRequest = {
                        action,
                        input_path: normalizePath(f.input_path),
                        output_path: outputPath,
                        maintain_aspect: gifResizeMaintainAR,
                    };
                    if (processTargetExt !== 'gif') {
                        req.output_format = processTargetExt;
                    }
                    if (action === 'change_speed') {
                        req.speed_factor = speedFactor;
                    } else if (action === 'compress') {
                        req.quality = gifCompressQuality;
                    } else if (action === 'resize') {
                        req.width = resizeWidth;
                        req.height = resizeHeight;
                        req.maintain_aspect = gifResizeMaintainAR;
                    }
                    try {
                        const res = await appAny.SplitGIF(req);
                        if (!res?.success) {
                            const normalizedError = resolveGifErrorMessage(res?.error_code, res?.error);
                            if (isCancellationError(res?.error)) {
                                cancelRequestedRef.current = true;
                                setCancelRequested(true);
                                break;
                            }
                            failed++;
                            reportTaskFailure(`GIF ${gifMode}`, f.input_path, normalizedError, '处理失败');
                        }
                    } catch (err) {
                        if (isCancellationError(err) || cancelRequestedRef.current) {
                            cancelRequestedRef.current = true;
                            setCancelRequested(true);
                            break;
                        }
                        console.error(`Failed to process GIF ${f.input_path}:`, err);
                        failed++;
                        reportTaskFailure(`GIF ${gifMode}`, f.input_path, err, '处理失败');
                    }
                    completed++;
                    seq += 1;
                    setProgress((completed / pureGifFiles.length) * 100);
                }
                const cancelled = cancelRequestedRef.current;
                setRunSummary({ total: pureGifFiles.length, failed, skipped, outputDir: outDir });
                const success = Math.max(0, completed - failed - skipped);
                const extraParts = [
                    failed > 0 ? `失败 ${failed}` : '',
                    skipped > 0 ? `跳过 ${skipped}` : '',
                ].filter(Boolean);
                const extra = extraParts.length > 0 ? `（${extraParts.join('，')}）` : '';
                if (cancelled || failed > 0 || skipped > 0) {
                    setLastMessage(cancelled
                        ? `${gifMode}已停止：成功 ${success}/${pureGifFiles.length} 项${extra}`
                        : `${gifMode}失败：成功 ${success}/${pureGifFiles.length} 项${extra}`);
                }
                return;
            }

            if (id === 'grid') {
                if (!appAny.CutImageGrid) {
                    setLastMessage('后端未接入切图接口');
                    return;
                }
                const gridSizeMap: Record<string, number> = { '四宫格': 2, '九宫格': 3, '十六宫格': 4 };
                const sizeN = gridSizeMap[gridSize] ?? 3;
                const fmt = (gridFormat || 'PNG').toLowerCase();
                const batch = await runSequentialToolBatch({
                    files,
                    cancelRequestedRef,
                    setCancelled: setCancelRequested,
                    resolveOutput: (f, seq) => resolveToolOutputPath(f, {
                        ext: '', suffix: '_grid', op: 'grid', seq,
                    }),
                    execute: (f, outputDir) => appAny.CutImageGrid!({
                            input_path: normalizePath(f.input_path),
                            output_dir: outputDir,
                            grid_size: sizeN,
                            output_format: fmt,
                    }),
                    onFailure: (f, reason) => reportTaskFailure('宫格切图', f.input_path, reason, '切图失败'),
                    onProgress: (done, total) => setProgress((done / total) * 100),
                });
                setProgress(batch.cancelled ? (batch.completed / files.length) * 100 : 100);
                setRunSummary({ total: files.length, failed: batch.failed, skipped: batch.skipped, outputDir: outDir });
                const extraParts = [batch.failed > 0 ? `失败 ${batch.failed}` : '', batch.skipped > 0 ? `跳过 ${batch.skipped}` : ''].filter(Boolean);
                const extra = extraParts.length ? `（${extraParts.join('，')}）` : '';
                if (batch.cancelled || batch.failed > 0 || batch.skipped > 0) {
                    setLastMessage(batch.cancelled
                        ? `切图已停止：成功 ${batch.succeeded}/${files.length} 项${extra}`
                        : `切图失败：成功 ${batch.succeeded}/${files.length} 项${extra}`);
                } else {
                    const tilesPerImage = sizeN * sizeN;
                    setLastMessage(`切图完成：${files.length} 张 → ${batch.succeeded * tilesPerImage} 块`);
                }
                return;
            }

            if (id === 'split') {
                if (!appAny.SplitImage) {
                    setLastMessage('后端未接入分割接口');
                    return;
                }
                const fmtLc = (splitParams.format || 'PNG').toLowerCase();
                let tiles = 0;
                const batch = await runSequentialToolBatch({
                    files,
                    cancelRequestedRef,
                    setCancelled: setCancelRequested,
                    resolveOutput: (f, seq) => resolveToolOutputPath(f, {
                        ext: '', suffix: '_split', op: 'split', seq,
                    }),
                    execute: async (f, outputDir) => {
                        const res = await appAny.SplitImage!({
                            input_path: normalizePath(f.input_path),
                            output_dir: outputDir,
                            rows: splitParams.rows,
                            cols: splitParams.cols,
                            mode: splitParams.mode,
                            output_format: fmtLc,
                        });
                        if (res?.success) {
                            tiles += Number(res.tile_count) || 0;
                        }
                        return res;
                    },
                    onFailure: (f, reason) => reportTaskFailure('图片布局', f.input_path, reason, '分割失败'),
                    onProgress: (done, total) => setProgress((done / total) * 100),
                });
                setProgress(batch.cancelled ? (batch.completed / files.length) * 100 : 100);
                setRunSummary({ total: files.length, failed: batch.failed, skipped: batch.skipped, outputDir: outDir });
                if (!batch.cancelled && batch.failed === 0 && batch.skipped === 0) setLastMessage(`分割完成：${files.length} 张 → ${tiles} 块`);
                else if (batch.failed > 0 || batch.skipped > 0 || batch.cancelled) {
                    setLastMessage(batch.cancelled
                        ? `分割已停止：成功 ${batch.succeeded}/${files.length}${batch.skipped ? `（跳过 ${batch.skipped}）` : ''}`
                        : `分割失败：成功 ${batch.succeeded}/${files.length}（失败 ${batch.failed}${batch.skipped ? `，跳过 ${batch.skipped}` : ''}）`);
                }
                return;
            }

            if (id === 'watermark') {
                if (!appAny.AddWatermark) {
                    setLastMessage('后端未接入水印接口');
                    return;
                }
                const style: WatermarkStyle = wmParams;
                const batch = await runSequentialToolBatch({
                    files,
                    cancelRequestedRef,
                    setCancelled: setCancelRequested,
                    resolveOutput: (f, seq) => resolveToolOutputPath(f, {
                        ext: 'png', suffix: '_wm', op: 'watermark', seq,
                    }),
                    execute: (f, outputPath) => appAny.AddWatermark!({
                            action: 'apply',
                            input_path: normalizePath(f.input_path),
                            output_path: outputPath,
                            text: style.text,
                            font_name: style.fontName,
                            size_mode: style.sizeMode,
                            font_size: style.fontSize,
                            color: style.color,
                            opacity: style.opacity,
                            rotation: style.rotation,
                            position: style.position,
                            margin: style.margin,
                            tiled: style.tiled,
                            tile_gap_x: style.tileGapX,
                            tile_gap_y: style.tileGapY,
                            stroke_width: style.strokeWidth,
                            stroke_color: style.strokeColor,
                            shadow: style.shadow,
                    }),
                    onFailure: (f, reason) => reportTaskFailure('批量水印', f.input_path, reason, '水印失败'),
                    onProgress: (done, total) => setProgress((done / total) * 100),
                });
                setProgress(batch.cancelled ? (batch.completed / files.length) * 100 : 100);
                setRunSummary({ total: files.length, failed: batch.failed, skipped: batch.skipped, outputDir: outDir });
                if (!(batch.cancelled || batch.failed > 0 || batch.skipped > 0)) setLastMessage(`水印完成：成功 ${batch.succeeded}/${files.length}`);
                else setLastMessage(batch.cancelled
                    ? `水印已停止：成功 ${batch.succeeded}/${files.length}${batch.skipped ? `（跳过 ${batch.skipped}）` : ''}`
                    : `水印失败：成功 ${batch.succeeded}/${files.length}（失败 ${batch.failed}${batch.skipped ? `，跳过 ${batch.skipped}` : ''}）`);
                return;
            }

            if (id === 'resize') {
                if (!appAny.ResizeImage) {
                    setLastMessage('后端未接入尺寸调整接口');
                    return;
                }
                const SAFE_OUT_EXTS = new Set(['png', 'jpg', 'jpeg', 'webp', 'bmp', 'tiff']);
                const batch = await runSequentialToolBatch({
                    files,
                    cancelRequestedRef,
                    setCancelled: setCancelRequested,
                    resolveOutput: (f, seq) => {
                    const origExt = extname(f.input_path) || 'png';
                    let outExt: string;
                    if (resizeParams.format === 'auto') {
                        outExt = SAFE_OUT_EXTS.has(origExt.toLowerCase()) ? origExt : 'png';
                    } else {
                        outExt = resizeParams.format.toLowerCase();
                    }
                        return resolveToolOutputPath(f, {
                        ext: outExt, suffix: '_resized', op: 'resize', seq,
                        });
                    },
                    execute: (f, outputPath) => appAny.ResizeImage!({
                            input_path: normalizePath(f.input_path),
                            output_path: outputPath,
                            mode: resizeParams.mode,
                            value: resizeParams.value,
                            output_format: resizeParams.format === 'auto' ? undefined : resizeParams.format.toLowerCase(),
                    }),
                    onFailure: (f, reason) => reportTaskFailure('尺寸调整', f.input_path, reason, '调整失败'),
                    onProgress: (done, total) => setProgress((done / total) * 100),
                });
                setProgress(batch.cancelled ? (batch.completed / files.length) * 100 : 100);
                setRunSummary({ total: files.length, failed: batch.failed, skipped: batch.skipped, outputDir: outDir });
                if (!(batch.cancelled || batch.failed > 0 || batch.skipped > 0)) setLastMessage(`尺寸调整完成：成功 ${batch.succeeded}/${files.length}`);
                else setLastMessage(batch.cancelled
                    ? `已停止：成功 ${batch.succeeded}/${files.length}${batch.skipped ? `（跳过 ${batch.skipped}）` : ''}`
                    : `失败：成功 ${batch.succeeded}/${files.length}（失败 ${batch.failed}${batch.skipped ? `，跳过 ${batch.skipped}` : ''}）`);
                return;
            }

            if (id === 'stitch') {
                if (!appAny.StitchImages) {
                    setLastMessage('后端未接入拼接接口');
                    return;
                }
                if (files.length < 1) {
                    setLastMessage('请先拖入图片');
                    return;
                }
                try {
                    const outputPath = await resolveToolOutputPath(files[0], {
                        ext: stitchParams.outputFormat.toLowerCase(),
                        suffix: '_stitched',
                        op: 'stitch',
                        seq: 1,
                    });
                    if (!outputPath) {
                        setRunSummary({ total: 1, failed: 0, skipped: 1, outputDir: outDir });
                        setProgress(100);
                        setLastMessage('输出文件已存在，已跳过（冲突策略：跳过同名）');
                        return;
                    }
                    const lengthKey = stitchParams.direction === 'vertical' ? 'target_width' : 'target_height';
                    const payload: Record<string, unknown> = {
                        input_paths: files.map((f) => normalizePath(f.input_path)),
                        output_path: outputPath,
                        direction: stitchParams.direction,
                        gap: stitchParams.gap,
                        background: stitchParams.background,
                        border_mode: stitchParams.border > 0 && files.length === 1,
                        border: stitchParams.border,
                        output_format: stitchParams.outputFormat.toLowerCase(),
                    };
                    if (stitchParams.targetLength > 0) payload[lengthKey] = stitchParams.targetLength;
                    const res = await appAny.StitchImages(payload as any);
                    setProgress(100);
                    if (!res?.success) {
                        if (isCancellationError(res?.error)) {
                            cancelRequestedRef.current = true;
                            setCancelRequested(true);
                            setLastMessage('拼接已取消');
                            return;
                        }
                        setRunSummary({ total: 1, failed: 1, skipped: 0, outputDir: outDir });
                        reportTaskFailure('长图拼接', files[0]?.input_path || '', res?.error || res?.error_code, '拼接失败');
                        setLastMessage(res?.error || '拼接失败');
                    } else {
                        setRunSummary({ total: 1, failed: 0, skipped: 0, outputDir: outDir });
                        setLastMessage(`拼接完成：${res.width}×${res.height}`);
                    }
                } catch (err) {
                    if (isCancellationError(err) || cancelRequestedRef.current) {
                        cancelRequestedRef.current = true;
                        setCancelRequested(true);
                        setLastMessage('拼接已取消');
                    } else {
                        console.error('Stitch error:', err);
                        setRunSummary({ total: 1, failed: 1, skipped: 0, outputDir: outDir });
                        reportBatchTaskFailure('长图拼接', files, err, '拼接失败');
                        setLastMessage('拼接失败');
                    }
                }
                return;
            }

            if (id === 'richtext') {
                if (!appAny.RenderRichText) {
                    setLastMessage('后端未接入富文本渲染');
                    return;
                }
                try {
                    const name = buildOutputName(stripExtension(rtParams.fileName.trim() || 'richtext'), {
                        template: outputTemplate,
                        prefix: outputPrefix,
                        seq: 1,
                        op: 'richtext',
                        date: batchTime,
                    });
                    const outputPath = await resolveUniquePath(joinPath(outDir, `${name}.png`));
                    if (!outputPath) {
                        setRunSummary({ total: 1, failed: 0, skipped: 1, outputDir: outDir });
                        setProgress(100);
                        setLastMessage('输出文件已存在，已跳过（冲突策略：跳过同名）');
                        return;
                    }
                    const res = await appAny.RenderRichText({
                        blocks: rtParams.blocks as unknown as Array<Record<string, any>>,
                        width: rtParams.width,
                        padding: rtParams.padding,
                        background: rtParams.background,
                        text_color: rtParams.textColor,
                        heading_color: rtParams.headingColor,
                        accent_color: rtParams.accentColor,
                        font_size: rtParams.fontSize,
                        line_height: rtParams.lineHeight,
                        font_name: rtParams.fontName,
                        output_path: outputPath,
                    });
                    setProgress(100);
                    if (!res?.success) {
                        setRunSummary({ total: 1, failed: 1, skipped: 0, outputDir: outDir });
                        reportTaskFailure('富文本制图', '', res?.error || res?.error_code, '渲染失败');
                        setLastMessage(res?.error || '渲染失败');
                    } else {
                        setRunSummary({ total: 1, failed: 0, skipped: 0, outputDir: outDir });
                        setLastMessage(`制图完成：${res.width}×${res.height}`);
                        if (Array.isArray(res.missing_images) && res.missing_images.length > 0) {
                            showToast('部分图片路径无效，已跳过', 'error');
                        }
                    }
                } catch (err) {
                    setProgress(100);
                    setRunSummary({ total: 1, failed: 1, skipped: 0, outputDir: outDir });
                    reportTaskFailure('富文本制图', '', err, '渲染失败');
                    setLastMessage('渲染失败');
                }
                return;
            }

            setLastMessage('当前工具不支持此处理操作');
        } catch (e: any) {
            console.error(e);
            reportBatchTaskFailure(feature.title, currentRunFiles.length > 0 ? currentRunFiles : (dropResult?.files || []), e, '处理失败');
            if (isCancellationError(e)) {
                setLastMessage('任务已取消');
            } else {
                setLastMessage(typeof e?.message === 'string' ? e.message : '处理失败');
            }
        } finally {
            if (runId !== processingRunIdRef.current) return;
            const failedSet = currentRunFailedPathsRef.current;
            if (failedSet && currentRunFiles.length > 0) {
                const nextFailed = currentRunFiles.filter((f) => failedSet.has(normalizePath(f.input_path)));
                setFailedRecords(nextFailed);
                if (nextFailed.length === 0) {
                    setRetryFailedOnly(false);
                }
            }
            if (!cancelRequestedRef.current) {
                const cfg = openOutputOnCompleteRef.current;
                if (cfg.enabled && cfg.dir) {
                    try {
                        const openPath = (getAppBindings() as any)?.OpenPath;
                        if (typeof openPath === 'function') {
                            void openPath({ path: cfg.dir });
                        }
                    } catch (err) {
                        console.error(err);
                    }
                }
            }
            openOutputOnCompleteRef.current = { enabled: false, dir: '' };
            currentRunFailedPathsRef.current = null;
            cancelRequestedRef.current = false;
            setCancelRequested(false);
            setIsProcessing(false);
            processingLockRef.current = false;
        }
    };

    const acceptedInputFormats = id === 'compressor'
        ? COMPRESSOR_ACCEPTED_FORMATS
        : DEFAULT_IMAGE_ACCEPTED_FORMATS;
    const inputFileDialogFilters = id === 'compressor'
        ? COMPRESSOR_FILE_DIALOG_FILTERS
        : DEFAULT_IMAGE_FILE_DIALOG_FILTERS;
    const dropZone = (
        <FileDropZone
            isActive={isActive}
            onPathsExpanded={handlePathsExpanded}
            onExpansionStateChange={handleExpansionStateChange}
            acceptedFormats={acceptedInputFormats}
            fileDialogFilters={inputFileDialogFilters}
            allowMultiple={true}
            subTitle=""
        />
    );

    const renderInputSection = (compact = false) => (
        <div className={`${compact ? 'space-y-1 pb-2' : 'space-y-2 pb-3'} border-b border-gray-100 dark:border-white/5 ${compact ? 'mb-2' : 'mb-3'}`}>
            <div className="flex items-center justify-between gap-2 flex-wrap text-sm">
                <div className="flex items-center gap-2">
                    <span className="text-gray-700 dark:text-gray-300 font-medium">输入项</span>
                    <span className="text-gray-500 dark:text-gray-400 font-mono text-xs">{inputCount}</span>
                </div>
                <button
                    type="button"
                    onClick={handleSelectOutputDir}
                    disabled={isProcessing}
                    className="shrink-0 px-2.5 py-1.5 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-[13px] font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]"
                >
                    选择输出位置
                </button>
            </div>
            {effectiveOutputDir && (
                <div
                    className="text-xs text-gray-500 dark:text-gray-400 font-mono break-all bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2"
                    style={clampTwoLinesStyle}
                >
                    {effectiveOutputDir}
                </div>
            )}
            {lastMessage && (
                <div
                    role="status"
                    aria-live="polite"
                    className="text-xs text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2 break-all"
                    style={clampTwoLinesStyle}
                >
                    {lastMessage}
                </div>
            )}
        </div>
    );

    const handleOpenOutputDir = async () => {
        const dir = runSummary?.outputDir || effectiveOutputDir;
        if (!dir) {
            showToast('未选择输出目录', 'error');
            return;
        }
        try {
            const openPath = (getAppBindings() as any)?.OpenPath;
            if (typeof openPath === 'function') {
                await openPath({ path: dir });
            }
        } catch (err) {
            console.error(err);
            showToast('打开目录失败', 'error');
        }
    };

    const tweenOriginalBytes = useTweenNumber(compressStats?.totalOriginalBytes ?? 0);
    const tweenCompressedBytes = useTweenNumber(compressStats?.totalCompressedBytes ?? 0);

    const renderActionContent = (compact = false) => (
        <>
            {!isProcessing && runSummary && runSummary.total > 0 && (
                <div role="status" aria-live="polite" className="text-xs rounded-xl px-3 py-2.5 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 space-y-1.5 spring-enter">
                    <div className="flex items-center justify-between gap-2">
                        <span className="font-medium text-gray-700 dark:text-gray-200">处理完成</span>
                        <span className="text-gray-500 dark:text-gray-400">
                            成功 {runSummary.total - runSummary.failed - runSummary.skipped} / {runSummary.total}
                            {runSummary.failed > 0 ? ` · 失败 ${runSummary.failed}` : ''}
                            {runSummary.skipped > 0 ? ` · 跳过 ${runSummary.skipped}` : ''}
                        </span>
                    </div>
                    {compressStats && compressStats.totalOriginalBytes > 0 && (
                        <div className="flex items-center justify-between gap-2 font-mono">
                            <span className="text-gray-500 dark:text-gray-400">
                                {formatBytes(tweenOriginalBytes)} → {formatBytes(tweenCompressedBytes)}
                            </span>
                            <span className={tweenCompressedBytes < tweenOriginalBytes ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-400'}>
                                节省 {formatBytes(Math.max(0, tweenOriginalBytes - tweenCompressedBytes))}
                                {' '}({Math.round((1 - tweenCompressedBytes / tweenOriginalBytes) * 100)}%)
                            </span>
                        </div>
                    )}
                    <div className="flex items-center gap-2 pt-0.5">
                        <button
                            type="button"
                            onClick={() => void handleOpenOutputDir()}
                            className="spring-tap-sm flex-1 py-1.5 rounded-lg border border-gray-200 dark:border-white/10 text-[11px] font-medium text-gray-700 dark:text-gray-200 hover:bg-white dark:hover:bg-white/10 transition-colors"
                        >
                            打开目录
                        </button>
                        <button
                            type="button"
                            onClick={() => { void handleStartProcessing(); }}
                            className="spring-tap-sm flex-1 py-1.5 rounded-lg bg-gradient-to-r from-[#007AFF] to-[#0055FF] hover:to-[#0044DD] text-[11px] font-medium text-white shadow-sm shadow-blue-500/25 transition-colors"
                        >
                            再处理一批
                        </button>
                    </div>
                </div>
            )}
            {(isProcessing || progress > 0) && (
                <ProgressBar progress={progress} label={isProcessing ? "正在处理..." : "已完成"} />
            )}
            {id !== 'palette' && (
                <button
                    type="button"
                    onClick={isProcessing ? requestCancelProcessing : () => { void handleStartProcessing(); }}
                    disabled={(isProcessing && cancelRequested) || (!isProcessing && isExpandingPaths)}
                    className={`spring-tap-sm w-full ${compact ? 'py-3' : 'py-3.5'} rounded-xl font-semibold shadow-lg flex items-center justify-center gap-2 text-white transition-all duration-300 ${(isProcessing && cancelRequested) || (!isProcessing && isExpandingPaths) ? 'bg-gray-400 cursor-not-allowed shadow-none' : isProcessing ? 'bg-amber-500 hover:bg-amber-600 shadow-amber-500/20' : 'bg-gradient-to-r from-[#007AFF] to-[#0055FF] hover:to-[#0044DD] shadow-blue-500/25'}`}
                >
                    <Icon name={isProcessing ? 'Close' : 'ArrowRight'} size={18} /> {isProcessing ? (cancelRequested ? '停止中...' : '停止处理') : isExpandingPaths ? '正在读取文件...' : '开始处理'}
                </button>
            )}
        </>
    );

    const renderActionSection = (compact = false) => (
        <div className={`${compact ? 'pt-3' : 'pt-4'} border-t border-gray-100 dark:border-white/5 mt-auto shrink-0 space-y-3`}>
            {failedRecords.length > 0 && (
                <div className="text-xs bg-amber-50 dark:bg-amber-500/10 border border-amber-200 dark:border-amber-500/20 rounded-xl px-3 py-2 text-amber-700 dark:text-amber-300 space-y-2">
                    <div>失败项：{failedRecords.length}，可仅重试失败文件。</div>
                    <div className="flex items-center gap-2">
                        <button
                            onClick={() => setRetryFailedOnly((v) => !v)}
                            className={`spring-tap-sm px-2.5 py-1.5 rounded-lg border text-xs font-medium ${retryFailedOnly ? 'border-amber-500 text-amber-700 dark:text-amber-300 bg-amber-100/60 dark:bg-amber-500/20' : 'border-amber-200 dark:border-amber-500/20 text-amber-700 dark:text-amber-300'}`}
                        >
                            {retryFailedOnly ? '仅重试失败项：开启' : '仅重试失败项：关闭'}
                        </button>
                        <button
                            onClick={() => {
                                setFailedRecords([]);
                                setRetryFailedOnly(false);
                            }}
                            className="spring-tap-sm px-2.5 py-1.5 rounded-lg border border-amber-200 dark:border-amber-500/20 text-xs font-medium text-amber-700 dark:text-amber-300"
                        >
                            清空失败列表
                        </button>
                    </div>
                </div>
            )}
            {renderActionContent(compact)}
        </div>
    );

    const settingsPanel = (
        <div
            className={`imageflow-settings-panel bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col overflow-hidden h-full min-h-0`}
        >
            {renderInputSection()}
            <div className="imageflow-settings-scroll flex-1 min-h-0 overflow-y-auto no-scrollbar px-1 pb-2">
                {renderSettings()}
            </div>

            {renderActionSection()}
        </div>
    );

    return (
        <div
            className="imageflow-detail-page imageflow-detail-page--standard min-h-full flex flex-col p-1"
            data-feature-id={feature.id}
            role="region"
            aria-label={feature.title}
        >
            <div className="imageflow-detail-layout imageflow-detail-layout--standard flex-1 gap-6 min-h-0">
                {id === 'grid' ? (
                    <div className="h-full min-h-0 flex flex-col gap-3 min-w-0">
                        <GridPreviewCard
                            file={dropResult?.files?.[0] || null}
                            rows={gridSize === '四宫格' ? 2 : gridSize === '十六宫格' ? 4 : 3}
                            cols={gridSize === '四宫格' ? 2 : gridSize === '十六宫格' ? 4 : 3}
                            totalCount={inputCount}
                        />
                        <div className="shrink-0 h-[190px]">
                            {dropZone}
                        </div>
                    </div>
                ) : id === 'split' ? (
                    <div className="h-full min-h-0 flex flex-col gap-3 min-w-0">
                        <GridPreviewCard
                            file={dropResult?.files?.[0] || null}
                            rows={splitParams.rows}
                            cols={splitParams.cols}
                            totalCount={inputCount}
                        />
                        <div className="shrink-0 h-[190px]">
                            {dropZone}
                        </div>
                    </div>
                ) : id === 'palette' ? (
                    <div className="h-full min-h-0 flex flex-col gap-3 min-w-0">
                        <GridPreviewCard
                            file={dropResult?.files?.[0] || null}
                            rows={1}
                            cols={1}
                            totalCount={inputCount}
                        />
                        <div className="shrink-0 h-[190px]">
                            {dropZone}
                        </div>
                    </div>
                ) : id === 'watermark' ? (
                    <div className="h-full min-h-0 flex flex-col gap-3 min-w-0">
                        <div className="flex-1 min-h-0 flex flex-col bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-100 dark:border-white/5 shadow-sm p-4 overflow-hidden">
                            <div className="flex items-center justify-between gap-3 pb-2 shrink-0">
                                <span className="text-sm font-medium text-gray-700 dark:text-gray-300 truncate min-w-0">
                                    {dropResult?.files?.[0]
                                        ? (dropResult.files[0].relative_path.split('/').pop() || '')
                                        : ''}
                                </span>
                                <span className="shrink-0 text-xs text-gray-400 font-mono">{inputCount > 1 ? `1/${inputCount}` : ''}</span>
                            </div>
                            <div className="flex-1 min-h-0 flex items-center justify-center overflow-hidden rounded-xl bg-gray-50 dark:bg-black/20">
                                {!dropResult?.files?.[0] ? (
                                    <div className="text-sm text-gray-300 dark:text-gray-600">拖入图片</div>
                                ) : !wmPreviewUrl ? (
                                    <div className="text-xs text-gray-400">加载中...</div>
                                ) : (
                                    <img src={wmPreviewUrl} alt="" draggable={false} className="max-w-full max-h-full object-contain spring-pop" />
                                )}
                            </div>
                        </div>
                        <div className="shrink-0 h-[190px]">
                            {dropZone}
                        </div>
                    </div>
                ) : id === 'richtext' ? (
                    <div className="h-full min-h-0 flex flex-col bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-100 dark:border-white/5 shadow-sm p-4 overflow-hidden">
                        <div className="flex items-center justify-between gap-3 pb-2 shrink-0">
                            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">实时预览</span>
                        </div>
                        <div className="flex-1 min-h-0 overflow-auto rounded-xl bg-gray-50 dark:bg-black/20 imageflow-page-scroll">
                            {!rtPreviewUrl ? (
                                <div className="h-full min-h-[200px] flex items-center justify-center text-xs text-gray-400">输入内容</div>
                            ) : (
                                <img src={rtPreviewUrl} alt="" draggable={false} className="w-full h-auto block" />
                            )}
                        </div>
                    </div>
                ) : (
                    <div className="imageflow-standard-input h-full min-h-0">
                        {dropZone}
                    </div>
                )}

                {/* Right Side: Specific Settings Panel (Secondary Menu) */}
                <div className="h-full min-h-0">
                    {settingsPanel}
                </div>
            </div>
        </div>
    );
};

export default memo(DetailView);

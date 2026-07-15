import React, { useCallback, useMemo, useState, useEffect, useRef, memo } from 'react';
import Icon from './Icon';
import { FEATURES } from '../constants';
import { ViewState } from '../types';
import { Switch, StyledSlider, CustomSelect, SegmentedControl, PositionGrid, FileDropZone, ProgressBar } from './Controls';
import {
    buildGifProcessSuffix,
    detectAnimatedImagePath,
    normalizeGifSpeedPercent,
    planIcoConversionSizes,
    resolveConverterOverwritePath,
    resolveGifAction,
    resolveWatermarkBackendPosition,
    selectAnimatedProbeCandidatePaths,
} from './gifHelpers';
import { resolveGifErrorMessage } from './gifErrors';
import GifSettingsPanel from './GifSettingsPanel';
import { useGifResizeState } from './hooks/useGifResizeState';
import { useThrottledProgress } from './hooks/useThrottledProgress';
import { useImagePreview } from './hooks/useImagePreview';
import {
    useConverterParams,
    useCompressorParams,
    usePdfParams,
    useAdjustParams,
    useFilterParams,
    useGifParams,
    useWatermarkParams,
} from './hooks/useFeatureParams';
import {
    defaultOutputSettings,
    loadFeaturePresetStore,
    saveFeaturePresetStore,
    type DroppedFile,
    type ExpandDroppedPathsResult,
    type FeaturePreset,
    type FeaturePresetStore,
    type OutputSettings,
    getBatchChunkSize as sharedGetBatchChunkSize,
    normalizePath as sharedNormalizePath,
    dirname as sharedDirname,
    joinPath as sharedJoinPath,
    basename as sharedBasename,
    stripExtension as sharedStripExtension,
    extname as sharedExtname,
    clampNumber as sharedClampNumber,
} from './detail/detailTypes';
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
} from '../types/wails-api';
import type { models } from '../types/backend-models';
import { ConverterSettings } from './detail/ConverterSettingsPanel';
import { CompressorSettings } from './detail/CompressorSettingsPanel';
import { WatermarkSettings } from './detail/WatermarkSettingsPanel';
import { AdjustCropControls, AdjustSettings } from './detail/AdjustSettingsPanel';
import { FILTER_LABELS, FILTER_PRESETS, FilterControls, FilterSettings } from './detail/FilterSettingsPanel';
import { PdfSettings } from './detail/PdfSettingsPanel';
import { InfoSettings } from './detail/InfoSettingsPanel';
import { runGenericBatch, runConvertBatch, runCompressBatch } from './detail/batchRunners';

const DEFAULT_IMAGE_INPUT_EXTENSIONS = [
    '.jpg',
    '.jpeg',
    '.png',
    '.webp',
    '.gif',
    '.bmp',
    '.avif',
    '.ico',
    '.tiff',
    '.tif',
    '.svg',
];

const INFO_IMAGE_INPUT_EXTENSIONS = [
    ...DEFAULT_IMAGE_INPUT_EXTENSIONS,
    '.heic',
    '.heif',
];

const COMPRESSOR_INPUT_EXTENSIONS = [
    '.jpg',
    '.jpeg',
    '.png',
    '.webp',
    '.bmp',
    '.avif',
    '.ico',
    '.tiff',
    '.tif',
];

const extensionsToAccept = (extensions: string[]) => extensions.join(',');
const extensionsToDialogPattern = (extensions: string[]) => extensions.map((ext) => `*${ext}`).join(';');

const DEFAULT_IMAGE_ACCEPTED_FORMATS = extensionsToAccept(DEFAULT_IMAGE_INPUT_EXTENSIONS);
const INFO_IMAGE_ACCEPTED_FORMATS = extensionsToAccept(INFO_IMAGE_INPUT_EXTENSIONS);
const COMPRESSOR_ACCEPTED_FORMATS = extensionsToAccept(COMPRESSOR_INPUT_EXTENSIONS);
const DEFAULT_IMAGE_FILE_DIALOG_FILTERS = [{
    DisplayName: 'Images',
    Pattern: extensionsToDialogPattern(DEFAULT_IMAGE_INPUT_EXTENSIONS),
}];
const INFO_IMAGE_FILE_DIALOG_FILTERS = [{
    DisplayName: 'Images',
    Pattern: extensionsToDialogPattern(INFO_IMAGE_INPUT_EXTENSIONS),
}];
const COMPRESSOR_FILE_DIALOG_FILTERS = [{
    DisplayName: 'Compressible bitmaps',
    Pattern: extensionsToDialogPattern(COMPRESSOR_INPUT_EXTENSIONS),
}];

interface DetailViewProps {
    id: ViewState;
    onBack: () => void;
    isActive?: boolean;
    onTaskFailure?: (payload: { taskName: string; imageName: string; reason: string }) => void;
}



const DetailView: React.FC<DetailViewProps> = ({ id, isActive = true, onTaskFailure }) => {
    const feature = FEATURES.find(f => f.id === id);
    if (!feature) return null;

    const isInfo = id === 'info';
    const isAdjustOrFilter = id === 'adjust' || id === 'filter';
    const isWatermark = id === 'watermark';
    const isPreviewFeature = isAdjustOrFilter || isWatermark;
    const [dropResult, setDropResult] = useState<ExpandDroppedPathsResult | null>(null);
    const [outputDir, setOutputDir] = useState('');
    const [isProcessing, setIsProcessing] = useState(false);
    const [cancelRequested, setCancelRequested] = useState(false);
    const cancelRequestedRef = useRef(false);
    const infoRequestIdRef = useRef(0);
    const infoRequestTimerRef = useRef<number | null>(null);
    const {
        progress,
        setProgress,
        setProgressThrottled,
        flushProgress,
        resetProgress,
    } = useThrottledProgress(0);

    const [lastMessage, setLastMessage] = useState<string>('');
    const [outputSettings, setOutputSettings] = useState<OutputSettings>(defaultOutputSettings);
    const [featurePresets, setFeaturePresets] = useState<FeaturePresetStore>(() => loadFeaturePresetStore());
    const [presetNameDraft, setPresetNameDraft] = useState('');
    const [selectedPresetId, setSelectedPresetId] = useState('');
    const [isPresetSectionCollapsed, setIsPresetSectionCollapsed] = useState(true);
    const [failedRecords, setFailedRecords] = useState<DroppedFile[]>([]);
    const [retryFailedOnly, setRetryFailedOnly] = useState(false);
    const currentRunFailedPathsRef = useRef<Set<string> | null>(null);

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
    } = usePdfParams();
    const {
        exposure: adjustExposure, setExposure: setAdjustExposure,
        contrast: adjustContrast, setContrast: setAdjustContrast,
        saturation: adjustSaturation, setSaturation: setAdjustSaturation,
        sharpness: adjustSharpness, setSharpness: setAdjustSharpness,
        vibrance: adjustVibrance, setVibrance: setAdjustVibrance,
        hue: adjustHue, setHue: setAdjustHue,
        rotate: adjustRotate, setRotate: setAdjustRotate,
        flipH: adjustFlipH, setFlipH: setAdjustFlipH,
        flipV: adjustFlipV, setFlipV: setAdjustFlipV,
        cropRatio: adjustCropRatio, setCropRatio: setAdjustCropRatio,
    } = useAdjustParams();
    const {
        intensity: filterIntensity, setIntensity: setFilterIntensity,
        grain: filterGrain, setGrain: setFilterGrain,
        vignette: filterVignette, setVignette: setFilterVignette,
        selected: filterSelected, setSelected: setFilterSelected,
    } = useFilterParams();
    const {
        mode: gifMode, setMode: setGifMode,
        exportFormat: gifExportFormat, setExportFormat: setGifExportFormat,
        convertFormat: gifConvertFormat, setConvertFormat: setGifConvertFormat,
        speedPercent: gifSpeedPercent, setSpeedPercent: setGifSpeedPercent,
        compressQuality: gifCompressQuality, setCompressQuality: setGifCompressQuality,
        buildFps: gifBuildFps, setBuildFps: setGifBuildFps,
    } = useGifParams();
    const [infoFilePath, setInfoFilePath] = useState('');
    const [infoPreview, setInfoPreview] = useState<any | null>(null);
    const [previewPath, setPreviewPath] = useState('');
    const { previewDataUrl, previewLoadError, setPreviewDataUrl, setPreviewLoadError } = useImagePreview({
        enabled: isPreviewFeature,
        path: previewPath,
        debounceMs: 120,
    });
    const previewContainerRef = useRef<HTMLDivElement>(null);
    const [previewContainerSize, setPreviewContainerSize] = useState({ width: 0, height: 0 });
    const [previewImageSize, setPreviewImageSize] = useState({ width: 0, height: 0 });
    const [cropOffset, setCropOffset] = useState({ x: 0, y: 0 });
    const [isCropDragging, setIsCropDragging] = useState(false);
    const cropDragRef = useRef<{ startX: number; startY: number; offsetX: number; offsetY: number } | null>(null);
    const [isComparing, setIsComparing] = useState(false);
    const {
        type: watermarkType, setType: setWatermarkType,
        text: watermarkText, setText: setWatermarkText,
        imagePath: watermarkImagePath, setImagePath: setWatermarkImagePath,
        position: watermarkPosition, setPosition: setWatermarkPosition,
        opacity: watermarkOpacity, setOpacity: setWatermarkOpacity,
        rotate: watermarkRotate, setRotate: setWatermarkRotate,
        size: watermarkSize, setSize: setWatermarkSize,
        tiled: watermarkTiled, setTiled: setWatermarkTiled,
        blendMode: watermarkBlendMode, setBlendMode: setWatermarkBlendMode,
        shadow: watermarkShadow, setShadow: setWatermarkShadow,
        margin: watermarkMargin, setMargin: setWatermarkMargin,
        font: watermarkFont, setFont: setWatermarkFont,
        color: watermarkColor, setColor: setWatermarkColor,
        useSystemFonts, setUseSystemFonts,
        systemFonts, setSystemFonts,
        isSystemFontsLoading, setIsSystemFontsLoading,
        imageSize: watermarkImageSize, setImageSize: setWatermarkImageSize,
    } = useWatermarkParams();

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
            conflict_strategy: typeof raw.conflict_strategy === 'string' && raw.conflict_strategy.trim() === 'rename'
                ? 'rename'
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
    const normalizePath = useCallback((p: string) => sharedNormalizePath(p), []);
    const dirname = useCallback((p: string) => sharedDirname(p), []);
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
    const currentFeaturePresets = useMemo(() => {
        return (featurePresets[id] || []).slice().sort((a, b) => b.updated_at - a.updated_at);
    }, [featurePresets, id]);

    useEffect(() => {
        if (currentFeaturePresets.length === 0) {
            setSelectedPresetId('');
            return;
        }
        if (!selectedPresetId || !currentFeaturePresets.some((p) => p.id === selectedPresetId)) {
            setSelectedPresetId(currentFeaturePresets[0].id);
        }
    }, [currentFeaturePresets, selectedPresetId]);

    const previewSrc = useMemo(() => {
        if (!previewPath) return '';
        if (previewDataUrl) return previewDataUrl;
        return toFileUrl(previewPath);
    }, [previewDataUrl, previewPath]);
    const watermarkImageSrc = useMemo(() => {
        if (!watermarkImagePath) return '';
        return toFileUrl(watermarkImagePath);
    }, [watermarkImagePath]);
    const baseFontOptions = useMemo(() => ([
        'Sans Serif',
        'Serif',
        'Mono',
        'Handwriting',
    ]), []);
    const systemFontOptions = useMemo(() => systemFonts.map((fontPath) => ({
        label: fontPath.replace(/\\/g, '/').split('/').pop() || fontPath,
        value: fontPath,
    })), [systemFonts]);
    const fontOptions = useMemo(() => (
        useSystemFonts ? [...baseFontOptions, ...systemFontOptions] : baseFontOptions
    ), [useSystemFonts, baseFontOptions, systemFontOptions]);

    useEffect(() => {
        setPreviewImageSize({ width: 0, height: 0 });
    }, [previewSrc]);

    useEffect(() => {
        if (useSystemFonts) return;
        if (baseFontOptions.includes(watermarkFont)) return;
        setWatermarkFont('Sans Serif');
    }, [useSystemFonts, watermarkFont, baseFontOptions]);

    useEffect(() => {
        if (!useSystemFonts) return;
        if (systemFonts.length > 0) return;
        const appAny = getAppBindings();
        if (!appAny?.ListSystemFonts) return;
        let active = true;
        setIsSystemFontsLoading(true);
        (async () => {
            try {
                const res = await appAny.ListSystemFonts();
                if (!active) return;
                if (Array.isArray(res)) {
                    setSystemFonts(res);
                } else {
                    setSystemFonts([]);
                }
            } catch (err) {
                if (active) {
                    console.error(err);
                    setSystemFonts([]);
                }
            } finally {
                if (active) {
                    setIsSystemFontsLoading(false);
                }
            }
        })();
        return () => {
            active = false;
        };
    }, [useSystemFonts, systemFonts.length]);

    useEffect(() => {
        if (!watermarkImageSrc) {
            setWatermarkImageSize({ width: 0, height: 0 });
            return;
        }
        let active = true;
        const img = new Image();
        img.onload = () => {
            if (!active) return;
            setWatermarkImageSize({
                width: img.naturalWidth,
                height: img.naturalHeight,
            });
        };
        img.onerror = () => {
            if (active) {
                setWatermarkImageSize({ width: 0, height: 0 });
            }
        };
        img.src = watermarkImageSrc;
        return () => {
            active = false;
        };
    }, [watermarkImageSrc]);
    const previewFilter = useMemo(() => {
        if (!isAdjustOrFilter) return '';
        if (id === 'adjust') {
            return buildAdjustPreviewFilter(
                adjustExposure,
                adjustContrast,
                adjustSaturation,
                adjustVibrance,
                adjustHue,
                adjustSharpness,
            );
        }
        return buildFilterPreviewFilter(filterSelected, filterIntensity);
    }, [
        adjustExposure,
        adjustContrast,
        adjustSaturation,
        adjustVibrance,
        adjustHue,
        adjustSharpness,
        filterSelected,
        filterIntensity,
        id,
        isAdjustOrFilter,
    ]);
    const previewTransform = useMemo(() => {
        if (!isAdjustOrFilter || id !== 'adjust') return '';
        const rotate = ((adjustRotate % 360) + 360) % 360;
        const scaleX = adjustFlipH ? -1 : 1;
        const scaleY = adjustFlipV ? -1 : 1;
        return `rotate(${rotate}deg) scale(${scaleX}, ${scaleY})`;
    }, [adjustFlipH, adjustFlipV, adjustRotate, id, isAdjustOrFilter]);
    const previewCropRatio = useMemo(() => {
        if (!isAdjustOrFilter || id !== 'adjust') return null;
        return parseCropRatio(adjustCropRatio);
    }, [adjustCropRatio, id, isAdjustOrFilter]);
    const previewFrame = useMemo(() => {
        const containerWidth = previewContainerSize.width;
        const containerHeight = previewContainerSize.height;
        if (!containerWidth || !containerHeight) {
            return { width: 0, height: 0, left: 0, top: 0 };
        }
        if (!previewCropRatio) {
            return { width: containerWidth, height: containerHeight, left: 0, top: 0 };
        }
        const containerRatio = containerWidth / containerHeight;
        const targetRatio = previewCropRatio.w / previewCropRatio.h;
        if (!Number.isFinite(containerRatio) || !Number.isFinite(targetRatio)) {
            return { width: containerWidth, height: containerHeight, left: 0, top: 0 };
        }
        let frameWidth = containerWidth;
        let frameHeight = containerHeight;
        if (containerRatio > targetRatio) {
            frameHeight = containerHeight;
            frameWidth = frameHeight * targetRatio;
        } else {
            frameWidth = containerWidth;
            frameHeight = frameWidth / targetRatio;
        }
        const left = (containerWidth - frameWidth) / 2;
        const top = (containerHeight - frameHeight) / 2;
        return {
            width: Math.max(0, frameWidth),
            height: Math.max(0, frameHeight),
            left: Math.max(0, left),
            top: Math.max(0, top),
        };
    }, [previewContainerSize, previewCropRatio]);
    const showCropFrame = Boolean(previewCropRatio);
    const isCropInteractive = showCropFrame && id === 'adjust' && Boolean(previewSrc);
    const cropImageMetrics = useMemo(() => {
        if (!showCropFrame) return null;
        if (!previewImageSize.width || !previewImageSize.height) return null;
        if (!previewFrame.width || !previewFrame.height) return null;
        const rotation = ((adjustRotate % 360) + 360) % 360;
        const rotated = rotation === 90 || rotation === 270;
        const rotatedWidth = rotated ? previewImageSize.height : previewImageSize.width;
        const rotatedHeight = rotated ? previewImageSize.width : previewImageSize.height;
        const scale = Math.max(
            previewFrame.width / rotatedWidth,
            previewFrame.height / rotatedHeight,
        );
        const drawnWidth = previewImageSize.width * scale;
        const drawnHeight = previewImageSize.height * scale;
        const rotatedDrawnWidth = rotated ? drawnHeight : drawnWidth;
        const rotatedDrawnHeight = rotated ? drawnWidth : drawnHeight;
        const maxOffsetX = Math.max(0, (rotatedDrawnWidth - previewFrame.width) / 2);
        const maxOffsetY = Math.max(0, (rotatedDrawnHeight - previewFrame.height) / 2);
        return {
            drawnWidth,
            drawnHeight,
            rotatedDrawnWidth,
            rotatedDrawnHeight,
            maxOffsetX,
            maxOffsetY,
        };
    }, [showCropFrame, previewImageSize, previewFrame, adjustRotate]);
    const cropFocus = useMemo(() => {
        if (!cropImageMetrics) {
            return { x: 0.5, y: 0.5 };
        }
        const x = clampNumber(0.5 - cropOffset.x / cropImageMetrics.rotatedDrawnWidth, 0, 1);
        const y = clampNumber(0.5 - cropOffset.y / cropImageMetrics.rotatedDrawnHeight, 0, 1);
        return { x, y };
    }, [cropImageMetrics, cropOffset]);
    const watermarkFontFace = useMemo(() => {
        if (!useSystemFonts) return null;
        const text = watermarkFont || '';
        const isPath = /[\\/]/.test(text) || /\.(ttf|otf|ttc)$/i.test(text);
        if (!isPath) return null;
        const safe = text.replace(/[^a-zA-Z0-9]/g, '_');
        const name = `wm_font_${safe.slice(-24)}`;
        return {
            name,
            css: `@font-face { font-family: '${name}'; src: url('${toFileUrl(text)}'); }`,
        };
    }, [useSystemFonts, watermarkFont]);
    const watermarkFontFamily = useMemo(() => {
        if (watermarkFontFace?.name) return watermarkFontFace.name;
        const fallbackMap: Record<string, string> = {
            'Sans Serif': 'Arial, "Helvetica Neue", sans-serif',
            'Serif': '"Times New Roman", serif',
            'Mono': '"Courier New", monospace',
            'Handwriting': '"Comic Sans MS", cursive',
        };
        return fallbackMap[watermarkFont] || watermarkFont || 'Arial, sans-serif';
    }, [watermarkFont, watermarkFontFace]);
    const previewBaseMetrics = useMemo(() => {
        if (!previewContainerSize.width || !previewContainerSize.height) return null;
        if (!previewImageSize.width || !previewImageSize.height) return null;
        const scale = Math.min(
            previewContainerSize.width / previewImageSize.width,
            previewContainerSize.height / previewImageSize.height,
        );
        const width = previewImageSize.width * scale;
        const height = previewImageSize.height * scale;
        const left = (previewContainerSize.width - width) / 2;
        const top = (previewContainerSize.height - height) / 2;
        return {
            width,
            height,
            left,
            top,
            scale,
        };
    }, [previewContainerSize, previewImageSize]);
    const watermarkPreviewConfig = useMemo(() => {
        if (!isWatermark || !previewBaseMetrics) return null;
        const textValue = watermarkText.trim() || '© ImageFlow';
        const opacity = clampNumber(watermarkOpacity / 100, 0, 1);
        const scale = clampNumber(watermarkSize / 100, 0.02, 1);
        const fontSize = Math.max(8, Math.round(36 * (watermarkSize / 40)));
        const previewFontSize = Math.max(8, Math.round(fontSize * previewBaseMetrics.scale));
        const offsetX = Math.max(0, Number(watermarkMargin.x) || 0) * previewBaseMetrics.scale;
        const offsetY = Math.max(0, Number(watermarkMargin.y) || 0) * previewBaseMetrics.scale;
        const blendMap: Record<string, string> = {
            '正常': 'normal',
            '正片叠底 (Multiply)': 'multiply',
            '滤色 (Screen)': 'screen',
            '叠加 (Overlay)': 'overlay',
            '柔光 (Soft Light)': 'soft-light',
        };
        return {
            textValue,
            opacity,
            scale,
            fontSize,
            previewFontSize,
            offsetX,
            offsetY,
            blendMode: blendMap[watermarkBlendMode] || 'normal',
        };
    }, [
        isWatermark,
        previewBaseMetrics,
        watermarkText,
        watermarkOpacity,
        watermarkSize,
        watermarkMargin,
        watermarkBlendMode,
    ]);
    const watermarkTextMetrics = useMemo(() => {
        if (!watermarkPreviewConfig) return null;
        if (typeof document === 'undefined') return null;
        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        if (!ctx) return null;
        ctx.font = `${watermarkPreviewConfig.fontSize}px ${watermarkFontFamily}`;
        const metrics = ctx.measureText(watermarkPreviewConfig.textValue);
        const width = Math.max(1, Math.ceil(metrics.width));
        const height = Math.max(1, Math.ceil(watermarkPreviewConfig.fontSize * 1.2));
        return { width, height };
    }, [watermarkPreviewConfig, watermarkFontFamily]);
    const watermarkImagePreviewSize = useMemo(() => {
        if (!previewBaseMetrics || !watermarkPreviewConfig) return null;
        if (!watermarkImageSize.width || !watermarkImageSize.height) return null;
        const width = previewBaseMetrics.width * watermarkPreviewConfig.scale;
        const height = width * (watermarkImageSize.height / watermarkImageSize.width);
        return { width, height };
    }, [previewBaseMetrics, watermarkPreviewConfig, watermarkImageSize]);
    const watermarkTileMetrics = useMemo(() => {
        if (!previewBaseMetrics || !watermarkPreviewConfig) return null;
        if (watermarkType === '图片') {
            return watermarkImagePreviewSize;
        }
        if (!watermarkTextMetrics) return null;
        return {
            width: watermarkTextMetrics.width * previewBaseMetrics.scale,
            height: watermarkTextMetrics.height * previewBaseMetrics.scale,
        };
    }, [previewBaseMetrics, watermarkPreviewConfig, watermarkType, watermarkImagePreviewSize, watermarkTextMetrics]);
    const watermarkPositionStyle = useMemo(() => {
        if (!watermarkPreviewConfig) return {};
        const offsetX = watermarkPreviewConfig.offsetX;
        const offsetY = watermarkPreviewConfig.offsetY;
        const style: React.CSSProperties = {};
        switch (watermarkPosition) {
            case 'tl':
                style.left = offsetX;
                style.top = offsetY;
                break;
            case 'tc':
                style.left = '50%';
                style.top = offsetY;
                style.transform = 'translateX(-50%)';
                break;
            case 'tr':
                style.right = offsetX;
                style.top = offsetY;
                break;
            case 'ml':
                style.left = offsetX;
                style.top = '50%';
                style.transform = 'translateY(-50%)';
                break;
            case 'mc':
                style.left = '50%';
                style.top = '50%';
                style.transform = 'translate(-50%, -50%)';
                break;
            case 'mr':
                style.right = offsetX;
                style.top = '50%';
                style.transform = 'translateY(-50%)';
                break;
            case 'bl':
                style.left = offsetX;
                style.bottom = offsetY;
                break;
            case 'bc':
                style.left = '50%';
                style.bottom = offsetY;
                style.transform = 'translateX(-50%)';
                break;
            case 'br':
            default:
                style.right = offsetX;
                style.bottom = offsetY;
                break;
        }
        const baseTransform = style.transform ? `${style.transform} ` : '';
        style.transform = `${baseTransform}rotate(${watermarkRotate}deg)`;
        style.transformOrigin = 'center';
        return style;
    }, [watermarkPreviewConfig, watermarkPosition, watermarkRotate]);
    const watermarkTileGrid = useMemo(() => {
        if (!watermarkTiled || !previewBaseMetrics || !watermarkPreviewConfig || !watermarkTileMetrics) return null;
        const gapX = watermarkPreviewConfig.offsetX;
        const gapY = watermarkPreviewConfig.offsetY;
        const stepX = Math.max(1, watermarkTileMetrics.width + gapX);
        const stepY = Math.max(1, watermarkTileMetrics.height + gapY);
        const coverWidth = previewBaseMetrics.width * 2;
        const coverHeight = previewBaseMetrics.height * 2;
        let cols = Math.ceil(coverWidth / stepX);
        let rows = Math.ceil(coverHeight / stepY);
        const total = cols * rows;
        if (total > 400) {
            const ratio = Math.sqrt(400 / total);
            cols = Math.max(1, Math.floor(cols * ratio));
            rows = Math.max(1, Math.floor(rows * ratio));
        }
        return {
            cols,
            rows,
            coverWidth,
            coverHeight,
        };
    }, [watermarkTiled, previewBaseMetrics, watermarkPreviewConfig, watermarkTileMetrics]);
    const watermarkTilePositions = useMemo(() => {
        if (!watermarkTileGrid || !watermarkTileMetrics || !watermarkPreviewConfig) return null;
        const gapX = watermarkPreviewConfig.offsetX;
        const gapY = watermarkPreviewConfig.offsetY;
        const stepX = Math.max(1, watermarkTileMetrics.width + gapX);
        const stepY = Math.max(1, watermarkTileMetrics.height + gapY);
        const startX = -watermarkTileMetrics.width;
        const startY = -watermarkTileMetrics.height;
        const positions: Array<{ x: number; y: number }> = [];
        for (let row = 0; row < watermarkTileGrid.rows; row += 1) {
            const y = startY + row * stepY;
            for (let col = 0; col < watermarkTileGrid.cols; col += 1) {
                const x = startX + col * stepX;
                positions.push({ x, y });
            }
        }
        return positions;
    }, [watermarkTileGrid, watermarkTileMetrics, watermarkPreviewConfig]);
    const previewGrainOpacity = isAdjustOrFilter && id === 'filter'
        ? clampNumber(filterGrain / 100 * 0.35, 0, 0.35)
        : 0;
    const previewVignetteOpacity = isAdjustOrFilter && id === 'filter'
        ? clampNumber(filterVignette / 100 * 0.6, 0, 0.6)
        : 0;
    const isCompareActive = isComparing && Boolean(previewSrc);
    const effectivePreviewFilter = isCompareActive ? '' : previewFilter;
    const effectivePreviewTransform = isCompareActive ? '' : previewTransform;
    const effectiveGrainOpacity = isCompareActive ? 0 : previewGrainOpacity;
    const effectiveVignetteOpacity = isCompareActive ? 0 : previewVignetteOpacity;

    useEffect(() => {
        if (!showCropFrame) {
            setCropOffset({ x: 0, y: 0 });
            return;
        }
        setCropOffset({ x: 0, y: 0 });
    }, [adjustCropRatio, previewPath, showCropFrame]);

    useEffect(() => {
        if (!cropImageMetrics) return;
        setCropOffset((prev) => {
            const nextX = clampNumber(prev.x, -cropImageMetrics.maxOffsetX, cropImageMetrics.maxOffsetX);
            const nextY = clampNumber(prev.y, -cropImageMetrics.maxOffsetY, cropImageMetrics.maxOffsetY);
            if (nextX === prev.x && nextY === prev.y) return prev;
            return { x: nextX, y: nextY };
        });
    }, [cropImageMetrics]);

    useEffect(() => {
        const el = previewContainerRef.current;
        if (!el || typeof ResizeObserver === 'undefined') return;
        const observer = new ResizeObserver((entries) => {
            if (!entries.length) return;
            const { width, height } = entries[0].contentRect;
            setPreviewContainerSize((prev) => (
                prev.width === width && prev.height === height ? prev : { width, height }
            ));
        });
        observer.observe(el);
        return () => observer.disconnect();
    }, []);

    const handleCropPointerDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
        if (!isCropInteractive || !cropImageMetrics) return;
        e.preventDefault();
        e.stopPropagation();
        setIsCropDragging(true);
        (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
        cropDragRef.current = {
            startX: e.clientX,
            startY: e.clientY,
            offsetX: cropOffset.x,
            offsetY: cropOffset.y,
        };
    }, [cropOffset, cropImageMetrics, isCropInteractive]);

    const handleCropPointerMove = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
        if (!cropDragRef.current || !cropImageMetrics) return;
        const dx = e.clientX - cropDragRef.current.startX;
        const dy = e.clientY - cropDragRef.current.startY;
        const nextX = clampNumber(
            cropDragRef.current.offsetX + dx,
            -cropImageMetrics.maxOffsetX,
            cropImageMetrics.maxOffsetX,
        );
        const nextY = clampNumber(
            cropDragRef.current.offsetY + dy,
            -cropImageMetrics.maxOffsetY,
            cropImageMetrics.maxOffsetY,
        );
        setCropOffset((prev) => (
            prev.x === nextX && prev.y === nextY ? prev : { x: nextX, y: nextY }
        ));
    }, [cropImageMetrics]);

    const handleCropPointerUp = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
        if (!cropDragRef.current) return;
        try {
            (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
        } catch {
            // ignore
        }
        cropDragRef.current = null;
        setIsCropDragging(false);
    }, []);

    useEffect(() => {
        loadOutputSettings();
    }, [id]);

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
                    />
                );
            case 'watermark':
                return (
                    <WatermarkSettings
                        type={watermarkType}
                        setType={setWatermarkType}
                        text={watermarkText}
                        setText={setWatermarkText}
                        imagePath={watermarkImagePath}
                        setImagePath={setWatermarkImagePath}
                        position={watermarkPosition}
                        setPosition={setWatermarkPosition}
                        opacity={watermarkOpacity}
                        setOpacity={setWatermarkOpacity}
                        rotate={watermarkRotate}
                        setRotate={setWatermarkRotate}
                        size={watermarkSize}
                        setSize={setWatermarkSize}
                        tiled={watermarkTiled}
                        setTiled={setWatermarkTiled}
                        blendMode={watermarkBlendMode}
                        setBlendMode={setWatermarkBlendMode}
                        shadow={watermarkShadow}
                        setShadow={setWatermarkShadow}
                        margin={watermarkMargin}
                        setMargin={setWatermarkMargin}
                        font={watermarkFont}
                        setFont={setWatermarkFont}
                        color={watermarkColor}
                        setColor={setWatermarkColor}
                        useSystemFonts={useSystemFonts}
                        setUseSystemFonts={setUseSystemFonts}
                        isSystemFontsLoading={isSystemFontsLoading}
                        fontOptions={fontOptions}
                        systemFontsCount={systemFonts.length}
                    />
                );
            case 'adjust':
                return (
                    <AdjustSettings
                        exposure={adjustExposure}
                        setExposure={setAdjustExposure}
                        contrast={adjustContrast}
                        setContrast={setAdjustContrast}
                        saturation={adjustSaturation}
                        setSaturation={setAdjustSaturation}
                        sharpness={adjustSharpness}
                        setSharpness={setAdjustSharpness}
                        vibrance={adjustVibrance}
                        setVibrance={setAdjustVibrance}
                        hue={adjustHue}
                        setHue={setAdjustHue}
                        showCrop={false}
                    />
                );
            case 'filter':
                return (
                    <FilterSettings
                        setIntensity={setFilterIntensity}
                        setGrain={setFilterGrain}
                        setVignette={setFilterVignette}
                        selected={filterSelected}
                        setSelected={setFilterSelected}
                        previewSrc={previewSrc}
                        getPreviewFilter={(index) => buildFilterPreviewFilter(index, filterIntensity)}
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
            case 'info': {
                const onExportJSON = () => {
                    if (!infoPreview?.success) return;
                    try {
                        const content = JSON.stringify(infoPreview, null, 2);
                        const blob = new Blob([content], { type: 'application/json;charset=utf-8' });
                        const url = URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        const name = (infoFilePath || 'image').replace(/\\/g, '/').split('/').pop() || 'image';
                        a.href = url;
                        a.download = `${name}.json`;
                        a.click();
                        URL.revokeObjectURL(url);
                    } catch (e) {
                        console.error(e);
                    }
                };

                const onClearPrivacy = async () => {
                    if (!infoFilePath) return;
                    try {
                        const normalized = infoFilePath.replace(/\\/g, '/');
                        const idx = normalized.lastIndexOf('/');
                        const dir = idx >= 0 ? normalized.slice(0, idx) : '';
                        const name = idx >= 0 ? normalized.slice(idx + 1) : normalized;
                        const baseName = stripExtension(name);
                        const ext = extname(name);
                        const outputName = buildOutputName(`${baseName}_clean`, {
                            template: outputSettings.output_template,
                            prefix: outputSettings.output_prefix,
                            seq: 1,
                            op: 'privacy',
                            date: new Date(),
                        });
                        const rawPath = (dir ? `${dir}/` : '') + (ext ? `${outputName}.${ext}` : outputName);
                        const appAny = getAppBindings();

                        if (!appAny?.StripMetadata) {
                            setLastMessage('后端未接入隐私清理接口');
                            return;
                        }
                        let resolvedPath = normalizePath(rawPath);
                        if (appAny?.ResolveOutputPath && outputSettings.conflict_strategy === 'rename') {
                            try {
                                const res = await appAny.ResolveOutputPath({
                                    base_path: normalizePath(rawPath),
                                    strategy: 'rename',
                                    reserved: [],
                                });
                                if (res?.success && res.output_path) {
                                    resolvedPath = normalizePath(res.output_path);
                                }
                            } catch (err) {
                                console.error(err);
                            }
                        }
                        setLastMessage('');
                        const res = await appAny.StripMetadata({ input_path: infoFilePath, output_path: resolvedPath, overwrite: false });
                        if (res?.success) {
                            setLastMessage(`隐私清理完成：${res.output_path || resolvedPath}`);
                            const refreshed = await appAny.GetInfo?.({ input_path: res.output_path || resolvedPath });
                            setInfoFilePath(res.output_path || resolvedPath);
                            setInfoPreview(refreshed || null);
                        } else {
                            setLastMessage(res?.error || '隐私清理失败');
                        }
                    } catch (e: any) {
                        console.error(e);
                        setLastMessage(typeof e?.message === 'string' ? e.message : '隐私清理失败');
                    }
                };

                const onEditMetadata = async (key: string, value: any) => {
                    if (!infoFilePath) return;
                    const appAny = getAppBindings();
                    if (!appAny?.EditMetadata) {
                        setLastMessage('后端未接入元数据编辑接口');
                        return;
                    }
                    setLastMessage('');
                    try {
                        const res = await appAny.EditMetadata({
                            input_path: infoFilePath,
                            output_path: infoFilePath,
                            exif_data: { [key]: value },
                            overwrite: true,
                        });
                        if (res?.success) {
                            setLastMessage('元数据已更新');
                            const refreshed = await appAny.GetInfo?.({ input_path: infoFilePath });
                            setInfoPreview(refreshed || null);
                        } else {
                            setLastMessage(res?.error || '元数据编辑失败');
                        }
                    } catch (e: any) {
                        console.error(e);
                        setLastMessage(typeof e?.message === 'string' ? e.message : '元数据编辑失败');
                    }
                };

                return (
                    <InfoSettings
                        filePath={infoFilePath}
                        info={infoPreview}
                        onExportJSON={onExportJSON}
                        onClearPrivacy={onClearPrivacy}
                        onEditMetadata={onEditMetadata}
                    />
                );
            }
            default: return <div className="text-gray-400 text-sm text-center py-10">暂无特殊设置</div>;
        }
    };

    const handleFilesSelected = (_files: File[]) => {};

    const inputCount = dropResult?.files?.length ?? 0;
    const effectiveOutputDir = useMemo(() => {
        if (outputDir) return outputDir;
        if (dropResult?.files?.[0]?.source_root) return dropResult.files[0].source_root;
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

    const dedupeDroppedFiles = (files: DroppedFile[]) => {
        const seen = new Set<string>();
        const next: DroppedFile[] = [];
        files.forEach((file) => {
            const normalizedInput = normalizePath(file.input_path || '');
            if (!normalizedInput) return;
            const key = normalizePath(normalizedInput).toLowerCase();
            if (seen.has(key)) return;
            seen.add(key);
            next.push({
                ...file,
                input_path: normalizedInput,
                source_root: normalizePath(file.source_root || ''),
                relative_path: (file.relative_path || '').replace(/\\/g, '/'),
            });
        });
        return next;
    };
    const joinPath = (base: string, rel: string) => `${normalizePath(base)}/${rel.replace(/^\/+/, '')}`;
    const pathLookupKey = (path: string) => normalizePath(path).toLowerCase();
    const basename = (p: string) => p.replace(/\\/g, '/').split('/').pop() || p;
    const extname = (p: string) => {
        const normalized = p.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('.');
        if (idx === -1) return '';
        return normalized.slice(idx + 1).toLowerCase();
    };
    const sanitizeFileName = (name: string) => (name || '').trim().replace(/[\\/:*?"<>|]+/g, '_');
    const stripExtension = (name: string) => {
        const idx = name.lastIndexOf('.');
        if (idx <= 0) return name;
        return name.slice(0, idx);
    };
    const sanitizeOutputName = (name: string) => {
        const cleaned = sanitizeFileName(name).replace(/[. ]+$/g, '').trim();
        if (!cleaned) return '';
        const upper = cleaned.toUpperCase();
        const reserved = new Set([
            'CON', 'PRN', 'AUX', 'NUL',
            'COM1', 'COM2', 'COM3', 'COM4', 'COM5', 'COM6', 'COM7', 'COM8', 'COM9',
            'LPT1', 'LPT2', 'LPT3', 'LPT4', 'LPT5', 'LPT6', 'LPT7', 'LPT8', 'LPT9',
        ]);
        if (reserved.has(upper)) {
            return `_${cleaned}`;
        }
        return cleaned;
    };
    const formatDatePart = (date: Date, pattern: string) => {
        const yyyy = `${date.getFullYear()}`;
        const MM = `${date.getMonth() + 1}`.padStart(2, '0');
        const dd = `${date.getDate()}`.padStart(2, '0');
        const HH = `${date.getHours()}`.padStart(2, '0');
        const mm = `${date.getMinutes()}`.padStart(2, '0');
        const ss = `${date.getSeconds()}`.padStart(2, '0');
        return pattern
            .replace(/YYYY/g, yyyy)
            .replace(/MM/g, MM)
            .replace(/DD/g, dd)
            .replace(/HH/g, HH)
            .replace(/mm/g, mm)
            .replace(/ss/g, ss);
    };
    const applyOutputTemplate = (
        template: string,
        vars: {
            basename: string;
            prefix: string;
            seq: number;
            op: string;
            date: Date;
        },
    ) => template.replace(/\{([a-z_]+)(?::([^}]+))?\}/gi, (_match, keyRaw, fmtRaw) => {
        const key = String(keyRaw).toLowerCase();
        const fmt = typeof fmtRaw === 'string' ? fmtRaw : '';
        if (key === 'basename') return vars.basename;
        if (key === 'prefix') return vars.prefix;
        if (key === 'op') return vars.op;
        if (key === 'date') {
            const pattern = fmt || 'YYYYMMDD';
            return formatDatePart(vars.date, pattern);
        }
        if (key === 'time') {
            const pattern = fmt || 'HHmmss';
            return formatDatePart(vars.date, pattern);
        }
        if (key === 'seq') {
            const width = /^\d+$/.test(fmt) ? Number(fmt) : 0;
            const text = String(vars.seq);
            return width > 0 ? text.padStart(width, '0') : text;
        }
        return '';
    });
    const buildOutputName = (
        baseName: string,
        options: {
            template: string;
            prefix: string;
            seq: number;
            op: string;
            date: Date;
        },
    ) => {
        const template = (options.template || defaultOutputSettings.output_template).trim() || defaultOutputSettings.output_template;
        const prefixClean = sanitizeFileName(options.prefix || '');
        const prefixValue = prefixClean ? `${prefixClean}_` : '';
        const templateHasPrefix = /\{prefix(?:[:}])/i.test(template);
        const rendered = applyOutputTemplate(template, {
            basename: baseName,
            prefix: prefixValue,
            seq: options.seq,
            op: options.op,
            date: options.date,
        }).trim();
        let name = rendered || baseName;
        if (!templateHasPrefix && prefixValue) {
            name = `${prefixValue}${name}`;
        }
        name = sanitizeOutputName(name);
        if (!name) {
            name = sanitizeOutputName(baseName) || 'output';
        }
        if (name.length > 200) {
            name = name.slice(0, 200);
        }
        return name;
    };
    const getRelPath = (file: DroppedFile, preserveStructure: boolean) => {
        if (preserveStructure && file.is_from_dir_drop) {
            return file.relative_path || basename(file.input_path);
        }
        return basename(file.input_path);
    };
    const getRelDir = (relPath: string) => {
        const normalized = relPath.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('/');
        if (idx === -1) return '';
        return normalized.slice(0, idx);
    };
    const buildOutputRelPath = (
        file: DroppedFile,
        options: {
            ext?: string;
            suffix?: string;
            seq: number;
            op: string;
            template: string;
            prefix: string;
            preserveStructure: boolean;
            date: Date;
        },
    ) => {
        const rel = getRelPath(file, options.preserveStructure).replace(/\\/g, '/');
        const relDir = getRelDir(rel);
        const fileName = basename(rel);
        let baseName = stripExtension(fileName);
        const suffix = sanitizeFileName(options.suffix || '');
        if (suffix.startsWith('_ico')) {
            baseName = baseName.replace(/_ico\d+(?:-\d+)*$/i, '');
        }
        const ext = (options.ext || extname(fileName)).toLowerCase();
        let name = buildOutputName(baseName, {
            template: options.template,
            prefix: options.prefix,
            seq: options.seq,
            op: options.op,
            date: options.date,
        });
        if (suffix && !name.endsWith(suffix)) {
            const maxBaseLen = Math.max(1, 200 - suffix.length);
            if (name.length > maxBaseLen) {
                name = name.slice(0, maxBaseLen);
            }
            name = sanitizeOutputName(`${name}${suffix}`) || `${name}${suffix}`;
        }
        const fullName = ext ? `${name}.${ext}` : name;
        return relDir ? `${relDir}/${fullName}` : fullName;
    };
    const sanitizePdfInputName = (name: string) => sanitizeFileName(stripExtension(name || ''));
    const normalizePdfFileName = (name: string) => {
        const cleaned = sanitizeFileName(stripExtension(name || ''));
        return cleaned || 'document';
    };
    const buildSuggestedPdfName = (list: DroppedFile[], hasDirectory?: boolean) => {
        if (!list.length) return 'document';
        const baseFromPath = (p: string) => stripExtension(basename(p));
        let name = '';
        if (hasDirectory && list[0]?.source_root) {
            name = baseFromPath(list[0].source_root);
        }
        const baseNames = list.map((f) => baseFromPath(f.input_path)).filter(Boolean);
        if (!name && baseNames.length > 0) {
            if (baseNames.length === 1) {
                name = baseNames[0];
            } else {
                let prefix = baseNames[0];
                for (const item of baseNames.slice(1)) {
                    while (prefix && !item.startsWith(prefix)) {
                        prefix = prefix.slice(0, -1);
                    }
                    if (!prefix) break;
                }
                const trimmed = prefix.replace(/[_\-\s.]+$/g, '').trim();
                name = trimmed.length >= 3 ? trimmed : baseNames[0];
                if (!hasDirectory) {
                    name = `${name}_merged`;
                }
            }
        }
        return normalizePdfFileName(name);
    };
    const buildCurrentPresetPayload = (): Record<string, any> => {
        const shared = {
            output_settings: outputSettings,
            output_dir: outputDir,
        };
        if (id === 'converter') {
            return {
                ...shared,
                convFormat,
                convQuality,
                convCompressLevel,
                convIcoSizes,
                convResizeMode,
                convScalePercent,
                convFixedWidth,
                convFixedHeight,
                convLongEdge,
                convKeepMetadata,
                convMaintainAR,
                convOverwriteSource,
            };
        }
        if (id === 'compressor') {
            return {
                ...shared,
                compMode,
                compTargetSize,
                compTargetSizeKB,
                compEngine,
                compOverwriteSource,
            };
        }
        if (id === 'pdf') {
            return {
                ...shared,
                pdfSize,
                pdfLayout,
                pdfFit,
                pdfMarginMm,
                pdfCompression,
                pdfFileName,
                pdfTitle,
                pdfAuthor,
            };
        }
        if (id === 'gif') {
            return {
                ...shared,
                gifMode,
                gifExportFormat,
                gifConvertFormat,
                gifSpeedPercent,
                gifCompressQuality,
                gifBuildFps,
                gifResizeWidth,
                gifResizeHeight,
                gifResizeMaintainAR,
            };
        }
        if (id === 'watermark') {
            return {
                ...shared,
                watermarkType,
                watermarkText,
                watermarkImagePath,
                watermarkPosition,
                watermarkOpacity,
                watermarkRotate,
                watermarkSize,
                watermarkTiled,
                watermarkBlendMode,
                watermarkShadow,
                watermarkMargin,
                watermarkFont,
                watermarkColor,
            };
        }
        if (id === 'adjust') {
            return {
                ...shared,
                adjustExposure,
                adjustContrast,
                adjustSaturation,
                adjustSharpness,
                adjustVibrance,
                adjustHue,
                adjustRotate,
                adjustFlipH,
                adjustFlipV,
                adjustCropRatio,
            };
        }
        if (id === 'filter') {
            return {
                ...shared,
                filterIntensity,
                filterGrain,
                filterVignette,
                filterSelected,
            };
        }
        return shared;
    };
    const applyPresetPayload = (payload: Record<string, any>) => {
        if (!payload || typeof payload !== 'object') return;
        if (payload.output_settings && typeof payload.output_settings === 'object') {
            setOutputSettings(normalizeOutputSettings(payload.output_settings));
        }
        if (typeof payload.output_dir === 'string') {
            setOutputDir(payload.output_dir);
        }
        if (id === 'converter') {
            if (typeof payload.convFormat === 'string') setConvFormat(payload.convFormat);
            if (typeof payload.convQuality === 'number') setConvQuality(payload.convQuality);
            if (typeof payload.convCompressLevel === 'number') setConvCompressLevel(payload.convCompressLevel);
            if (Array.isArray(payload.convIcoSizes)) setConvIcoSizes(payload.convIcoSizes.filter((n: unknown) => typeof n === 'number'));
            if (typeof payload.convResizeMode === 'string') setConvResizeMode(payload.convResizeMode);
            if (typeof payload.convScalePercent === 'number') setConvScalePercent(payload.convScalePercent);
            if (typeof payload.convFixedWidth === 'number') setConvFixedWidth(payload.convFixedWidth);
            if (typeof payload.convFixedHeight === 'number') setConvFixedHeight(payload.convFixedHeight);
            if (typeof payload.convLongEdge === 'number') setConvLongEdge(payload.convLongEdge);
            if (typeof payload.convKeepMetadata === 'boolean') setConvKeepMetadata(payload.convKeepMetadata);
            if (typeof payload.convMaintainAR === 'boolean') setConvMaintainAR(payload.convMaintainAR);
            if (typeof payload.convOverwriteSource === 'boolean') setConvOverwriteSource(payload.convOverwriteSource);
        } else if (id === 'compressor') {
            if (typeof payload.compMode === 'string') setCompMode(payload.compMode);
            if (typeof payload.compTargetSize === 'boolean') setCompTargetSize(payload.compTargetSize);
            if (typeof payload.compTargetSizeKB === 'number') setCompTargetSizeKB(payload.compTargetSizeKB);
            if (typeof payload.compEngine === 'string') setCompEngine(payload.compEngine);
            if (typeof payload.compOverwriteSource === 'boolean') setCompOverwriteSource(payload.compOverwriteSource);
        } else if (id === 'pdf') {
            if (typeof payload.pdfSize === 'string') setPdfSize(payload.pdfSize);
            if (typeof payload.pdfLayout === 'string') setPdfLayout(payload.pdfLayout);
            if (typeof payload.pdfFit === 'string') setPdfFit(payload.pdfFit);
            if (typeof payload.pdfMarginMm === 'number') setPdfMarginMm(payload.pdfMarginMm);
            if (typeof payload.pdfCompression === 'string') setPdfCompression(payload.pdfCompression);
            if (typeof payload.pdfFileName === 'string') setPdfFileName(payload.pdfFileName);
            if (typeof payload.pdfTitle === 'string') setPdfTitle(payload.pdfTitle);
            if (typeof payload.pdfAuthor === 'string') setPdfAuthor(payload.pdfAuthor);
        } else if (id === 'gif') {
            if (typeof payload.gifMode === 'string') setGifMode(payload.gifMode);
            if (typeof payload.gifExportFormat === 'string') setGifExportFormat(payload.gifExportFormat);
            if (typeof payload.gifConvertFormat === 'string') setGifConvertFormat(payload.gifConvertFormat);
            if (typeof payload.gifSpeedPercent === 'number') {
                setGifSpeedPercent(normalizeGifSpeedPercent(payload.gifSpeedPercent));
            }
            if (typeof payload.gifCompressQuality === 'number') setGifCompressQuality(payload.gifCompressQuality);
            if (typeof payload.gifBuildFps === 'number') setGifBuildFps(payload.gifBuildFps);
            if (typeof payload.gifResizeWidth === 'number') handleGifResizeWidthChange(payload.gifResizeWidth);
            if (typeof payload.gifResizeHeight === 'number') handleGifResizeHeightChange(payload.gifResizeHeight);
            if (typeof payload.gifResizeMaintainAR === 'boolean') setGifResizeMaintainAR(payload.gifResizeMaintainAR);
        } else if (id === 'watermark') {
            if (typeof payload.watermarkType === 'string') setWatermarkType(payload.watermarkType);
            if (typeof payload.watermarkText === 'string') setWatermarkText(payload.watermarkText);
            if (typeof payload.watermarkImagePath === 'string') setWatermarkImagePath(payload.watermarkImagePath);
            if (typeof payload.watermarkPosition === 'string') setWatermarkPosition(payload.watermarkPosition);
            if (typeof payload.watermarkOpacity === 'number') setWatermarkOpacity(payload.watermarkOpacity);
            if (typeof payload.watermarkRotate === 'number') setWatermarkRotate(payload.watermarkRotate);
            if (typeof payload.watermarkSize === 'number') setWatermarkSize(payload.watermarkSize);
            if (typeof payload.watermarkTiled === 'boolean') setWatermarkTiled(payload.watermarkTiled);
            if (typeof payload.watermarkBlendMode === 'string') setWatermarkBlendMode(payload.watermarkBlendMode);
            if (typeof payload.watermarkShadow === 'boolean') setWatermarkShadow(payload.watermarkShadow);
            if (payload.watermarkMargin && typeof payload.watermarkMargin === 'object') {
                setWatermarkMargin({
                    x: Number(payload.watermarkMargin.x) || 0,
                    y: Number(payload.watermarkMargin.y) || 0,
                });
            }
            if (typeof payload.watermarkFont === 'string') setWatermarkFont(payload.watermarkFont);
            if (typeof payload.watermarkColor === 'string') setWatermarkColor(payload.watermarkColor);
        } else if (id === 'adjust') {
            if (typeof payload.adjustExposure === 'number') setAdjustExposure(payload.adjustExposure);
            if (typeof payload.adjustContrast === 'number') setAdjustContrast(payload.adjustContrast);
            if (typeof payload.adjustSaturation === 'number') setAdjustSaturation(payload.adjustSaturation);
            if (typeof payload.adjustSharpness === 'number') setAdjustSharpness(payload.adjustSharpness);
            if (typeof payload.adjustVibrance === 'number') setAdjustVibrance(payload.adjustVibrance);
            if (typeof payload.adjustHue === 'number') setAdjustHue(payload.adjustHue);
            if (typeof payload.adjustRotate === 'number') setAdjustRotate(payload.adjustRotate);
            if (typeof payload.adjustFlipH === 'boolean') setAdjustFlipH(payload.adjustFlipH);
            if (typeof payload.adjustFlipV === 'boolean') setAdjustFlipV(payload.adjustFlipV);
            if (typeof payload.adjustCropRatio === 'string') setAdjustCropRatio(payload.adjustCropRatio);
        } else if (id === 'filter') {
            if (typeof payload.filterIntensity === 'number') setFilterIntensity(payload.filterIntensity);
            if (typeof payload.filterGrain === 'number') setFilterGrain(payload.filterGrain);
            if (typeof payload.filterVignette === 'number') setFilterVignette(payload.filterVignette);
            if (typeof payload.filterSelected === 'number') setFilterSelected(payload.filterSelected);
        }
    };
    const saveCurrentPreset = () => {
        if (id === 'info') return;
        const name = presetNameDraft.trim() || `${feature.title}预设 ${new Date().toLocaleString()}`;
        const now = Date.now();
        const preset: FeaturePreset = {
            id: `${id}-${now}`,
            name,
            feature_id: id,
            created_at: now,
            updated_at: now,
            payload: buildCurrentPresetPayload(),
        };
        setFeaturePresets((prev) => {
            const next: FeaturePresetStore = {
                ...prev,
                [id]: [preset, ...(prev[id] || [])],
            };
            saveFeaturePresetStore(next);
            return next;
        });
        setSelectedPresetId(preset.id);
        setPresetNameDraft('');
        setLastMessage(`预设已保存：${name}`);
    };
    const applySelectedPreset = () => {
        const target = currentFeaturePresets.find((p) => p.id === selectedPresetId);
        if (!target) return;
        applyPresetPayload(target.payload || {});
        setLastMessage(`已应用预设：${target.name}`);
    };
    const deleteSelectedPreset = () => {
        if (!selectedPresetId) return;
        const target = currentFeaturePresets.find((p) => p.id === selectedPresetId);
        setFeaturePresets((prev) => {
            const nextList = (prev[id] || []).filter((p) => p.id !== selectedPresetId);
            const next: FeaturePresetStore = {
                ...prev,
                [id]: nextList,
            };
            saveFeaturePresetStore(next);
            return next;
        });
        setSelectedPresetId('');
        if (target) {
            setLastMessage(`预设已删除：${target.name}`);
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
    function toFileUrl(p: string) {
        if (!p) return '';
        const normalized = p.replace(/\\/g, '/');
        if (/^[a-zA-Z]:/.test(normalized)) {
            return `file:///${encodeURI(normalized)}`;
        }
        if (normalized.startsWith('/')) {
            return `file://${encodeURI(normalized)}`;
        }
        return `file:///${encodeURI(normalized)}`;
    }
    function clampNumber(value: number, min: number, max: number) {
        return Math.min(max, Math.max(min, value));
    }
    function parseCropRatio(value: string) {
        const text = (value || '').trim().toLowerCase();
        if (!text || text === '自由' || text === 'free' || text === 'original') {
            return null;
        }
        if (!text.includes(':')) return null;
        const parts = text.split(':');
        if (parts.length !== 2) return null;
        const w = Number(parts[0]);
        const h = Number(parts[1]);
        if (!Number.isFinite(w) || !Number.isFinite(h) || w <= 0 || h <= 0) {
            return null;
        }
        return { w, h };
    }
    function buildAdjustPreviewFilter(
        exposure: number,
        contrast: number,
        saturation: number,
        vibrance: number,
        hue: number,
        sharpness: number,
    ) {
        const brightness = clampNumber(1 + exposure / 100, 0, 2);
        const baseContrast = clampNumber(1 + contrast / 100, 0, 2);
        const satCombined = saturation + vibrance * 0.6;
        const saturate = clampNumber(1 + satCombined / 100, 0, 3);
        const blurPx = sharpness < 0 ? clampNumber(Math.abs(sharpness) / 35, 0, 6) : 0;
        const sharpnessBoost = sharpness > 0 ? clampNumber(1 + sharpness / 200, 1, 1.8) : 1;
        const hueDeg = clampNumber(hue, -180, 180);

        const parts = [
            `brightness(${brightness})`,
            `contrast(${(baseContrast * sharpnessBoost).toFixed(3)})`,
            `saturate(${saturate})`,
            `hue-rotate(${hueDeg}deg)`,
        ];
        if (blurPx > 0) {
            parts.push(`blur(${blurPx.toFixed(2)}px)`);
        }
        return parts.join(' ');
    }
    function buildFilterPreviewFilter(index: number, intensity: number) {
        const t = clampNumber(intensity / 100, 0, 1);
        const mix = (base: number, target: number) => base + (target - base) * t;

        switch (index) {
            case 1: // 鲜艳
                return `saturate(${mix(1, 1.8)}) contrast(${mix(1, 1.25)})`;
            case 2: // 黑白
                return `grayscale(${t})`;
            case 3: // 复古
                return `sepia(${mix(0, 0.8)}) contrast(${mix(1, 1.15)})`;
            case 4: // 冷调
                return `saturate(${mix(1, 1.2)}) hue-rotate(${mix(0, -18)}deg)`;
            case 5: // 暖阳
                return `sepia(${mix(0, 0.35)}) hue-rotate(${mix(0, 12)}deg) saturate(${mix(1, 1.2)})`;
            case 6: // 胶片
                return `sepia(${mix(0, 1)}) contrast(${mix(1, 1.2)}) brightness(${mix(1, 1.05)})`;
            case 7: // 赛博
                return `hue-rotate(${mix(0, 90)}deg) saturate(${mix(1, 1.5)}) contrast(${mix(1, 1.2)})`;
            case 8: // 清新
                return `brightness(${mix(1, 1.08)}) saturate(${mix(1, 1.25)})`;
            case 9: // 日系
                return `brightness(${mix(1, 1.12)}) contrast(${mix(1, 0.88)}) saturate(${mix(1, 1.1)})`;
            case 10: // Lomo
                return `contrast(${mix(1, 1.3)}) saturate(${mix(1, 1.35)})`;
            case 11: // HDR
                return `contrast(${mix(1, 1.45)}) saturate(${mix(1, 1.2)})`;
            case 12: // 褪色
                return `saturate(${mix(1, 0.65)}) brightness(${mix(1, 1.08)}) contrast(${mix(1, 0.9)})`;
            case 13: // 磨砂
                return `blur(${mix(0, 2)}px)`;
            case 14: // 电影
                return `contrast(${mix(1, 1.25)}) saturate(${mix(1, 0.9)})`;
            case 15: // 拍立得
                return `sepia(${mix(0, 1)}) contrast(${mix(1, 1.1)}) brightness(${mix(1, 1.08)})`;
            case 16: // 夕阳
                return `sepia(${mix(0, 0.35)}) saturate(${mix(1, 1.3)}) hue-rotate(${mix(0, -10)}deg) brightness(${mix(1, 1.05)})`;
            case 17: // 海蓝
                return `hue-rotate(${mix(0, -25)}deg) saturate(${mix(1, 1.2)}) brightness(${mix(1, 1.02)})`;
            case 18: // 森系
                return `hue-rotate(${mix(0, 18)}deg) saturate(${mix(1, 1.15)}) contrast(${mix(1, 1.05)})`;
            case 19: // 紫雾
                return `hue-rotate(${mix(0, 35)}deg) saturate(${mix(1, 1.2)})`;
            case 20: // 琥珀
                return `sepia(${mix(0, 0.5)}) saturate(${mix(1, 1.1)}) contrast(${mix(1, 1.08)})`;
            case 21: // 北欧
                return `brightness(${mix(1, 1.08)}) saturate(${mix(1, 0.9)}) contrast(${mix(1, 0.92)})`;
            case 22: // 旧照片
                return `sepia(${mix(0, 0.7)}) contrast(${mix(1, 0.9)}) brightness(${mix(1, 1.05)}) saturate(${mix(1, 0.8)})`;
            case 23: // 黑金
                return `grayscale(${mix(0, 1)}) contrast(${mix(1, 1.35)})`;
            case 24: // 高调
                return `brightness(${mix(1, 1.2)}) contrast(${mix(1, 0.9)})`;
            case 25: // 低调
                return `brightness(${mix(1, 0.75)}) contrast(${mix(1, 1.2)})`;
            case 26: // 雾霭
                return `contrast(${mix(1, 0.85)}) brightness(${mix(1, 1.05)})`;
            case 27: // 霓虹
                return `saturate(${mix(1, 1.6)}) contrast(${mix(1, 1.15)}) hue-rotate(${mix(0, 15)}deg)`;
            case 28: // 哑光
                return `contrast(${mix(1, 0.85)}) saturate(${mix(1, 0.9)}) brightness(${mix(1, 1.03)})`;
            case 29: // 冰感
                return `hue-rotate(${mix(0, -10)}deg) saturate(${mix(1, 0.85)}) brightness(${mix(1, 1.1)})`;
            case 30: // 咖啡
                return `sepia(${mix(0, 0.5)}) brightness(${mix(1, 0.98)}) contrast(${mix(1, 1.05)})`;
            case 31: // 焦糖
                return `sepia(${mix(0, 0.4)}) saturate(${mix(1, 1.2)}) contrast(${mix(1, 1.1)})`;
            case 32: // 青橙
                return `hue-rotate(${mix(0, 12)}deg) saturate(${mix(1, 1.25)}) contrast(${mix(1, 1.1)})`;
            case 33: // 银盐
                return `grayscale(${mix(0, 0.4)}) contrast(${mix(1, 1.15)}) brightness(${mix(1, 1.02)})`;
            case 34: // 清锐
                return `contrast(${mix(1, 1.25)}) saturate(${mix(1, 1.1)})`;
            case 35: // 低对比
                return `contrast(${mix(1, 0.8)}) saturate(${mix(1, 0.95)})`;
            default:
                return '';
        }
    }

    useEffect(() => {
        if (!dropResult || dropResult.files.length === 0) {
            setPdfFileName('');
            return;
        }
        const suggested = buildSuggestedPdfName(dropResult.files, dropResult.has_directory);
        setPdfFileName(suggested);
    }, [dropResult]);

    useEffect(() => {
        if (!isPreviewFeature) {
            setPreviewPath('');
            setPreviewLoadError('');
            return;
        }
        const first = dropResult?.files?.[0];
        if (first?.input_path) {
            setPreviewPath(normalizePath(first.input_path));
        } else {
            setPreviewPath('');
        }
    }, [dropResult, isPreviewFeature]);


    const loadInfoForPath = useCallback(async (p: string) => {
        const normalized = normalizePath(p);
        const requestId = ++infoRequestIdRef.current;
        setIsProcessing(true);
        resetProgress(0);
        setLastMessage('');
        setInfoFilePath(normalized);
        setInfoPreview(null);
        if (infoRequestTimerRef.current) {
            window.clearTimeout(infoRequestTimerRef.current);
            infoRequestTimerRef.current = null;
        }

        infoRequestTimerRef.current = window.setTimeout(() => {
            infoRequestTimerRef.current = null;
            void (async () => {
                const appAny = getAppBindings();
                if (!appAny?.GetInfo) {
                    if (infoRequestIdRef.current === requestId) {
                        setLastMessage('未检测到桌面宿主运行环境');
                        setProgress(100);
                        setIsProcessing(false);
                    }
                    return;
                }
                try {
                    const info = await appAny.GetInfo({ input_path: normalized });
                    if (infoRequestIdRef.current !== requestId) {
                        return;
                    }
                    setInfoPreview(info);
                    if (info?.success) {
                        const fileName = normalized.split('/').pop() || normalized;
                        setLastMessage(`信息读取完成：${fileName}`);
                    } else {
                        setLastMessage(info?.error || '信息读取失败');
                    }
                } catch (err: any) {
                    if (infoRequestIdRef.current !== requestId) {
                        return;
                    }
                    console.error(`Failed to get info ${p}:`, err);
                    const msg = typeof err?.message === 'string' ? err.message : '信息读取失败';
                    setInfoPreview({ success: false, error: msg });
                    setLastMessage(msg);
                } finally {
                    if (infoRequestIdRef.current === requestId) {
                        setProgress(100);
                        setIsProcessing(false);
                    }
                }
            })();
        }, 120);
    }, [normalizePath]);

    useEffect(() => () => {
        if (infoRequestTimerRef.current) {
            window.clearTimeout(infoRequestTimerRef.current);
            infoRequestTimerRef.current = null;
        }
    }, []);

    const handlePathsExpanded = useCallback((result: ExpandDroppedPathsResult) => {
        setDropResult(result);
        const firstFile = result?.files?.[0];
        const inputDirForHistory = result?.has_directory
            ? (firstFile?.source_root || '')
            : dirname(firstFile?.input_path || '');
        if (inputDirForHistory) {
            void rememberRecentDirs({ inputDir: inputDirForHistory });
        }
        if (id !== 'info') {
            setLastMessage('');
            resetProgress(0);
            return;
        }
        const selected = infoFilePath
            ? result?.files?.some((f) => normalizePath(f.input_path) === normalizePath(infoFilePath))
            : false;
        if (selected) return;
        const selectedFirst = result?.files?.[0];
        if (selectedFirst?.input_path) {
            void loadInfoForPath(selectedFirst.input_path);
            return;
        }
        setInfoFilePath('');
        setInfoPreview(null);
        setLastMessage('');
    }, [dirname, id, infoFilePath, loadInfoForPath, normalizePath, rememberRecentDirs]);

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

    const handleStartProcessing = async (overrideFiles?: DroppedFile[]) => {
        if (isProcessing) return;
        cancelRequestedRef.current = false;
        setCancelRequested(false);
        setLastMessage('');
        resetProgress(0);
        const useOverrideFiles = Array.isArray(overrideFiles);
        const normalizedOverrideFiles = useOverrideFiles ? dedupeDroppedFiles(overrideFiles || []) : [];
        if (!useOverrideFiles && (!dropResult || dropResult.files.length === 0)) {
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
            id !== 'info' &&
            !((id === 'converter' && convOverwriteSource) || (id === 'compressor' && compOverwriteSource));
        if (requiresOutputDir && !outDir) {
            setLastMessage('请选择输出目录');
            return;
        }

        const appSettingsSnapshot = await loadOutputSettings();
        const preserveStructure = Boolean(appSettingsSnapshot.preserve_folder_structure);
        const outputTemplate = appSettingsSnapshot.output_template || defaultOutputSettings.output_template;
        const outputPrefix = appSettingsSnapshot.output_prefix || defaultOutputSettings.output_prefix;
        const conflictStrategy = appSettingsSnapshot.conflict_strategy || defaultOutputSettings.conflict_strategy;
        const reservedPaths = new Set<string>();
        const batchTime = new Date();
        let currentRunFiles: DroppedFile[] = [];
        const activeHasDirectory = useOverrideFiles ? true : Boolean(dropResult?.has_directory);
        const resolveUniquePath = async (candidate: string) => {
            const normalized = normalizePath(candidate);
            if (conflictStrategy !== 'rename') {
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
            if (conflictStrategy !== 'rename' || !appAny?.ResolveOutputPaths) {
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

        currentRunFailedPathsRef.current = new Set<string>();
        setIsProcessing(true);
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
            const getBatchChunkSize = sharedGetBatchChunkSize;

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
                });
                return;
            }

            if (id === 'compressor') {
                if (!appAny.Compress || !appAny.CompressBatch) {
                    setLastMessage('后端未接入压缩接口');
                    return;
                }
                const levelMap: Record<string, number> = {
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
                    overwriteSource: compOverwriteSource,
                    outputDir: outDir,
                    outputTemplate,
                    outputPrefix,
                    preserveStructure,
                    batchTime,
                    buildOutputRelPath,
                    resolveUniquePath,
                    joinPath,
                });
                return;
            }

            if (id === 'watermark') {
                if (!appAny.AddWatermark) {
                    setLastMessage('后端未接入水印接口');
                    return;
                }
                const isText = watermarkType === '文字';
                if (!isText && !watermarkImagePath) {
                    setLastMessage('请先选择水印图片');
                    return;
                }
                const fontMap: Record<string, string> = {
                    'Sans Serif': 'arial',
                    'Serif': 'times',
                    'Mono': 'cour',
                    'Handwriting': 'comic sans ms',
                };
                const blendMap: Record<string, string> = {
                    '正常': 'normal',
                    '正片叠底 (Multiply)': 'multiply',
                    '滤色 (Screen)': 'screen',
                    '叠加 (Overlay)': 'overlay',
                    '柔光 (Soft Light)': 'soft_light',
                };
                const textValue = watermarkText.trim() || '© ImageFlow';
                const opacity = clampNumber(watermarkOpacity / 100, 0, 1);
                const scale = clampNumber(watermarkSize / 100, 0.02, 1);
                const fontSize = Math.max(8, Math.round(36 * (watermarkSize / 40)));
                const fontName = fontMap[watermarkFont] || watermarkFont;
                const blendMode = blendMap[watermarkBlendMode] || 'normal';
                const resolvedPosition = resolveWatermarkBackendPosition(watermarkPosition);

                await runGenericBatch({
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
                    taskName: '图片水印',
                    label: '水印',
                    fallbackError: '水印失败',
                    canBatch: Boolean(appAny.AddWatermarkBatch),
                    runSingle: (item) => appAny.AddWatermark!(item),
                    runBatch: appAny.AddWatermarkBatch ? (items) => appAny.AddWatermarkBatch!(items) : undefined,
                    buildChunk: async (group, seqStart) => {
                        let seq = seqStart;
                        const chunk: models.WatermarkRequest[] = [];
                        for (const f of group) {
                            if (cancelRequestedRef.current) break;
                            const outRel = buildOutputRelPath(f, {
                                suffix: '_watermark',
                                seq,
                                op: 'watermark',
                                template: outputTemplate,
                                prefix: outputPrefix,
                                preserveStructure,
                                date: batchTime,
                            });
                            chunk.push({
                                input_path: normalizePath(f.input_path),
                                output_path: await resolveUniquePath(joinPath(outDir, outRel)),
                                watermark_type: isText ? 'text' : 'image',
                                text: textValue,
                                image_path: watermarkImagePath ? normalizePath(watermarkImagePath) : '',
                                position: resolvedPosition,
                                opacity,
                                scale,
                                font_size: fontSize,
                                font_color: watermarkColor,
                                rotation: watermarkRotate,
                                font_name: fontName,
                                blend_mode: blendMode,
                                tiled: watermarkTiled,
                                shadow: watermarkShadow,
                                offset_x: Math.max(0, Number(watermarkMargin.x) || 0),
                                offset_y: Math.max(0, Number(watermarkMargin.y) || 0),
                            });
                            seq += 1;
                        }
                        return { chunk, nextSeq: seq };
                    },
                });
                return;
            }

            if (id === 'adjust') {
                if (!appAny.Adjust) {
                    setLastMessage('后端未接入调整接口');
                    return;
                }
                await runGenericBatch({
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
                    taskName: '图片调整',
                    label: '调整',
                    fallbackError: '调整失败',
                    canBatch: Boolean(appAny.AdjustBatch),
                    runSingle: (item) => appAny.Adjust!(item),
                    runBatch: appAny.AdjustBatch ? (items) => appAny.AdjustBatch!(items) : undefined,
                    buildChunk: async (group, seqStart) => {
                        let seq = seqStart;
                        const cropRatio = adjustCropRatio === '自由' ? '' : adjustCropRatio;
                        const cropMode = cropRatio ? `focus:${cropFocus.x.toFixed(4)},${cropFocus.y.toFixed(4)}` : '';
                        const chunk: models.AdjustRequest[] = [];
                        for (const f of group) {
                            if (cancelRequestedRef.current) break;
                            const outRel = buildOutputRelPath(f, {
                                suffix: '_adjusted',
                                seq,
                                op: 'adjust',
                                template: outputTemplate,
                                prefix: outputPrefix,
                                preserveStructure,
                                date: batchTime,
                            });
                            chunk.push({
                                input_path: normalizePath(f.input_path),
                                output_path: await resolveUniquePath(joinPath(outDir, outRel)),
                                rotate: adjustRotate,
                                flip_h: adjustFlipH,
                                flip_v: adjustFlipV,
                                brightness: 0,
                                exposure: adjustExposure,
                                contrast: adjustContrast,
                                saturation: adjustSaturation,
                                vibrance: adjustVibrance,
                                hue: adjustHue,
                                sharpness: adjustSharpness,
                                crop_ratio: cropRatio,
                                crop_mode: cropMode,
                            });
                            seq += 1;
                        }
                        return { chunk, nextSeq: seq };
                    },
                });
                return;
            }

            if (id === 'filter') {
                if (!appAny.ApplyFilter) {
                    setLastMessage('后端未接入滤镜接口');
                    return;
                }
                const filterPreset = FILTER_PRESETS[filterSelected] || 'none';
                const intensity = clampNumber(filterIntensity / 100, 0, 1);
                const grain = clampNumber(filterGrain / 100, 0, 1);
                const vignette = clampNumber(filterVignette / 100, 0, 1);
                await runGenericBatch({
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
                    taskName: '图片滤镜',
                    label: '滤镜',
                    fallbackError: '滤镜失败',
                    canBatch: Boolean(appAny.ApplyFilterBatch),
                    runSingle: (item) => appAny.ApplyFilter!(item),
                    runBatch: appAny.ApplyFilterBatch ? (items) => appAny.ApplyFilterBatch!(items) : undefined,
                    buildChunk: async (group, seqStart) => {
                        let seq = seqStart;
                        const chunk: models.FilterRequest[] = [];
                        for (const f of group) {
                            if (cancelRequestedRef.current) break;
                            const outRel = buildOutputRelPath(f, {
                                suffix: '_filtered',
                                seq,
                                op: 'filter',
                                template: outputTemplate,
                                prefix: outputPrefix,
                                preserveStructure,
                                date: batchTime,
                            });
                            chunk.push({
                                input_path: normalizePath(f.input_path),
                                output_path: await resolveUniquePath(joinPath(outDir, outRel)),
                                filter_type: filterPreset,
                                intensity,
                                grain,
                                vignette,
                            });
                            seq += 1;
                        }
                        return { chunk, nextSeq: seq };
                    },
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
                const req: models.PDFRequest = {
                    image_paths: files.map(f => normalizePath(f.input_path)),
                    output_path: await resolveUniquePath(joinPath(outDir, `${pdfName}.pdf`)),
                    page_size: sizeMap[pdfSize] ?? 'A4',
                    layout: pdfLayout === '横向' ? 'landscape' : 'portrait',
                    margin: marginPoints,
                    compression_level: compressionMap[pdfCompression] ?? 0,
                    fit_mode: fitMap[pdfFit] ?? 'contain',
                    title: pdfTitle.trim(),
                    author: pdfAuthor.trim(),
                };
                const res = await appAny.GeneratePDF(req);
                setProgress(100);
                if (res?.success) {
                    setLastMessage(`PDF 已生成：${req.output_path}`);
                } else {
                    if (isCancellationError(res?.error)) {
                        cancelRequestedRef.current = true;
                        setCancelRequested(true);
                        setLastMessage('PDF 处理已取消');
                        return;
                    }
                    reportTaskFailure('转 PDF', files[0]?.input_path || '', res?.error, 'PDF 生成失败');
                    setLastMessage(res?.error || 'PDF 生成失败');
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
                        const res = await appAny.SplitGIF({
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
                        const success = Math.max(0, completed - failed);
                        const extra = failed > 0 ? `（失败 ${failed}）` : '';
                        setLastMessage(cancelled
                            ? `导出已停止：成功 ${success}/${animatedFiles.length} 项${extra}`
                            : `导出完成：成功 ${success}/${animatedFiles.length} 项${extra}`);
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
                        try {
                            const req: models.GIFSplitRequest = {
                                action: 'build_gif',
                                input_paths: files.map(f => normalizePath(f.input_path)),
                                output_path: outputPath,
                                fps: gifBuildFps,
                                maintain_aspect: true,
                            };
                            const res = await appAny.SplitGIF(req);
                            if (res?.success) {
                                setLastMessage(`合成完成：${res.output_path || outputPath}`);
                            } else {
                                const normalizedError = resolveGifErrorMessage(res?.error_code, res?.error);
                                if (isCancellationError(res?.error)) {
                                    cancelRequestedRef.current = true;
                                    setCancelRequested(true);
                                    setLastMessage('GIF 合成已取消');
                                    return;
                                }
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
                            console.error('Failed to build GIF:', err);
                            reportTaskFailure('GIF 合成', files[0]?.input_path || '', err, '合成失败');
                            setLastMessage('合成失败');
                    } finally {
                        setProgress(100);
                    }
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
                    const success = Math.max(0, completed - failed);
                    const extra = failed > 0 ? `（失败 ${failed}）` : '';
                    setLastMessage(cancelled
                        ? `互转已停止：成功 ${success}/${animatedFiles.length} 项${extra}`
                        : `互转完成：成功 ${success}/${animatedFiles.length} 项${extra}`);
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
                    const outRel = buildOutputRelPath(f, {
                        suffix,
                        seq,
                        op: 'gif',
                        template: outputTemplate,
                        prefix: outputPrefix,
                        preserveStructure,
                        date: batchTime,
                    });
                    const outputPath = await resolveUniquePath(joinPath(outDir, outRel));
                    const req: models.GIFSplitRequest = {
                        action,
                        input_path: normalizePath(f.input_path),
                        output_path: outputPath,
                        maintain_aspect: gifResizeMaintainAR,
                    };
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
                const success = Math.max(0, completed - failed);
                const extra = failed > 0 ? `（失败 ${failed}）` : '';
                setLastMessage(cancelled
                    ? `${gifMode}已停止：成功 ${success}/${pureGifFiles.length} 项${extra}`
                    : `${gifMode}完成：成功 ${success}/${pureGifFiles.length} 项${extra}`);
                return;
            }

            if (id === 'info') {
                const f = files[0];
                await loadInfoForPath(f.input_path);
                return;
            }

            setLastMessage('该功能暂未接入');
        } catch (e: any) {
            console.error(e);
            reportBatchTaskFailure(feature.title, currentRunFiles.length > 0 ? currentRunFiles : (dropResult?.files || []), e, '处理失败');
            if (isCancellationError(e)) {
                setLastMessage('任务已取消');
            } else {
                setLastMessage(typeof e?.message === 'string' ? e.message : '处理失败');
            }
        } finally {
            const failedSet = currentRunFailedPathsRef.current;
            if (failedSet && currentRunFiles.length > 0) {
                const nextFailed = currentRunFiles.filter((f) => failedSet.has(normalizePath(f.input_path)));
                setFailedRecords(nextFailed);
                if (nextFailed.length === 0) {
                    setRetryFailedOnly(false);
                }
            }
            currentRunFailedPathsRef.current = null;
            cancelRequestedRef.current = false;
            setCancelRequested(false);
            setIsProcessing(false);
        }
    };

    const selectedDropPath = id === 'info' ? infoFilePath : isPreviewFeature ? previewPath : undefined;
    const acceptedInputFormats = id === 'compressor'
        ? COMPRESSOR_ACCEPTED_FORMATS
        : id === 'info'
            ? INFO_IMAGE_ACCEPTED_FORMATS
            : DEFAULT_IMAGE_ACCEPTED_FORMATS;
    const inputFileDialogFilters = id === 'compressor'
        ? COMPRESSOR_FILE_DIALOG_FILTERS
        : id === 'info'
            ? INFO_IMAGE_FILE_DIALOG_FILTERS
            : DEFAULT_IMAGE_FILE_DIALOG_FILTERS;
    const dropZone = (
        <FileDropZone 
            isActive={isActive}
            onFilesSelected={handleFilesSelected}
            onPathsExpanded={handlePathsExpanded}
            onItemSelect={(file) => {
                if (!file?.input_path) return;
                if (id === 'info') {
                    void loadInfoForPath(file.input_path);
                    return;
                }
                if (isPreviewFeature) {
                    setPreviewPath(normalizePath(file.input_path));
                }
            }}
            selectedPath={selectedDropPath}
            acceptedFormats={acceptedInputFormats}
            fileDialogFilters={inputFileDialogFilters}
            allowMultiple={true}
            title="拖拽文件 / 文件夹到这里"
            subTitle=""
        />
    );

    const showInputInSettings = !isInfo && id !== 'adjust';
    const showActionInSettings = !isInfo && id !== 'adjust' && id !== 'filter';

    const renderInputSection = (compact = false) => (
        <div className={`${compact ? 'space-y-1 pb-2' : 'space-y-2 pb-3'} border-b border-gray-100 dark:border-white/5 ${compact ? 'mb-2' : 'mb-3'}`}>
            <div className="flex items-center justify-between gap-2 flex-wrap text-sm">
                <div className="flex items-center gap-2">
                    <span className="text-gray-700 dark:text-gray-300 font-medium">输入项</span>
                    <span className="text-gray-500 dark:text-gray-400 font-mono text-xs">{inputCount}</span>
                </div>
                <button
                    onClick={handleSelectOutputDir}
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
            {id !== 'info' && (
                <div className="space-y-2">
                    <button
                        type="button"
                        onClick={() => setIsPresetSectionCollapsed((previous) => !previous)}
                        className="w-full flex items-center justify-between gap-3 text-left rounded-xl border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 px-3 py-2 hover:bg-white dark:hover:bg-white/10 transition-colors"
                    >
                        <div className="min-w-0">
                            <div className="text-xs text-gray-600 dark:text-gray-300 font-medium">批处理预设</div>
                            <div className="text-[11px] text-gray-500 dark:text-gray-400">
                                {currentFeaturePresets.length > 0 ? `已保存 ${currentFeaturePresets.length} 个预设` : '暂无已保存预设'}
                            </div>
                        </div>
                        <Icon
                            name="ChevronDown"
                            size={16}
                            className={`shrink-0 text-gray-400 transition-transform ${isPresetSectionCollapsed ? '-rotate-90' : 'rotate-0'}`}
                        />
                    </button>
                    {!isPresetSectionCollapsed && (
                        <div className="space-y-2 animate-enter">
                            <div className="flex gap-2">
                                <input
                                    value={presetNameDraft}
                                    onChange={(e) => setPresetNameDraft(e.target.value)}
                                    placeholder="输入预设名称"
                                    className="flex-1 px-3 py-2 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-xs outline-none dark:text-white"
                                />
                                <button
                                    onClick={saveCurrentPreset}
                                    className="px-3 py-2 rounded-xl border border-gray-200 dark:border-white/10 text-xs font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5"
                                >
                                    保存
                                </button>
                            </div>
                            <div className="flex gap-2">
                                <select
                                    value={selectedPresetId}
                                    onChange={(e) => setSelectedPresetId(e.target.value)}
                                    className="flex-1 px-3 py-2 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-xs outline-none dark:text-white"
                                >
                                    <option value="">选择预设</option>
                                    {currentFeaturePresets.map((preset) => (
                                        <option key={preset.id} value={preset.id}>{preset.name}</option>
                                    ))}
                                </select>
                                <button
                                    onClick={applySelectedPreset}
                                    disabled={!selectedPresetId}
                                    className="px-3 py-2 rounded-xl border border-gray-200 dark:border-white/10 text-xs font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    应用
                                </button>
                                <button
                                    onClick={deleteSelectedPreset}
                                    disabled={!selectedPresetId}
                                    className="px-3 py-2 rounded-xl border border-red-200 dark:border-red-500/30 text-xs font-medium text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    删除
                                </button>
                            </div>
                        </div>
                    )}
                </div>
            )}
            {lastMessage && (
                <div
                    className="text-xs text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2 break-all"
                    style={clampTwoLinesStyle}
                >
                    {lastMessage}
                </div>
            )}
        </div>
    );

    const renderActionContent = (compact = false) => (
        <>
            {(isProcessing || progress > 0) && (
                <ProgressBar progress={progress} label={isProcessing ? "正在处理..." : "已完成"} />
            )}
            <button 
                onClick={isProcessing ? requestCancelProcessing : () => { void handleStartProcessing(); }}
                disabled={isProcessing && cancelRequested}
                className={`w-full ${compact ? 'py-3' : 'py-3.5'} rounded-xl font-semibold shadow-lg shadow-blue-500/25 transition-all active:scale-[0.98] flex items-center justify-center gap-2 text-white ${isProcessing ? (cancelRequested ? 'bg-gray-400 cursor-not-allowed' : 'bg-amber-500 hover:bg-amber-600') : 'bg-gradient-to-r from-[#007AFF] to-[#0055FF] hover:to-[#0044DD]'}`}
            >
                <Icon name={isProcessing ? 'Close' : 'Wand2'} size={18} /> {isProcessing ? (cancelRequested ? '停止中...' : '停止处理') : '开始处理'}
            </button>
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
                            className={`px-2.5 py-1.5 rounded-lg border text-xs font-medium ${retryFailedOnly ? 'border-amber-500 text-amber-700 dark:text-amber-300 bg-amber-100/60 dark:bg-amber-500/20' : 'border-amber-200 dark:border-amber-500/20 text-amber-700 dark:text-amber-300'}`}
                        >
                            {retryFailedOnly ? '仅重试失败项：开启' : '仅重试失败项：关闭'}
                        </button>
                        <button
                            onClick={() => {
                                setFailedRecords([]);
                                setRetryFailedOnly(false);
                            }}
                            className="px-2.5 py-1.5 rounded-lg border border-amber-200 dark:border-amber-500/20 text-xs font-medium text-amber-700 dark:text-amber-300"
                        >
                            清空失败列表
                        </button>
                    </div>
                </div>
            )}
            {renderActionContent(compact)}
        </div>
    );

    const filterControlsPanel = (
        <FilterControls
            intensity={filterIntensity}
            setIntensity={setFilterIntensity}
            grain={filterGrain}
            setGrain={setFilterGrain}
            vignette={filterVignette}
            setVignette={setFilterVignette}
            footer={renderActionContent(true)}
        />
    );

    const settingsPanel = (
        <div
            className={`bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col overflow-hidden h-full min-h-0 ${isInfo ? 'lg:col-span-4' : ''}`}
        >
            {showInputInSettings && renderInputSection()}
            {isInfo && lastMessage && (
                <div
                    className="mb-4 text-xs text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2 break-all"
                    style={clampTwoLinesStyle}
                >
                    {lastMessage}
                </div>
            )}
            
            <div className="flex-1 min-h-0 overflow-y-auto no-scrollbar px-1 pb-2">
                {renderSettings()}
            </div>

            {showActionInSettings && renderActionSection()}
        </div>
    );

    const watermarkPreviewOverlay = isWatermark && !isCompareActive && previewSrc && previewBaseMetrics && watermarkPreviewConfig ? (
        <div className="absolute inset-0 pointer-events-none">
            {watermarkFontFace?.css && (
                <style>{watermarkFontFace.css}</style>
            )}
            <div
                className="absolute"
                style={{
                    left: previewBaseMetrics.left,
                    top: previewBaseMetrics.top,
                    width: previewBaseMetrics.width,
                    height: previewBaseMetrics.height,
                    overflow: 'hidden',
                }}
            >
                {watermarkTiled && watermarkTilePositions && watermarkTileMetrics ? (
                    watermarkTilePositions.map((pos, idx) => (
                        watermarkType === '图片' ? (
                            watermarkImageSrc ? (
                                <img
                                    key={`wm-img-${idx}`}
                                    src={watermarkImageSrc}
                                    className="absolute"
                                    style={{
                                        left: pos.x,
                                        top: pos.y,
                                        width: watermarkTileMetrics.width,
                                        height: watermarkTileMetrics.height,
                                        opacity: watermarkPreviewConfig.opacity,
                                        mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                        transform: `rotate(${watermarkRotate}deg)`,
                                        transformOrigin: 'center',
                                        filter: watermarkShadow ? 'drop-shadow(0 2px 6px rgba(0,0,0,0.35))' : undefined,
                                    }}
                                    alt="watermark-tile"
                                />
                            ) : null
                        ) : (
                            <div
                                key={`wm-text-${idx}`}
                                className="absolute"
                                style={{
                                    left: pos.x,
                                    top: pos.y,
                                    fontFamily: watermarkFontFamily,
                                    fontSize: watermarkPreviewConfig.previewFontSize,
                                    lineHeight: 1.2,
                                    color: watermarkColor,
                                    opacity: watermarkPreviewConfig.opacity,
                                    mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                    transform: `rotate(${watermarkRotate}deg)`,
                                    transformOrigin: 'center',
                                    whiteSpace: 'pre-line',
                                    textShadow: watermarkShadow ? '0 2px 6px rgba(0,0,0,0.35)' : undefined,
                                }}
                            >
                                {watermarkPreviewConfig.textValue}
                            </div>
                        )
                    ))
                ) : (
                    watermarkType === '图片' ? (
                        watermarkImageSrc && watermarkImagePreviewSize ? (
                            <img
                                src={watermarkImageSrc}
                                className="absolute"
                                style={{
                                    ...watermarkPositionStyle,
                                    width: watermarkImagePreviewSize.width,
                                    height: watermarkImagePreviewSize.height,
                                    opacity: watermarkPreviewConfig.opacity,
                                    mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                    filter: watermarkShadow ? 'drop-shadow(0 2px 6px rgba(0,0,0,0.35))' : undefined,
                                }}
                                alt="watermark"
                            />
                        ) : null
                    ) : (
                        <div
                            className="absolute"
                            style={{
                                ...watermarkPositionStyle,
                                fontFamily: watermarkFontFamily,
                                fontSize: watermarkPreviewConfig.previewFontSize,
                                lineHeight: 1.2,
                                color: watermarkColor,
                                opacity: watermarkPreviewConfig.opacity,
                                mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                whiteSpace: 'pre-line',
                                textShadow: watermarkShadow ? '0 2px 6px rgba(0,0,0,0.35)' : undefined,
                            }}
                        >
                            {watermarkPreviewConfig.textValue}
                        </div>
                    )
                )}
            </div>
        </div>
    ) : null;

    const previewPanel = (
        <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col h-full min-h-0 overflow-hidden">
            <div className="flex items-center justify-between mb-4 gap-3">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">实时预览</span>
                <div className="flex items-center gap-2">
                    <button
                        type="button"
                        disabled={!previewSrc}
                        aria-pressed={isCompareActive}
                        onPointerDown={() => setIsComparing(true)}
                        onPointerUp={() => setIsComparing(false)}
                        onPointerLeave={() => setIsComparing(false)}
                        onPointerCancel={() => setIsComparing(false)}
                        onBlur={() => setIsComparing(false)}
                        className={`flex items-center gap-1.5 text-[11px] px-2 py-1 rounded-full border transition ${
                            previewSrc
                                ? 'text-gray-600 dark:text-gray-300 border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 hover:text-gray-900 dark:hover:text-white'
                                : 'text-gray-300 dark:text-gray-600 border-gray-100 dark:border-white/5 bg-gray-50 dark:bg-white/5 cursor-not-allowed'
                        }`}
                    >
                        <Icon name="Layers" size={12} />
                        {isCompareActive ? '原图' : '按住对比'}
                    </button>
                </div>
            </div>
            <div
                ref={previewContainerRef}
                className="relative w-full flex-1 min-h-[220px] rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 overflow-hidden flex items-center justify-center"
            >
                {previewSrc ? (
                    showCropFrame && cropImageMetrics ? (
                        <div className="w-full h-full relative">
                            <div className="absolute inset-0 flex items-center justify-center pointer-events-none select-none">
                                <div
                                    className="relative"
                                    style={{
                                        width: cropImageMetrics.drawnWidth,
                                        height: cropImageMetrics.drawnHeight,
                                        transform: `translate(${cropOffset.x}px, ${cropOffset.y}px)`,
                                    }}
                                >
                                    <div
                                        className="relative w-full h-full"
                                        style={{
                                            transform: effectivePreviewTransform || 'none',
                                            transformOrigin: 'center',
                                        }}
                                    >
                                        <img
                                            src={previewSrc}
                                            className="w-full h-full object-cover transition-all duration-150 pointer-events-none"
                                            style={{
                                                filter: effectivePreviewFilter || 'none',
                                            }}
                                            onLoad={(e) => {
                                                const img = e.currentTarget;
                                                if (img.naturalWidth && img.naturalHeight) {
                                                    setPreviewImageSize({
                                                        width: img.naturalWidth,
                                                        height: img.naturalHeight,
                                                    });
                                                }
                                            }}
                                            alt="preview"
                                        />
                                        <div
                                            className="pointer-events-none absolute inset-0 transition-opacity duration-150"
                                            style={{
                                                opacity: effectiveVignetteOpacity,
                                                background: 'radial-gradient(circle at center, rgba(0,0,0,0) 35%, rgba(0,0,0,0.85) 100%)',
                                            }}
                                        />
                                        <div
                                            className="pointer-events-none absolute inset-0 mix-blend-soft-light transition-opacity duration-150"
                                            style={{
                                                opacity: effectiveGrainOpacity,
                                                backgroundImage: 'repeating-linear-gradient(0deg, rgba(0,0,0,0.2) 0, rgba(0,0,0,0.2) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 3px), repeating-linear-gradient(90deg, rgba(0,0,0,0.15) 0, rgba(0,0,0,0.15) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 4px)',
                                            }}
                                        />
                                    </div>
                                </div>
                            </div>
                            <div
                                className={`absolute border-2 border-dashed border-gray-300/80 dark:border-white/40 ${isCropInteractive ? (isCropDragging ? 'cursor-grabbing' : 'cursor-grab') : ''}`}
                                style={{
                                    width: previewFrame.width,
                                    height: previewFrame.height,
                                    left: previewFrame.left,
                                    top: previewFrame.top,
                                    boxShadow: '0 0 0 9999px rgba(0,0,0,0.35)',
                                    touchAction: 'none',
                                }}
                                onPointerDown={handleCropPointerDown}
                                onPointerMove={handleCropPointerMove}
                                onPointerUp={handleCropPointerUp}
                                onPointerCancel={handleCropPointerUp}
                            />
                        </div>
                    ) : (
                        <div className="w-full h-full flex items-center justify-center pointer-events-none select-none">
                            <div
                                className="relative w-full h-full"
                                style={{
                                    transform: effectivePreviewTransform || 'none',
                                    transformOrigin: 'center',
                                }}
                            >
                                <img
                                    src={previewSrc}
                                    className="w-full h-full object-contain transition-all duration-150 pointer-events-none"
                                    style={{
                                        filter: effectivePreviewFilter || 'none',
                                    }}
                                    onLoad={(e) => {
                                        const img = e.currentTarget;
                                        if (img.naturalWidth && img.naturalHeight) {
                                            setPreviewImageSize({
                                                width: img.naturalWidth,
                                                height: img.naturalHeight,
                                            });
                                        }
                                    }}
                                    alt="preview"
                                />
                                <div
                                    className="pointer-events-none absolute inset-0 transition-opacity duration-150"
                                    style={{
                                        opacity: effectiveVignetteOpacity,
                                        background: 'radial-gradient(circle at center, rgba(0,0,0,0) 35%, rgba(0,0,0,0.85) 100%)',
                                    }}
                                />
                                <div
                                    className="pointer-events-none absolute inset-0 mix-blend-soft-light transition-opacity duration-150"
                                    style={{
                                        opacity: effectiveGrainOpacity,
                                        backgroundImage: 'repeating-linear-gradient(0deg, rgba(0,0,0,0.2) 0, rgba(0,0,0,0.2) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 3px), repeating-linear-gradient(90deg, rgba(0,0,0,0.15) 0, rgba(0,0,0,0.15) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 4px)',
                                    }}
                                />
                            </div>
                        </div>
                    )
                ) : (
                    <div className={`text-xs ${previewLoadError ? 'text-red-500' : 'text-gray-400'} px-4 text-center break-all`}>
                        {previewLoadError || '拖入图片后显示预览'}
                    </div>
                )}
                {watermarkPreviewOverlay}
            </div>
        </div>
    );

    if (isWatermark) {
        return (
            <div className="h-full flex flex-col p-1">
                <div className="grid grid-cols-1 lg:grid-cols-[minmax(360px,4fr)_minmax(0,6fr)] gap-6 min-h-0">
                    <div className="h-[320px] lg:h-[420px]">
                        {dropZone}
                    </div>
                    <div className="h-[320px] lg:h-[420px]">
                        {previewPanel}
                    </div>
                </div>
                <div className="mt-6 flex-1 min-h-0">
                    {settingsPanel}
                </div>
            </div>
        );
    }

    return (
        <div className="h-full flex flex-col p-1">
            <div className={`flex-1 grid grid-cols-1 ${isInfo ? 'lg:grid-cols-6' : isAdjustOrFilter ? 'lg:grid-cols-[280px_minmax(0,1fr)]' : 'lg:grid-cols-3'} gap-6 min-h-0`}>
                {/* Left Side: Upload Area (Shared) - Replaced with FileDropZone */}
                <div className={`h-full min-h-0 ${isAdjustOrFilter ? '' : isInfo ? 'lg:col-span-2' : 'lg:col-span-2'}`}>
                    {id === 'adjust' ? (
                        <div className="h-full flex flex-col gap-4 min-h-0">
                            <div className="flex-1 min-h-0">
                                {dropZone}
                            </div>
                            <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-4 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col gap-4 min-h-0 overflow-hidden">
                                <div className="flex-1 overflow-y-auto no-scrollbar">
                                    {renderInputSection(true)}
                                    <AdjustCropControls
                                        cropRatio={adjustCropRatio}
                                        setCropRatio={setAdjustCropRatio}
                                        rotate={adjustRotate}
                                        setRotate={setAdjustRotate}
                                        flipH={adjustFlipH}
                                        setFlipH={setAdjustFlipH}
                                        flipV={adjustFlipV}
                                        setFlipV={setAdjustFlipV}
                                    />
                                </div>
                                {renderActionSection(true)}
                            </div>
                        </div>
                    ) : id === 'filter' ? (
                        <div className="h-full flex flex-col gap-4 min-h-0">
                            <div className="flex-1 min-h-0">
                                {dropZone}
                            </div>
                            <div className="shrink-0">
                                {filterControlsPanel}
                            </div>
                        </div>
                    ) : (
                        dropZone
                    )}
                </div>

                {/* Right Side: Specific Settings Panel (Secondary Menu) */}
                <div className={`h-full min-h-0 ${isAdjustOrFilter ? '' : isInfo ? 'lg:col-span-4' : ''}`}>
                    {isAdjustOrFilter ? (
                        <div className="h-full flex flex-col gap-4 min-h-0 overflow-hidden">
                            <div className={`${id === 'adjust' ? 'flex-[1.2]' : 'flex-1'} min-h-0`}>
                                {previewPanel}
                            </div>
                            <div className={`${id === 'adjust' ? 'flex-[0.8]' : 'flex-1'} min-h-0`}>
                                {settingsPanel}
                            </div>
                        </div>
                    ) : (
                        settingsPanel
                    )}
                </div>
            </div>
        </div>
    );
};

export default memo(DetailView);

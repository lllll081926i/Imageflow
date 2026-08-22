import { useCallback, useState } from 'react';

export function useConverterParams() {
    const [format, setFormat] = useState('JPG');
    const [quality, setQuality] = useState(80);
    const [compressLevel, setCompressLevel] = useState(6);
    const [icoSizes, setIcoSizes] = useState<number[]>([16, 32, 48, 64, 128, 256]);
    const [resizeMode, setResizeMode] = useState('原图尺寸');
    const [scalePercent, setScalePercent] = useState(100);
    const [fixedWidth, setFixedWidth] = useState(0);
    const [fixedHeight, setFixedHeight] = useState(0);
    const [longEdge, setLongEdge] = useState(1920);
    const [keepMetadata, setKeepMetadata] = useState(false);
    const [maintainAR, setMaintainAR] = useState(true);
    const [overwriteSource, setOverwriteSource] = useState(false);

    return {
        format, setFormat,
        quality, setQuality,
        compressLevel, setCompressLevel,
        icoSizes, setIcoSizes,
        resizeMode, setResizeMode,
        scalePercent, setScalePercent,
        fixedWidth, setFixedWidth,
        fixedHeight, setFixedHeight,
        longEdge, setLongEdge,
        keepMetadata, setKeepMetadata,
        maintainAR, setMaintainAR,
        overwriteSource, setOverwriteSource,
    };
}

export function useCompressorParams() {
    const [mode, setMode] = useState('智能（推荐）');
    const [targetSize, setTargetSize] = useState(false);
    const [targetSizeKB, setTargetSizeKB] = useState(500);
    const [engine, setEngine] = useState('自动 (推荐)');
    const [overwriteSource, setOverwriteSource] = useState(false);
    const [resizeMode, setResizeMode] = useState('不缩放');
    const [resizePercent, setResizePercent] = useState(100);
    const [resizeWidth, setResizeWidth] = useState(1920);
    const [resizeHeight, setResizeHeight] = useState(1080);
    const [maintainAspect, setMaintainAspect] = useState(true);

    return {
        mode, setMode,
        targetSize, setTargetSize,
        targetSizeKB, setTargetSizeKB,
        engine, setEngine,
        overwriteSource, setOverwriteSource,
        resizeMode, setResizeMode,
        resizePercent, setResizePercent,
        resizeWidth, setResizeWidth,
        resizeHeight, setResizeHeight,
        maintainAspect, setMaintainAspect,
    };
}

export function usePdfParams() {
    const [size, setSize] = useState('A4 (210 x 297 mm)');
    const [layout, setLayout] = useState('纵向');
    const [fit, setFit] = useState('适应页面 (保持比例)');
    const [marginMm, setMarginMm] = useState(25.4);
    const [compression, setCompression] = useState('不压缩');
    const [fileName, setFileName] = useState('');
    const [title, setTitle] = useState('');
    const [author, setAuthor] = useState('');
    const [password, setPassword] = useState('');

    return {
        size, setSize,
        layout, setLayout,
        fit, setFit,
        marginMm, setMarginMm,
        compression, setCompression,
        fileName, setFileName,
        title, setTitle,
        author, setAuthor,
        password, setPassword,
    };
}

export function useGifParams() {
    const [mode, setMode] = useState('导出');
    const [exportFormat, setExportFormat] = useState('PNG');
    const [convertFormat, setConvertFormat] = useState('WEBP');
    const [speedPercent, setSpeedPercent] = useState(100);
    const [compressQuality, setCompressQuality] = useState(90);
    const [buildFps, setBuildFps] = useState(10);

    return {
        mode, setMode,
        exportFormat, setExportFormat,
        convertFormat, setConvertFormat,
        speedPercent, setSpeedPercent,
        compressQuality, setCompressQuality,
        buildFps, setBuildFps,
    };
}

export function useGridParams() {
    const [gridSize, setGridSize] = useState('九宫格');
    const [format, setFormat] = useState('PNG');

    return {
        gridSize, setGridSize,
        format, setFormat,
    };
}

export function useSplitParams() {
    const [rows, setRows] = useState(2);
    const [cols, setCols] = useState(3);
    const [mode, setMode] = useState<'square' | 'full'>('square');
    const [format, setFormat] = useState('PNG');

    return {
        rows, setRows,
        cols, setCols,
        mode, setMode,
        format, setFormat,
    };
}

export type WatermarkStyle = {
    text: string;
    fontName: string;
    sizeMode: 'px' | 'percent';
    fontSize: number;
    color: string;
    opacity: number;
    rotation: number;
    position: string;
    margin: number;
    tiled: boolean;
    tileGapX: number;
    tileGapY: number;
    strokeWidth: number;
    strokeColor: string;
    shadow: boolean;
};

export function useWatermarkParams() {
    const [text, setText] = useState('ImageFlow');
    const [fontName, setFontName] = useState('微软雅黑');
    const [sizeMode, setSizeMode] = useState<'px' | 'percent'>('percent');
    const [fontSize, setFontSize] = useState(6);
    const [color, setColor] = useState('#FFFFFF');
    const [opacity, setOpacity] = useState(60);
    const [rotation, setRotation] = useState(0);
    const [position, setPosition] = useState('br');
    const [margin, setMargin] = useState(40);
    const [tiled, setTiled] = useState(false);
    const [tileGapX, setTileGapX] = useState(120);
    const [tileGapY, setTileGapY] = useState(120);
    const [strokeWidth, setStrokeWidth] = useState(0);
    const [strokeColor, setStrokeColor] = useState('#000000');
    const [shadow, setShadow] = useState(true);

    const patch = useCallback((partial: Partial<WatermarkStyle>) => {
        setText((prev) => partial.text ?? prev);
        setFontName((prev) => partial.fontName ?? prev);
        setSizeMode((prev) => partial.sizeMode ?? prev);
        setFontSize((prev) => partial.fontSize ?? prev);
        setColor((prev) => partial.color ?? prev);
        setOpacity((prev) => partial.opacity ?? prev);
        setRotation((prev) => partial.rotation ?? prev);
        setPosition((prev) => partial.position ?? prev);
        setMargin((prev) => partial.margin ?? prev);
        setTiled((prev) => partial.tiled ?? prev);
        setTileGapX((prev) => partial.tileGapX ?? prev);
        setTileGapY((prev) => partial.tileGapY ?? prev);
        setStrokeWidth((prev) => partial.strokeWidth ?? prev);
        setStrokeColor((prev) => partial.strokeColor ?? prev);
        setShadow((prev) => partial.shadow ?? prev);
    }, []);

    return {
        text, setText,
        fontName, setFontName,
        sizeMode, setSizeMode,
        fontSize, setFontSize,
        color, setColor,
        opacity, setOpacity,
        rotation, setRotation,
        position, setPosition,
        margin, setMargin,
        tiled, setTiled,
        tileGapX, setTileGapX,
        tileGapY, setTileGapY,
        strokeWidth, setStrokeWidth,
        strokeColor, setStrokeColor,
        shadow, setShadow,
        patch,
    };
}

export function useResizeParams() {
    const [mode, setMode] = useState<'percent' | 'long_edge' | 'short_edge'>('long_edge');
    const [value, setValue] = useState(1920);
    const [format, setFormat] = useState<'auto' | 'PNG' | 'JPG'>('auto');

    return {
        mode, setMode,
        value, setValue,
        format, setFormat,
    };
}

export function useStitchParams() {
    const [direction, setDirection] = useState<'vertical' | 'horizontal'>('vertical');
    const [gap, setGap] = useState(0);
    const [background, setBackground] = useState('#FFFFFF');
    const [targetLength, setTargetLength] = useState(0);
    const [border, setBorder] = useState(0);
    const [outputFormat, setOutputFormat] = useState('PNG');

    return {
        direction, setDirection,
        gap, setGap,
        background, setBackground,
        targetLength, setTargetLength,
        border, setBorder,
        outputFormat, setOutputFormat,
    };
}

export type RichTextBlock = {
    type: 'heading' | 'paragraph' | 'list' | 'quote' | 'image';
    text?: string;
    level?: number;
    ordered?: boolean;
    items?: string[];
    path?: string;
    width_percent?: number;
};

export function useRichTextParams() {
    const [blocks, setBlocks] = useState<RichTextBlock[]>([
        { type: 'heading', text: '标题', level: 1 },
        { type: 'paragraph', text: '在这里输入正文内容…' },
    ]);
    const [width, setWidth] = useState(800);
    const [padding, setPadding] = useState(48);
    const [background, setBackground] = useState('#FFFFFF');
    const [textColor, setTextColor] = useState('#1F2937');
    const [headingColor, setHeadingColor] = useState('#111827');
    const [accentColor, setAccentColor] = useState('#007AFF');
    const [fontSize, setFontSize] = useState(18);
    const [lineHeight, setLineHeight] = useState(1.6);
    const [fontName, setFontName] = useState('微软雅黑');
    const [fileName, setFileName] = useState('');

    return {
        blocks, setBlocks,
        width, setWidth,
        padding, setPadding,
        background, setBackground,
        textColor, setTextColor,
        headingColor, setHeadingColor,
        accentColor, setAccentColor,
        fontSize, setFontSize,
        lineHeight, setLineHeight,
        fontName, setFontName,
        fileName, setFileName,
    };
}

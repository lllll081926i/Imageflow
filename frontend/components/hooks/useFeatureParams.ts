import { useState } from 'react';

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
    const [mode, setMode] = useState('标准');
    const [targetSize, setTargetSize] = useState(false);
    const [targetSizeKB, setTargetSizeKB] = useState(500);
    const [engine, setEngine] = useState('自动 (推荐)');
    const [overwriteSource, setOverwriteSource] = useState(false);

    return {
        mode, setMode,
        targetSize, setTargetSize,
        targetSizeKB, setTargetSizeKB,
        engine, setEngine,
        overwriteSource, setOverwriteSource,
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

    return {
        size, setSize,
        layout, setLayout,
        fit, setFit,
        marginMm, setMarginMm,
        compression, setCompression,
        fileName, setFileName,
        title, setTitle,
        author, setAuthor,
    };
}

export function useAdjustParams() {
    const [exposure, setExposure] = useState(0);
    const [contrast, setContrast] = useState(0);
    const [saturation, setSaturation] = useState(0);
    const [sharpness, setSharpness] = useState(0);
    const [vibrance, setVibrance] = useState(0);
    const [hue, setHue] = useState(0);
    const [rotate, setRotate] = useState(0);
    const [flipH, setFlipH] = useState(false);
    const [flipV, setFlipV] = useState(false);
    const [cropRatio, setCropRatio] = useState('自由');

    return {
        exposure, setExposure,
        contrast, setContrast,
        saturation, setSaturation,
        sharpness, setSharpness,
        vibrance, setVibrance,
        hue, setHue,
        rotate, setRotate,
        flipH, setFlipH,
        flipV, setFlipV,
        cropRatio, setCropRatio,
    };
}

export function useFilterParams() {
    const [intensity, setIntensity] = useState(80);
    const [grain, setGrain] = useState(0);
    const [vignette, setVignette] = useState(0);
    const [selected, setSelected] = useState(0);

    return {
        intensity, setIntensity,
        grain, setGrain,
        vignette, setVignette,
        selected, setSelected,
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

export function useWatermarkParams() {
    const [type, setType] = useState('文字');
    const [text, setText] = useState('© ImageFlow');
    const [imagePath, setImagePath] = useState('');
    const [position, setPosition] = useState('br');
    const [opacity, setOpacity] = useState(85);
    const [rotate, setRotate] = useState(0);
    const [size, setSize] = useState(40);
    const [tiled, setTiled] = useState(false);
    const [blendMode, setBlendMode] = useState('正常');
    const [shadow, setShadow] = useState(false);
    const [margin, setMargin] = useState({ x: 20, y: 20 });
    const [font, setFont] = useState('Sans Serif');
    const [color, setColor] = useState('#FFFFFF');
    const [useSystemFonts, setUseSystemFonts] = useState(false);
    const [systemFonts, setSystemFonts] = useState<string[]>([]);
    const [isSystemFontsLoading, setIsSystemFontsLoading] = useState(false);
    const [imageSize, setImageSize] = useState({ width: 0, height: 0 });

    return {
        type, setType,
        text, setText,
        imagePath, setImagePath,
        position, setPosition,
        opacity, setOpacity,
        rotate, setRotate,
        size, setSize,
        tiled, setTiled,
        blendMode, setBlendMode,
        shadow, setShadow,
        margin, setMargin,
        font, setFont,
        color, setColor,
        useSystemFonts, setUseSystemFonts,
        systemFonts, setSystemFonts,
        isSystemFontsLoading, setIsSystemFontsLoading,
        imageSize, setImageSize,
    };
}

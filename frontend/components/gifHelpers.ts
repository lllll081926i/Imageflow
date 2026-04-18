export type GifProcessAction = 'reverse' | 'change_speed' | 'compress' | 'resize' | 'convert_animation';
export type GifInputKind = 'empty' | 'images' | 'gif' | 'animated' | 'mixed';
export type GifInputSummary = {
    kind: GifInputKind;
    hasAnimated: boolean;
    hasStatic: boolean;
    hasGif: boolean;
    hasNonGifAnimated: boolean;
};

const FULL_GIF_MODE_OPTIONS = ['导出', '互转', '倒放', '修改帧率', '压缩', '缩放'] as const;
const ANIMATED_MODE_OPTIONS = ['导出', '互转'] as const;

const FORMAT_EXTENSION_GROUPS: Record<string, string[]> = {
    jpg: ['jpg', 'jpeg'],
    jpeg: ['jpg', 'jpeg'],
    tif: ['tif', 'tiff'],
    tiff: ['tif', 'tiff'],
};

const PROBE_ANIMATED_EXTENSIONS = new Set(['png', 'webp']);

function normalizePath(path: string): string {
    return String(path || '').replace(/\\/g, '/');
}

function getPathExtension(path: string): string {
    const normalized = normalizePath(path);
    const idx = normalized.lastIndexOf('.');
    if (idx === -1) {
        return '';
    }
    return normalized.slice(idx + 1).toLowerCase();
}

function getFormatExtensions(format: string): string[] {
    const normalized = String(format || '').trim().toLowerCase();
    if (!normalized) {
        return [];
    }
    return FORMAT_EXTENSION_GROUPS[normalized] || [normalized];
}

export function resolveGifAction(mode: string): GifProcessAction {
    if (mode === '倒放') {
        return 'reverse';
    }
    if (mode === '修改帧率') {
        return 'change_speed';
    }
    if (mode === '压缩') {
        return 'compress';
    }
    if (mode === '缩放') {
        return 'resize';
    }
    if (mode === '互转') {
        return 'convert_animation';
    }
    return 'change_speed';
}

export function buildGifProcessSuffix(
    action: GifProcessAction,
    speedPercent: number,
    compressQuality: number,
    resizeWidth = 0,
    resizeHeight = 0,
    convertFormat = 'GIF',
): string {
    if (action === 'reverse') {
        return '_reverse';
    }
    if (action === 'change_speed') {
        return `_speed_${speedPercent}`;
    }
    if (action === 'convert_animation') {
        return `_to_${String(convertFormat || 'GIF').toLowerCase()}`;
    }
    if (action === 'resize') {
        const width = Math.max(0, Math.round(Number(resizeWidth) || 0));
        const height = Math.max(0, Math.round(Number(resizeHeight) || 0));
        return `_resize_${width}x${height}`;
    }
    return `_compress_q${compressQuality}`;
}

export function resolveConverterOverwritePath(inputPath: string, targetFormat: string): string {
    const normalizedPath = normalizePath(inputPath);
    const currentExt = getPathExtension(normalizedPath);
    const targetExtensions = getFormatExtensions(targetFormat);
    if (!normalizedPath || targetExtensions.length === 0) {
        return normalizedPath;
    }
    if (targetExtensions.includes(currentExt)) {
        return normalizedPath;
    }
    const nextExt = targetExtensions[0];
    const lastSlash = normalizedPath.lastIndexOf('/');
    const fileName = lastSlash === -1 ? normalizedPath : normalizedPath.slice(lastSlash + 1);
    const baseDir = lastSlash === -1 ? '' : normalizedPath.slice(0, lastSlash);
    const dotIndex = fileName.lastIndexOf('.');
    const nextFileName = dotIndex > 0 ? `${fileName.slice(0, dotIndex)}.${nextExt}` : `${fileName}.${nextExt}`;
    return baseDir ? `${baseDir}/${nextFileName}` : nextFileName;
}

export function planIcoConversionSizes(selectedSizes: number[], overwriteSource: boolean): number[][] {
    const normalized = Array.isArray(selectedSizes)
        ? Array.from(new Set(selectedSizes.filter((size) => Number.isFinite(size) && size > 0))).sort((a, b) => a - b)
        : [];
    if (normalized.length === 0) {
        return [[]];
    }
    if (overwriteSource) {
        return normalized.map((size) => [size]);
    }
    return [normalized];
}

export function getGifModesForInputKind(kind: GifInputKind): string[] {
    if (kind === 'animated') {
        return [...ANIMATED_MODE_OPTIONS];
    }
    return [...FULL_GIF_MODE_OPTIONS];
}

export function getPreferredGifModeForInputKind(kind: GifInputKind): string {
    if (kind === 'animated') {
        return '互转';
    }
    return '导出';
}

export async function detectAnimatedImagePath(
    inputPath: string,
    probeFrameCount: (inputPath: string) => Promise<number | null>,
): Promise<boolean> {
    const normalizedPath = normalizePath(inputPath);
    const ext = getPathExtension(normalizedPath);
    if (ext === 'gif' || ext === 'apng') {
        return true;
    }
    if (!PROBE_ANIMATED_EXTENSIONS.has(ext)) {
        return false;
    }
    const frameCount = await probeFrameCount(normalizedPath);
    return typeof frameCount === 'number' && frameCount > 1;
}

export async function summarizeGifInputPaths(
    inputPaths: string[],
    probeFrameCount: (inputPath: string) => Promise<number | null>,
): Promise<GifInputSummary> {
    const normalizedPaths = Array.isArray(inputPaths)
        ? inputPaths.map((path) => normalizePath(path)).filter(Boolean)
        : [];

    if (normalizedPaths.length === 0) {
        return {
            kind: 'empty',
            hasAnimated: false,
            hasStatic: false,
            hasGif: false,
            hasNonGifAnimated: false,
        };
    }

    const animatedFlags = await Promise.all(
        normalizedPaths.map((path) => detectAnimatedImagePath(path, probeFrameCount))
    );

    let hasAnimated = false;
    let hasStatic = false;
    let hasGif = false;
    let hasNonGifAnimated = false;

    normalizedPaths.forEach((path, index) => {
        const isAnimated = animatedFlags[index];
        const ext = getPathExtension(path);
        if (isAnimated) {
            hasAnimated = true;
            if (ext === 'gif') {
                hasGif = true;
            } else {
                hasNonGifAnimated = true;
            }
            return;
        }
        hasStatic = true;
    });

    if (hasAnimated && hasStatic) {
        return {
            kind: 'mixed',
            hasAnimated,
            hasStatic,
            hasGif,
            hasNonGifAnimated,
        };
    }

    if (hasAnimated) {
        return {
            kind: hasNonGifAnimated ? 'animated' : 'gif',
            hasAnimated,
            hasStatic,
            hasGif,
            hasNonGifAnimated,
        };
    }

    return {
        kind: 'images',
        hasAnimated,
        hasStatic,
        hasGif,
        hasNonGifAnimated,
    };
}

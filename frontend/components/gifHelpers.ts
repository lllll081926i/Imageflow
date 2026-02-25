export type GifProcessAction = 'reverse' | 'change_speed' | 'compress' | 'resize' | 'convert_animation';

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

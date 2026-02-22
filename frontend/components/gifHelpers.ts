export type GifProcessAction = 'reverse' | 'change_speed' | 'compress';

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
    return 'change_speed';
}

export function buildGifProcessSuffix(
    action: GifProcessAction,
    speedPercent: number,
    compressQuality: number,
): string {
    if (action === 'reverse') {
        return '_reverse';
    }
    if (action === 'change_speed') {
        return `_speed_${speedPercent}`;
    }
    return `_compress_q${compressQuality}`;
}


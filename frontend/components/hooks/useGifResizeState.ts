import { useCallback, useEffect, useMemo, useState } from 'react';
import { getAppBindings } from '../../types/wails-api';

type GifSourceFile = {
    input_path: string;
};

type UseGifResizeStateArgs = {
    featureId: string;
    files: GifSourceFile[];
    isGifPath: (path: string) => boolean;
    normalizePath: (path: string) => string;
};

export function useGifResizeState({
    featureId,
    files,
    isGifPath,
    normalizePath,
}: UseGifResizeStateArgs) {
    const [gifResizeWidth, setGifResizeWidth] = useState(0);
    const [gifResizeHeight, setGifResizeHeight] = useState(0);
    const [gifResizeMaintainAR, setGifResizeMaintainAR] = useState(true);
    const [gifOriginalSize, setGifOriginalSize] = useState({ width: 0, height: 0 });

    const gifReferencePath = useMemo(() => {
        if (featureId !== 'gif') return '';
        const firstGif = (files || []).find((f) => isGifPath(f.input_path));
        return firstGif ? normalizePath(firstGif.input_path) : '';
    }, [featureId, files, isGifPath, normalizePath]);

    const gifAspectRatio = useMemo(() => {
        if (gifOriginalSize.width > 0 && gifOriginalSize.height > 0) {
            return gifOriginalSize.width / gifOriginalSize.height;
        }
        if (gifResizeWidth > 0 && gifResizeHeight > 0) {
            return gifResizeWidth / gifResizeHeight;
        }
        return 0;
    }, [gifOriginalSize.width, gifOriginalSize.height, gifResizeWidth, gifResizeHeight]);

    const onResizeWidthChange = useCallback((value: number) => {
        const nextWidth = Math.max(0, Math.round(Number(value) || 0));
        setGifResizeWidth(nextWidth);
        if (!gifResizeMaintainAR || nextWidth <= 0 || gifAspectRatio <= 0) return;
        setGifResizeHeight(Math.max(1, Math.round(nextWidth / gifAspectRatio)));
    }, [gifResizeMaintainAR, gifAspectRatio]);

    const onResizeHeightChange = useCallback((value: number) => {
        const nextHeight = Math.max(0, Math.round(Number(value) || 0));
        setGifResizeHeight(nextHeight);
        if (!gifResizeMaintainAR || nextHeight <= 0 || gifAspectRatio <= 0) return;
        setGifResizeWidth(Math.max(1, Math.round(nextHeight * gifAspectRatio)));
    }, [gifResizeMaintainAR, gifAspectRatio]);

    useEffect(() => {
        if (!gifReferencePath) {
            setGifOriginalSize({ width: 0, height: 0 });
            setGifResizeWidth(0);
            setGifResizeHeight(0);
            return;
        }
        const app = getAppBindings();
        if (!app?.GetInfo) {
            setGifOriginalSize({ width: 0, height: 0 });
            setGifResizeWidth(0);
            setGifResizeHeight(0);
            return;
        }
        let active = true;
        (async () => {
            try {
                const info = await app.GetInfo({ input_path: gifReferencePath });
                if (!active) return;
                if (info?.success && Number(info.width) > 0 && Number(info.height) > 0) {
                    const width = Math.round(Number(info.width));
                    const height = Math.round(Number(info.height));
                    setGifOriginalSize({ width, height });
                    setGifResizeWidth(width);
                    setGifResizeHeight(height);
                    return;
                }
                setGifOriginalSize({ width: 0, height: 0 });
                setGifResizeWidth(0);
                setGifResizeHeight(0);
            } catch {
                if (!active) return;
                setGifOriginalSize({ width: 0, height: 0 });
                setGifResizeWidth(0);
                setGifResizeHeight(0);
            }
        })();
        return () => {
            active = false;
        };
    }, [gifReferencePath]);

    useEffect(() => {
        if (!gifResizeMaintainAR || gifAspectRatio <= 0) return;
        if (gifResizeWidth > 0) {
            const matchedHeight = Math.max(1, Math.round(gifResizeWidth / gifAspectRatio));
            setGifResizeHeight((prev) => (prev === matchedHeight ? prev : matchedHeight));
            return;
        }
        if (gifResizeHeight > 0) {
            const matchedWidth = Math.max(1, Math.round(gifResizeHeight * gifAspectRatio));
            setGifResizeWidth((prev) => (prev === matchedWidth ? prev : matchedWidth));
        }
    }, [gifResizeMaintainAR, gifAspectRatio, gifResizeWidth, gifResizeHeight]);

    return {
        gifResizeWidth,
        gifResizeHeight,
        gifResizeMaintainAR,
        setGifResizeMaintainAR,
        gifOriginalSize,
        onResizeWidthChange,
        onResizeHeightChange,
    };
}

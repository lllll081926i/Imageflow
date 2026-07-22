import { useEffect, useState } from 'react';
import { getAppBindings } from '../../types/wails-api';

type UseImagePreviewOptions = {
    enabled: boolean;
    path: string;
    debounceMs?: number;
};

export function useImagePreview({ enabled, path, debounceMs = 120 }: UseImagePreviewOptions) {
    const [previewDataUrl, setPreviewDataUrl] = useState('');
    const [previewLoadError, setPreviewLoadError] = useState('');

    useEffect(() => {
        if (!enabled) {
            setPreviewDataUrl('');
            setPreviewLoadError('');
            return;
        }
        if (!path) {
            setPreviewDataUrl('');
            setPreviewLoadError('');
            return;
        }

        let cancelled = false;
        const appAny = getAppBindings();
        if (!appAny?.GetImagePreview) {
            setPreviewDataUrl('');
            setPreviewLoadError('当前环境不支持预览生成');
            return;
        }

        setPreviewLoadError('');
        const timer = window.setTimeout(() => {
            void (async () => {
                try {
                    const res = await appAny.GetImagePreview?.({ input_path: path });
                    if (cancelled) return;
                    if (res?.success && res.data_url) {
                        setPreviewDataUrl(res.data_url);
                        setPreviewLoadError('');
                    } else {
                        setPreviewDataUrl('');
                        const rawError = typeof res?.error === 'string' ? res.error.trim() : '';
                        const msg = rawError === 'PREVIEW_SKIPPED'
                            ? '当前文件暂不支持预览'
                            : (rawError || '预览加载失败');
                        setPreviewLoadError(msg);
                    }
                } catch (err) {
                    if (!cancelled) {
                        setPreviewDataUrl('');
                        const msg = typeof (err as any)?.message === 'string' && (err as any).message.trim()
                            ? (err as any).message.trim()
                            : '预览加载失败';
                        setPreviewLoadError(msg);
                    }
                }
            })();
        }, debounceMs);

        return () => {
            cancelled = true;
            window.clearTimeout(timer);
        };
    }, [debounceMs, enabled, path]);

    return {
        previewDataUrl,
        previewLoadError,
        setPreviewDataUrl,
        setPreviewLoadError,
    };
}

export default useImagePreview;

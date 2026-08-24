import { useEffect, useRef, useState } from 'react';
import { getAppBindings } from '../types/app-api';

export type ThumbnailTarget = {
    input_path: string;
    mod_time?: number;
    size?: number;
};

type CacheEntry = { dataUrl: string; byteSize: number };

const THUMB_CACHE_MAX = 320;
const THUMB_CACHE_MAX_BYTES = 32 * 1024 * 1024;
const THUMB_CACHE_MAX_ENTRY_BYTES = 8 * 1024 * 1024;
const MAX_INFLIGHT = 4;
const cache = new Map<string, CacheEntry>();
const pending = new Map<string, Promise<string | null>>();
const inflight: string[] = [];
const queue: Array<{ key: string; run: () => void }> = [];
let cacheBytes = 0;

type IdleCapableWindow = Window & {
    requestIdleCallback?: (
        callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void,
        options?: { timeout: number },
    ) => number;
    cancelIdleCallback?: (handle: number) => void;
};

function scheduleThumbnailFetch(callback: () => void): () => void {
    const idleWindow = window as IdleCapableWindow;
    if (typeof idleWindow.requestIdleCallback === 'function') {
        const handle = idleWindow.requestIdleCallback(callback, { timeout: 180 });
        return () => idleWindow.cancelIdleCallback?.(handle);
    }
    const handle = window.setTimeout(callback, 32);
    return () => window.clearTimeout(handle);
}

function cacheKey(file: ThumbnailTarget, maxEdge: number): string {
    const mtime = Number(file.mod_time) || 0;
    const size = Number(file.size) || 0;
    return `${file.input_path}|${mtime}|${size}|${maxEdge}`;
}

function estimateDataUrlBytes(dataUrl: string): number {
    const comma = dataUrl.indexOf(',');
    if (comma < 0) return dataUrl.length * 2;
    const payloadLength = Math.max(0, dataUrl.length - comma - 1);
    return Math.ceil(payloadLength * 0.75);
}

function rememberCache(key: string, dataUrl: string) {
    const byteSize = estimateDataUrlBytes(dataUrl);
    if (byteSize > THUMB_CACHE_MAX_ENTRY_BYTES) return;
    const previous = cache.get(key);
    if (previous) {
        cacheBytes -= previous.byteSize;
        cache.delete(key);
    }
    const entry: CacheEntry = { dataUrl, byteSize };
    cache.set(key, entry);
    cacheBytes += byteSize;
    while (cache.size > THUMB_CACHE_MAX || cacheBytes > THUMB_CACHE_MAX_BYTES) {
        const oldest = cache.keys().next().value;
        if (oldest === undefined) break;
        const evicted = cache.get(oldest);
        if (evicted) cacheBytes -= evicted.byteSize;
        cache.delete(oldest);
    }
}

function pumpQueue() {
    while (inflight.length < MAX_INFLIGHT && queue.length > 0) {
        const next = queue.shift();
        if (!next) break;
        inflight.push(next.key);
        next.run();
    }
}

function requestThumbnail(key: string, loader: () => Promise<string | null>): Promise<string | null> {
    const existingPending = pending.get(key);
    if (existingPending) return existingPending;
    const promise = new Promise<string | null>((resolve) => {
        const run = () => {
            Promise.resolve()
                .then(loader)
                .then((value) => resolve(value))
                .catch(() => resolve(null))
                .finally(() => {
                    const index = inflight.indexOf(key);
                    if (index !== -1) inflight.splice(index, 1);
                    pending.delete(key);
                    pumpQueue();
                });
        };
        queue.push({ key, run });
    });
    pending.set(key, promise);
    pumpQueue();
    return promise;
}

// Narrow test seam for the process-wide scheduler. Keeping this here lets the
// concurrency contract be verified without mounting hundreds of React rows.
export const __thumbnailSchedulerForTests = {
    request: requestThumbnail,
    reset() {
        cache.clear();
        cacheBytes = 0;
        pending.clear();
        inflight.splice(0, inflight.length);
        queue.splice(0, queue.length);
    },
};

async function fetchThumbnailDataUrl(path: string, maxEdge: number): Promise<string | null> {
    const bindings = getAppBindings() as any;
    if (!bindings?.GetThumbnail) return null;
    const res = await bindings.GetThumbnail({ path, max_edge: maxEdge });
    if (res?.success && typeof res.data_url === 'string' && res.data_url.startsWith('data:')) {
        return res.data_url;
    }
    return null;
}

/**
 * Fetch a thumbnail for a file row. Returns '' while loading or on failure.
 * Results are cached process-wide and deduplicated across rows.
 */
export function useThumbnail(file: ThumbnailTarget | null, maxEdge = 56): string {
    const key = file ? cacheKey(file, maxEdge) : '';
    const [dataUrl, setDataUrl] = useState<string>(() => (key && cache.get(key)?.dataUrl) || '');
    const currentKeyRef = useRef(key);
    currentKeyRef.current = key;

    useEffect(() => {
        if (!key || !file) {
            setDataUrl('');
            return;
        }
        const cached = cache.get(key);
        if (cached) {
            setDataUrl(cached.dataUrl);
            return;
        }
        let cancelled = false;
        setDataUrl('');
        const path = file.input_path;
        const edge = maxEdge;
        // Thumbnail decoding/bridge traffic is deliberately started after the
        // dropped-file list has painted, preventing the first visible rows from
        // competing with the drop interaction itself.
        const cancelScheduledFetch = scheduleThumbnailFetch(() => {
            if (cancelled) return;
            void requestThumbnail(key, async () => {
                const value = await fetchThumbnailDataUrl(path, edge);
                if (value) rememberCache(key, value);
                return value;
            }).then((value) => {
                if (!cancelled && value && currentKeyRef.current === key) {
                    setDataUrl(value);
                }
            });
        });
        return () => {
            cancelled = true;
            cancelScheduledFetch();
        };
    }, [key, file?.input_path, maxEdge]);

    return dataUrl;
}

/** Fetch one larger preview image for the lightbox. */
export function usePreviewImage(path: string | null, maxEdge = 1024): string {
    const [dataUrl, setDataUrl] = useState('');
    useEffect(() => {
        if (!path) {
            setDataUrl('');
            return;
        }
        const key = `${path}|preview|${maxEdge}`;
        let cancelled = false;
        setDataUrl('');
        // Large previews deliberately bypass the long-lived thumbnail cache:
        // reopening is still fast through the backend disk cache, while an
        // overwritten file at the same path cannot show a stale preview and
        // multiple 1024px base64 images do not occupy the renderer heap.
        void requestThumbnail(key, () => fetchThumbnailDataUrl(path, maxEdge)).then((value) => {
            if (!cancelled && value) setDataUrl(value);
        });
        return () => {
            cancelled = true;
        };
    }, [path, maxEdge]);
    return dataUrl;
}

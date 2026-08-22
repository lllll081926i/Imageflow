import { useEffect, useRef, useState } from 'react';
import { getAppBindings } from '../types/app-api';

export type ThumbnailTarget = {
    input_path: string;
    mod_time?: number;
    size?: number;
};

type CacheEntry = { dataUrl: string };

const THUMB_CACHE_MAX = 600;
const MAX_INFLIGHT = 4;
const cache = new Map<string, CacheEntry>();
const pending = new Map<string, Promise<string | null>>();
const inflight: string[] = [];
const queue: Array<{ key: string; run: () => void }> = [];

function cacheKey(file: ThumbnailTarget, maxEdge: number): string {
    const mtime = Number(file.mod_time) || 0;
    const size = Number(file.size) || 0;
    return `${file.input_path}|${mtime}|${size}|${maxEdge}`;
}

function rememberCache(key: string, entry: CacheEntry) {
    if (cache.has(key)) cache.delete(key);
    cache.set(key, entry);
    if (cache.size > THUMB_CACHE_MAX) {
        const oldest = cache.keys().next().value;
        if (oldest !== undefined) cache.delete(oldest);
    }
}

function pumpQueue() {
    while (inflight.length < MAX_INFLIGHT && queue.length > 0) {
        const next = queue.shift();
        if (!next) break;
        next.run();
    }
}

function requestThumbnail(key: string, loader: () => Promise<string | null>): Promise<string | null> {
    const existingPending = pending.get(key);
    if (existingPending) return existingPending;
    const promise = new Promise<string | null>((resolve) => {
        const run = () => {
            loader()
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
        void requestThumbnail(key, async () => {
            const value = await fetchThumbnailDataUrl(path, edge);
            if (value) rememberCache(key, { dataUrl: value });
            return value;
        }).then((value) => {
            if (!cancelled && value && currentKeyRef.current === key) {
                setDataUrl(value);
            }
        });
        return () => {
            cancelled = true;
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
        const cached = cache.get(key);
        if (cached) {
            setDataUrl(cached.dataUrl);
            return;
        }
        let cancelled = false;
        setDataUrl('');
        void requestThumbnail(key, async () => {
            const value = await fetchThumbnailDataUrl(path, maxEdge);
            if (value) rememberCache(key, { dataUrl: value });
            return value;
        }).then((value) => {
            if (!cancelled && value) setDataUrl(value);
        });
        return () => {
            cancelled = true;
        };
    }, [path, maxEdge]);
    return dataUrl;
}

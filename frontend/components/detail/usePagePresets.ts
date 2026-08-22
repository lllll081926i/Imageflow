import { useCallback, useEffect, useMemo, useState } from 'react';

export type PagePreset<T = Record<string, unknown>> = {
    id: string;
    name: string;
    updated_at: number;
    payload: T;
};

const STORAGE_PREFIX = 'imageflow:page-presets:v2:';

function readStore<T>(scope: string): PagePreset<T>[] {
    try {
        const raw = window.localStorage?.getItem(STORAGE_PREFIX + scope);
        if (!raw) return [];
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return [];
        return parsed
            .filter((item: any) => item && typeof item === 'object' && typeof item.name === 'string')
            .map((item: any, index: number) => ({
                id: typeof item.id === 'string' ? item.id : `${scope}-${index}-${Date.now()}`,
                name: item.name,
                updated_at: Number(item.updated_at) || Date.now(),
                payload: (item.payload && typeof item.payload === 'object' ? item.payload : {}) as T,
            }))
            .sort((a, b) => b.updated_at - a.updated_at);
    } catch {
        return [];
    }
}

function writeStore<T>(scope: string, presets: PagePreset<T>[]) {
    try {
        window.localStorage?.setItem(STORAGE_PREFIX + scope, JSON.stringify(presets));
    } catch {
        // ignore storage failures
    }
}

/**
 * Page-scoped preset store. Every feature page that wants "save current
 * settings / apply / delete" gets one of these with its own localStorage scope.
 */
export function usePagePresets<T extends Record<string, unknown>>(scope: string) {
    const [presets, setPresets] = useState<PagePreset<T>[]>(() => readStore<T>(scope));
    const [activeId, setActiveId] = useState('');

    useEffect(() => {
        setPresets(readStore<T>(scope));
        setActiveId('');
    }, [scope]);

    /** Save (insert or update-by-name) and return the stored entry. */
    const save = useCallback((name: string, payload: T): PagePreset<T> => {
        const trimmed = name.trim() || `预设 ${new Date().toLocaleString()}`;
        const current = readStore<T>(scope);
        const existing = current.find((p) => p.name === trimmed);
        const entry: PagePreset<T> = existing
            ? { ...existing, payload, updated_at: Date.now() }
            : {
                id: `${scope}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
                name: trimmed,
                updated_at: Date.now(),
                payload,
            };
        const next = [entry, ...current.filter((p) => p.id !== entry.id)];
        writeStore(scope, next);
        setPresets(next);
        setActiveId(entry.id);
        return entry;
    }, [scope]);

    const remove = useCallback((id: string) => {
        setPresets((prev) => {
            const next = prev.filter((p) => p.id !== id);
            writeStore(scope, next);
            return next;
        });
        setActiveId((current) => (current === id ? '' : current));
    }, [scope]);

    const apply = useCallback((id: string): T | null => {
        const found = readStore<T>(scope).find((p) => p.id === id);
        return found ? found.payload : null;
    }, [scope]);

    const sorted = useMemo(() => presets, [presets]);

    return { presets: sorted, activeId, setActiveId, save, remove, apply };
}

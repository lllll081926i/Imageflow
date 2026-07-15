export type DroppedFile = {
    input_path: string;
    source_root: string;
    relative_path: string;
    is_from_dir_drop: boolean;
};

export type ExpandDroppedPathsResult = {
    files: DroppedFile[];
    has_directory: boolean;
};

export type OutputSettings = {
    output_prefix: string;
    output_template: string;
    preserve_folder_structure: boolean;
    conflict_strategy: string;
};

export type FeaturePreset = {
    id: string;
    name: string;
    feature_id: string;
    created_at: number;
    updated_at: number;
    payload: Record<string, any>;
};

export type FeaturePresetStore = Record<string, FeaturePreset[]>;

export const defaultOutputSettings: OutputSettings = {
    output_prefix: 'IF',
    output_template: '{prefix}{basename}',
    preserve_folder_structure: true,
    conflict_strategy: 'rename',
};

export const FEATURE_PRESETS_STORAGE_KEY = 'imageflow:feature-presets:v1';

export function loadFeaturePresetStore(): FeaturePresetStore {
    try {
        const raw = window.localStorage?.getItem(FEATURE_PRESETS_STORAGE_KEY);
        if (!raw) return {};
        const parsed = JSON.parse(raw);
        if (!parsed || typeof parsed !== 'object') return {};
        const store: FeaturePresetStore = {};
        Object.keys(parsed).forEach((featureId) => {
            const items = Array.isArray((parsed as any)[featureId]) ? (parsed as any)[featureId] : [];
            store[featureId] = items
                .map((item: any) => ({
                    id: typeof item?.id === 'string' ? item.id : `${featureId}-${Date.now()}`,
                    name: typeof item?.name === 'string' && item.name.trim() ? item.name.trim() : '未命名预设',
                    feature_id: featureId,
                    created_at: Number(item?.created_at) || Date.now(),
                    updated_at: Number(item?.updated_at) || Date.now(),
                    payload: item?.payload && typeof item.payload === 'object' ? item.payload : {},
                }))
                .sort((a: FeaturePreset, b: FeaturePreset) => b.updated_at - a.updated_at);
        });
        return store;
    } catch {
        return {};
    }
}

export function saveFeaturePresetStore(store: FeaturePresetStore) {
    try {
        window.localStorage?.setItem(FEATURE_PRESETS_STORAGE_KEY, JSON.stringify(store));
    } catch {
        // ignore storage failures
    }
}

export function normalizePath(p: string) {
    return p.replace(/\\/g, '/').replace(/\/+$/, '');
}

export function dirname(p: string) {
    const normalized = p.replace(/\\/g, '/');
    const idx = normalized.lastIndexOf('/');
    if (idx <= 0) return '';
    return normalized.slice(0, idx);
}

export function joinPath(...parts: string[]) {
    return parts
        .filter(Boolean)
        .map((part, index) => {
            const normalized = part.replace(/\\/g, '/');
            if (index === 0) return normalized.replace(/\/+$/, '');
            return normalized.replace(/^\/+|\/+$/g, '');
        })
        .filter(Boolean)
        .join('/');
}

export function basename(p: string) {
    const normalized = p.replace(/\\/g, '/');
    const idx = normalized.lastIndexOf('/');
    return idx >= 0 ? normalized.slice(idx + 1) : normalized;
}

export function stripExtension(name: string) {
    const idx = name.lastIndexOf('.');
    if (idx <= 0) return name;
    return name.slice(0, idx);
}

export function extname(name: string) {
    const idx = name.lastIndexOf('.');
    if (idx <= 0) return '';
    return name.slice(idx + 1);
}

export function clampNumber(value: number, min: number, max: number) {
    return Math.max(min, Math.min(max, value));
}

export function getBatchChunkSize(itemCount: number, requestsPerItem = 1) {
    const safeRequestsPerItem = Math.max(1, Math.round(Number(requestsPerItem) || 1));
    const base = Math.max(1, Math.floor(64 / safeRequestsPerItem));
    return Math.max(1, Math.min(itemCount, base));
}

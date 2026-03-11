import type * as AppModule from '../wailsjs/go/main/App';

export type AppBindings = typeof AppModule;

export type AppSettingsSnapshot = {
    max_concurrency: number;
    output_prefix: string;
    output_template: string;
    preserve_folder_structure: boolean;
    conflict_strategy: string;
    default_output_dir: string;
    recent_input_dirs: string[];
    recent_output_dirs: string[];
};

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));
const MAX_RECENT_PATHS = 4;

export const DEFAULT_APP_SETTINGS: AppSettingsSnapshot = {
    max_concurrency: 8,
    output_prefix: 'IF',
    output_template: '{prefix}{basename}',
    preserve_folder_structure: true,
    conflict_strategy: 'rename',
    default_output_dir: '',
    recent_input_dirs: [],
    recent_output_dirs: [],
};

const normalizeSavedPath = (value: unknown) => {
    if (typeof value !== 'string') return '';
    const trimmed = value.trim();
    if (!trimmed) return '';
    if (trimmed === '/' || trimmed === '\\') return trimmed;
    const normalized = trimmed.replace(/[\\/]+$/, '');
    if (/^[A-Za-z]:$/.test(normalized)) {
        return trimmed;
    }
    return normalized || trimmed;
};

const normalizeRecentPaths = (value: unknown) => {
    if (!Array.isArray(value)) return [] as string[];
    const seen = new Set<string>();
    const paths: string[] = [];
    for (const item of value) {
        const path = normalizeSavedPath(item);
        if (!path) continue;
        const key = path.toLowerCase();
        if (seen.has(key)) continue;
        seen.add(key);
        paths.push(path);
        if (paths.length >= MAX_RECENT_PATHS) break;
    }
    return paths;
};

export function normalizeAppSettings(raw?: Partial<AppSettingsSnapshot> | null): AppSettingsSnapshot {
    if (!raw) return { ...DEFAULT_APP_SETTINGS };
    return {
        max_concurrency: clamp(Number(raw.max_concurrency || DEFAULT_APP_SETTINGS.max_concurrency), 1, 32),
        output_prefix: typeof raw.output_prefix === 'string' && raw.output_prefix.trim()
            ? raw.output_prefix
            : DEFAULT_APP_SETTINGS.output_prefix,
        output_template: typeof raw.output_template === 'string' && raw.output_template.trim()
            ? raw.output_template
            : DEFAULT_APP_SETTINGS.output_template,
        preserve_folder_structure: typeof raw.preserve_folder_structure === 'boolean'
            ? raw.preserve_folder_structure
            : DEFAULT_APP_SETTINGS.preserve_folder_structure,
        conflict_strategy: typeof raw.conflict_strategy === 'string' && raw.conflict_strategy.trim() === 'rename'
            ? 'rename'
            : DEFAULT_APP_SETTINGS.conflict_strategy,
        default_output_dir: normalizeSavedPath(raw.default_output_dir),
        recent_input_dirs: normalizeRecentPaths(raw.recent_input_dirs),
        recent_output_dirs: normalizeRecentPaths(raw.recent_output_dirs),
    };
}

export async function loadAppSettings(options?: { throwOnError?: boolean }): Promise<AppSettingsSnapshot> {
    const app = getAppBindings();
    if (!app?.GetSettings) {
        return { ...DEFAULT_APP_SETTINGS };
    }
    try {
        const settings = await app.GetSettings();
        return normalizeAppSettings(settings as Partial<AppSettingsSnapshot>);
    } catch (error) {
        console.error(error);
        if (options?.throwOnError) {
            throw error;
        }
        return { ...DEFAULT_APP_SETTINGS };
    }
}

export async function saveAppSettings(settings: AppSettingsSnapshot): Promise<AppSettingsSnapshot> {
    const app = getAppBindings();
    const normalized = normalizeAppSettings(settings);
    if (!app?.SaveSettings) {
        return normalized;
    }
    const saved = await app.SaveSettings(normalized as any);
    return normalizeAppSettings((saved as Partial<AppSettingsSnapshot>) || normalized);
}

export function pushRecentPath(paths: string[], nextPath: string): string[] {
    const normalized = normalizeSavedPath(nextPath);
    if (!normalized) return normalizeRecentPaths(paths);
    return normalizeRecentPaths([normalized, ...paths]);
}

export async function updateRecentPaths(payload: { inputDir?: string; outputDir?: string }): Promise<AppSettingsSnapshot> {
    const app = getAppBindings();
    if (!app?.UpdateRecentPaths) {
        return loadAppSettings();
    }
    const saved = await app.UpdateRecentPaths({
        input_dir: payload.inputDir || '',
        output_dir: payload.outputDir || '',
    } as any);
    return normalizeAppSettings(saved as Partial<AppSettingsSnapshot>);
}

export function getAppBindings(): Partial<AppBindings> | null {
    const app = window.go?.main?.App;
    if (!app) return null;
    return app as Partial<AppBindings>;
}

import type { DroppedFile } from './detailTypes';
import { defaultOutputSettings } from './detailTypes';

export const normalizePath = (p: string) => (p || '').replace(/\\/g, '/').replace(/\/+$/, '');

export const basename = (p: string) => normalizePath(p).split('/').pop() || p;

export const extname = (p: string) => {
    const normalized = normalizePath(p);
    const idx = normalized.lastIndexOf('.');
    if (idx === -1) return '';
    return normalized.slice(idx + 1).toLowerCase();
};

export const dirname = (p: string) => {
    const normalized = normalizePath(p);
    const idx = normalized.lastIndexOf('/');
    if (idx === -1) return '';
    return normalized.slice(0, idx);
};

export const joinPath = (base: string, rel: string) =>
    `${normalizePath(base)}/${rel.replace(/^\/+/, '')}`;

export const pathLookupKey = (path: string) => normalizePath(path).toLowerCase();

export const sanitizeFileName = (name: string) =>
    (name || '').trim().replace(/[\\/:*?"<>|]+/g, '_');

export const stripExtension = (name: string) => {
    const idx = name.lastIndexOf('.');
    if (idx <= 0) return name;
    return name.slice(0, idx);
};

export const sanitizeOutputName = (name: string) => {
    const cleaned = sanitizeFileName(name).replace(/[. ]+$/g, '').trim();
    if (!cleaned) return '';
    const upper = cleaned.toUpperCase();
    const reserved = new Set([
        'CON', 'PRN', 'AUX', 'NUL',
        'COM1', 'COM2', 'COM3', 'COM4', 'COM5', 'COM6', 'COM7', 'COM8', 'COM9',
        'LPT1', 'LPT2', 'LPT3', 'LPT4', 'LPT5', 'LPT6', 'LPT7', 'LPT8', 'LPT9',
    ]);
    if (reserved.has(upper)) {
        return `_${cleaned}`;
    }
    return cleaned;
};

export const formatDatePart = (date: Date, pattern: string) => {
    const yyyy = `${date.getFullYear()}`;
    const MM = `${date.getMonth() + 1}`.padStart(2, '0');
    const dd = `${date.getDate()}`.padStart(2, '0');
    const HH = `${date.getHours()}`.padStart(2, '0');
    const mm = `${date.getMinutes()}`.padStart(2, '0');
    const ss = `${date.getSeconds()}`.padStart(2, '0');
    return pattern
        .replace(/YYYY/g, yyyy)
        .replace(/MM/g, MM)
        .replace(/DD/g, dd)
        .replace(/HH/g, HH)
        .replace(/mm/g, mm)
        .replace(/ss/g, ss);
};

export const applyOutputTemplate = (
    template: string,
    vars: {
        basename: string;
        prefix: string;
        seq: number;
        op: string;
        date: Date;
    },
) =>
    template.replace(/\{([a-z_]+)(?::([^}]+))?\}/gi, (_match, keyRaw, fmtRaw) => {
        const key = String(keyRaw).toLowerCase();
        const fmt = typeof fmtRaw === 'string' ? fmtRaw : '';
        if (key === 'basename') return vars.basename;
        if (key === 'prefix') return vars.prefix;
        if (key === 'op') return vars.op;
        if (key === 'date') {
            const pattern = fmt || 'YYYYMMDD';
            return formatDatePart(vars.date, pattern);
        }
        if (key === 'time') {
            const pattern = fmt || 'HHmmss';
            return formatDatePart(vars.date, pattern);
        }
        if (key === 'seq') {
            const width = /^\d+$/.test(fmt) ? Number(fmt) : 0;
            const text = String(vars.seq);
            return width > 0 ? text.padStart(width, '0') : text;
        }
        return '';
    });

export const buildOutputName = (
    baseName: string,
    options: {
        template: string;
        prefix: string;
        seq: number;
        op: string;
        date: Date;
    },
) => {
    const template =
        (options.template || defaultOutputSettings.output_template).trim() ||
        defaultOutputSettings.output_template;
    const prefixClean = sanitizeFileName(options.prefix || '');
    const prefixValue = prefixClean ? `${prefixClean}_` : '';
    const templateHasPrefix = /\{prefix(?:[:}])/i.test(template);
    const rendered = applyOutputTemplate(template, {
        basename: baseName,
        prefix: prefixValue,
        seq: options.seq,
        op: options.op,
        date: options.date,
    }).trim();
    let name = rendered || baseName;
    if (!templateHasPrefix && prefixValue) {
        name = `${prefixValue}${name}`;
    }
    name = sanitizeOutputName(name);
    if (!name) {
        name = sanitizeOutputName(baseName) || 'output';
    }
    if (name.length > 200) {
        name = name.slice(0, 200);
    }
    return name;
};

export const getRelPath = (file: DroppedFile, preserveStructure: boolean) => {
    if (preserveStructure && file.is_from_dir_drop) {
        return file.relative_path || basename(file.input_path);
    }
    return basename(file.input_path);
};

export const getRelDir = (relPath: string) => {
    const normalized = normalizePath(relPath);
    const idx = normalized.lastIndexOf('/');
    if (idx === -1) return '';
    return normalized.slice(0, idx);
};

export const buildOutputRelPath = (
    file: DroppedFile,
    options: {
        ext?: string;
        suffix?: string;
        seq: number;
        op: string;
        template: string;
        prefix: string;
        preserveStructure: boolean;
        date: Date;
    },
) => {
    const rel = getRelPath(file, options.preserveStructure).replace(/\\/g, '/');
    const relDir = getRelDir(rel);
    const fileName = basename(rel);
    let baseName = stripExtension(fileName);
    const suffix = sanitizeFileName(options.suffix || '');
    if (suffix.startsWith('_ico')) {
        baseName = baseName.replace(/_ico\d+(?:-\d+)*$/i, '');
    }
    const ext = (options.ext === undefined ? extname(fileName) : options.ext).toLowerCase();
    let name = buildOutputName(baseName, {
        template: options.template,
        prefix: options.prefix,
        seq: options.seq,
        op: options.op,
        date: options.date,
    });
    if (suffix && !name.endsWith(suffix)) {
        const maxBaseLen = Math.max(1, 200 - suffix.length);
        if (name.length > maxBaseLen) {
            name = name.slice(0, maxBaseLen);
        }
        name = sanitizeOutputName(`${name}${suffix}`) || `${name}${suffix}`;
    }
    const fullName = ext ? `${name}.${ext}` : name;
    return relDir ? `${relDir}/${fullName}` : fullName;
};

export const sanitizePdfInputName = (name: string) => sanitizeFileName(stripExtension(name || ''));

export const normalizePdfFileName = (name: string) => {
    const cleaned = sanitizeFileName(stripExtension(name || ''));
    return cleaned || 'document';
};

export const buildSuggestedPdfName = (list: DroppedFile[], hasDirectory?: boolean) => {
    if (!list.length) return 'document';
    const baseFromPath = (p: string) => stripExtension(basename(p));
    let name = '';
    if (hasDirectory && list[0]?.source_root) {
        name = baseFromPath(list[0].source_root);
    }
    const baseNames = list.map((f) => baseFromPath(f.input_path)).filter(Boolean);
    if (!name && baseNames.length > 0) {
        if (baseNames.length === 1) {
            name = baseNames[0];
        } else {
            let prefix = baseNames[0];
            for (const item of baseNames.slice(1)) {
                while (prefix && !item.startsWith(prefix)) {
                    prefix = prefix.slice(0, -1);
                }
                if (!prefix) break;
            }
            const trimmed = prefix.replace(/[_\-\s.]+$/g, '').trim();
            name = trimmed.length >= 3 ? trimmed : baseNames[0];
            if (!hasDirectory) {
                name = `${name}_merged`;
            }
        }
    }
    return normalizePdfFileName(name);
};

export const dedupeDroppedFiles = (files: DroppedFile[]) => {
    const seen = new Set<string>();
    const next: DroppedFile[] = [];
    files.forEach((file) => {
        const normalizedInput = normalizePath(file.input_path || '');
        if (!normalizedInput) return;
        const key = normalizedInput.toLowerCase();
        if (seen.has(key)) return;
        seen.add(key);
        next.push({
            ...file,
            input_path: normalizedInput,
            source_root: normalizePath(file.source_root || ''),
            relative_path: (file.relative_path || '').replace(/\\/g, '/'),
        });
    });
    return next;
};

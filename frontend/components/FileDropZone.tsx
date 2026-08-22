import React, { useState, useRef, useEffect, useMemo, useCallback, useDeferredValue, useId, useLayoutEffect, memo, startTransition } from 'react';
import VirtualList from './VirtualList';
import { createPortal } from 'react-dom';
import Icon from './Icon';
import { getAppBindings, resolveSelectedFilePaths } from '../types/app-api';
import { useThumbnail, usePreviewImage } from './thumbnails';

// --- FileDropZone ---

interface FileDropZoneProps {
    onFilesSelected?: (files: File[]) => void;
    onPathsExpanded?: (result: ExpandDroppedPathsResult) => void;
    onItemSelect?: (file: DroppedFile) => void;
    selectedPath?: string;
    acceptedFormats?: string;
    fileDialogFilters?: FileDialogFilter[];
    allowMultiple?: boolean;
    title?: string;
    subTitle?: string;
    isActive?: boolean;
}

type FileDialogFilter = {
    DisplayName: string;
    Pattern: string;
};

const DEFAULT_IMAGE_FILE_DIALOG_FILTERS: FileDialogFilter[] = [{
    DisplayName: "Images",
    Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.gif;*.bmp;*.avif;*.heic;*.heif;*.ico;*.tiff;*.tif;*.svg",
}];

type DroppedFile = {
    input_path: string;
    source_root: string;
    relative_path: string;
    is_from_dir_drop: boolean;
    size: number;
    mod_time: number;
};

type SortKey = 'name' | 'time' | 'size';
type SortOrder = 'asc' | 'desc';

type ExpandDroppedPathsResult = {
    files: DroppedFile[];
    has_directory: boolean;
};

type FileTreeNode = {
    name: string;
    path: string;
    isFolder: true;
    children: Record<string, FileTreeNode>;
    files: DroppedFile[];
    itemCount?: number;
    initiallyOpen?: boolean;
    sortedChildren?: FileTreeNode[];
    sortedFiles?: DroppedFile[];
};

type FileTreeRoot = Record<string, FileTreeNode>;
type RuntimeDropMarker = {
    signature: string;
    expiresAt: number;
};

const normalizePath = (p: string) => p.replace(/\\/g, '/');
const pathKey = (p: string) => normalizePath(p).toLowerCase();
const basename = (p: string) => normalizePath(p).split('/').pop() || p;
const getPathExtension = (p: string) => {
    const name = basename(p);
    const idx = name.lastIndexOf('.');
    return idx === -1 ? '' : name.slice(idx).toLowerCase();
};
const parseAcceptedExtensionSet = (acceptedFormats: string) => {
    const parts = String(acceptedFormats || '')
        .split(',')
        .map((part) => part.trim().toLowerCase())
        .filter(Boolean);
    if (parts.length === 0 || parts.some((part) => part.includes('/') || part.includes('*'))) {
        return null;
    }
    const extensions = parts
        .filter((part) => part.startsWith('.'))
        .map((part) => part.replace(/\s+/g, ''));
    return extensions.length > 0 ? new Set(extensions) : null;
};
const filterExpandedResultByExtensions = (
    result: ExpandDroppedPathsResult,
    acceptedExtensions: Set<string> | null,
): ExpandDroppedPathsResult => {
    if (!acceptedExtensions || acceptedExtensions.size === 0) {
        return result;
    }
    return {
        ...result,
        files: result.files.filter((file) => {
            const ext = getPathExtension(file.input_path || file.relative_path || '');
            return acceptedExtensions.has(ext);
        }),
    };
};
const getDirectDropPath = (file: File) => {
    const candidate = file as File & { path?: string | null; pywebviewFullPath?: string | null };
    if (typeof candidate.path === 'string' && candidate.path.trim()) {
        return candidate.path.trim();
    }
    if (typeof candidate.pywebviewFullPath === 'string' && candidate.pywebviewFullPath.trim()) {
        return candidate.pywebviewFullPath.trim();
    }
    return '';
};
const buildRuntimeDropSignature = (paths: string[]) => paths.map(pathKey).sort().join('|');
const compareNames = (a: string, b: string) =>
    a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' });
const getRelativeName = (file: DroppedFile) => {
    const cleaned = normalizePath(file.relative_path);
    return cleaned.split('/').pop() || basename(file.input_path);
};
const sortFiles = (files: DroppedFile[], sortKey: SortKey, sortOrder: SortOrder) => {
    const sorted = files.slice().sort((a, b) => {
        let res = 0;
        if (sortKey === 'name') {
            res = compareNames(getRelativeName(a), getRelativeName(b));
        } else if (sortKey === 'size') {
            res = (a.size || 0) - (b.size || 0);
        } else if (sortKey === 'time') {
            res = (a.mod_time || 0) - (b.mod_time || 0);
        }
        return sortOrder === 'asc' ? res : -res;
    });
    return sorted;
};
const buildFileTree = (files: DroppedFile[]) => {
    const root: FileTreeRoot = {};

    files.forEach((file) => {
        if (!file.is_from_dir_drop) return;

        const relPath = file.relative_path.replace(/\\/g, '/');
        const parts = relPath.split('/');

        const topKey = file.source_root;
        if (!root[topKey]) {
            root[topKey] = {
                name: basename(topKey),
                path: topKey,
                isFolder: true,
                children: {},
                files: []
            };
        }

        let currentLevel = root[topKey];

        for (let i = 0; i < parts.length - 1; i++) {
            const part = parts[i];
            if (!currentLevel.children[part]) {
                currentLevel.children[part] = {
                    name: part,
                    path: `${currentLevel.path}/${part}`,
                    isFolder: true,
                    children: {},
                    files: []
                };
            }
            currentLevel = currentLevel.children[part];
        }

        currentLevel.files.push(file);
    });

    return root;
};

const ROOT_AUTO_EXPAND_LIMIT = 40;
const CHILD_AUTO_EXPAND_LIMIT = 24;

const sortTreeNode = (node: FileTreeNode, sortKey: SortKey, sortOrder: SortOrder, depth = 0) => {
    const sortedChildren = Object.values(node.children || {});
    sortedChildren.forEach((child) => sortTreeNode(child, sortKey, sortOrder, depth + 1));
    sortedChildren.sort((a, b) => {
        const res = compareNames(a.name, b.name);
        return sortOrder === 'asc' ? res : -res;
    });
    node.sortedChildren = sortedChildren;
    node.sortedFiles = sortFiles(node.files || [], sortKey, sortOrder);
    node.itemCount = node.sortedFiles.length + sortedChildren.reduce((sum, child) => sum + (child.itemCount || 0), 0);
    const autoExpandLimit = depth === 0 ? ROOT_AUTO_EXPAND_LIMIT : CHILD_AUTO_EXPAND_LIMIT;
    node.initiallyOpen = (node.itemCount || 0) <= autoExpandLimit;
};
const buildSortedFileTree = (files: DroppedFile[], sortKey: SortKey, sortOrder: SortOrder) => {
    const root = buildFileTree(files);
    Object.values(root).forEach((node) => sortTreeNode(node, sortKey, sortOrder));
    return root;
};

type TreeNodeProps = {
    node: FileTreeNode;
    depth: number;
    selectedKey: string;
    onItemSelect?: (file: DroppedFile) => void;
    onRemoveFolder: (path: string, e: React.MouseEvent<HTMLButtonElement>) => void;
    onRemoveFile: (path: string, e: React.MouseEvent<HTMLButtonElement>) => void;
    onOpenPreview: (file: DroppedFile) => void;
};

type FileThumbProps = {
    file: DroppedFile;
    onOpenPreview: (file: DroppedFile) => void;
};

const FileThumb: React.FC<FileThumbProps> = ({ file, onOpenPreview }) => {
    const dataUrl = useThumbnail(file, 56);
    const [loaded, setLoaded] = useState(false);
    useEffect(() => {
        if (!dataUrl) setLoaded(false);
    }, [dataUrl]);
    return (
        <button
            type="button"
            title="预览"
            onClick={(e) => {
                e.stopPropagation();
                onOpenPreview(file);
            }}
            className="w-7 h-7 shrink-0 rounded-md overflow-hidden bg-gray-100 dark:bg-white/10 flex items-center justify-center hover:ring-2 hover:ring-[#007AFF]/40 transition-all duration-200 hover:scale-110"
        >
            {dataUrl ? (
                <img
                    src={dataUrl}
                    alt=""
                    draggable={false}
                    onLoad={() => setLoaded(true)}
                    className={`w-full h-full object-cover transition-opacity duration-300 ${loaded ? 'opacity-100' : 'opacity-0'}`}
                />
            ) : (
                <div className="w-full h-full thumb-shimmer" />
            )}
        </button>
    );
};

const PreviewOverlay: React.FC<{
    file: DroppedFile | null;
    onClose: () => void;
}> = ({ file, onClose }) => {
    const path = file?.input_path || null;
    const dataUrl = usePreviewImage(path, 1024);
    const [exiting, setExiting] = useState(false);
    const exitingRef = useRef(false);

    useEffect(() => {
        if (!path) {
            exitingRef.current = false;
            setExiting(false);
            return;
        }
        const handler = (e: KeyboardEvent) => {
            if (e.key === 'Escape') requestClose();
        };
        window.addEventListener('keydown', handler);
        return () => window.removeEventListener('keydown', handler);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [path]);

    const requestClose = () => {
        if (exitingRef.current) return;
        exitingRef.current = true;
        setExiting(true);
        window.setTimeout(onClose, 170);
    };

    if (!file) return null;
    return createPortal(
        <div
            role="dialog"
            aria-modal="true"
            onClick={requestClose}
            className={`fixed inset-0 z-[80] flex items-center justify-center bg-black/70 p-8 transition-opacity duration-150 ${exiting ? 'opacity-0' : 'opacity-100'}`}
        >
            <div
                onClick={(e) => e.stopPropagation()}
                className={`spring-pop material-overlay max-w-full max-h-full rounded-2xl overflow-hidden shadow-2xl ${exiting ? 'overlay-exit' : ''}`}
            >
                {dataUrl ? (
                    <img src={dataUrl} alt={file.relative_path || ''} className="max-w-[80vw] max-h-[72vh] object-contain block" draggable={false} />
                ) : (
                    <div className="w-64 h-64 flex items-center justify-center text-sm text-gray-400">
                        加载中...
                    </div>
                )}
                <div className="px-4 py-2.5 text-xs text-gray-500 dark:text-gray-400 truncate">
                    {file.relative_path || ''}
                </div>
            </div>
        </div>,
        document.body,
    );
};

const TreeNode: React.FC<TreeNodeProps> = memo(({
    node,
    depth,
    selectedKey,
    onItemSelect,
    onRemoveFolder,
    onRemoveFile,
    onOpenPreview,
}) => {
    const [open, setOpen] = useState(() => node.initiallyOpen ?? true);
    const [shouldRenderChildren, setShouldRenderChildren] = useState(() => node.initiallyOpen ?? true);
    const previousAutoStateRef = useRef({
        path: node.path,
        itemCount: node.itemCount || 0,
    });

    useEffect(() => {
        const previous = previousAutoStateRef.current;
        const itemCount = node.itemCount || 0;
        const shouldResetOpenState = previous.path !== node.path
            || (previous.itemCount !== itemCount && node.initiallyOpen === false);
        if (shouldResetOpenState) {
            const nextOpen = node.initiallyOpen ?? true;
            setOpen(nextOpen);
            setShouldRenderChildren(nextOpen);
        }
        previousAutoStateRef.current = { path: node.path, itemCount };
    }, [node.initiallyOpen, node.itemCount, node.path]);

    useEffect(() => {
        if (open) {
            setShouldRenderChildren(true);
            return;
        }

        const timer = window.setTimeout(() => {
            setShouldRenderChildren(false);
        }, 200);

        return () => window.clearTimeout(timer);
    }, [open]);

    const paddingLeft = 16 + (depth * 12);
    const sortedChildren = node.sortedChildren || [];
    const sortedFiles = node.sortedFiles || [];

    return (
        <div className="border-b border-gray-200/60 dark:border-white/10 last:border-0">
            <div
                role="button"
                tabIndex={0}
                onClick={(e) => {
                    e.stopPropagation();
                    setOpen((prev) => !prev);
                }}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        e.stopPropagation();
                        setOpen((prev) => !prev);
                    }
                }}
                className="w-full py-3 flex items-center gap-3 hover:bg-black/5 dark:hover:bg-white/5 transition-colors pr-4"
                style={{ paddingLeft }}
            >
                <div className={`transition-transform duration-200 ${open ? 'rotate-0' : '-rotate-90'}`}>
                    <Icon name="ChevronDown" size={16} className="text-gray-400" />
                </div>
                <div className="flex-1 min-w-0 text-left">
                    <div className="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">{node.name}</div>
                    <div className="text-[11px] text-gray-500 dark:text-gray-400">
                        {node.itemCount || 0} 项, {sortedChildren.length} 文件夹
                    </div>
                </div>
                {depth === 0 && (
                    <button
                        onClick={(e) => onRemoveFolder(node.path, e)}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 transition-colors"
                    >
                        <Icon name="Trash2" size={14} />
                    </button>
                )}
            </div>

            <div
                className={`grid overflow-hidden transition-[grid-template-rows,opacity] duration-200 ease-[cubic-bezier(0.2,0.8,0.2,1)] ${
                    open ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0'
                }`}
            >
                <div className="min-h-0">
                    {shouldRenderChildren && (
                        <div>
                            {sortedChildren.map((child) => (
                                <TreeNode
                                    key={child.path}
                                    node={child}
                                    depth={depth + 1}
                                    selectedKey={selectedKey}
                                    onItemSelect={onItemSelect}
                                    onRemoveFolder={onRemoveFolder}
                                    onRemoveFile={onRemoveFile}
                                    onOpenPreview={onOpenPreview}
                                />
                            ))}

                            <VirtualList
                                items={sortedFiles}
                                itemHeight={40}
                                height={Math.min(360, Math.max(160, sortedFiles.length * 40))}
                                getKey={(f) => f.input_path}
                                renderItem={(f) => {
                                    const name = getRelativeName(f);
                                    const isSelected = selectedKey && normalizePath(f.input_path) === selectedKey;
                                    return (
                                        <div
                                            onClick={() => onItemSelect?.(f)}
                                            onKeyDown={(e) => {
                                                if (!onItemSelect) return;
                                                if (e.key === 'Enter' || e.key === ' ') {
                                                    e.preventDefault();
                                                    onItemSelect(f);
                                                }
                                            }}
                                            role={onItemSelect ? 'button' : undefined}
                                            tabIndex={onItemSelect ? 0 : -1}
                                            aria-selected={Boolean(isSelected)}
                                            className={`py-2 flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300 group/file pr-9 ${
                                                isSelected
                                                    ? 'bg-[#007AFF]/10 dark:bg-[#0A84FF]/15'
                                                    : 'hover:bg-black/5 dark:hover:bg-white/5'
                                            } ${onItemSelect ? 'cursor-pointer' : ''}`}
                                            style={{ paddingLeft: paddingLeft + 24 }}
                                        >
                                            <FileThumb file={f} onOpenPreview={onOpenPreview} />
                                            <div className="flex-1 min-w-0 truncate">{name}</div>
                                            <button
                                                onClick={(e) => onRemoveFile(f.input_path, e)}
                                                className="opacity-0 group-hover/file:opacity-100 p-1 rounded-md text-gray-400 hover:text-red-500 transition-all"
                                            >
                                                <Icon name="X" size={14} />
                                            </button>
                                        </div>
                                    );
                                }}
                            />
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
});

const FileDropZoneImpl: React.FC<FileDropZoneProps> = ({ 
    onFilesSelected, 
    onPathsExpanded,
    onItemSelect,
    selectedPath,
    acceptedFormats = "image/*,.svg,.avif,.heic,.heif,.ico",
    fileDialogFilters,
    allowMultiple = true,
    title = "",
    subTitle = "",
    isActive = true,
}) => {
    const [isDragOver, setIsDragOver] = useState(false);
    const [selectionError, setSelectionError] = useState('');
    const [selectionStatus, setSelectionStatus] = useState('');
    const [previewResult, setPreviewResult] = useState<ExpandDroppedPathsResult | null>(null);
    const [sortKey, setSortKey] = useState<SortKey>('name');
    const [sortOrder, setSortOrder] = useState<SortOrder>('asc');
    const [isSortMenuOpen, setIsSortMenuOpen] = useState(false);
    const [previewFile, setPreviewFile] = useState<DroppedFile | null>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const directoryInputRef = useRef<HTMLInputElement>(null);
    const sortButtonRef = useRef<HTMLButtonElement>(null);
    const previewResultRef = useRef<ExpandDroppedPathsResult | null>(null);
    const runtimeDropMarkerRef = useRef<RuntimeDropMarker | null>(null);
    const runtimeDropTimeoutRef = useRef<number | null>(null);
    const selectedKey = selectedPath ? normalizePath(selectedPath) : '';
    const deferredPreviewResult = useDeferredValue(previewResult);
    const isPreviewPending = deferredPreviewResult !== previewResult;
    const activeFileDialogFilters = fileDialogFilters && fileDialogFilters.length > 0
        ? fileDialogFilters
        : DEFAULT_IMAGE_FILE_DIALOG_FILTERS;
    const acceptedExtensionSet = useMemo(
        () => parseAcceptedExtensionSet(acceptedFormats),
        [acceptedFormats],
    );
    const mergeResults = (base: ExpandDroppedPathsResult | null, incoming: ExpandDroppedPathsResult) => {
        if (!base) return incoming;
        const seen = new Set(base.files.map((f) => pathKey(f.input_path)));
        const merged = base.files.slice();
        for (const f of incoming.files) {
            const key = pathKey(f.input_path);
            if (seen.has(key)) continue;
            seen.add(key);
            merged.push(f);
        }
        return {
            has_directory: base.has_directory || incoming.has_directory,
            files: merged,
        };
    };
    const snapshotSignature = useCallback((r: ExpandDroppedPathsResult | null) => {
        if (!r) return '';
        return `${r.has_directory ? 'd' : 'f'}#${r.files.length}#${
            r.files.map((f) => `${f.input_path}|${f.size}|${f.mod_time}`).join(';')
        }`;
    }, []);

    const mergeAndPublishResult = useCallback((incoming: ExpandDroppedPathsResult) => {
        const filteredIncoming = filterExpandedResultByExtensions(incoming, acceptedExtensionSet);
        if (acceptedExtensionSet && incoming.files.length > 0 && filteredIncoming.files.length === 0) {
            setSelectionError('没有符合当前功能格式限制的文件');
        } else {
            setSelectionError('');
        }
        const mergedSnapshot = mergeResults(previewResultRef.current, filteredIncoming);
        // Skip redundant publishes (e.g. background enrich that adds nothing new):
        // identical snapshots would just re-render the whole tree for nothing.
        if (snapshotSignature(mergedSnapshot) === snapshotSignature(previewResultRef.current)) {
            return;
        }
        previewResultRef.current = mergedSnapshot;
        startTransition(() => {
            setPreviewResult(mergedSnapshot);
            onPathsExpanded?.(mergedSnapshot);
        });
    }, [acceptedExtensionSet, onPathsExpanded, snapshotSignature]);
    const markRuntimeDropAsHandled = useCallback((paths: string[]) => {
        if (paths.length === 0) {
            runtimeDropMarkerRef.current = null;
            return;
        }
        runtimeDropMarkerRef.current = {
            signature: buildRuntimeDropSignature(paths),
            expiresAt: Date.now() + 2000,
        };
    }, []);
    const shouldIgnoreRuntimeDrop = useCallback((paths: string[]) => {
        const marker = runtimeDropMarkerRef.current;
        if (!marker) {
            return false;
        }
        if (marker.expiresAt <= Date.now()) {
            runtimeDropMarkerRef.current = null;
            return false;
        }
        if (marker.signature !== buildRuntimeDropSignature(paths)) {
            return false;
        }
        runtimeDropMarkerRef.current = null;
        return true;
    }, []);
    const clearRuntimeDropWait = useCallback(() => {
        if (runtimeDropTimeoutRef.current !== null) {
            window.clearTimeout(runtimeDropTimeoutRef.current);
            runtimeDropTimeoutRef.current = null;
        }
    }, []);
    const beginSelectionStatus = useCallback((message: string) => {
        clearRuntimeDropWait();
        setSelectionError('');
        setSelectionStatus(message);
    }, [clearRuntimeDropWait]);
    const endSelectionStatus = useCallback(() => {
        clearRuntimeDropWait();
        setSelectionStatus('');
    }, [clearRuntimeDropWait]);
    const waitForRuntimeDropPaths = useCallback(() => {
        beginSelectionStatus('正在解析文件路径...');
        runtimeDropTimeoutRef.current = window.setTimeout(() => {
            runtimeDropTimeoutRef.current = null;
            setSelectionStatus('');
            setSelectionError('等待桌面端返回文件路径超时，请重试或使用文件选择器。');
        }, 5000);
    }, [beginSelectionStatus]);
    const expandPathsFromFiles = async (files: File[], resolvedPaths?: string[]) => {
        const app = getAppBindings();
        if (!app?.ExpandDroppedPaths) return null;
        const paths = resolvedPaths && resolvedPaths.length > 0
            ? resolvedPaths
            : await resolveSelectedFilePaths(files);
        if (!paths.length) return null;
        try {
            const result = await app.ExpandDroppedPaths(paths);
            return result as ExpandDroppedPathsResult;
        } catch (e) {
            console.error(e);
            return null;
        }
    };
    const buildLocalExpandResult = (files: File[], paths: string[]): ExpandDroppedPathsResult => ({
        has_directory: files.some((f) => Boolean(f.webkitRelativePath)),
        files: files.map((f: File, index: number) => {
            const anyFile = f as any;
            const inputPath = (paths[index] || (typeof anyFile.path === 'string' ? anyFile.path : '') || '').trim();
            const relative = f.webkitRelativePath || f.name;
            const sourceRoot = f.webkitRelativePath ? f.webkitRelativePath.split('/')[0] : '';
            return {
                input_path: inputPath,
                source_root: sourceRoot,
                relative_path: relative,
                is_from_dir_drop: Boolean(f.webkitRelativePath),
                size: f.size || 0,
                mod_time: Math.floor((f.lastModified || Date.now()) / 1000),
            };
        }).filter((item) => item.input_path),
    });

    const applyFileSelection = async (files: File[], resolvedPaths?: string[]) => {
        onFilesSelected?.(files);
        const hasResolved = Boolean(resolvedPaths && resolvedPaths.length > 0);
        beginSelectionStatus(hasResolved ? '正在加载...' : '正在解析文件路径...');
        try {
            // Fast path: known absolute paths — paint list immediately, refine in background.
            if (hasResolved && resolvedPaths) {
                const quick = buildLocalExpandResult(files, resolvedPaths);
                if (quick.files.length > 0) {
                    mergeAndPublishResult(quick);
                    setSelectionStatus('');
                }
                // Background enrich via host (size/mod_time accuracy for dirs); ignore failures.
                void (async () => {
                    try {
                        const expanded = await expandPathsFromFiles(files, resolvedPaths);
                        if (expanded && expanded.files?.length) {
                            // Replace with host result for accuracy
                            const filtered = filterExpandedResultByExtensions(expanded, acceptedExtensionSet);
                            previewResultRef.current = filtered;
                            setPreviewResult(filtered);
                            onPathsExpanded?.(filtered);
                        }
                    } catch (e) {
                        console.error(e);
                    }
                })();
                return;
            }

            const normalizedPaths = resolvedPaths && resolvedPaths.length > 0
                ? resolvedPaths
                : await resolveSelectedFilePaths(files);
            if (normalizedPaths.length > 0) {
                // Optimistic list before host expand
                const quick = buildLocalExpandResult(files, normalizedPaths);
                if (quick.files.length > 0) {
                    mergeAndPublishResult(quick);
                }
                setSelectionStatus('正在扫描文件...');
            }
            const expanded = await expandPathsFromFiles(files, normalizedPaths);
            if (expanded) {
                mergeAndPublishResult(expanded);
                return;
            }

            if (!normalizedPaths.length) {
                setSelectionError('当前环境无法解析所选文件的本地路径，请优先使用桌面端原生文件选择器或直接拖入文件。');
                return;
            }

            const result: ExpandDroppedPathsResult = {
                has_directory: files.some((f) => Boolean(f.webkitRelativePath)),
                files: files.map((f: File, index: number) => {
                    const anyFile = f as any;
                    const inputPath = typeof anyFile.path === 'string' && anyFile.path
                        ? anyFile.path
                        : (normalizedPaths[index] || '');
                    const relative = f.webkitRelativePath || f.name;
                    const sourceRoot = f.webkitRelativePath ? f.webkitRelativePath.split('/')[0] : '';
                    return {
                        input_path: inputPath,
                        source_root: sourceRoot,
                        relative_path: relative,
                        is_from_dir_drop: Boolean(f.webkitRelativePath),
                        size: f.size,
                        mod_time: Math.floor((f.lastModified || Date.now()) / 1000),
                    };
                })
            };
            mergeAndPublishResult(result);
        } finally {
            endSelectionStatus();
        }
    };

    useEffect(() => {
        if (!isActive) return;
        if (!window.runtime?.OnFileDrop) return;

        window.runtime.OnFileDrop(async (_x: number, _y: number, paths: string[]) => {
            if (!paths || paths.length === 0) return;
            if (shouldIgnoreRuntimeDrop(paths)) return;
            const app = getAppBindings();
            if (!app?.ExpandDroppedPaths) return;

            beginSelectionStatus('正在加载...');
            try {
                // Optimistic entries so UI reacts immediately on drop
                if (paths.length <= 64) {
                    const quick: ExpandDroppedPathsResult = {
                        has_directory: false,
                        files: paths.map((input_path) => ({
                            input_path,
                            source_root: '',
                            relative_path: basename(input_path),
                            is_from_dir_drop: false,
                            size: 0,
                            mod_time: Math.floor(Date.now() / 1000),
                        })),
                    };
                    mergeAndPublishResult(quick);
                }
                const result = await app.ExpandDroppedPaths(paths);
                mergeAndPublishResult(result as ExpandDroppedPathsResult);
            } catch (e) {
                console.error(e);
                setSelectionError('扫描拖入文件失败，请重试。');
            } finally {
                endSelectionStatus();
            }
        }, true);

        return () => {
            window.runtime?.OnFileDropOff?.();
        };
    }, [beginSelectionStatus, endSelectionStatus, isActive, mergeAndPublishResult, shouldIgnoreRuntimeDrop]);

    useEffect(() => () => {
        clearRuntimeDropWait();
    }, [clearRuntimeDropWait]);

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver((prev) => (prev ? prev : true));
    };

    const handleDragLeave = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver((prev) => (prev ? false : prev));
    };

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver(false);
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
            const files: File[] = Array.from(e.dataTransfer.files);
            const directPaths = files.map((file) => getDirectDropPath(file));
            const hasDirectPaths = files.length > 0 && directPaths.every((path) => path !== '');
            if (hasDirectPaths) {
                markRuntimeDropAsHandled(directPaths);
                void (async () => {
                    await applyFileSelection(files, directPaths);
                })();
                return;
            }
        }
        if (window.runtime?.OnFileDrop) {
            waitForRuntimeDropPaths();
            return;
        }
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
            const files: File[] = Array.from(e.dataTransfer.files);
            void applyFileSelection(files);
        }
    };

    const handleClick = async () => {
        try {
            const app = getAppBindings();
            if (app?.SelectInputFiles) {
                const res = await app.SelectInputFiles({
                    title: '选择文件',
                    canChooseFiles: true,
                    canChooseDirectories: false,
                    allowsMultipleSelection: allowMultiple,
                    filters: activeFileDialogFilters,
                });
                const paths = Array.isArray(res) ? res : (typeof res === 'string' && res ? [res] : []);
                if (paths.length === 0) return;
                if (!app?.ExpandDroppedPaths) {
                    inputRef.current?.click();
                    return;
                }

                const result = await app.ExpandDroppedPaths(paths);
                mergeAndPublishResult(result as ExpandDroppedPathsResult);
                return;
            }
            if (window.runtime?.OpenFileDialog) {
                const res = await window.runtime.OpenFileDialog({
                    title: '选择文件',
                    canChooseFiles: true,
                    canChooseDirectories: false,
                    allowsMultipleSelection: allowMultiple,
                    filters: activeFileDialogFilters,
                } as any);

                if (res === undefined || res === null) {
                    inputRef.current?.click();
                    return;
                }
                const paths = Array.isArray(res) ? res : (typeof res === 'string' && res ? [res] : []);
                if (paths.length === 0) return;
                if (!app?.ExpandDroppedPaths) {
                    inputRef.current?.click();
                    return;
                }

                const result = await app.ExpandDroppedPaths(paths);
                mergeAndPublishResult(result as ExpandDroppedPathsResult);
                return;
            }
        } catch (e) {
            console.error(e);
        }
        inputRef.current?.click();
    };

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files.length > 0) {
            const files: File[] = Array.from(e.target.files);
            void applyFileSelection(files);
        }
    };
    const handleDirectoryClick = (e?: React.MouseEvent) => {
        e?.stopPropagation();
        void (async () => {
            try {
                const app = getAppBindings();
                if (app?.SelectInputDirectory) {
                    const dir = await app.SelectInputDirectory();
                    if (!dir || !dir.trim()) return;
                    if (!app?.ExpandDroppedPaths) {
                        directoryInputRef.current?.click();
                        return;
                    }
                    const result = await app.ExpandDroppedPaths([dir]);
                    mergeAndPublishResult(result as ExpandDroppedPathsResult);
                    return;
                }
            } catch (err) {
                console.error(err);
            }
            directoryInputRef.current?.click();
        })();
    };
    const handleDirectoryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files.length > 0) {
            const files: File[] = Array.from(e.target.files);
            void applyFileSelection(files);
        }
    };

    const handleRemoveFile = useCallback((path: string, e: React.MouseEvent<HTMLButtonElement>) => {
        e.stopPropagation();
        setPreviewResult((prev) => {
            if (!prev) return prev;
            const newFiles = prev.files.filter((f) => f.input_path !== path);
            const newResult = { ...prev, files: newFiles };
            previewResultRef.current = newResult;
            onPathsExpanded?.(newResult);
            return newResult;
        });
    }, [onPathsExpanded]);

    const handleRemoveFolder = useCallback((root: string, e: React.MouseEvent<HTMLButtonElement>) => {
        e.stopPropagation();
        setPreviewResult((prev) => {
            if (!prev) return prev;
            const newFiles = prev.files.filter((f) => f.source_root !== root);
            const newResult = { ...prev, files: newFiles };
            previewResultRef.current = newResult;
            onPathsExpanded?.(newResult);
            return newResult;
        });
    }, [onPathsExpanded]);

    const treeRoot = useMemo<FileTreeRoot>(() => {
        if (!deferredPreviewResult?.files) return {};
        return buildSortedFileTree(deferredPreviewResult.files, sortKey, sortOrder);
    }, [deferredPreviewResult, sortKey, sortOrder]);

    const looseFiles = useMemo(() => {
        const files = deferredPreviewResult?.files || [];
        const filtered = files.filter((f) => !f.is_from_dir_drop);
        return sortFiles(filtered, sortKey, sortOrder);
    }, [deferredPreviewResult, sortKey, sortOrder]);

    const hasPreview = (previewResult?.files?.length || 0) > 0;

    const toggleSortOrder = () => {
        setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc');
    };

    return (
        <div 
            className={`
                h-full w-full rounded-3xl border-2 border-dashed flex flex-col transition-all duration-300 relative overflow-hidden z-0
                ${hasPreview ? 'items-stretch justify-start p-4 cursor-default' : 'items-center justify-center p-8 cursor-pointer group'}
                ${isDragOver 
                    ? 'border-[#007AFF] bg-blue-50/50 dark:bg-blue-500/10 scale-[0.99]' 
                    : 'border-gray-200 dark:border-white/10 bg-white dark:bg-[#2C2C2E] hover:border-[#007AFF]/50 dark:hover:border-[#007AFF]/50 hover:bg-gray-50/50 dark:hover:bg-white/[0.02] shadow-sm hover:shadow-md'
                }
            `}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            onClick={hasPreview ? undefined : handleClick}
            style={{ ['--if-drop-target' as any]: 'drop' }}
        >
            <input 
                ref={inputRef}
                type="file" 
                className="hidden" 
                multiple={allowMultiple} 
                accept={acceptedFormats} 
                onChange={handleInputChange} 
            />
            <input
                ref={(node) => {
                    directoryInputRef.current = node;
                    if (!node) {
                        return;
                    }
                    node.setAttribute('webkitdirectory', '');
                    node.setAttribute('directory', '');
                }}
                type="file"
                className="hidden"
                multiple={true}
                onChange={handleDirectoryChange}
            />

            {hasPreview ? (
                <div className="w-full h-full relative z-10 flex flex-col">
                    <div className="flex-1 bg-gray-50 dark:bg-[#232326] border border-gray-200 dark:border-white/10 rounded-2xl overflow-hidden shadow-inner flex flex-col h-full min-h-0 relative">
                        <div className="px-4 py-3 flex items-center justify-between border-b border-gray-200/60 dark:border-white/10 shrink-0 bg-gray-50/50 dark:bg-[#1F1F22]/80 backdrop-blur-sm z-10">
                            <div className="flex items-center gap-2">
                                <div className="text-xs font-medium text-gray-600 dark:text-gray-300">
                                    已添加 {previewResult?.files.length} 项
                                </div>
                                {isPreviewPending && (
                                    <div className="text-[11px] font-medium text-[#007AFF]">
                                        正在整理列表...
                                    </div>
                                )}
                                {selectionStatus && (
                                    <div className="text-[11px] font-medium text-[#007AFF]">
                                        {selectionStatus}
                                    </div>
                                )}
                            </div>
                            
                            <button
                                ref={sortButtonRef}
                                onClick={() => setIsSortMenuOpen(!isSortMenuOpen)}
                                className="flex items-center gap-1.5 px-2 py-1 rounded-lg hover:bg-black/5 dark:hover:bg-white/10 text-xs font-medium text-gray-600 dark:text-gray-300 transition-colors"
                            >
                                <Icon name="ListFilter" size={14} />
                                <span>排序</span>
                            </button>
                            {isSortMenuOpen && sortButtonRef.current && createPortal(
                                <>
                                    <div className="fixed inset-0 z-40" onClick={() => setIsSortMenuOpen(false)} />
                                    <div 
                                        className="imageflow-select-menu fixed z-50"
                                        data-placement="bottom"
                                        style={{
                                            top: sortButtonRef.current.getBoundingClientRect().bottom + 4,
                                            left: Math.max(8, Math.min(
                                                window.innerWidth - 168,
                                                sortButtonRef.current.getBoundingClientRect().right - 160,
                                            )),
                                            width: 160,
                                            maxHeight: 240,
                                        }}
                                    >
                                        {[
                                            { label: '名称', key: 'name' as SortKey },
                                            { label: '时间', key: 'time' as SortKey },
                                            { label: '大小', key: 'size' as SortKey },
                                        ].map(opt => (
                                            <button
                                                key={opt.key}
                                                type="button"
                                                onClick={() => {
                                                    setSortKey(opt.key);
                                                    setIsSortMenuOpen(false);
                                                }}
                                                aria-selected={sortKey === opt.key}
                                                data-active={sortKey === opt.key}
                                                className="imageflow-select-option"
                                            >
                                                <span className="min-w-0 flex-1 truncate">{opt.label}</span>
                                                {sortKey === opt.key && <Icon name="Check" size={14} className="ml-3 shrink-0 text-[#007AFF]" />}
                                            </button>
                                        ))}
                                        <div className="h-px bg-gray-200 dark:bg-white/10 my-1 mx-1" />
                                        <button
                                            type="button"
                                            onClick={() => {
                                                toggleSortOrder();
                                                setIsSortMenuOpen(false);
                                            }}
                                            className="imageflow-select-option"
                                        >
                                            <span className="min-w-0 flex-1 truncate">{sortOrder === 'asc' ? '升序' : '降序'}</span>
                                            <Icon name={sortOrder === 'asc' ? 'ArrowUp' : 'ArrowDown'} size={14} className="ml-3 shrink-0 text-gray-400 dark:text-gray-500" />
                                        </button>
                                    </div>
                                </>,
                                document.body
                            )}
                        </div>

                        <div className="flex-1 overflow-y-auto overflow-x-hidden relative scrollbar-dark">
                            {/* Scrollable Content */}
                            <div className="absolute inset-0 w-full">
                                {isPreviewPending && Object.keys(treeRoot).length === 0 && looseFiles.length === 0 && (
                                    <div className="h-full flex items-center justify-center px-6 text-xs text-gray-400">
                                        正在整理文件列表，请稍候...
                                    </div>
                                )}
                                {(Object.values(treeRoot) as FileTreeNode[]).map((node) => (
                                    <TreeNode
                                        key={node.path}
                                        node={node}
                                        depth={0}
                                        selectedKey={selectedKey}
                                        onItemSelect={onItemSelect}
                                        onRemoveFolder={handleRemoveFolder}
                                        onRemoveFile={handleRemoveFile}
                                        onOpenPreview={setPreviewFile}
                                    />
                                ))}

                                <VirtualList
                                    items={looseFiles}
                                    itemHeight={40}
                                    height={Math.min(420, Math.max(180, looseFiles.length * 40))}
                                    getKey={(f) => f.input_path}
                                    renderItem={(f) => {
                                        const name = getRelativeName(f);
                                        const isSelected = selectedKey && normalizePath(f.input_path) === selectedKey;
                                        return (
                                            <div
                                                onClick={() => onItemSelect?.(f)}
                                                onKeyDown={(e) => {
                                                    if (!onItemSelect) return;
                                                    if (e.key === 'Enter' || e.key === ' ') {
                                                        e.preventDefault();
                                                        onItemSelect(f);
                                                    }
                                                }}
                                                role={onItemSelect ? 'button' : undefined}
                                                tabIndex={onItemSelect ? 0 : -1}
                                                aria-selected={Boolean(isSelected)}
                                                className={`py-2 px-3 flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300 group/file ${
                                                    isSelected
                                                        ? 'bg-[#007AFF]/10 dark:bg-[#0A84FF]/15'
                                                        : 'hover:bg-black/5 dark:hover:bg-white/5'
                                                } ${onItemSelect ? 'cursor-pointer' : ''}`}
                                            >
                                                <FileThumb file={f} onOpenPreview={setPreviewFile} />
                                                <div className="flex-1 min-w-0 truncate">{name}</div>
                                                <button
                                                    onClick={(e) => handleRemoveFile(f.input_path, e)}
                                                    className="opacity-0 group-hover/file:opacity-100 p-1 rounded-md text-gray-400 hover:text-red-500 transition-all"
                                                >
                                                    <Icon name="X" size={14} />
                                                </button>
                                            </div>
                                        );
                                    }}
                                />
                            </div>
                        </div>
                    </div>
                    
                    <div className="mt-4 flex items-center justify-center gap-4 text-xs text-gray-400">
                        <button 
                            onClick={(e) => {
                                e.stopPropagation();
                                handleClick();
                            }}
                            className="hover:text-[#007AFF] transition-colors"
                        >
                            点击添加更多文件...
                        </button>
                        <button
                            type="button"
                            onClick={(e) => handleDirectoryClick(e)}
                            className="hover:text-[#007AFF] transition-colors"
                        >
                            添加文件夹
                        </button>
                    </div>
                </div>
            ) : (
                <>
                    <div className={`dropzone-orb w-16 h-16 bg-blue-50 dark:bg-white/5 rounded-full flex items-center justify-center mb-5 transition-all duration-300 text-[#007AFF] shadow-inner relative z-10 ${isDragOver ? 'scale-110 rotate-12 bg-blue-100 dark:bg-white/10' : 'group-hover:scale-110 group-hover:rotate-3'}`}>
                        <Icon name="Upload" size={32} />
                    </div>
                    
                    {title && (
                        <p className="text-xl font-semibold text-gray-900 dark:text-white mb-2 text-center pointer-events-none relative z-10">
                            {title}
                        </p>
                    )}
                    {subTitle && (
                        <p className="text-sm text-gray-500 dark:text-gray-400 text-center pointer-events-none relative z-10 px-8 leading-relaxed">
                            {subTitle}
                        </p>
                    )}
                    {selectionStatus && (
                        <div className="mt-3 flex items-center gap-2 text-xs text-[#007AFF] dark:text-[#64AFFF] relative z-10">
                            <span className="w-2 h-2 rounded-full bg-current animate-pulse" />
                            <span>{selectionStatus}</span>
                        </div>
                    )}
                    {selectionError && (
                        <div className="mt-3 text-xs text-red-500 dark:text-red-300 text-center px-6 relative z-10">
                            {selectionError}
                        </div>
                    )}
                    <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                        <button
                            type="button"
                            onClick={(e) => handleDirectoryClick(e)}
                            className="underline underline-offset-2 hover:text-[#007AFF]"
                        >
                            点击选择文件夹
                        </button>
                    </div>
                </>
            )}

            {/* Animated Overlay for Drag State */}
            {isDragOver && (
                <div className="absolute inset-0 bg-blue-500/5 dark:bg-blue-500/10 pointer-events-none animate-pulse z-0" />
            )}

            <PreviewOverlay file={previewFile} onClose={() => setPreviewFile(null)} />
        </div>
    );
};

export const FileDropZone = memo(FileDropZoneImpl);

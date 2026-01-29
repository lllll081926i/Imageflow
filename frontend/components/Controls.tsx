import React, { useState, useRef, useEffect, useMemo, useCallback, memo } from 'react';
import { createPortal } from 'react-dom';
import Icon from './Icon';

// --- Reusable UI Components ---

export const Switch: React.FC<{ checked: boolean; onChange: (checked: boolean) => void; label?: string }> = memo(({ checked, onChange, label }) => (
    <div className="flex items-center justify-between cursor-pointer group select-none" onClick={() => onChange(!checked)}>
        {label && <span className="text-sm font-medium text-gray-700 dark:text-gray-300 group-hover:text-gray-900 dark:group-hover:text-white transition-colors">{label}</span>}
        <div className={`w-11 h-6 rounded-full p-1 transition-colors duration-200 ease-in-out ${checked ? 'bg-[#007AFF]' : 'bg-gray-300 dark:bg-white/20'}`}>
            <div className={`bg-white w-4 h-4 rounded-full shadow-sm transform transition-transform duration-200 ease-in-out ${checked ? 'translate-x-5' : 'translate-x-0'}`} />
        </div>
    </div>
));

export const PositionGrid: React.FC<{ value: string; onChange: (val: string) => void }> = memo(({ value, onChange }) => {
    const positions = ['tl', 'tc', 'tr', 'cl', 'c', 'cr', 'bl', 'bc', 'br'];
    return (
        <div className="grid grid-cols-3 gap-1 w-[88px] h-[88px] bg-gray-100 dark:bg-white/5 p-1 rounded-lg border border-gray-200 dark:border-white/10 shrink-0">
            {positions.map(pos => (
                <button
                    key={pos}
                    onClick={() => onChange(pos)}
                    className={`rounded transition-all ${
                        value === pos 
                        ? 'bg-[#007AFF] shadow-sm' 
                        : 'hover:bg-gray-200 dark:hover:bg-white/10'
                    }`}
                />
            ))}
        </div>
    );
});

export const SegmentedControl: React.FC<{ options: string[]; value: string; onChange: (val: string) => void }> = memo(({ options, value, onChange }) => {
    const activeIndex = options.indexOf(value);
    
    return (
        <div className="relative flex bg-gray-100 dark:bg-black/20 p-1 rounded-xl isolate">
            <div 
                className="absolute top-1 bottom-1 bg-white dark:bg-[#636366] shadow-[0_1px_3px_0_rgba(0,0,0,0.1)] rounded-lg transition-all duration-300 ease-[cubic-bezier(0.25,0.1,0.25,1)]"
                style={{
                    width: `calc((100% - 8px) / ${options.length})`,
                    left: 4,
                    transform: `translateX(${activeIndex * 100}%)`
                }}
            />
            {options.map((opt) => (
                <button
                    key={opt}
                    onClick={() => onChange(opt)}
                    className={`relative flex-1 py-1.5 text-sm font-medium z-10 transition-colors duration-200 select-none truncate px-1 ${
                        value === opt 
                        ? 'text-gray-900 dark:text-white' 
                        : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                    }`}
                >
                    {opt}
                </button>
            ))}
        </div>
    );
});

export const StyledSlider: React.FC<{ label?: string; value: number; min?: number; max?: number; unit?: string; onChange: (val: number) => void; step?: number; className?: string }> = memo(({ label, value, min = 0, max = 100, unit = "", onChange, step = 1, className = "" }) => {
    const percent = ((value - min) / (max - min)) * 100;
    
    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        let val = parseInt(e.target.value);
        if (isNaN(val)) val = 0;
        // Allow temporary invalid input but clamp on blur or submit if needed
        // For now, clamp strictly to ensure valid state
        val = Math.max(min, Math.min(max, val));
        onChange(val);
    };

    return (
        <div className={`space-y-3 select-none ${className}`}>
            <div className="flex justify-between items-center gap-3">
                {label !== undefined && (
                    <label className="flex-1 min-w-0 text-sm font-medium text-gray-700 dark:text-gray-300 truncate">
                        {label}
                    </label>
                )}
                <div className="flex items-center gap-1.5 ml-auto">
                    {/* Optional percentage display can be removed if input is enough, or kept as aux */}
                    {/* <span className="text-sm font-medium text-[#007AFF]">{Math.round(percent)}%</span> */}
                    <div className="relative group">
                        <input 
                            type="number" 
                            value={value}
                            min={min}
                            max={max}
                            onChange={handleInputChange}
                            className="w-12 text-center text-gray-600 dark:text-gray-300 font-mono text-xs bg-gray-100 dark:bg-white/10 px-1 py-0.5 rounded outline-none focus:ring-1 focus:ring-[#007AFF] appearance-none m-0 border border-transparent focus:border-[#007AFF] transition-all"
                        />
                    </div>
                </div>
            </div>
            <div className="relative h-6 w-full flex items-center group">
                <div className="absolute w-full h-1.5 bg-gray-200 dark:bg-white/10 rounded-full overflow-hidden">
                    <div className="h-full bg-[#007AFF] transition-all duration-150 ease-out" style={{ width: `${percent}%` }} />
                </div>
                <div 
                    className="absolute h-5 w-5 bg-white shadow-[0_2px_4px_rgba(0,0,0,0.2)] border border-gray-100 rounded-full flex items-center justify-center pointer-events-none transition-all duration-150 ease-out z-20 group-active:scale-110"
                    style={{ left: `${percent}%`, transform: 'translateX(-50%)' }}
                >
                    <div className="w-1.5 h-1.5 bg-[#007AFF] rounded-full" />
                </div>
                <input 
                    type="range" min={min} max={max} step={step} value={value} 
                    onChange={(e) => onChange(Number(e.target.value))}
                    className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-30" 
                />
            </div>
        </div>
    );
});

export const ProgressBar: React.FC<{ progress: number; label?: string }> = memo(({ progress, label }) => (
    <div className="w-full space-y-2">
        {label && (
            <div className="flex justify-between text-xs font-medium text-gray-600 dark:text-gray-300">
                <span>{label}</span>
                <span>{Math.round(progress)}%</span>
            </div>
        )}
        <div className="h-2 w-full bg-gray-100 dark:bg-white/10 rounded-full overflow-hidden">
            <div 
                className="h-full bg-gradient-to-r from-[#007AFF] to-[#0055FF] transition-all duration-300 ease-out rounded-full"
                style={{ width: `${Math.min(100, Math.max(0, progress))}%` }}
            />
        </div>
    </div>
));

export const CustomSelect: React.FC<{ label?: string; options: string[]; value: string; onChange: (val: string) => void }> = memo(({ label, options, value, onChange }) => {
    const [isOpen, setIsOpen] = useState(false);
    const buttonRef = useRef<HTMLButtonElement>(null);
    const [menuStyle, setMenuStyle] = useState<{top: number, left: number, width: number} | null>(null);

    const handleToggle = () => {
        if (!isOpen && buttonRef.current) {
            const rect = buttonRef.current.getBoundingClientRect();
            setMenuStyle({
                top: rect.bottom + 6,
                left: rect.left,
                width: rect.width
            });
            setIsOpen(true);
        } else {
            setIsOpen(false);
        }
    };

    useEffect(() => {
        if (!isOpen) return;
        const controller = new AbortController();
        const handleClose = () => setIsOpen(false);
        const handleWindowClick = (e: MouseEvent) => {
            if (buttonRef.current && !buttonRef.current.contains(e.target as Node)) {
                setIsOpen(false);
            }
        };
        window.addEventListener('scroll', handleClose, { capture: true, signal: controller.signal });
        window.addEventListener('resize', handleClose, { signal: controller.signal });
        window.addEventListener('click', handleWindowClick, { signal: controller.signal });

        return () => {
            controller.abort();
        };
    }, [isOpen]);

    const renderMenu = () => {
        if (!isOpen || !menuStyle) return null;
        
        const menu = (
            <div 
                className="fixed z-[9999] bg-white dark:bg-[#1C1C1E] border border-gray-100 dark:border-white/10 rounded-xl shadow-xl shadow-black/10 max-h-60 overflow-y-auto no-scrollbar animate-enter p-1.5 origin-top"
                style={{ 
                    top: menuStyle.top, 
                    left: menuStyle.left, 
                    width: menuStyle.width 
                }}
                onClick={(e) => e.stopPropagation()}
            >
                {options.map((opt, i) => (
                    <button
                        key={i}
                        onClick={(e) => { 
                            onChange(opt); 
                            setIsOpen(false); 
                        }}
                        className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors mb-0.5 last:mb-0 ${
                            opt === value ? 'bg-[#007AFF]/10 text-[#007AFF] font-medium' : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5'
                        }`}
                    >
                        {opt}
                    </button>
                ))}
            </div>
        );
        return createPortal(menu, document.body);
    };

    return (
        <div className="space-y-2">
            {label && <label className="text-sm font-medium text-gray-700 dark:text-gray-300">{label}</label>}
            <button
                ref={buttonRef}
                onClick={handleToggle}
                className={`w-full flex items-center justify-between px-4 py-2.5 rounded-xl bg-gray-50 hover:bg-gray-100 dark:bg-white/5 dark:hover:bg-white/10 border text-sm transition-all duration-200 outline-none ${
                    isOpen ? 'border-[#007AFF] ring-2 ring-[#007AFF]/20 bg-white dark:bg-white/10' : 'border-gray-200 dark:border-white/10'
                }`}
            >
                <span className="text-gray-900 dark:text-white truncate">{value}</span>
                <Icon name="ChevronDown" size={16} className={`text-gray-400 transition-transform duration-300 ${isOpen ? 'rotate-180' : ''}`} />
            </button>
            {renderMenu()}
        </div>
    );
});

// --- FileDropZone ---

interface FileDropZoneProps {
    onFilesSelected: (files: File[]) => void;
    onPathsExpanded?: (result: ExpandDroppedPathsResult) => void;
    onItemSelect?: (file: DroppedFile) => void;
    selectedPath?: string;
    acceptedFormats?: string; 
    allowMultiple?: boolean;
    title?: string;
    subTitle?: string;
    isActive?: boolean;
}

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
    sortedChildren?: FileTreeNode[];
    sortedFiles?: DroppedFile[];
};

type FileTreeRoot = Record<string, FileTreeNode>;

const normalizePath = (p: string) => p.replace(/\\/g, '/');
const basename = (p: string) => normalizePath(p).split('/').pop() || p;
const getExt = (name: string) => {
    const idx = name.lastIndexOf('.');
    return idx >= 0 ? name.slice(idx + 1).toUpperCase() : '';
};
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
const sortTreeNode = (node: FileTreeNode, sortKey: SortKey, sortOrder: SortOrder) => {
    const sortedChildren = Object.values(node.children || {});
    sortedChildren.forEach((child) => sortTreeNode(child, sortKey, sortOrder));
    sortedChildren.sort((a, b) => {
        const res = compareNames(a.name, b.name);
        return sortOrder === 'asc' ? res : -res;
    });
    node.sortedChildren = sortedChildren;
    node.sortedFiles = sortFiles(node.files || [], sortKey, sortOrder);
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
};

const TreeNode: React.FC<TreeNodeProps> = memo(({
    node,
    depth,
    selectedKey,
    onItemSelect,
    onRemoveFolder,
    onRemoveFile,
}) => {
    const [open, setOpen] = useState(true);
    const [shouldRenderChildren, setShouldRenderChildren] = useState(true);

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
            <button
                type="button"
                onClick={(e) => {
                    e.stopPropagation();
                    setOpen((prev) => !prev);
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
                        {sortedFiles.length} 文件, {sortedChildren.length} 文件夹
                    </div>
                </div>
                {depth === 0 && (
                    <button
                        onClick={(e) => onRemoveFolder(node.path, e)}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 transition-colors"
                        title="移除文件夹"
                    >
                        <Icon name="Trash2" size={14} />
                    </button>
                )}
            </button>

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
                                />
                            ))}

                            {sortedFiles.map((f) => {
                                const name = getRelativeName(f);
                                const isSelected = selectedKey && normalizePath(f.input_path) === selectedKey;
                                return (
                                    <div
                                        key={f.input_path}
                                        onClick={() => onItemSelect?.(f)}
                                        className={`py-2 flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300 group/file pr-9 ${
                                            isSelected
                                                ? 'bg-[#007AFF]/10 dark:bg-[#0A84FF]/15'
                                                : 'hover:bg-black/5 dark:hover:bg-white/5'
                                        } ${onItemSelect ? 'cursor-pointer' : ''}`}
                                        style={{ paddingLeft: paddingLeft + 24 }}
                                    >
                                        <div className="w-2 h-2 rounded-full bg-[#007AFF]/60 shrink-0" />
                                        <div className="flex-1 min-w-0 truncate">{name}</div>
                                        <button
                                            onClick={(e) => onRemoveFile(f.input_path, e)}
                                            className="opacity-0 group-hover/file:opacity-100 p-1 rounded-md text-gray-400 hover:text-red-500 transition-all"
                                            title="移除文件"
                                        >
                                            <Icon name="X" size={14} />
                                        </button>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
});

export const FileDropZone: React.FC<FileDropZoneProps> = ({ 
    onFilesSelected, 
    onPathsExpanded,
    onItemSelect,
    selectedPath,
    acceptedFormats = "image/*,.svg", 
    allowMultiple = true,
    title = "拖拽图片到这里",
    subTitle = "或点击选择文件 (支持批量处理)",
    isActive = true,
}) => {
    const [isDragOver, setIsDragOver] = useState(false);
    const [previewResult, setPreviewResult] = useState<ExpandDroppedPathsResult | null>(null);
    const [sortKey, setSortKey] = useState<SortKey>('name');
    const [sortOrder, setSortOrder] = useState<SortOrder>('asc');
    const [isSortMenuOpen, setIsSortMenuOpen] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);
    const sortButtonRef = useRef<HTMLButtonElement>(null);
    const selectedKey = selectedPath ? normalizePath(selectedPath) : '';
    const mergeResults = (base: ExpandDroppedPathsResult | null, incoming: ExpandDroppedPathsResult) => {
        if (!base) return incoming;
        const seen = new Set(base.files.map((f) => normalizePath(f.input_path)));
        const merged = base.files.slice();
        for (const f of incoming.files) {
            const key = normalizePath(f.input_path);
            if (seen.has(key)) continue;
            seen.add(key);
            merged.push(f);
        }
        return {
            has_directory: base.has_directory || incoming.has_directory,
            files: merged,
        };
    };

    useEffect(() => {
        if (!isActive) return;
        if (!window.runtime?.OnFileDrop) return;

        window.runtime.OnFileDrop(async (_x: number, _y: number, paths: string[]) => {
            if (!paths || paths.length === 0) return;
            if (!window.go?.main?.App?.ExpandDroppedPaths) return;

            try {
                const result = await window.go.main.App.ExpandDroppedPaths(paths);
                const merged = mergeResults(previewResult, result as ExpandDroppedPathsResult);
                setPreviewResult(merged);
                onPathsExpanded?.(merged);
            } catch (e) {
                console.error(e);
            }
        }, true);

        return () => {
            window.runtime?.OnFileDropOff?.();
        };
    }, [onPathsExpanded, isActive, previewResult]);

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver(true);
    };

    const handleDragLeave = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver(false);
    };

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragOver(false);
        if (window.runtime?.OnFileDrop) {
            return;
        }
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
            const files: File[] = Array.from(e.dataTransfer.files);
            onFilesSelected(files);
            const result: ExpandDroppedPathsResult = {
                has_directory: false,
                files: files.map((f: File) => ({
                    input_path: f.name,
                    source_root: '',
                    relative_path: f.name,
                    is_from_dir_drop: false,
                    size: f.size,
                    mod_time: Math.floor((f.lastModified || Date.now()) / 1000),
                }))
            };
            const merged = mergeResults(previewResult, result);
            setPreviewResult(merged);
            onPathsExpanded?.(merged);
        }
    };

    const handleClick = async () => {
        try {
            if (window.runtime?.OpenFileDialog) {
                const res = await window.runtime.OpenFileDialog({
                    title: '选择文件',
                    canChooseFiles: true,
                    canChooseDirectories: false,
                    allowsMultipleSelection: allowMultiple,
                    filters: [{
                        DisplayName: "Images",
                        Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.gif;*.bmp;*.tiff;*.tif;*.heic;*.heif;*.svg"
                    }]
                } as any);

                const paths = Array.isArray(res) ? res : (typeof res === 'string' && res ? [res] : []);
                if (paths.length === 0) return;
                if (!window.go?.main?.App?.ExpandDroppedPaths) return;

                const result = await window.go.main.App.ExpandDroppedPaths(paths);
                const merged = mergeResults(previewResult, result as ExpandDroppedPathsResult);
                setPreviewResult(merged);
                onPathsExpanded?.(merged);
                return;
            }
        } catch (e) {
            console.error(e);
        }
        inputRef.current?.click();
    };

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (window.runtime?.OnFileDrop) {
            return;
        }
        if (e.target.files && e.target.files.length > 0) {
            const files: File[] = Array.from(e.target.files);
            onFilesSelected(files);
            const result: ExpandDroppedPathsResult = {
                has_directory: false,
                files: files.map((f: File) => ({
                    input_path: f.name,
                    source_root: '',
                    relative_path: f.name,
                    is_from_dir_drop: false,
                    size: f.size,
                    mod_time: Math.floor((f.lastModified || Date.now()) / 1000),
                }))
            };
            const merged = mergeResults(previewResult, result);
            setPreviewResult(merged);
            onPathsExpanded?.(merged);
        }
    };

    const handleRemoveFile = useCallback((path: string, e: React.MouseEvent<HTMLButtonElement>) => {
        e.stopPropagation();
        setPreviewResult((prev) => {
            if (!prev) return prev;
            const newFiles = prev.files.filter((f) => f.input_path !== path);
            const newResult = { ...prev, files: newFiles };
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
            onPathsExpanded?.(newResult);
            return newResult;
        });
    }, [onPathsExpanded]);

    const treeRoot = useMemo<FileTreeRoot>(() => {
        if (!previewResult?.files) return {};
        return buildSortedFileTree(previewResult.files, sortKey, sortOrder);
    }, [previewResult, sortKey, sortOrder]);

    const looseFiles = useMemo(() => {
        const files = previewResult?.files || [];
        const filtered = files.filter((f) => !f.is_from_dir_drop);
        return sortFiles(filtered, sortKey, sortOrder);
    }, [previewResult, sortKey, sortOrder]);

    const hasPreview = (previewResult?.files?.length || 0) > 0;

    const toggleSortOrder = () => {
        setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc');
    };

    const SortMenu = () => {
        if (!isSortMenuOpen || !sortButtonRef.current) return null;

        const options: { label: string; key: SortKey }[] = [
            { label: '名称', key: 'name' },
            { label: '时间', key: 'time' },
            { label: '大小', key: 'size' },
        ];

        return createPortal(
            <>
                <div className="fixed inset-0 z-40" onClick={() => setIsSortMenuOpen(false)} />
                <div 
                    className="fixed z-50 bg-white dark:bg-[#1C1C1E] rounded-xl shadow-xl border border-gray-200 dark:border-white/10 p-1 min-w-[120px] animate-enter"
                    style={{
                        top: sortButtonRef.current.getBoundingClientRect().bottom + 4,
                        left: sortButtonRef.current.getBoundingClientRect().right - 120, // Align right
                    }}
                >
                    {options.map(opt => (
                        <button
                            key={opt.key}
                            onClick={() => {
                                setSortKey(opt.key);
                                setIsSortMenuOpen(false);
                            }}
                            className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm transition-colors ${
                                sortKey === opt.key 
                                    ? 'bg-[#007AFF] text-white' 
                                    : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-white/5'
                            }`}
                        >
                            <span>{opt.label}</span>
                            {sortKey === opt.key && <Icon name="Check" size={14} />}
                        </button>
                    ))}
                    <div className="h-px bg-gray-200 dark:bg-white/10 my-1 mx-1" />
                    <button
                        onClick={() => {
                            toggleSortOrder();
                            setIsSortMenuOpen(false);
                        }}
                        className="w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors"
                    >
                        <span>{sortOrder === 'asc' ? '升序' : '降序'}</span>
                        <Icon name={sortOrder === 'asc' ? 'ArrowUp' : 'ArrowDown'} size={14} />
                    </button>
                </div>
            </>,
            document.body
        );
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
            style={{ ['--wails-drop-target' as any]: 'drop' }}
        >
            <input 
                ref={inputRef}
                type="file" 
                className="hidden" 
                multiple={allowMultiple} 
                accept={acceptedFormats} 
                onChange={handleInputChange} 
            />

            {hasPreview ? (
                <div className="w-full h-full relative z-10 flex flex-col">
                    <div className="flex-1 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-2xl overflow-hidden shadow-inner flex flex-col h-full min-h-0 relative">
                        <div className="px-4 py-3 flex items-center justify-between border-b border-gray-200/60 dark:border-white/10 shrink-0 bg-gray-50/50 dark:bg-white/5 backdrop-blur-sm z-10">
                            <div className="flex items-center gap-2">
                                <div className="text-xs font-medium text-gray-600 dark:text-gray-300">
                                    已添加 {previewResult?.files.length} 项
                                </div>
                            </div>
                            
                            <button
                                ref={sortButtonRef}
                                onClick={() => setIsSortMenuOpen(!isSortMenuOpen)}
                                className="flex items-center gap-1.5 px-2 py-1 rounded-lg hover:bg-black/5 dark:hover:bg-white/10 text-xs font-medium text-gray-600 dark:text-gray-300 transition-colors"
                            >
                                <Icon name="ListFilter" size={14} />
                                <span>排序</span>
                            </button>
                            <SortMenu />
                        </div>

                        <div className="flex-1 overflow-y-auto overflow-x-hidden relative">
                            {/* Scrollable Content */}
                            <div className="absolute inset-0 w-full">
                                {Object.values(treeRoot).map((node) => (
                                    <TreeNode
                                        key={node.path}
                                        node={node}
                                        depth={0}
                                        selectedKey={selectedKey}
                                        onItemSelect={onItemSelect}
                                        onRemoveFolder={handleRemoveFolder}
                                        onRemoveFile={handleRemoveFile}
                                    />
                                ))}

                                {looseFiles.map((f) => {
                                    const name = basename(f.input_path);
                                    const ext = getExt(name);
                                    const isSelected = selectedKey && normalizePath(f.input_path) === selectedKey;
                                    return (
                                        <div
                                            key={f.input_path}
                                            onClick={() => onItemSelect?.(f)}
                                            className={`px-4 py-3 flex items-center gap-3 border-b border-gray-200/60 dark:border-white/10 last:border-0 transition-colors group/file ${
                                                isSelected
                                                    ? 'bg-[#007AFF]/10 dark:bg-[#0A84FF]/15'
                                                    : 'hover:bg-black/5 dark:hover:bg-white/5'
                                            } ${onItemSelect ? 'cursor-pointer' : ''}`}
                                        >
                                            <div className="w-2 h-2 rounded-full bg-[#5856D6]/60 shrink-0" />
                                            <div className="flex-1 min-w-0">
                                                <div className="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">{name}</div>
                                                <div className="text-[11px] text-gray-500 dark:text-gray-400">文件</div>
                                            </div>
                                            <button 
                                                onClick={(e) => handleRemoveFile(f.input_path, e)}
                                                className="opacity-0 group-hover/file:opacity-100 p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 transition-all"
                                                title="移除文件"
                                            >
                                                <Icon name="Trash2" size={14} />
                                            </button>
                                        </div>
                                    );
                                })}
                            </div>
                        </div>
                    </div>
                    
                    <button 
                        onClick={(e) => {
                            e.stopPropagation();
                            handleClick();
                        }}
                        className="mt-4 text-xs text-gray-400 hover:text-[#007AFF] transition-colors text-center"
                    >
                        点击添加更多文件...
                    </button>
                </div>
            ) : (
                <>
                    <div className={`w-16 h-16 bg-blue-50 dark:bg-white/5 rounded-full flex items-center justify-center mb-5 transition-all duration-300 text-[#007AFF] shadow-inner relative z-10 ${isDragOver ? 'scale-110 rotate-12 bg-blue-100 dark:bg-white/10' : 'group-hover:scale-110 group-hover:rotate-3'}`}>
                        <Icon name="Upload" size={32} />
                    </div>
                    
                    <p className="text-xl font-semibold text-gray-900 dark:text-white mb-2 text-center pointer-events-none relative z-10">
                        {title}
                    </p>
                    <p className="text-sm text-gray-500 dark:text-gray-400 text-center pointer-events-none relative z-10 px-8 leading-relaxed">
                        {subTitle}
                        <span className="block mt-1 text-xs opacity-70">支持 JPG, PNG, WEBP, GIF, SVG 等常见格式</span>
                        <span className="block mt-0.5 text-xs opacity-70">也可直接拖入整个文件夹</span>
                    </p>
                </>
            )}

            {/* Animated Overlay for Drag State */}
            {isDragOver && (
                <div className="absolute inset-0 bg-blue-500/5 dark:bg-blue-500/10 pointer-events-none animate-pulse z-0" />
            )}
        </div>
    );
};

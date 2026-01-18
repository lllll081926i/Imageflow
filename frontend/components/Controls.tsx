import React, { useState, useRef, useEffect, memo } from 'react';
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
    const positions = ['tl', 'tc', 'tr', 'ml', 'mc', 'mr', 'bl', 'bc', 'br'];
    return (
        <div className="grid grid-cols-3 gap-1 w-24 h-24 bg-gray-100 dark:bg-white/5 p-1 rounded-lg border border-gray-200 dark:border-white/10 shrink-0">
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

export const StyledSlider: React.FC<{ label: string; value: number; min?: number; max?: number; unit?: string; onChange: (val: number) => void; step?: number }> = memo(({ label, value, min = 0, max = 100, unit = "", onChange, step = 1 }) => {
    const percent = ((value - min) / (max - min)) * 100;
    return (
        <div className="space-y-3 select-none">
            <label className="text-sm font-medium text-gray-700 dark:text-gray-300 flex justify-between">
                <span>{label}</span>
                <span className="text-[#007AFF] font-mono text-xs bg-[#007AFF]/10 px-2 py-0.5 rounded min-w-[32px] text-center">{value}{unit}</span>
            </label>
            <div className="relative h-6 w-full flex items-center group">
                <div className="absolute w-full h-1.5 bg-gray-200 dark:bg-white/10 rounded-full overflow-hidden">
                    <div className="h-full bg-[#007AFF] transition-none" style={{ width: `${percent}%` }} />
                </div>
                <div 
                    className="absolute h-5 w-5 bg-white shadow-[0_2px_4px_rgba(0,0,0,0.2)] border border-gray-100 rounded-full flex items-center justify-center pointer-events-none transition-transform duration-75 ease-out z-20 group-active:scale-110"
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
        const handleClose = () => setIsOpen(false);
        window.addEventListener('scroll', handleClose, true);
        window.addEventListener('resize', handleClose);
        window.addEventListener('click', (e) => {
            if (buttonRef.current && !buttonRef.current.contains(e.target as Node)) {
                setIsOpen(false);
            }
        });

        return () => {
            window.removeEventListener('scroll', handleClose, true);
            window.removeEventListener('resize', handleClose);
            window.removeEventListener('click', handleClose); 
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
    acceptedFormats?: string; 
    allowMultiple?: boolean;
    title?: string;
    subTitle?: string;
}

export const FileDropZone: React.FC<FileDropZoneProps> = ({ 
    onFilesSelected, 
    acceptedFormats = "image/*", 
    allowMultiple = true,
    title = "拖拽图片到这里",
    subTitle = "或点击选择文件 (支持批量处理)"
}) => {
    const [isDragOver, setIsDragOver] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);

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
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
            const files = Array.from(e.dataTransfer.files);
            onFilesSelected(files);
        }
    };

    const handleClick = () => {
        inputRef.current?.click();
    };

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files.length > 0) {
            const files = Array.from(e.target.files);
            onFilesSelected(files);
        }
    };

    return (
        <div 
            className={`
                h-full w-full rounded-3xl border-2 border-dashed flex flex-col items-center justify-center p-12 transition-all duration-300 cursor-pointer group relative overflow-hidden z-0
                ${isDragOver 
                    ? 'border-[#007AFF] bg-blue-50/50 dark:bg-blue-500/10 scale-[0.99]' 
                    : 'border-gray-200 dark:border-white/10 bg-white dark:bg-[#2C2C2E] hover:border-[#007AFF]/50 dark:hover:border-[#007AFF]/50 hover:bg-gray-50/50 dark:hover:bg-white/[0.02] shadow-sm hover:shadow-md'
                }
            `}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            onClick={handleClick}
        >
            <input 
                ref={inputRef}
                type="file" 
                className="hidden" 
                multiple={allowMultiple} 
                accept={acceptedFormats} 
                onChange={handleInputChange} 
            />

            {/* Icon Wrapper */}
            <div className={`w-20 h-20 bg-blue-50 dark:bg-white/5 rounded-full flex items-center justify-center mb-6 transition-all duration-300 text-[#007AFF] shadow-inner relative z-10 ${isDragOver ? 'scale-110 rotate-12 bg-blue-100 dark:bg-white/10' : 'group-hover:scale-110 group-hover:rotate-3'}`}>
                <Icon name="Upload" size={32} />
            </div>
            
            {/* Text Content - Ensure High Z-Index and Contrast */}
            <p className="text-xl font-semibold text-gray-900 dark:text-white mb-2 text-center pointer-events-none relative z-10">
                {title}
            </p>
            <p className="text-sm text-gray-500 dark:text-gray-400 text-center pointer-events-none relative z-10">
                {subTitle}
            </p>
            
            {/* Format Badge */}
            <div className="mt-8 px-4 py-2 bg-gray-100 dark:bg-white/5 rounded-full text-xs font-mono text-gray-500 dark:text-gray-400 border border-gray-200 dark:border-white/5 pointer-events-none relative z-10">
                {acceptedFormats.replace(/image\//g, '').toUpperCase()}
            </div>

            {/* Animated Overlay for Drag State */}
            {isDragOver && (
                <div className="absolute inset-0 bg-blue-500/5 dark:bg-blue-500/10 pointer-events-none animate-pulse z-0" />
            )}
        </div>
    );
};
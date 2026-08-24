import React, { useState, useRef, useEffect, useMemo, useCallback, useDeferredValue, useId, useLayoutEffect, memo } from 'react';
import VirtualList from './VirtualList';
import { createPortal } from 'react-dom';
import Icon from './Icon';
import { getAppBindings, resolveSelectedFilePaths } from '../types/app-api';

// --- Reusable UI Components ---

export const Switch: React.FC<{ checked: boolean; onChange: (checked: boolean) => void; label?: string }> = memo(({ checked, onChange, label }) => (
    <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label || '切换开关'}
        onClick={() => onChange(!checked)}
        className="w-full flex items-center justify-between cursor-pointer group select-none text-left"
    >
        {label && <span className="text-sm font-medium text-gray-700 dark:text-gray-300 group-hover:text-gray-900 dark:group-hover:text-white transition-colors">{label}</span>}
        <div className={`w-11 h-6 rounded-full p-1 transition-colors duration-200 ease-in-out ${checked ? 'bg-[#007AFF]' : 'bg-gray-300 dark:bg-white/20'}`}>
            <div className={`spring-switch-thumb bg-white w-4 h-4 rounded-full shadow-sm transform ${checked ? 'translate-x-5' : 'translate-x-0'}`} />
        </div>
    </button>
));

export const SegmentedControl: React.FC<{ options: string[]; value: string; onChange: (val: string) => void }> = memo(({ options, value, onChange }) => {
    const activeIndex = Math.max(0, options.indexOf(value));
    if (options.length === 0) return null;
    
    return (
        <div className="relative flex bg-gray-100 dark:bg-black/20 p-1 rounded-xl isolate" role="tablist">
            <div 
                className="spring-segment-pill absolute top-1 bottom-1 bg-white dark:bg-[#636366] shadow-[0_1px_3px_0_rgba(0,0,0,0.1)] rounded-lg"
                style={{
                    width: `calc((100% - 8px) / ${options.length})`,
                    left: 4,
                    transform: `translateX(${activeIndex * 100}%)`
                }}
            />
            {options.map((opt) => (
                <button
                    key={opt}
                    type="button"
                    role="tab"
                    aria-selected={value === opt}
                    onClick={() => onChange(opt)}
                    className={`relative flex-1 min-w-0 py-1.5 text-sm font-medium z-10 transition-colors duration-200 select-none truncate px-1 ${
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
    const range = max - min;
    const percent = range > 0 ? Math.max(0, Math.min(100, ((value - min) / range) * 100)) : 0;
    const inputLabel = unit ? `${label ?? '数值'} (${unit})` : label ?? '数值';
    
    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        let val = Number(e.target.value);
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
                            aria-label={inputLabel}
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
                    <div
                        className="h-full w-full origin-left bg-[#007AFF] spring-progress-fill"
                        style={{ transform: `scaleX(${percent / 100})` }}
                    />
                </div>
                <div 
                    className="absolute h-5 w-5 bg-white shadow-[0_2px_4px_rgba(0,0,0,0.2)] border border-gray-100 rounded-full flex items-center justify-center pointer-events-none z-20 transition-transform duration-150 ease-out group-active:scale-110"
                    style={{ left: `${percent}%`, transform: 'translateX(-50%)' }}
                >
                    <div className="w-1.5 h-1.5 bg-[#007AFF] rounded-full" />
                </div>
                <input 
                    type="range" min={min} max={max} step={step} value={value} 
                    aria-label={inputLabel}
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
        <div
            className="h-2 w-full bg-gray-100 dark:bg-white/10 rounded-full overflow-hidden"
            role="progressbar"
            aria-label={label || '处理进度'}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(Math.min(100, Math.max(0, progress)))}
        >
            <div 
                className={`h-full bg-gradient-to-r from-[#007AFF] to-[#0055FF] spring-progress-fill rounded-full ${!progress || progress < 100 ? '' : 'progress-fill-done'}`}
                style={{ transform: `scaleX(${Math.min(100, Math.max(0, progress)) / 100})`, transformOrigin: 'left' }}
            />
        </div>
    </div>
));

type SelectOption = string | { label: string; value: string };

type SelectMenuStyle = {
    top: number;
    left: number;
    width: number;
    maxHeight: number;
    placement: 'top' | 'bottom';
};

export const CustomSelect: React.FC<{ label?: string; options: SelectOption[]; value: string; onChange: (val: string) => void }> = memo(({ label, options, value, onChange }) => {
    const [isOpen, setIsOpen] = useState(false);
    const [activeIndex, setActiveIndex] = useState(-1);
    const buttonRef = useRef<HTMLButtonElement>(null);
    const menuRef = useRef<HTMLDivElement>(null);
    const listboxId = useId();
    const [menuStyle, setMenuStyle] = useState<SelectMenuStyle | null>(null);
    const normalizedOptions = useMemo(() => options.map((opt) => (
        typeof opt === 'string' ? { label: opt, value: opt } : opt
    )), [options]);
    const selectedIndex = useMemo(() => normalizedOptions.findIndex((opt) => opt.value === value), [normalizedOptions, value]);
    const selectedLabel = useMemo(() => {
        const matched = normalizedOptions.find((opt) => opt.value === value);
        return matched?.label ?? value;
    }, [normalizedOptions, value]);

    const calculateMenuStyle = useCallback((menuHeight = 0): SelectMenuStyle | null => {
        const button = buttonRef.current;
        if (!button) {
            return null;
        }

        const rect = button.getBoundingClientRect();
        const viewportPadding = 8;
        const gap = 6;
        const preferredMaxHeight = 320;
        const spaceBelow = window.innerHeight - rect.bottom - viewportPadding;
        const spaceAbove = rect.top - viewportPadding;
        const placement: SelectMenuStyle['placement'] = spaceBelow < 220 && spaceAbove > spaceBelow ? 'top' : 'bottom';
        const availableSpace = Math.max(120, placement === 'top' ? spaceAbove - gap : spaceBelow - gap);
        const maxHeight = Math.min(preferredMaxHeight, availableSpace);
        const visibleHeight = menuHeight > 0 ? Math.min(menuHeight, maxHeight) : maxHeight;
        const unclampedTop = placement === 'top'
            ? rect.top - gap - visibleHeight
            : rect.bottom + gap;
        const top = Math.max(
            viewportPadding,
            Math.min(unclampedTop, window.innerHeight - viewportPadding - visibleHeight),
        );
        const left = Math.min(
            Math.max(viewportPadding, rect.left),
            Math.max(viewportPadding, window.innerWidth - rect.width - viewportPadding),
        );

        return {
            top,
            left,
            width: rect.width,
            maxHeight,
            placement,
        };
    }, []);

    const openMenu = useCallback((nextActiveIndex = selectedIndex >= 0 ? selectedIndex : 0) => {
        const nextStyle = calculateMenuStyle();
        if (nextStyle) {
            setMenuStyle(nextStyle);
        }
        const maxIndex = Math.max(0, normalizedOptions.length - 1);
        setActiveIndex(Math.min(Math.max(nextActiveIndex, 0), maxIndex));
        setIsOpen(true);
    }, [calculateMenuStyle, normalizedOptions.length, selectedIndex]);

    const closeMenu = useCallback(() => {
        setIsOpen(false);
        setActiveIndex(-1);
    }, []);

    const handleToggle = useCallback(() => {
        if (!isOpen) {
            openMenu();
            return;
        }
        closeMenu();
    }, [closeMenu, isOpen, openMenu]);

    useLayoutEffect(() => {
        if (!isOpen || !menuRef.current) {
            return;
        }

        const nextStyle = calculateMenuStyle(menuRef.current.getBoundingClientRect().height);
        if (nextStyle) {
            setMenuStyle(nextStyle);
        }
    }, [calculateMenuStyle, isOpen, normalizedOptions.length]);

    const selectOptionAtIndex = useCallback((index: number) => {
        const option = normalizedOptions[index];
        if (!option) {
            return;
        }
        onChange(option.value);
        closeMenu();
    }, [closeMenu, normalizedOptions, onChange]);

    const moveActiveIndex = useCallback((delta: number) => {
        if (!normalizedOptions.length) {
            return;
        }
        const start = activeIndex >= 0 ? activeIndex : (selectedIndex >= 0 ? selectedIndex : 0);
        const next = (start + delta + normalizedOptions.length) % normalizedOptions.length;
        setActiveIndex(next);
    }, [activeIndex, normalizedOptions.length, selectedIndex]);

    const handleKeyDown = useCallback((event: React.KeyboardEvent<HTMLButtonElement>) => {
        if (!normalizedOptions.length) {
            if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault();
            }
            if (event.key === 'Escape' && isOpen) {
                event.preventDefault();
                closeMenu();
            }
            return;
        }

        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                if (!isOpen) {
                    openMenu(selectedIndex >= 0 ? (selectedIndex + 1) % normalizedOptions.length : 0);
                    return;
                }
                moveActiveIndex(1);
                return;
            case 'ArrowUp':
                event.preventDefault();
                if (!isOpen) {
                    const fallbackIndex = selectedIndex >= 0
                        ? (selectedIndex - 1 + normalizedOptions.length) % normalizedOptions.length
                        : Math.max(0, normalizedOptions.length - 1);
                    openMenu(fallbackIndex);
                    return;
                }
                moveActiveIndex(-1);
                return;
            case 'Enter':
            case ' ':
                event.preventDefault();
                if (isOpen) {
                    const nextIndex = activeIndex >= 0 ? activeIndex : selectedIndex;
                    if (nextIndex >= 0) {
                        selectOptionAtIndex(nextIndex);
                    }
                    return;
                }
                openMenu();
                return;
            case 'Escape':
                if (isOpen) {
                    event.preventDefault();
                    closeMenu();
                }
                return;
            default:
                return;
        }
    }, [activeIndex, closeMenu, isOpen, moveActiveIndex, normalizedOptions.length, openMenu, selectOptionAtIndex, selectedIndex]);

    useEffect(() => {
        if (!isOpen) return;
        const controller = new AbortController();
        const handleClose = () => closeMenu();
        const handleWindowClick = (e: MouseEvent) => {
            if (buttonRef.current && !buttonRef.current.contains(e.target as Node)) {
                closeMenu();
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
                ref={menuRef}
                id={listboxId}
                role="listbox"
                aria-label={label || '下拉选项'}
                data-placement={menuStyle.placement}
                className="imageflow-select-menu fixed z-[9999] no-scrollbar"
                style={{ 
                    top: menuStyle.top, 
                    left: menuStyle.left, 
                    width: menuStyle.width,
                    maxHeight: menuStyle.maxHeight,
                }}
                onClick={(e) => e.stopPropagation()}
            >
                {normalizedOptions.map((opt, i) => (
                    <button
                        key={i}
                        id={`${listboxId}-option-${i}`}
                        type="button"
                        role="option"
                        aria-selected={opt.value === value}
                        data-active={i === activeIndex}
                        onMouseEnter={() => setActiveIndex(i)}
                        onClick={(e) => { 
                            e.preventDefault();
                            selectOptionAtIndex(i);
                        }}
                        className="imageflow-select-option"
                    >
                        <span className="min-w-0 flex-1 truncate text-left">{opt.label}</span>
                        {opt.value === value && <Icon name="Check" size={14} className="ml-3 shrink-0 text-[#007AFF]" />}
                    </button>
                ))}
            </div>
        );
        return createPortal(menu, document.body);
    };

    return (
        <div className="space-y-2 min-w-0">
            {label && <label className="imageflow-select-label text-sm font-medium text-gray-700 dark:text-gray-300">{label}</label>}
            <button
                ref={buttonRef}
                type="button"
                onClick={handleToggle}
                onKeyDown={handleKeyDown}
                aria-haspopup="listbox"
                aria-expanded={isOpen}
                aria-controls={isOpen ? listboxId : undefined}
                aria-activedescendant={isOpen && activeIndex >= 0 ? `${listboxId}-option-${activeIndex}` : undefined}
                aria-label={label}
                data-open={isOpen}
                className="imageflow-select-trigger"
            >
                <span className="min-w-0 flex-1 truncate text-left text-gray-900 dark:text-white">{selectedLabel}</span>
                <span className="ml-3 flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-gray-100/80 dark:bg-white/10">
                    <Icon name="ChevronDown" size={14} className={`text-gray-500 dark:text-gray-300 transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`} />
                </span>
            </button>
            {renderMenu()}
        </div>
    );
});

export { FileDropZone } from './FileDropZone';

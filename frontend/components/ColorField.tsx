import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

type HSV = { h: number; s: number; v: number };

function hexToHsv(hex: string): HSV {
    const text = hex.replace('#', '');
    const full = text.length === 3 ? text.split('').map((c) => c + c).join('') : text.padEnd(6, '0');
    const r = parseInt(full.slice(0, 2), 16) / 255;
    const g = parseInt(full.slice(2, 4), 16) / 255;
    const b = parseInt(full.slice(4, 6), 16) / 255;
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    const d = max - min;
    let h = 0;
    if (d !== 0) {
        if (max === r) h = ((g - b) / d) % 6;
        else if (max === g) h = (b - r) / d + 2;
        else h = (r - g) / d + 4;
        h *= 60;
        if (h < 0) h += 360;
    }
    return { h, s: max === 0 ? 0 : d / max, v: max };
}

function hsvToHex(h: number, s: number, v: number): string {
    const c = v * s;
    const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
    const m = v - c;
    let rgb: [number, number, number] = [0, 0, 0];
    if (h < 60) rgb = [c, x, 0];
    else if (h < 120) rgb = [x, c, 0];
    else if (h < 180) rgb = [0, c, x];
    else if (h < 240) rgb = [0, x, c];
    else if (h < 300) rgb = [x, 0, c];
    else rgb = [c, 0, x];
    const out = rgb.map((ch) => Math.round((ch + m) * 255).toString(16).padStart(2, '0'));
    return `#${out.join('')}`.toUpperCase();
}

const PRESET_COLORS = [
    '#FFFFFF', '#C7C7CC', '#8E8E93', '#3A3A3C', '#000000',
    '#FF3B30', '#FF9500', '#FFCC00', '#34C759', '#00C7BE',
    '#007AFF', '#5856D6', '#AF52DE', '#FF2D55', '#A2845E',
];

type PopStyle = { top: number; left: number };

export const ColorField: React.FC<{
    value: string;
    onChange: (hex: string) => void;
    label?: string;
}> = ({ value, onChange, label }) => {
    const [open, setOpen] = useState(false);
    const [popStyle, setPopStyle] = useState<PopStyle>({ top: 0, left: 0 });
    const triggerRef = useRef<HTMLButtonElement>(null);
    const popRef = useRef<HTMLDivElement>(null);
    const hsv = hexToHsv(value);

    const commit = useCallback((next: HSV) => {
        onChange(hsvToHex(next.h, next.s, next.v));
    }, [onChange]);

    useLayoutEffect(() => {
        if (!open || !triggerRef.current) return;
        const rect = triggerRef.current.getBoundingClientRect();
        const width = 248;
        const height = 300;
        const left = Math.max(8, Math.min(rect.left, window.innerWidth - width - 8));
        const top = rect.bottom + 6 + height > window.innerHeight - 8
            ? Math.max(8, rect.top - height - 6)
            : rect.bottom + 6;
        setPopStyle({ top, left });
    }, [open]);

    useEffect(() => {
        if (!open) return;
        const onPointerDown = (e: MouseEvent) => {
            const target = e.target as Node;
            if (triggerRef.current?.contains(target) || popRef.current?.contains(target)) return;
            setOpen(false);
        };
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') setOpen(false);
        };
        window.addEventListener('mousedown', onPointerDown);
        window.addEventListener('keydown', onKey);
        return () => {
            window.removeEventListener('mousedown', onPointerDown);
            window.removeEventListener('keydown', onKey);
        };
    }, [open]);

    // SV area drag
    const svRef = useRef<HTMLDivElement>(null);
    const handleSvPointer = useCallback((e: React.PointerEvent) => {
        const el = svRef.current;
        if (!el) return;
        const rect = el.getBoundingClientRect();
        const s = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
        const v = 1 - Math.min(1, Math.max(0, (e.clientY - rect.top) / rect.height));
        commit({ h: hsv.h, s, v });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [hsv.h, commit]);

    const hueRef = useRef<HTMLDivElement>(null);
    const handleHuePointer = useCallback((e: React.PointerEvent) => {
        const el = hueRef.current;
        if (!el) return;
        const rect = el.getBoundingClientRect();
        const h = Math.min(359, Math.max(0, ((e.clientX - rect.left) / rect.width) * 360));
        commit({ h, s: hsv.s, v: hsv.v });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [hsv.s, hsv.v, commit]);

    const startDrag = (handler: (e: React.PointerEvent) => void) => (e: React.PointerEvent) => {
        e.preventDefault();
        handler(e);
        const move = (ev: PointerEvent) => handler(ev as unknown as React.PointerEvent);
        const up = () => {
            window.removeEventListener('pointermove', move);
            window.removeEventListener('pointerup', up);
        };
        window.addEventListener('pointermove', move);
        window.addEventListener('pointerup', up);
    };

    return (
        <>
            <button
                ref={triggerRef}
                type="button"
                onClick={() => setOpen((v) => !v)}
                className="spring-tap-sm w-full flex items-center gap-2 px-2 py-1.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 hover:border-[#007AFF]/50 transition-colors"
                aria-label={label ? `选择${label}颜色` : '选择颜色'}
            >
                <span
                    className="w-7 h-7 rounded-lg border border-black/10 dark:border-white/20 shrink-0 shadow-inner"
                    style={{ background: value }}
                />
                <span className="flex-1 min-w-0 text-left text-xs font-mono text-gray-700 dark:text-gray-200 truncate">
                    {value.toUpperCase()}
                </span>
                <span className="shrink-0 w-4 h-4 rounded-full border border-gray-300 dark:border-white/20 overflow-hidden flex">
                    <span className="w-2 h-full bg-white" />
                    <span className="w-2 h-full bg-black" />
                </span>
            </button>

            {open && createPortal(
                <div
                    ref={popRef}
                    role="dialog"
                    className="spring-pop material-overlay fixed z-[140] rounded-2xl border border-gray-200/80 dark:border-white/10 shadow-xl p-3 select-none"
                    style={{ top: popStyle.top, left: popStyle.left, width: 248 }}
                >
                    {/* SV area */}
                    <div
                        ref={svRef}
                        onPointerDown={startDrag(handleSvPointer)}
                        className="relative h-36 rounded-xl cursor-crosshair touch-none overflow-hidden"
                        style={{
                            background: `linear-gradient(to top, #000, transparent), linear-gradient(to right, #fff, hsl(${Math.round(hsv.h)},100%,50%))`,
                        }}
                    >
                        <div
                            className="absolute w-4 h-4 rounded-full border-2 border-white shadow-md pointer-events-none -translate-x-1/2 -translate-y-1/2"
                            style={{ left: `${hsv.s * 100}%`, top: `${(1 - hsv.v) * 100}%`, background: value }}
                        />
                    </div>

                    {/* Hue bar */}
                    <div
                        ref={hueRef}
                        onPointerDown={startDrag(handleHuePointer)}
                        className="relative h-3 mt-3 rounded-full cursor-pointer touch-none"
                        style={{
                            background: 'linear-gradient(to right,#f00,#ff0,#0f0,#0ff,#00f,#f0f,#f00)',
                        }}
                    >
                        <div
                            className="absolute top-1/2 w-4 h-4 rounded-full border-2 border-white shadow pointer-events-none -translate-x-1/2 -translate-y-1/2"
                            style={{ left: `${(hsv.h / 360) * 100}%`, background: `hsl(${Math.round(hsv.h)},100%,50%)` }}
                        />
                    </div>

                    {/* Presets */}
                    <div className="grid grid-cols-10 gap-1.5 mt-3">
                        {PRESET_COLORS.map((hex) => (
                            <button
                                key={hex}
                                type="button"
                                title={hex}
                                onClick={() => onChange(hex.toUpperCase())}
                                className={`spring-tap-sm aspect-square rounded-md border ${hex.toUpperCase() === value.toUpperCase() ? 'ring-2 ring-[#007AFF]' : 'border-black/10 dark:border-white/20'}`}
                                style={{ background: hex }}
                            />
                        ))}
                    </div>

                    {/* Hex input */}
                    <input
                        type="text"
                        value={value.toUpperCase()}
                        onChange={(e) => {
                            const text = e.target.value.trim();
                            if (/^#?[0-9a-fA-F]{6}$/.test(text)) onChange(('#' + text.replace('#', '')).toUpperCase());
                        }}
                        maxLength={7}
                        className="mt-3 w-full px-2.5 py-2 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-xs font-mono outline-none dark:text-white focus:border-[#007AFF] transition-colors"
                    />
                </div>,
                document.body,
            )}
        </>
    );
};

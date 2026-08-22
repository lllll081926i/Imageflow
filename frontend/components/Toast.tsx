import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

export type ToastKind = 'success' | 'error' | 'info';

type ToastItem = {
    id: number;
    message: string;
    kind: ToastKind;
    leaving: boolean;
};

type ToastListener = (toast: ToastItem) => void;

const listeners = new Set<ToastListener>();
let seq = 0;
const VISIBLE_MS = 2400;
const EXIT_MS = 220;

export function showToast(message: string, kind: ToastKind = 'info') {
    const text = String(message || '').trim();
    if (!text) return;
    const toast: ToastItem = { id: ++seq, message: text, kind, leaving: false };
    listeners.forEach((listener) => listener(toast));
}

const KIND_STYLES: Record<ToastKind, string> = {
    success: 'border-green-200/80 dark:border-green-500/25 text-green-700 dark:text-green-300',
    error: 'border-red-200/80 dark:border-red-500/25 text-red-600 dark:text-red-300',
    info: 'border-gray-200/80 dark:border-white/10 text-gray-700 dark:text-gray-200',
};

const KIND_DOT: Record<ToastKind, string> = {
    success: 'bg-green-500',
    error: 'bg-red-500',
    info: 'bg-[#007AFF]',
};

export const ToastHost: React.FC = () => {
    const [items, setItems] = useState<ToastItem[]>([]);

    useEffect(() => {
        const listener: ToastListener = (toast) => {
            setItems((prev) => [...prev.slice(-3), toast]);
            window.setTimeout(() => {
                setItems((prev) => prev.map((item) => (item.id === toast.id ? { ...item, leaving: true } : item)));
            }, VISIBLE_MS);
            window.setTimeout(() => {
                setItems((prev) => prev.filter((item) => item.id !== toast.id));
            }, VISIBLE_MS + EXIT_MS);
        };
        listeners.add(listener);
        return () => {
            listeners.delete(listener);
        };
    }, []);

    return createPortal(
        <div className="fixed top-11 right-4 z-[130] flex flex-col items-end gap-2 pointer-events-none">
            {items.map((toast) => (
                <div
                    key={toast.id}
                    role="status"
                    className={`spring-pop material-overlay border rounded-xl px-3.5 py-2 shadow-lg flex items-center gap-2 max-w-[320px] ${KIND_STYLES[toast.kind]} transition-all duration-200 ${toast.leaving ? 'overlay-exit' : ''}`}
                >
                    <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${KIND_DOT[toast.kind]}`} />
                    <span className="text-xs font-medium break-all min-w-0">{toast.message}</span>
                </div>
            ))}
        </div>,
        document.body,
    );
};

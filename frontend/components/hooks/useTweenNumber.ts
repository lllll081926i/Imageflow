import { useEffect, useRef, useState } from 'react';

/**
 * Smoothly interpolates toward the target number (ease-out cubic).
 * Snaps instantly when reduced-motion is enabled.
 */
export function useTweenNumber(target: number, durationMs = 500): number {
    const [value, setValue] = useState(() => (Number.isFinite(target) ? target : 0));
    const valueRef = useRef(value);
    const rafRef = useRef(0);

    useEffect(() => {
        if (!Number.isFinite(target)) return;
        const reduced = typeof document !== 'undefined'
            && document.documentElement.classList.contains('reduced-motion');
        if (reduced) {
            cancelAnimationFrame(rafRef.current);
            valueRef.current = target;
            setValue(target);
            return;
        }
        const from = valueRef.current;
        if (from === target) return;
        const start = performance.now();
        const step = (now: number) => {
            const t = Math.min(1, (now - start) / durationMs);
            const eased = 1 - Math.pow(1 - t, 3);
            const next = from + (target - from) * eased;
            valueRef.current = next;
            setValue(next);
            if (t < 1) {
                rafRef.current = requestAnimationFrame(step);
            } else {
                valueRef.current = target;
                setValue(target);
            }
        };
        rafRef.current = requestAnimationFrame(step);
        return () => cancelAnimationFrame(rafRef.current);
    }, [target, durationMs]);

    return value;
}

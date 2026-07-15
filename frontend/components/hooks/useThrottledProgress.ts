import { useCallback, useEffect, useRef, useState } from 'react';

export function useThrottledProgress(initial = 0) {
    const [progress, setProgress] = useState(initial);
    const progressValueRef = useRef(initial);
    const progressRafRef = useRef<number | null>(null);

    const setProgressThrottled = useCallback((value: number) => {
        const next = Math.max(0, Math.min(100, Number(value) || 0));
        progressValueRef.current = next;
        if (progressRafRef.current != null) return;
        progressRafRef.current = window.requestAnimationFrame(() => {
            progressRafRef.current = null;
            setProgress(progressValueRef.current);
        });
    }, []);

    const flushProgress = useCallback(() => {
        if (progressRafRef.current != null) {
            window.cancelAnimationFrame(progressRafRef.current);
            progressRafRef.current = null;
        }
        setProgress(progressValueRef.current);
    }, []);

    const resetProgress = useCallback((value = 0) => {
        progressValueRef.current = value;
        if (progressRafRef.current != null) {
            window.cancelAnimationFrame(progressRafRef.current);
            progressRafRef.current = null;
        }
        setProgress(value);
    }, []);

    useEffect(() => () => {
        if (progressRafRef.current != null) {
            window.cancelAnimationFrame(progressRafRef.current);
            progressRafRef.current = null;
        }
    }, []);

    return {
        progress,
        setProgress,
        setProgressThrottled,
        flushProgress,
        resetProgress,
    };
}

export default useThrottledProgress;

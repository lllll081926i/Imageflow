import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';

export type VirtualListProps<T> = {
    items: T[];
    itemHeight?: number;
    height?: number;
    className?: string;
    overscan?: number;
    getKey: (item: T, index: number) => string;
    renderItem: (item: T, index: number) => React.ReactNode;
};

export function VirtualList<T>({
    items,
    itemHeight = 40,
    height = 320,
    className = '',
    overscan = 8,
    getKey,
    renderItem,
}: VirtualListProps<T>) {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const [scrollTop, setScrollTop] = useState(0);

    const onScroll = useCallback((event: React.UIEvent<HTMLDivElement>) => {
        setScrollTop(event.currentTarget.scrollTop);
    }, []);

    useEffect(() => {
        // Keep scrollTop valid when list shrinks.
        const maxScroll = Math.max(0, items.length * itemHeight - height);
        if (scrollTop > maxScroll) {
            setScrollTop(maxScroll);
            if (containerRef.current) {
                containerRef.current.scrollTop = maxScroll;
            }
        }
    }, [height, itemHeight, items.length, scrollTop]);

    const { start, end, offsetY, totalHeight } = useMemo(() => {
        const total = items.length;
        const visibleCount = Math.ceil(height / itemHeight) + overscan * 2;
        const startIndex = Math.max(0, Math.floor(scrollTop / itemHeight) - overscan);
        const endIndex = Math.min(total, startIndex + visibleCount);
        return {
            start: startIndex,
            end: endIndex,
            offsetY: startIndex * itemHeight,
            totalHeight: total * itemHeight,
        };
    }, [height, itemHeight, items.length, overscan, scrollTop]);

    // Small lists: render fully to avoid virtualization overhead and keep expand/collapse simple.
    if (items.length <= 80) {
        return (
            <div className={className}>
                {items.map((item, index) => (
                    <React.Fragment key={getKey(item, index)}>{renderItem(item, index)}</React.Fragment>
                ))}
            </div>
        );
    }

    const slice = items.slice(start, end);

    return (
        <div
            ref={containerRef}
            className={className}
            style={{ height, overflowY: 'auto', position: 'relative' }}
            onScroll={onScroll}
        >
            <div style={{ height: totalHeight, position: 'relative' }}>
                <div style={{ transform: `translateY(${offsetY}px)` }}>
                    {slice.map((item, localIndex) => {
                        const index = start + localIndex;
                        return (
                            <div key={getKey(item, index)} style={{ height: itemHeight }}>
                                {renderItem(item, index)}
                            </div>
                        );
                    })}
                </div>
            </div>
        </div>
    );
}

export default VirtualList;

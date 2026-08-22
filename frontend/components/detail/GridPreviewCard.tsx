import React, { memo } from 'react';
import { usePreviewImage } from '../thumbnails';
import type { DroppedFile } from './detailTypes';

type GridPreviewCardProps = {
    file: DroppedFile | null;
    rows: number;
    cols: number;
    totalCount: number;
};

const GridPreviewCard: React.FC<GridPreviewCardProps> = memo(({
    file,
    rows,
    cols,
    totalCount,
}) => {
    const url = usePreviewImage(file?.input_path ?? null, 1024);
    const name = file
        ? (file.relative_path.split('/').pop() || file.input_path)
        : '';

    return (
        <div className="flex-1 min-h-0 flex flex-col bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-100 dark:border-white/5 shadow-sm p-4 overflow-hidden">
            <div className="flex items-center justify-between gap-3 pb-2 shrink-0">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300 truncate min-w-0">
                    {name}
                </span>
                <span className="shrink-0 text-xs text-gray-400 font-mono">
                    {totalCount > 1 ? `1/${totalCount}` : ''}
                </span>
            </div>

            <div className="flex-1 min-h-0 flex items-center justify-center">
                {!file ? (
                    <div className="text-sm text-gray-300 dark:text-gray-600">拖入图片</div>
                ) : !url ? (
                    <div className="text-xs text-gray-400">加载中...</div>
                ) : (
                    <div className="relative w-full max-w-[520px] aspect-square">
                        <img
                            src={url}
                            alt={name}
                            draggable={false}
                            className="absolute inset-0 w-full h-full object-cover rounded-xl"
                        />
                        <div className="absolute inset-0 pointer-events-none">
                            {Array.from({ length: cols - 1 }).map((_, i) => (
                                <div
                                    key={`v${i}`}
                                    className="absolute top-0 bottom-0 w-[2px] bg-black/75 spring-pop"
                                    style={{ left: `${((i + 1) / cols) * 100}%`, transform: 'translateX(-1px)', transition: 'left 320ms cubic-bezier(0.34,1.56,0.64,1)' }}
                                />
                            ))}
                            {Array.from({ length: rows - 1 }).map((_, i) => (
                                <div
                                    key={`h${i}`}
                                    className="absolute left-0 right-0 h-[2px] bg-black/75 spring-pop"
                                    style={{ top: `${((i + 1) / rows) * 100}%`, transform: 'translateY(-1px)', transition: 'top 320ms cubic-bezier(0.34,1.56,0.64,1)' }}
                                />
                            ))}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
});

export default GridPreviewCard;

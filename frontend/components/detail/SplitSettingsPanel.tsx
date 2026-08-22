import React, { memo, useCallback, useMemo } from 'react';
import { CustomSelect, SegmentedControl } from '../Controls';
import { PresetBar } from './PresetBar';

type SplitSettingsProps = {
    rows: number;
    setRows: (v: number) => void;
    cols: number;
    setCols: (v: number) => void;
    mode: 'square' | 'full';
    setMode: (v: 'square' | 'full') => void;
    format: string;
    setFormat: (v: string) => void;
};

export const SplitSettings = memo(({
    rows, setRows,
    cols, setCols,
    mode, setMode,
    format, setFormat,
}: SplitSettingsProps) => {
    const payload = useMemo(() => ({ rows, cols, mode, format }), [rows, cols, mode, format]);
    const apply = useCallback((data: Record<string, unknown>) => {
        if (typeof data.rows === 'number') setRows(Math.max(1, Math.min(50, Math.round(data.rows))));
        if (typeof data.cols === 'number') setCols(Math.max(1, Math.min(50, Math.round(data.cols))));
        if (data.mode === 'square' || data.mode === 'full') setMode(data.mode);
        if (typeof data.format === 'string') setFormat(data.format);
    }, [setRows, setCols, setMode, setFormat]);

    return (
        <div className="space-y-6">
            <PresetBar scope="split" buildPayload={() => payload} applyPayload={apply} />

            <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">行数</label>
                    <input
                        type="number"
                        min={1}
                        max={50}
                        value={rows}
                        onChange={(e) => setRows(Math.max(1, Math.min(50, Number(e.target.value) || 1)))}
                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white focus:border-[#007AFF] transition-colors"
                    />
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">列数</label>
                    <input
                        type="number"
                        min={1}
                        max={50}
                        value={cols}
                        onChange={(e) => setCols(Math.max(1, Math.min(50, Number(e.target.value) || 1)))}
                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white focus:border-[#007AFF] transition-colors"
                    />
                </div>
            </div>

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">切割模式</label>
                <SegmentedControl
                    options={['优先正方形', '完整覆盖']}
                    value={mode === 'square' ? '优先正方形' : '完整覆盖'}
                    onChange={(label) => setMode(label === '优先正方形' ? 'square' : 'full')}
                />
            </div>

            <CustomSelect
                label="输出格式"
                options={['PNG', 'JPG']}
                value={format}
                onChange={setFormat}
            />
        </div>
    );
});

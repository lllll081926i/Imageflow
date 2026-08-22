import React, { memo } from 'react';
import { CustomSelect, SegmentedControl } from '../Controls';

type GridSettingsProps = {
    gridSize: string;
    setGridSize: (v: string) => void;
    format: string;
    setFormat: (v: string) => void;
};

export const GridSettings = memo(({
    gridSize,
    setGridSize,
    format,
    setFormat,
}: GridSettingsProps) => {
    return (
        <div className="space-y-6">
            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">切割方式</label>
                <SegmentedControl
                    options={['四宫格', '九宫格', '十六宫格']}
                    value={gridSize}
                    onChange={setGridSize}
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

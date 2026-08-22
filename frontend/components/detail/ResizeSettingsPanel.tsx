import React, { memo, useCallback, useMemo } from 'react';
import { CustomSelect, SegmentedControl, StyledSlider } from '../Controls';
import { PresetBar } from './PresetBar';

type ResizeMode = 'percent' | 'long_edge' | 'short_edge';

type ResizeSettingsProps = {
    mode: ResizeMode;
    setMode: (v: ResizeMode) => void;
    value: number;
    setValue: (v: number) => void;
    format: 'auto' | 'PNG' | 'JPG';
    setFormat: (v: 'auto' | 'PNG' | 'JPG') => void;
};

const MODE_LABELS: Record<ResizeMode, string> = {
    percent: '固定比例',
    long_edge: '固定长边',
    short_edge: '固定短边',
};

export const ResizeSettings = memo(({
    mode, setMode,
    value, setValue,
    format, setFormat,
}: ResizeSettingsProps) => {
    const payload = useMemo(() => ({ mode, value, format }), [mode, value, format]);
    const apply = useCallback((data: Record<string, unknown>) => {
        if (typeof data.mode === 'string' && data.mode in MODE_LABELS) setMode(data.mode as ResizeMode);
        if (typeof data.value === 'number') setValue(Math.max(1, Math.round(data.value)));
        if (data.format === 'auto' || data.format === 'PNG' || data.format === 'JPG') setFormat(data.format);
    }, [setMode, setValue, setFormat]);

    return (
        <div className="space-y-6">
            <PresetBar scope="resize" buildPayload={() => payload} applyPayload={apply} />

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">调整模式</label>
                <SegmentedControl
                    options={[MODE_LABELS.percent, MODE_LABELS.long_edge, MODE_LABELS.short_edge]}
                    value={MODE_LABELS[mode]}
                    onChange={(label) => {
                        const entry = Object.entries(MODE_LABELS).find(([, text]) => text === label);
                        if (entry) setMode(entry[0] as ResizeMode);
                    }}
                />
            </div>

            {mode === 'percent' ? (
                <StyledSlider
                    label="缩放比例"
                    value={value}
                    min={1}
                    max={500}
                    step={1}
                    unit="%"
                    onChange={(v) => setValue(v)}
                />
            ) : (
                <StyledSlider
                    label={mode === 'long_edge' ? '长边' : '短边'}
                    value={value}
                    min={16}
                    max={8000}
                    step={10}
                    unit="px"
                    onChange={(v) => setValue(v)}
                />
            )}

            <CustomSelect
                label="输出格式"
                options={['保持原格式', 'PNG', 'JPG']}
                value={format === 'auto' ? '保持原格式' : format}
                onChange={(label) => {
                    setFormat(label === '保持原格式' ? 'auto' : (label as 'PNG' | 'JPG'));
                }}
            />
        </div>
    );
});

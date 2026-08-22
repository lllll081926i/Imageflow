import React, { memo, useCallback, useMemo } from 'react';
import { CustomSelect, SegmentedControl, StyledSlider, Switch } from '../Controls';
import { ColorField } from '../ColorField';
import { PresetBar } from './PresetBar';

type StitchSettingsProps = {
    direction: 'vertical' | 'horizontal';
    setDirection: (v: 'vertical' | 'horizontal') => void;
    gap: number;
    setGap: (v: number) => void;
    background: string;
    setBackground: (v: string) => void;
    targetLength: number;
    setTargetLength: (v: number) => void;
    border: number;
    setBorder: (v: number) => void;
    outputFormat: string;
    setOutputFormat: (v: string) => void;
};

export const StitchSettings = memo(({
    direction, setDirection,
    gap, setGap,
    background, setBackground,
    targetLength, setTargetLength,
    border, setBorder,
    outputFormat, setOutputFormat,
}: StitchSettingsProps) => {
    const payload = useMemo(() => ({ direction, gap, background, targetLength, border, outputFormat }), [direction, gap, background, targetLength, border, outputFormat]);
    const apply = useCallback((data: Record<string, unknown>) => {
        if (data.direction === 'vertical' || data.direction === 'horizontal') setDirection(data.direction);
        if (typeof data.gap === 'number') setGap(Math.max(0, Math.round(data.gap)));
        if (typeof data.background === 'string') setBackground(data.background);
        if (typeof data.targetLength === 'number') setTargetLength(Math.max(0, Math.round(data.targetLength)));
        if (typeof data.border === 'number') setBorder(Math.max(0, Math.round(data.border)));
        if (typeof data.outputFormat === 'string') setOutputFormat(data.outputFormat);
    }, [setDirection, setGap, setBackground, setTargetLength, setBorder, setOutputFormat]);

    return (
        <div className="space-y-6">
            <PresetBar scope="stitch" buildPayload={() => payload} applyPayload={apply} />

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">拼接方向</label>
                <SegmentedControl
                    options={['纵向长图', '横向长图']}
                    value={direction === 'vertical' ? '纵向长图' : '横向长图'}
                    onChange={(label) => setDirection(label === '纵向长图' ? 'vertical' : 'horizontal')}
                />
            </div>

            <StyledSlider
                label={direction === 'vertical' ? '统一宽度' : '统一高度'}
                value={targetLength}
                min={0}
                max={8000}
                step={20}
                unit="px"
                onChange={setTargetLength}
            />

            <StyledSlider
                label="图片间距"
                value={gap}
                min={0}
                max={200}
                unit="px"
                onChange={setGap}
            />

            <ColorField
                label="背景颜色"
                value={background}
                onChange={setBackground}
            />

            <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4 space-y-4">
                <Switch
                    checked={border > 0}
                    onChange={(checked) => setBorder(checked ? 40 : 0)}
                    label="单张时加边框"
                />
                {border > 0 && (
                    <StyledSlider label="边框宽度" value={border} min={2} max={400} unit="px" onChange={setBorder} />
                )}
            </div>

            <CustomSelect
                label="输出格式"
                options={['PNG', 'JPG']}
                value={outputFormat}
                onChange={setOutputFormat}
            />
        </div>
    );
});

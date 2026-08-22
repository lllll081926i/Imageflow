import React, { memo, useCallback, useMemo } from 'react';
import { CustomSelect, SegmentedControl, StyledSlider, Switch } from '../Controls';
import { ColorField } from '../ColorField';
import { PresetBar } from './PresetBar';
import type { WatermarkStyle } from '../hooks/useFeatureParams';

type WatermarkSettingsProps = {
    style: WatermarkStyle;
    patch: (partial: Partial<WatermarkStyle>) => void;
};

const FONT_OPTIONS = ['微软雅黑', '黑体', '宋体', '等线', 'Arial'];
const POSITION_OPTIONS = [
    { label: '左上', value: 'tl' }, { label: '上中', value: 'tc' }, { label: '右上', value: 'tr' },
    { label: '左中', value: 'cl' }, { label: '居中', value: 'c' }, { label: '右中', value: 'cr' },
    { label: '左下', value: 'bl' }, { label: '下中', value: 'bc' }, { label: '右下', value: 'br' },
];

export const WatermarkSettings = memo(({ style, patch }: WatermarkSettingsProps) => {
    const payload = useMemo(() => ({ ...style }), [style]);
    const apply = useCallback((data: Record<string, unknown>) => {
        patch(data as Partial<WatermarkStyle>);
    }, [patch]);

    return (
        <div className="space-y-6">
            <PresetBar scope="watermark" buildPayload={() => payload} applyPayload={apply} />

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">水印内容</label>
                <textarea
                    value={style.text}
                    onChange={(e) => patch({ text: e.target.value })}
                    rows={2}
                    className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white focus:border-[#007AFF] transition-colors resize-none"
                />
            </div>

            <CustomSelect
                label="字体"
                options={FONT_OPTIONS}
                value={style.fontName}
                onChange={(v) => patch({ fontName: v })}
            />

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">尺寸方式</label>
                <SegmentedControl
                    options={['按像素', '按宽度%']}
                    value={style.sizeMode === 'px' ? '按像素' : '按宽度%'}
                    onChange={(label) => patch({ sizeMode: label === '按像素' ? 'px' : 'percent' })}
                />
                <StyledSlider
                    label={style.sizeMode === 'px' ? '字号' : '字号（占宽）'}
                    value={style.fontSize}
                    min={style.sizeMode === 'px' ? 8 : 1}
                    max={style.sizeMode === 'px' ? 400 : 30}
                    unit={style.sizeMode === 'px' ? 'px' : '%'}
                    onChange={(v) => patch({ fontSize: v })}
                />
            </div>

            <div className="grid grid-cols-2 gap-3">
                <ColorField
                    label="颜色"
                    value={style.color}
                    onChange={(hex) => patch({ color: hex })}
                />
                <StyledSlider
                    label="不透明度"
                    value={style.opacity}
                    min={5}
                    max={100}
                    unit="%"
                    onChange={(v) => patch({ opacity: v })}
                />
            </div>

            <StyledSlider
                label="旋转角度"
                value={style.rotation}
                min={-180}
                max={180}
                unit="°"
                onChange={(v) => patch({ rotation: v })}
            />

            <div className="grid grid-cols-2 gap-3">
                <CustomSelect
                    label="位置"
                    options={POSITION_OPTIONS.map((p) => p.label)}
                    value={POSITION_OPTIONS.find((p) => p.value === style.position)?.label ?? '右下'}
                    onChange={(label) => {
                        const found = POSITION_OPTIONS.find((p) => p.label === label);
                        if (found) patch({ position: found.value });
                    }}
                />
                <StyledSlider
                    label="边距"
                    value={style.margin}
                    min={0}
                    max={400}
                    unit="px"
                    onChange={(v) => patch({ margin: v })}
                />
            </div>

            <Switch
                checked={style.tiled}
                onChange={(checked) => patch({ tiled: checked })}
                label="平铺整个画面"
            />
            {style.tiled && (
                <div className="grid grid-cols-2 gap-3">
                    <StyledSlider label="横向间距" value={style.tileGapX} min={10} max={600} unit="px" onChange={(v) => patch({ tileGapX: v })} />
                    <StyledSlider label="纵向间距" value={style.tileGapY} min={10} max={600} unit="px" onChange={(v) => patch({ tileGapY: v })} />
                </div>
            )}

            <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4 space-y-4">
                <StyledSlider
                    label="描边宽度"
                    value={style.strokeWidth}
                    min={0}
                    max={10}
                    onChange={(v) => patch({ strokeWidth: v })}
                />
                <ColorField
                    label="描边色"
                    value={style.strokeColor}
                    onChange={(hex) => patch({ strokeColor: hex })}
                />
                <Switch
                    checked={style.shadow}
                    onChange={(checked) => patch({ shadow: checked })}
                    label="投影"
                />
            </div>
        </div>
    );
});

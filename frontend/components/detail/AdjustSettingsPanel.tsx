import React, { memo } from 'react';
import Icon from '../Icon';
import { Switch, StyledSlider, CustomSelect, SegmentedControl } from '../Controls';

type AdjustSettingsProps = {
    exposure: number;
    setExposure: (v: number) => void;
    contrast: number;
    setContrast: (v: number) => void;
    saturation: number;
    setSaturation: (v: number) => void;
    sharpness: number;
    setSharpness: (v: number) => void;
    vibrance: number;
    setVibrance: (v: number) => void;
    hue: number;
    setHue: (v: number) => void;
    showCrop?: boolean;
    cropRatio?: string;
    setCropRatio?: (v: string) => void;
    rotate?: number;
    setRotate?: (v: number) => void;
    flipH?: boolean;
    setFlipH?: (v: boolean) => void;
    flipV?: boolean;
    setFlipV?: (v: boolean) => void;
};

type AdjustCropControlsProps = {
    cropRatio: string;
    setCropRatio: (v: string) => void;
    rotate: number;
    setRotate: (v: number) => void;
    flipH: boolean;
    setFlipH: (v: boolean) => void;
    flipV: boolean;
    setFlipV: (v: boolean) => void;
};

export const AdjustCropControls = memo(({
    cropRatio,
    setCropRatio,
    rotate,
    setRotate,
    flipH,
    setFlipH,
    flipV,
    setFlipV,
}: AdjustCropControlsProps) => (
    <div className="space-y-2">
        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">裁剪与旋转</label>
        <div className="flex flex-wrap gap-2">
            {['自由', '1:1', '4:3', '16:9', '9:16', '3:2'].map(r => {
                const active = cropRatio === r;
                return (
                    <button
                        key={r}
                        onClick={() => setCropRatio(r)}
                        className={`px-3 py-1.5 rounded-lg border text-xs font-medium whitespace-nowrap transition-colors ${
                            active
                                ? 'border-[#007AFF] text-[#007AFF] bg-[#007AFF]/10'
                                : 'border-gray-200 dark:border-white/10 text-gray-600 dark:text-gray-300 bg-white dark:bg-white/5 hover:border-[#007AFF] hover:text-[#007AFF]'
                        }`}
                    >
                        {r}
                    </button>
                );
            })}
        </div>
        <div className="grid grid-cols-2 gap-2 mt-2">
            <button
                onClick={() => setRotate((rotate + 90) % 360)}
                className="flex items-center justify-center gap-2 py-2 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]"
            >
                <Icon name="RotateCw" size={16} /> 向右旋转 90°
            </button>
            <button
                onClick={() => setFlipH(!flipH)}
                className={`flex items-center justify-center gap-2 py-2 rounded-xl border transition-all text-sm font-medium ${
                    flipH
                        ? 'border-[#007AFF]/60 text-[#007AFF] bg-[#007AFF]/10'
                        : 'border-gray-200 dark:border-white/10 text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E] hover:bg-gray-50 dark:hover:bg-white/5'
                }`}
            >
                <span className="scale-x-[-1] inline-block"><Icon name="RotateCw" size={16} /></span> 水平翻转
            </button>
            <button
                onClick={() => setFlipV(!flipV)}
                className={`flex items-center justify-center gap-2 py-2 rounded-xl border transition-all text-sm font-medium ${
                    flipV
                        ? 'border-[#007AFF]/60 text-[#007AFF] bg-[#007AFF]/10'
                        : 'border-gray-200 dark:border-white/10 text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E] hover:bg-gray-50 dark:hover:bg-white/5'
                }`}
            >
                <span className="rotate-90 inline-block"><Icon name="RotateCw" size={16} /></span> 垂直翻转
            </button>
        </div>
    </div>
));

export const AdjustSettings = memo(({
    exposure,
    setExposure,
    contrast,
    setContrast,
    saturation,
    setSaturation,
    sharpness,
    setSharpness,
    vibrance,
    setVibrance,
    hue,
    setHue,
    showCrop = true,
    cropRatio,
    setCropRatio,
    rotate,
    setRotate,
    flipH,
    setFlipH,
    flipV,
    setFlipV,
}: AdjustSettingsProps) => {
    const canShowCrop = showCrop
        && typeof cropRatio === 'string'
        && typeof setCropRatio === 'function'
        && typeof rotate === 'number'
        && typeof setRotate === 'function'
        && typeof flipH === 'boolean'
        && typeof setFlipH === 'function'
        && typeof flipV === 'boolean'
        && typeof setFlipV === 'function';

    return (
        <div className="space-y-4">
            {canShowCrop && (
                <AdjustCropControls
                    cropRatio={cropRatio}
                    setCropRatio={setCropRatio}
                    rotate={rotate}
                    setRotate={setRotate}
                    flipH={flipH}
                    setFlipH={setFlipH}
                    flipV={flipV}
                    setFlipV={setFlipV}
                />
            )}

            <div className="pt-3 border-t border-gray-100 dark:border-white/5">
                 <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                    <Icon name="Palette" size={16} className="text-[#007AFF]" />
                    专业调色
                 </label>
                 <div className="w-full">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <StyledSlider label="曝光度" value={exposure} min={-100} max={100} onChange={setExposure} className="space-y-2" />
                    <StyledSlider label="对比度" value={contrast} min={-100} max={100} onChange={setContrast} className="space-y-2" />
                    <StyledSlider label="自然饱和度" value={vibrance} min={-100} max={100} onChange={setVibrance} className="space-y-2" />
                    <StyledSlider label="饱和度" value={saturation} min={-100} max={100} onChange={setSaturation} className="space-y-2" />
                    <StyledSlider label="色相偏移" value={hue} min={-180} max={180} onChange={setHue} unit="°" className="space-y-2" />
                    <StyledSlider label="锐化/模糊" value={sharpness} min={-100} max={100} onChange={setSharpness} className="space-y-2" />
                    </div>
                 </div>
            </div>
        </div>
    );
});

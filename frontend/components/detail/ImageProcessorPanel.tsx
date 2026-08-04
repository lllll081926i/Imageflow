import React, { memo, useState } from 'react';
import Icon from '../Icon';
import { StyledSlider } from '../Controls';
import { AdjustCropControls } from './AdjustSettingsPanel';
import { FilterControls, FILTER_LABELS } from './FilterSettingsPanel';

type ImageProcessorPanelProps = {
    // Adjust params
    exposure: number; setExposure: (v: number) => void;
    contrast: number; setContrast: (v: number) => void;
    saturation: number; setSaturation: (v: number) => void;
    sharpness: number; setSharpness: (v: number) => void;
    vibrance: number; setVibrance: (v: number) => void;
    hue: number; setHue: (v: number) => void;
    rotate: number; setRotate: (v: number) => void;
    flipH: boolean; setFlipH: (v: boolean) => void;
    flipV: boolean; setFlipV: (v: boolean) => void;
    cropRatio: string; setCropRatio: (v: string) => void;
    // Filter params
    intensity: number; setIntensity: (v: number) => void;
    grain: number; setGrain: (v: number) => void;
    vignette: number; setVignette: (v: number) => void;
    filterSelected: number; setFilterSelected: (v: number) => void;
    // Preview
    previewSrc: string;
    getPreviewFilter: (index: number) => string;
};

export const ImageProcessorPanel = memo(({
    exposure, setExposure,
    contrast, setContrast,
    saturation, setSaturation,
    sharpness, setSharpness,
    vibrance, setVibrance,
    hue, setHue,
    rotate, setRotate,
    flipH, setFlipH,
    flipV, setFlipV,
    cropRatio, setCropRatio,
    intensity, setIntensity,
    grain, setGrain,
    vignette, setVignette,
    filterSelected, setFilterSelected,
    previewSrc,
    getPreviewFilter,
}: ImageProcessorPanelProps) => {
    const [tab, setTab] = useState<'adjust' | 'filter'>('adjust');
    const [advancedAdjustOpen, setAdvancedAdjustOpen] = useState(false);
    const [allFiltersOpen, setAllFiltersOpen] = useState(false);
    const commonFilterIndices = [0, 1, 2, 3, 4, 5, 14, 18];
    const renderFilterButton = (i: number) => {
        const f = FILTER_LABELS[i];
        return (
            <button
                key={i}
                type="button"
                onClick={() => setFilterSelected(i)}
                className={`group relative flex flex-col items-center gap-1 p-1 rounded-lg border transition-all duration-200 active:scale-95 ${
                    filterSelected === i
                        ? 'bg-[#007AFF]/5 border-[#007AFF] ring-1 ring-[#007AFF]/20 shadow-sm'
                        : 'bg-gray-50 dark:bg-white/5 border-transparent hover:border-gray-200 dark:hover:border-white/10 hover:shadow-sm'
                }`}
            >
                <div className="w-full aspect-square rounded-md bg-gradient-to-br from-gray-200 to-gray-300 dark:from-white/5 dark:to-white/10 overflow-hidden relative shadow-inner">
                    {previewSrc ? (
                        <img src={previewSrc} alt={f} className="w-full h-full object-cover" style={{ filter: getPreviewFilter(i) || 'none' }} loading="lazy" />
                    ) : (
                        <div className={`absolute inset-0 opacity-20 ${i === 1 ? 'bg-blue-500 mix-blend-overlay' : ''} ${i % 2 === 0 && i !== 0 ? 'bg-yellow-600 mix-blend-multiply' : ''} ${i % 3 === 0 && i !== 0 ? 'bg-purple-600 mix-blend-screen' : ''}`} />
                    )}
                </div>
                <span className={`text-[12px] font-medium w-full text-center leading-tight break-words min-h-[2.2em] ${filterSelected === i ? 'text-[#007AFF]' : 'text-gray-500 dark:text-gray-400 group-hover:text-gray-700 dark:group-hover:text-gray-200'}`}>{f}</span>
                {filterSelected === i && <div className="absolute top-1 right-1 w-1.5 h-1.5 bg-[#007AFF] rounded-full shadow-sm" />}
            </button>
        );
    };

    return (
        <div className="imageflow-processor-settings flex flex-col h-full min-h-0 min-w-0">
            {/* Tab switcher */}
            <div className="flex gap-1 mb-4 p-1 rounded-xl bg-gray-100 dark:bg-white/5 shrink-0">
                <button
                    onClick={() => setTab('adjust')}
                    className={`flex-1 py-2 text-sm font-medium rounded-lg transition-all ${
                        tab === 'adjust'
                            ? 'bg-white dark:bg-[#2C2C2E] shadow-sm text-gray-900 dark:text-white'
                            : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
                    }`}
                >
                    <Icon name="Palette" size={14} className="inline mr-1.5" />
                    基础调整
                </button>
                <button
                    onClick={() => setTab('filter')}
                    className={`flex-1 py-2 text-sm font-medium rounded-lg transition-all ${
                        tab === 'filter'
                            ? 'bg-white dark:bg-[#2C2C2E] shadow-sm text-gray-900 dark:text-white'
                            : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
                    }`}
                >
                    <Icon name="Palette" size={14} className="inline mr-1.5" />
                    滤镜库
                </button>
            </div>

            {/* Tab content */}
            <div className="imageflow-processor-scroll flex-1 min-h-0 overflow-y-auto no-scrollbar">
                {tab === 'adjust' ? (
                    <div className="space-y-4">
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
                        <div className="pt-3 border-t border-gray-100 dark:border-white/5">
                            <button type="button" onClick={() => setAdvancedAdjustOpen((previous) => !previous)} className="w-full flex items-center justify-between text-left text-sm font-medium text-gray-700 dark:text-gray-300">
                                <span className="flex items-center gap-2"><Icon name="Palette" size={16} className="text-[#007AFF]" />专业调色</span>
                                <Icon name="ChevronDown" size={15} className={`text-gray-400 transition-transform ${advancedAdjustOpen ? 'rotate-0' : '-rotate-90'}`} />
                            </button>
                            {advancedAdjustOpen && <div className="imageflow-adjust-grid grid gap-4 mt-4 animate-enter">
                                <StyledSlider label="曝光度" value={exposure} min={-100} max={100} onChange={setExposure} className="space-y-2" />
                                <StyledSlider label="对比度" value={contrast} min={-100} max={100} onChange={setContrast} className="space-y-2" />
                                <StyledSlider label="自然饱和度" value={vibrance} min={-100} max={100} onChange={setVibrance} className="space-y-2" />
                                <StyledSlider label="饱和度" value={saturation} min={-100} max={100} onChange={setSaturation} className="space-y-2" />
                                <StyledSlider label="色相偏移" value={hue} min={-180} max={180} onChange={setHue} unit="°" className="space-y-2" />
                                <StyledSlider label="锐化/模糊" value={sharpness} min={-100} max={100} onChange={setSharpness} className="space-y-2" />
                            </div>}
                        </div>
                    </div>
                ) : (
                    <div className="space-y-4">
                        <div className="flex items-center justify-between gap-3">
                            <label className="text-sm font-medium text-gray-700 dark:text-gray-300">滤镜</label>
                            <button type="button" onClick={() => { setFilterSelected(0); setIntensity(80); setGrain(0); setVignette(0); }} className="text-xs text-[#007AFF] hover:underline">重置</button>
                        </div>
                        <div className="imageflow-processor-filter-grid grid gap-2">
                            {commonFilterIndices.map(renderFilterButton)}
                        </div>
                        <button type="button" onClick={() => setAllFiltersOpen((previous) => !previous)} className="w-full flex items-center justify-between rounded-xl border border-gray-200 dark:border-white/10 px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors">
                            <span>{allFiltersOpen ? '收起全部' : `全部滤镜（${FILTER_LABELS.length}）`}</span>
                            <Icon name="ChevronDown" size={14} className={`transition-transform ${allFiltersOpen ? 'rotate-0' : '-rotate-90'}`} />
                        </button>
                        {allFiltersOpen && <div className="imageflow-processor-filter-grid grid gap-2 animate-enter">
                            {FILTER_LABELS.map((_label, index) => renderFilterButton(index))}
                        </div>}
                        <div className="border-t border-gray-100 dark:border-white/5 pt-4">
                            <FilterControls
                                intensity={intensity}
                                setIntensity={setIntensity}
                                grain={grain}
                                setGrain={setGrain}
                                vignette={vignette}
                                setVignette={setVignette}
                            />
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
});

import React, { memo } from 'react';
import { StyledSlider } from '../Controls';

export const FILTER_LABELS = [
    '原图', '鲜艳', '黑白', '复古', '冷调', '暖阳', '胶片', '赛博',
    '清新', '日系', 'Lomo', 'HDR', '褪色', '磨砂', '电影', '拍立得',
    '夕阳', '海蓝', '森系', '紫雾', '琥珀', '北欧', '旧照片', '黑金',
    '高调', '低调', '雾霭', '霓虹', '哑光', '冰感', '咖啡', '焦糖',
    '青橙', '银盐', '清锐', '低对比'
];
export const FILTER_PRESETS = [
    'none', 'vivid', 'bw', 'retro', 'cool', 'warm', 'film', 'cyber',
    'fresh', 'japan', 'lomo', 'hdr', 'fade', 'frosted', 'cinema', 'polaroid',
    'sunset', 'ocean', 'forest', 'purple', 'amber', 'nordic', 'oldphoto', 'noir',
    'highkey', 'lowkey', 'haze', 'neon', 'matte', 'ice', 'coffee', 'caramel',
    'teal_orange', 'silver', 'crisp', 'low_contrast'
];

type FilterSettingsProps = {
    setIntensity: (v: number) => void;
    setGrain: (v: number) => void;
    setVignette: (v: number) => void;
    selected: number;
    setSelected: (v: number) => void;
    previewSrc: string;
    getPreviewFilter: (index: number) => string;
};

type FilterControlsProps = {
    intensity: number;
    setIntensity: (v: number) => void;
    grain: number;
    setGrain: (v: number) => void;
    vignette: number;
    setVignette: (v: number) => void;
    footer?: React.ReactNode;
};

export const FilterControls = memo(({
    intensity,
    setIntensity,
    grain,
    setGrain,
    vignette,
    setVignette,
    footer,
}: FilterControlsProps) => (
    <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col gap-4">
        <div className="grid grid-cols-1 gap-4">
            <StyledSlider label="滤镜强度" value={intensity} onChange={setIntensity} unit="%" className="space-y-2" />
            <StyledSlider label="颗粒感 (Grain)" value={grain} onChange={setGrain} className="space-y-2" />
            <StyledSlider label="暗角 (Vignette)" value={vignette} onChange={setVignette} className="space-y-2" />
        </div>
        {footer && (
            <div className="pt-3 border-t border-gray-100 dark:border-white/5">
                {footer}
            </div>
        )}
    </div>
));

export const FilterSettings = memo(({
    setIntensity,
    setGrain,
    setVignette,
    selected,
    setSelected,
    previewSrc,
    getPreviewFilter,
}: FilterSettingsProps) => {
    const hasPreview = Boolean(previewSrc);

    return (
        <div className="flex flex-col h-full min-h-0">
            <div className="flex-1 space-y-3 min-h-0 flex flex-col">
                <div className="flex items-center justify-between shrink-0">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">滤镜库 (LUTs)</label>
                    <button
                        onClick={() => {
                            setSelected(0);
                            setIntensity(80);
                            setGrain(0);
                            setVignette(0);
                        }}
                        className="text-xs text-[#007AFF] hover:underline font-medium"
                    >
                        重置效果
                    </button>
                </div>
                <div className="flex-1 min-h-0 overflow-y-auto no-scrollbar pr-1 pb-3">
                    <div
                        className="grid gap-1.5 content-start w-full"
                        style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(88px, 1fr))' }}
                    >
                        {FILTER_LABELS.map((f, i) => (
                            <button 
                                key={i} 
                                onClick={() => setSelected(i)}
                                className={`
                                group relative flex flex-col items-center gap-1 p-1 rounded-md border transition-all duration-200 active:scale-95
                                ${selected === i 
                                    ? 'bg-[#007AFF]/5 border-[#007AFF] ring-1 ring-[#007AFF]/20 shadow-sm' 
                                    : 'bg-gray-50 dark:bg-white/5 border-transparent hover:border-gray-200 dark:hover:border-white/10 hover:shadow-sm'}
                            `}>
                                <div className={`w-full aspect-square rounded-md ${i===0 ? 'bg-gray-200 dark:bg-white/10' : 'bg-gradient-to-br from-gray-200 to-gray-300 dark:from-white/5 dark:to-white/10'} overflow-hidden relative shadow-inner`}>
                                    {hasPreview ? (
                                        <img
                                            src={previewSrc}
                                            alt={f}
                                            className="w-full h-full object-cover"
                                            style={{ filter: getPreviewFilter(i) || 'none' }}
                                            loading="lazy"
                                        />
                                    ) : (
                                        <div className={`absolute inset-0 opacity-20 ${i===1 ? 'bg-blue-500 mix-blend-overlay' : ''} ${i%2===0 && i!==0 ? 'bg-yellow-600 mix-blend-multiply' : ''} ${i%3===0 && i!==0 ? 'bg-purple-600 mix-blend-screen' : ''}`}></div>
                                    )}
                                </div>
                                <span className={`text-[12px] font-medium w-full text-center leading-tight break-words min-h-[2.2em] ${selected === i ? 'text-[#007AFF]' : 'text-gray-500 dark:text-gray-400 group-hover:text-gray-700 dark:group-hover:text-gray-200'}`}>
                                    {f}
                                </span>
                                {selected === i && <div className="absolute top-0.5 right-0.5 w-1.5 h-1.5 bg-[#007AFF] rounded-full shadow-sm animate-enter"></div>}
                            </button>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
});

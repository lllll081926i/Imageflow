import React, { memo, useState } from 'react';
import { Switch, CustomSelect, SegmentedControl, StyledSlider } from '../Controls';

type CompressorSettingsProps = {
    mode: string;
    setMode: (v: string) => void;
    targetSize: boolean;
    setTargetSize: (v: boolean) => void;
    targetSizeKB: number;
    setTargetSizeKB: (v: number) => void;
    engine: string;
    setEngine: (v: string) => void;
    overwriteSource: boolean;
    setOverwriteSource: (v: boolean) => void;
    resizeMode: string;
    setResizeMode: (v: string) => void;
    resizePercent: number;
    setResizePercent: (v: number) => void;
    resizeWidth: number;
    setResizeWidth: (v: number) => void;
    resizeHeight: number;
    setResizeHeight: (v: number) => void;
    maintainAspect: boolean;
    setMaintainAspect: (v: boolean) => void;
};

export const CompressorSettings = memo(({
    mode,
    setMode,
    targetSize,
    setTargetSize,
    targetSizeKB,
    setTargetSizeKB,
    engine,
    setEngine,
    overwriteSource,
    setOverwriteSource,
    resizeMode,
    setResizeMode,
    resizePercent,
    setResizePercent,
    resizeWidth,
    setResizeWidth,
    resizeHeight,
    setResizeHeight,
    maintainAspect,
    setMaintainAspect,
}: CompressorSettingsProps) => {
    const [advancedOpen, setAdvancedOpen] = useState(false);
    
    return (
        <div className="space-y-5 min-w-0">
            <div className="space-y-3">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">压缩效果</label>
                <SegmentedControl
                    options={['智能（推荐）', '更清晰', '更小']}
                    value={mode}
                    onChange={setMode}
                />
            </div>

            <div className="rounded-2xl border border-gray-200/80 dark:border-white/10 p-4 space-y-4">
                <div className="flex items-center justify-between gap-3">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">分辨率缩放</label>
                </div>
                <SegmentedControl
                    options={['不缩放', '百分比', '像素']}
                    value={resizeMode}
                    onChange={setResizeMode}
                />
                {resizeMode === '百分比' && (
                    <StyledSlider
                        label="缩放比例"
                        value={resizePercent}
                        min={1}
                        max={500}
                        unit="%"
                        onChange={setResizePercent}
                    />
                )}
                {resizeMode === '像素' && (
                    <div className="space-y-4 animate-enter">
                        <div className="grid grid-cols-2 gap-3">
                            <div className="space-y-1.5">
                                <label className="text-xs text-gray-500 dark:text-gray-400">宽度</label>
                                <input
                                    type="number"
                                    min={1}
                                    max={65535}
                                    value={resizeWidth || ''}
                                    onChange={(event) => setResizeWidth(Math.max(1, Math.min(65535, Number(event.target.value) || 1)))}
                                    className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF]/30 dark:text-white"
                                />
                            </div>
                            <div className="space-y-1.5">
                                <label className="text-xs text-gray-500 dark:text-gray-400">高度</label>
                                <input
                                    type="number"
                                    min={1}
                                    max={65535}
                                    value={resizeHeight || ''}
                                    onChange={(event) => setResizeHeight(Math.max(1, Math.min(65535, Number(event.target.value) || 1)))}
                                    className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF]/30 dark:text-white"
                                />
                            </div>
                        </div>
                        <Switch label="锁定横纵比" checked={maintainAspect} onChange={setMaintainAspect} />
                    </div>
                )}
            </div>

            <div className="rounded-2xl border border-gray-200/80 dark:border-white/10 overflow-hidden">
                <button
                    type="button"
                    onClick={() => setAdvancedOpen((previous) => !previous)}
                    className="w-full flex items-center justify-between gap-3 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-white/[0.04] transition-colors"
                >
                    <div className="text-sm font-medium text-gray-700 dark:text-gray-200">高级设置</div>
                    <span className={`text-gray-400 transition-transform ${advancedOpen ? 'rotate-0' : '-rotate-90'}`}>⌄</span>
                </button>
                {advancedOpen && <div className="px-4 pb-4 pt-1 space-y-4 border-t border-gray-100 dark:border-white/10 animate-enter">
                    <CustomSelect label="压缩引擎" options={['自动 (推荐)', 'MozJPEG (JPEG)', 'PNGQuant (PNG)', 'OxiPNG (PNG 无损)', 'Pillow (兼容)']} value={engine} onChange={setEngine} />
                    <div className="space-y-4 pt-2">
                        <Switch label="指定目标大小限制 (KB)" checked={targetSize} onChange={setTargetSize} />
                        {targetSize && (
                            <div className="animate-enter">
                                <input type="number" min={1} value={targetSizeKB} onChange={e => setTargetSizeKB(Math.max(1, Number(e.target.value) || 1))} placeholder="例如：500" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                            </div>
                        )}
                    </div>
                    <Switch label="直接覆盖源文件" checked={overwriteSource} onChange={setOverwriteSource} />
                </div>}
            </div>
        </div>
    );
});

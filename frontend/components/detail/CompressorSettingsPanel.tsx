import React, { memo } from 'react';
import { Switch, CustomSelect, SegmentedControl } from '../Controls';

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
}: CompressorSettingsProps) => {
    
    return (
        <div className="space-y-6">
            <div className="space-y-4">
                 <label className="text-sm font-medium text-gray-700 dark:text-gray-300">压缩策略</label>
                 <SegmentedControl 
                    options={['无损', '轻度', '标准', '强力', '极限']}
                    value={mode}
                    onChange={setMode}
                 />
            </div>

            <CustomSelect label="压缩引擎" options={['自动 (推荐)', 'MozJPEG (JPEG)', 'PNGQuant (PNG)', 'OxiPNG (PNG 无损)', 'Pillow (兼容)']} value={engine} onChange={setEngine} />

            <div className="space-y-4 pt-2">
                <Switch label="指定目标大小限制 (KB)" checked={targetSize} onChange={setTargetSize} />
                {targetSize && (
                     <div className="animate-enter">
                        <input type="number" value={targetSizeKB} onChange={e => setTargetSizeKB(Number(e.target.value))} placeholder="例如: 500" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                     </div>
                )}
            </div>

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <Switch label="直接覆盖源文件" checked={overwriteSource} onChange={setOverwriteSource} />
            </div>
        </div>
    );
});

import React, { memo } from 'react';
import { Switch, StyledSlider, CustomSelect } from './Controls';
import { getGifModesForInputKind, type GifInputKind } from './gifHelpers';

export type GifSettingsPanelProps = {
    mode: string;
    setMode: (v: string) => void;
    exportFormat: string;
    setExportFormat: (v: string) => void;
    convertFormat: string;
    setConvertFormat: (v: string) => void;
    speedPercent: number;
    setSpeedPercent: (v: number) => void;
    compressQuality: number;
    setCompressQuality: (v: number) => void;
    sourceType: GifInputKind;
    buildFps: number;
    setBuildFps: (v: number) => void;
    resizeWidth: number;
    resizeHeight: number;
    onResizeWidthChange: (v: number) => void;
    onResizeHeightChange: (v: number) => void;
    resizeMaintainAR: boolean;
    setResizeMaintainAR: (v: boolean) => void;
    originalWidth: number;
    originalHeight: number;
};

const GifSettingsPanel = memo(({
    mode,
    setMode,
    exportFormat,
    setExportFormat,
    convertFormat,
    setConvertFormat,
    speedPercent,
    setSpeedPercent,
    compressQuality,
    setCompressQuality,
    sourceType,
    buildFps,
    setBuildFps,
    resizeWidth,
    resizeHeight,
    onResizeWidthChange,
    onResizeHeightChange,
    resizeMaintainAR,
    setResizeMaintainAR,
    originalWidth,
    originalHeight,
}: GifSettingsPanelProps) => {
    const modeOptions = getGifModesForInputKind(sourceType);
    const modeGroups = [
        { label: '帧处理', options: ['导出', '倒放', '修改帧率'] },
        { label: '体积优化', options: ['压缩', '缩放'] },
        { label: '格式输出', options: ['合成 GIF', '互转'] },
    ]
        .map((group) => ({ ...group, options: group.options.filter((option) => modeOptions.includes(option)) }))
        .filter((group) => group.options.length > 0);

    if (sourceType === 'images') {
        return (
            <div className="space-y-6 min-w-0">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">合成 GIF</label>
                </div>
                <div className="space-y-3 spring-enter">
                    <StyledSlider
                        label="合成帧率 (FPS)"
                        value={buildFps}
                        min={1}
                        max={60}
                        onChange={setBuildFps}
                    />
                </div>
            </div>
        );
    }

    return (
        <div className="space-y-6 min-w-0">
                <div className="space-y-3">
                    <div>
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">处理模式</label>
                    </div>
                    {modeGroups.map((group) => (
                        <div key={group.label} className="space-y-1.5">
                            <div className="text-[11px] font-medium text-gray-400 uppercase tracking-wider">{group.label}</div>
                            <div className="imageflow-gif-grid grid gap-2">
                                {group.options.map((option) => {
                                    const isActive = option === mode;
                                    return (
                                        <button
                                            key={option}
                                            type="button"
                                            aria-pressed={isActive}
                                            onClick={() => setMode(option)}
                                            className={`spring-tap-sm rounded-xl border px-3 py-2 text-sm font-medium transition-all duration-200 ${
                                                isActive
                                                    ? 'border-[#007AFF]/20 bg-[#007AFF]/10 text-[#007AFF] shadow-sm dark:border-[#0A84FF]/40 dark:bg-[#0A84FF]/15 dark:text-[#5AC8FA]'
                                                    : 'border-gray-200 bg-gray-50 text-gray-600 hover:border-gray-300 hover:text-gray-800 dark:border-white/10 dark:bg-white/5 dark:text-gray-300 dark:hover:bg-white/10 dark:hover:text-white'
                                            }`}
                                        >
                                            {option}
                                        </button>
                                    );
                                })}
                            </div>
                        </div>
                    ))}
                </div>

            {mode === '导出' && (
                <div className="space-y-3 spring-enter">
                    <CustomSelect
                        label="导出帧格式"
                        options={['PNG', 'JPG', 'WEBP', 'BMP', 'TIFF']}
                        value={exportFormat}
                        onChange={setExportFormat}
                    />
                </div>
            )}

            {mode === '互转' && (
                <div className="space-y-3 spring-enter">
                    <CustomSelect
                        label="目标动图格式"
                        options={['GIF', 'APNG', 'WEBP']}
                        value={convertFormat}
                        onChange={setConvertFormat}
                    />
                </div>
            )}

            {['倒放', '修改帧率', '压缩', '缩放'].includes(mode) && (
                <div className="space-y-3 spring-enter">
                    <CustomSelect
                        label="输出格式"
                        options={['GIF', 'APNG', 'WEBP']}
                        value={convertFormat}
                        onChange={setConvertFormat}
                    />
                </div>
            )}

            {mode === '修改帧率' && (
                <div className="spring-enter space-y-3">
                    <StyledSlider
                        label="播放速度"
                        value={speedPercent}
                        min={50}
                        max={300}
                        step={10}
                        onChange={setSpeedPercent}
                    />
                    <div className="flex flex-wrap gap-1.5">
                        {[50, 100, 150, 200, 300].map((value) => (
                            <button
                                key={value}
                                type="button"
                                onClick={() => setSpeedPercent(value)}
                                className={`spring-tap-sm px-2.5 py-1 rounded-lg border text-xs transition-all duration-200 ${speedPercent === value ? 'border-[#007AFF] bg-[#007AFF]/10 text-[#007AFF]' : 'border-gray-200 dark:border-white/10 text-gray-500 hover:border-[#007AFF]/40'}`}
                            >
                                {value / 100}×
                            </button>
                        ))}
                    </div>
                </div>
            )}

            {mode === '压缩' && (
                <div className="spring-enter space-y-3">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">压缩质量</label>
                    <div className="flex items-center gap-3">
                        <input
                            type="range"
                            min={1}
                            max={100}
                            step={1}
                            value={compressQuality}
                            onChange={(e) => setCompressQuality(Number(e.target.value))}
                            className="w-full accent-[#007AFF]"
                        />
                        <div className="w-16 shrink-0 text-center font-mono text-sm text-[#007AFF] bg-[#007AFF]/10 border border-[#007AFF]/20 rounded-lg py-1.5">
                            {compressQuality}%
                        </div>
                    </div>
                </div>
            )}

            {mode === '缩放' && (
                <div className="spring-enter space-y-3">
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        原始尺寸：{originalWidth > 0 && originalHeight > 0 ? `${originalWidth} x ${originalHeight} px` : '未读取到 GIF 尺寸'}
                    </div>
                    <div className="imageflow-gif-grid grid gap-3">
                        <div className="space-y-1">
                            <label className="text-xs text-gray-500">宽度 (px)</label>
                            <input
                                type="number"
                                min={1}
                                value={resizeWidth || ''}
                                onChange={(e) => onResizeWidthChange(Number(e.target.value || 0))}
                                className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                                placeholder="自动"
                            />
                        </div>
                        <div className="space-y-1">
                            <label className="text-xs text-gray-500">高度 (px)</label>
                            <input
                                type="number"
                                min={1}
                                value={resizeHeight || ''}
                                onChange={(e) => onResizeHeightChange(Number(e.target.value || 0))}
                                className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                                placeholder="自动"
                            />
                        </div>
                    </div>
                    <Switch label="保持纵横比" checked={resizeMaintainAR} onChange={setResizeMaintainAR} />
                </div>
            )}

            {sourceType === 'mixed' && (
                <div className="text-xs text-red-500">
                    请不要混合动图和静态图输入，否则当前模式会处理失败。
                </div>
            )}
        </div>
    );
});

GifSettingsPanel.displayName = 'GifSettingsPanel';

export default GifSettingsPanel;
